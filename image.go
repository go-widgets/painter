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

	// The surface and the clip are both rectangles, and the top of the clip
	// stack is already the intersection of every clip in force, so where the
	// blit may write is ONE rectangle known before the first row. Deciding it
	// here is what lets the row paths run under a clip: testing per pixel is
	// what used to make a clipped blit fall back to the slow loop, and a
	// clipped blit is what a wallpaper and a scrolled page actually are.
	eff := intersect(dst, Rect{X: 0, Y: 0, W: p.Width, H: p.Height})
	if n := len(p.clip); n > 0 {
		eff = intersect(eff, p.clip[n-1])
	}
	if eff.W <= 0 || eff.H <= 0 {
		return
	}

	// An enlarged image draws several destination rows from ONE source row.
	// Building that row once and copying it to its repeats turns the cost from
	// the destination's height into the source's -- which is the whole point of
	// enlarging. prevRow is the byte offset of the last row written, prevSY the
	// source row it came from.
	prevRow, prevSY := -1, -1

	for y := eff.Y; y < eff.Y+eff.H; y++ {
		sy := (y - dst.Y) * srcH / dst.H
		srcRow := sy * srcW * 4
		dstRow := y * p.Width * 4
		lo, hi := dstRow+eff.X*4, dstRow+(eff.X+eff.W)*4

		// A caller may hand over a Buf shorter than Width*Height*4, exactly as
		// PutPixel tolerates; a row that does not fit is skipped rather than
		// fatal.
		if hi > len(p.Buf) {
			continue
		}

		// A repeat is settled before anything is read from the source. prevSY
		// only holds a source row that was written whole, so matching it proves
		// the row is opaque as well as already built -- re-scanning its alphas
		// would be half the bytes of the blit spent proving what is known.
		if sy == prevSY && prevRow >= 0 {
			copy(p.Buf[lo:hi], p.Buf[prevRow+eff.X*4:prevRow+(eff.X+eff.W)*4])
			prevRow = dstRow
			continue
		}

		// A fully opaque row replaces what was underneath, so it can be written
		// without consulting it. A row with any translucency cannot: the result
		// depends on the ground, which differs from row to row.
		if rowOpaque(src[srcRow : srcRow+srcW*4]) {
			if dst.W == srcW && eff.X == dst.X && eff.W == dst.W {
				copy(p.Buf[lo:hi], src[srcRow:srcRow+srcW*4])
			} else {
				scaleRow(p.Buf[lo:hi], src[srcRow:srcRow+srcW*4], srcW, dst.W, eff.X-dst.X)
			}
			prevRow, prevSY = dstRow, sy
			continue
		}

		for x := eff.X; x < eff.X+eff.W; x++ {
			sOff := srcRow + ((x-dst.X)*srcW/dst.W)*4
			dOff := dstRow + x*4
			if a := src[sOff+3]; a == 0xFF {
				copy(p.Buf[dOff:dOff+4], src[sOff:sOff+4])
			} else if a != 0 {
				p.blendInto(dOff, RGBA{
					R: src[sOff], G: src[sOff+1], B: src[sOff+2], A: a,
				})
			}
		}
		prevRow, prevSY = -1, -1
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

// scaleRow writes one destination row by nearest-neighbour sampling of one
// source row. Both are RGBA. dstW is the width the image was scaled to, which
// may be wider than dst when a clip trimmed it, so x0 says which column of that
// scaled row dst begins at.
func scaleRow(dst, src []byte, srcW, dstW, x0 int) {
	for i := 0; i < len(dst)/4; i++ {
		o := ((x0 + i) * srcW / dstW) * 4
		copy(dst[i*4:i*4+4], src[o:o+4])
	}
}

// rowOpaque reports whether every pixel of an RGBA row is fully opaque, which
// is what makes a wholesale copy equivalent to compositing it.
//
// The directive is not decoration. Inlined into the loop above -- which grew
// when the clip stopped being tested per pixel -- this scan measured 1,152,535
// ns/op on a full 1000x700 blit; kept out of line, the SAME code measures
// 238,252. The body is a tight strided scan whose cost lives entirely in
// register allocation, and folding it into an already register-hungry loop is
// what wrecks it. Remove the directive and the benchmark says so immediately.
//
//go:noinline
func rowOpaque(row []byte) bool {
	for i := 3; i < len(row); i += 4 {
		if row[i] != 0xFF {
			return false
		}
	}
	return true
}
