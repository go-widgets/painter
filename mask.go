// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// MaskPainter is an optional Painter capability: it paints ONE colour through
// an 8-bit coverage mask in a single call.
//
// This is what a glyph is. A rasterised glyph is not an image — it carries no
// colour of its own, only how much of each pixel the outline covers — and text
// is the one thing every widget draws. Before this, a font back-end had to walk
// the mask calling PutPixel, which shifts, bounds-tests, clip-tests and blends
// one pixel at a time, and had to scale the coverage by the ink's alpha itself
// on every pixel.
//
// mask holds one coverage byte per pixel, row-major with the given stride, and
// mask[0] is the pixel at dst's top-left. 0 leaves the surface untouched, 255
// lays down ink at its own alpha, and everything between composites — which is
// exactly what makes a glyph edge smooth rather than jagged.
//
// [ImagePainter] carries pixels that bring their own colour; this carries
// coverage for a colour the caller names. A back-end may implement either,
// both, or neither.
type MaskPainter interface {
	DrawMask(dst Rect, mask []byte, stride int, ink RGBA)
}

// DrawMask paints ink through mask. Implements MaskPainter.
//
// Where it may write is decided once, like every other primitive here, so the
// per-pixel work is a coverage lookup and a blend and nothing else.
func (p *PixelPainter) DrawMask(dst Rect, mask []byte, stride int, ink RGBA) {
	if dst.W <= 0 || dst.H <= 0 || stride <= 0 || ink.A == 0 {
		return
	}
	dst = shiftRect(p.off, dst)

	eff := intersect(dst, Rect{X: 0, Y: 0, W: p.Width, H: p.Height})
	if n := len(p.clip); n > 0 {
		eff = intersect(eff, p.clip[n-1])
	}
	if eff.W <= 0 || eff.H <= 0 {
		return
	}

	full := ink.A == 0xFF
	for y := eff.Y; y < eff.Y+eff.H; y++ {
		maskRow := (y - dst.Y) * stride
		dstRow := y * p.Width * 4
		if maskRow+(eff.X-dst.X)+eff.W > len(mask) {
			// A caller whose mask is shorter than the rectangle it named has a
			// bug; the painter drops the rows it cannot read rather than
			// panicking, exactly as it drops a short Buf.
			continue
		}
		if dstRow+(eff.X+eff.W)*4 > len(p.Buf) {
			continue
		}
		for x := eff.X; x < eff.X+eff.W; x++ {
			cov := mask[maskRow+(x-dst.X)]
			if cov == 0 {
				continue
			}
			a := uint32(cov)
			if !full {
				a = a * uint32(ink.A) / 255
			}
			off := dstRow + x*4
			if a == 0xFF {
				p.Buf[off], p.Buf[off+1], p.Buf[off+2], p.Buf[off+3] = ink.R, ink.G, ink.B, 0xFF
				continue
			}
			if a == 0 {
				continue
			}
			// The same arithmetic as blendInto, written out rather than
			// called: identical rounding, including the alpha byte, which
			// truncates where the colour bytes round. Getting that last detail
			// wrong is invisible over an opaque ground and shows up over a
			// translucent one, which is why TestDrawMaskBlendsExactlyLikePutPixel
			// sweeps every coverage against several grounds instead of trusting
			// a handful of cases.
			inv := 255 - a
			p.Buf[off] = uint8((uint32(ink.R)*a + uint32(p.Buf[off])*inv + 127) / 255)
			p.Buf[off+1] = uint8((uint32(ink.G)*a + uint32(p.Buf[off+1])*inv + 127) / 255)
			p.Buf[off+2] = uint8((uint32(ink.B)*a + uint32(p.Buf[off+2])*inv + 127) / 255)
			p.Buf[off+3] = uint8(a + uint32(p.Buf[off+3])*inv/255)
		}
	}
}

// DrawMask puts a coloured cell wherever the mask is more than half covered:
// a cell is either the glyph's colour or it is not, so the coverage has to
// resolve to a decision. Implements MaskPainter.
func (p *CellPainter) DrawMask(dst Rect, mask []byte, stride int, ink RGBA) {
	if dst.W <= 0 || dst.H <= 0 || stride <= 0 || ink.A == 0 {
		return
	}
	for dy := 0; dy < dst.H; dy++ {
		row := dy * stride
		if row+dst.W > len(mask) {
			continue
		}
		for dx := 0; dx < dst.W; dx++ {
			if mask[row+dx] < 128 {
				continue
			}
			// PutPixel already promotes a pixel to a filled cell and applies
			// the clip and translation.
			p.PutPixel(dst.X+dx, dst.Y+dy, ink)
		}
	}
}
