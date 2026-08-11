// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// ImagePainter is an optional Painter capability: it puts a block of RGBA
// pixels on the surface in one call.
//
// The base interface can draw rectangles, rounded rectangles, text and single
// pixels — and nothing that carries pixels of its own. Every widget showing an
// image therefore had to spell it out one pixel at a time: the toolkit's Image,
// Thumbnail, Wallpaper, Browser, ColorPicker and both font paths all loop over
// the destination calling PutPixel, which is an interface call per pixel —
// about 700,000 of them for a full 1000x700 window. Applications with their own
// framebuffer went further and bypassed the painter completely, reaching for
// the raw buffer, which is exactly what stops them being hosted by a back-end
// that hands out a Painter and nothing else.
//
// DrawImage scales src (srcW x srcH, 4 bytes per pixel, RGBA) into dst by
// nearest-neighbour sampling — the same mapping the hand-written loops used, so
// output is unchanged — and honours the active clip and translation like every
// other primitive.
//
// A src that is too short for srcW*srcH*4 is ignored rather than read past its
// end: the caller has a bug, and a painter is the wrong place to panic.
type ImagePainter interface {
	DrawImage(dst Rect, src []byte, srcW, srcH int)
}

// DrawImage blits src into dst. Implements ImagePainter.
//
// The fast path is one row at a time. When the destination is the same width as
// the source and the row is fully opaque and unclipped, the row is copied
// wholesale; otherwise each pixel is composited through the same blend the rest
// of the painter uses, so translucent images look identical to the per-PutPixel
// version that came before.
func (p *PixelPainter) DrawImage(dst Rect, src []byte, srcW, srcH int) {
	if srcW <= 0 || srcH <= 0 || dst.W <= 0 || dst.H <= 0 {
		return
	}
	if len(src) < srcW*srcH*4 {
		return
	}
	dst = shiftRect(p.off, dst)

	// A row can be copied wholesale when nothing has to be decided per pixel:
	// same width as the source, entirely on the surface, no clip in force, and
	// fully opaque. Scanning the alphas to find that out costs a quarter of the
	// copy and saves the blend on every pixel of the row.
	rowCopyable := dst.W == srcW && dst.X >= 0 && dst.X+dst.W <= p.Width && len(p.clip) == 0

	for dy := 0; dy < dst.H; dy++ {
		y := dst.Y + dy
		if y < 0 || y >= p.Height {
			continue
		}
		sy := dy * srcH / dst.H
		srcRow := sy * srcW * 4
		dstRow := y * p.Width * 4

		// The buffer bound is checked here and not with the rest of the
		// condition because a caller may hand over a Buf shorter than
		// Width*Height*4, exactly as PutPixel tolerates; the fast path must not
		// be the one place that panics on it.
		if end := dstRow + (dst.X+dst.W)*4; rowCopyable && end <= len(p.Buf) &&
			rowOpaque(src[srcRow:srcRow+srcW*4]) {
			copy(p.Buf[dstRow+dst.X*4:end], src[srcRow:srcRow+srcW*4])
			continue
		}

		for dx := 0; dx < dst.W; dx++ {
			x := dst.X + dx
			if x < 0 || x >= p.Width {
				continue
			}
			if !clipAllows(p.clip, x, y) {
				continue
			}
			sOff := srcRow + (dx*srcW/dst.W)*4
			dOff := dstRow + x*4
			if dOff < 0 || dOff+3 >= len(p.Buf) {
				continue
			}
			if a := src[sOff+3]; a == 0xFF {
				copy(p.Buf[dOff:dOff+4], src[sOff:sOff+4])
			} else if a != 0 {
				p.blendInto(dOff, RGBA{
					R: src[sOff], G: src[sOff+1], B: src[sOff+2], A: a,
				})
			}
		}
	}
}

// DrawImage maps the image onto the cell grid: each cell takes the colour of
// the source pixel it lands on, as a full-block glyph — which is what
// CellPainter.PutPixel already means by a pixel. Implements ImagePainter.
//
// A terminal cannot show an image, so this is the same honest degradation
// PutPixel already makes — a coloured cell rather than nothing at all.
func (p *CellPainter) DrawImage(dst Rect, src []byte, srcW, srcH int) {
	if srcW <= 0 || srcH <= 0 || dst.W <= 0 || dst.H <= 0 {
		return
	}
	if len(src) < srcW*srcH*4 {
		return
	}
	for dy := 0; dy < dst.H; dy++ {
		sy := dy * srcH / dst.H
		for dx := 0; dx < dst.W; dx++ {
			sOff := (sy*srcW + dx*srcW/dst.W) * 4
			c := RGBA{R: src[sOff], G: src[sOff+1], B: src[sOff+2], A: src[sOff+3]}
			if c.A == 0 {
				continue
			}
			// PutPixel already promotes a pixel to a filled cell and applies
			// the clip and translation; going through it keeps one definition
			// of what a pixel means on a grid.
			p.PutPixel(dst.X+dx, dst.Y+dy, c)
		}
	}
}

// rowOpaque reports whether every pixel of an RGBA row is fully opaque, which
// is what makes a wholesale copy equivalent to compositing it.
func rowOpaque(row []byte) bool {
	for i := 3; i < len(row); i += 4 {
		if row[i] != 0xFF {
			return false
		}
	}
	return true
}
