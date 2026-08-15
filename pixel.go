// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "github.com/go-gfx/gfx/vector"

// PixelPainter writes the primitive set into an RGBA byte buffer —
// the deployment target for the WUI (browser canvas + putImageData)
// and GUI (native window that consumes a []byte) families. The
// buffer + stride mirror the toolkit's current Draw signature; a
// widget migrated to Painter renders identically to today's toolkit
// output.
type PixelPainter struct {
	// Buf is the destination RGBA byte slice (4 bytes per pixel).
	// The buffer is written in place; callers own its lifecycle.
	Buf []byte

	// Width is the stride in pixels — number of pixels per row.
	// The buffer's actual byte-stride is Width*4.
	Width int

	// Height is the number of rows.
	Height int

	// clip is the active clip stack; the top rect confines every write. Empty
	// means the whole surface. Managed via PushClip / PopClip.
	clip []Rect

	// off is the active translation stack; the top is added to every
	// coordinate before it is clipped or written. Managed via
	// PushTranslate / PopTranslate — see Translator.
	off []offset

	// rz is the go-gfx/gfx/vector rasterizer that turns FillPath / StrokePath
	// outlines into per-pixel coverage. It owns reusable scratch (the coverage
	// accumulator, a per-stroke-segment temporary, and a scanline crossings
	// list), so a steady stream of vector draws amortises to ~zero
	// coverage-buffer allocation. It carries no state between calls; the painter
	// consumes each returned coverage grid immediately in composite.
	rz vector.Rasterizer
}

// PushClip confines subsequent drawing to r (intersected with any enclosing
// clip). Implements Clipper.
func (p *PixelPainter) PushClip(r Rect) { p.clip = pushClip(p.clip, shiftRect(p.off, r)) }

// PopClip removes the most recent PushClip. Implements Clipper.
func (p *PixelPainter) PopClip() { p.clip = popClip(p.clip) }

// PushTranslate shifts subsequent drawing by dx,dy, on top of any enclosing
// translation. Implements Translator.
func (p *PixelPainter) PushTranslate(dx, dy int) { p.off = pushOffset(p.off, dx, dy) }

// PopTranslate removes the most recent PushTranslate. Implements Translator.
func (p *PixelPainter) PopTranslate() { p.off = popOffset(p.off) }

// NewPixelPainter builds a fresh painter over an already-allocated
// buffer. The buffer must be exactly `4*width*height` bytes; a
// mismatch is not policed here (the primitive calls just no-op on
// out-of-bounds writes).
func NewPixelPainter(buf []byte, width, height int) *PixelPainter {
	return &PixelPainter{Buf: buf, Width: width, Height: height}
}

// FillRect fills r with c. Out-of-bounds bytes are dropped so a widget that
// ranges past the edge doesn't panic.
//
// This is the primitive the toolkit leans on hardest -- every background, every
// button, every table row -- and it used to be a PutPixel per pixel, which is a
// shift, two bounds tests, a clip test and a blend each: 700,000 of them for a
// window-sized fill. Where a fill may write is a rectangle, decided once; and
// an opaque fill writes the SAME four bytes everywhere, so one row is built and
// the rest of the rectangle is that row copied. A translucent fill still
// composites pixel by pixel, because its result depends on what was underneath.
func (p *PixelPainter) FillRect(r Rect, c RGBA) {
	if r.W <= 0 || r.H <= 0 || c.A == 0 {
		return
	}
	r = shiftRect(p.off, r)

	eff := intersect(r, Rect{X: 0, Y: 0, W: p.Width, H: p.Height})
	if n := len(p.clip); n > 0 {
		eff = intersect(eff, p.clip[n-1])
	}
	if eff.W <= 0 || eff.H <= 0 {
		return
	}

	first := -1
	for y := eff.Y; y < eff.Y+eff.H; y++ {
		dstRow := y * p.Width * 4
		lo, hi := dstRow+eff.X*4, dstRow+(eff.X+eff.W)*4
		// A caller may hand over a Buf shorter than Width*Height*4, exactly as
		// PutPixel tolerates; a row that does not fit is skipped, not fatal.
		if hi > len(p.Buf) {
			continue
		}

		if c.A != 0xFF {
			for off := lo; off < hi; off += 4 {
				p.blendInto(off, c)
			}
			continue
		}

		if first >= 0 {
			copy(p.Buf[lo:hi], p.Buf[first:first+eff.W*4])
			continue
		}

		// Build the first row by writing one pixel and doubling it: each copy
		// moves as many bytes as are already there, so a row of N pixels costs
		// log2(N) copies rather than N stores.
		row := p.Buf[lo:hi]
		row[0], row[1], row[2], row[3] = c.R, c.G, c.B, 0xFF
		for filled := 4; filled < len(row); filled *= 2 {
			copy(row[filled:], row[:filled])
		}
		first = lo
	}
}

// StrokeRect draws a 1-line-wide border around r. lineW is
// currently ignored — the pixel back-end can't easily draw thick
// strokes without antialiasing, which is out of scope for this
// prototype.
func (p *PixelPainter) StrokeRect(r Rect, c RGBA, lineW int) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	// lineW concentric outlines, drawn INWARD from r's edge: the border stays
	// inside the rect it frames, which is what a widget laying out to its own
	// bounds needs. It was ignored here until now -- accepted and discarded --
	// so every caller asking for a thicker border silently got one pixel, and a
	// toolkit could not scale its chrome for a HiDPI screen.
	if lineW < 1 {
		lineW = 1
	}
	for i := 0; i < lineW; i++ {
		in := Rect{X: r.X + i, Y: r.Y + i, W: r.W - 2*i, H: r.H - 2*i}
		if in.W <= 0 || in.H <= 0 {
			return // the rings met in the middle; anything further is outside
		}
		p.strokeOutline(in, c)
	}
}

// strokeOutline paints the one-pixel outline of r.
func (p *PixelPainter) strokeOutline(r Rect, c RGBA) {
	for x := r.X; x < r.X+r.W; x++ {
		p.PutPixel(x, r.Y, c)
		p.PutPixel(x, r.Y+r.H-1, c)
	}
	// Skip the corner rows here — the horizontal runs above already painted the
	// four corner pixels. Painting them again double-composites a translucent
	// stroke, darkening the corners relative to the edges.
	for y := r.Y + 1; y < r.Y+r.H-1; y++ {
		p.PutPixel(r.X, y, c)
		p.PutPixel(r.X+r.W-1, y, c)
	}
}

// PutPixel writes one RGBA at (x, y). Out-of-bounds writes are
// silently dropped.
//
// Semi-transparent colours are src-over composited onto the existing
// pixel, so a theme colour like WhiteSur's borders rgba(0,0,0,0.12)
// paints as a subtle 12%-black hairline instead of a harsh opaque line.
// The two common cases stay exact and allocation-free:
//   - A == 0xFF (the vast majority of widget paint) overwrites verbatim,
//     so opaque rendering is byte-identical to before.
//   - A == 0 (fully transparent) is a no-op.
//
// Compositing over an opaque destination yields an opaque result, so a
// surface stays fully opaque for the host compositor.
func (p *PixelPainter) PutPixel(x, y int, c RGBA) {
	// The translation is applied HERE, in the one write every primitive
	// funnels through — FillRect, StrokeRect and the rounded pair all reach
	// the surface by calling this. Shifting in each public method instead
	// would apply the offset once per layer of composition.
	x, y = shiftPoint(p.off, x, y)
	if x < 0 || y < 0 || x >= p.Width || y >= p.Height {
		return
	}
	if !clipAllows(p.clip, x, y) {
		return
	}
	off := (y*p.Width + x) * 4
	if off < 0 || off+3 >= len(p.Buf) {
		return
	}
	p.blendInto(off, c)
}

// blendInto writes c at byte offset off, which the caller has already proven is
// in range and clip-allowed. It is the shared pixel-write core of PutPixel and
// the path rasterizer's composite loop:
//   - A == 0xFF overwrites verbatim (opaque paint stays byte-identical to a raw
//     store);
//   - A == 0 is a no-op;
//   - otherwise src-over composites (out = src*a + dst*(1-a), rounded), alpha
//     byte included so the result over an opaque ground stays opaque.
func (p *PixelPainter) blendInto(off int, c RGBA) {
	if c.A == 0xFF {
		p.Buf[off] = c.R
		p.Buf[off+1] = c.G
		p.Buf[off+2] = c.B
		p.Buf[off+3] = 0xFF
		return
	}
	if c.A == 0 {
		return
	}
	a := uint32(c.A)
	ia := 255 - a
	blend := func(src, dst uint8) uint8 { return uint8((uint32(src)*a + uint32(dst)*ia + 127) / 255) }
	p.Buf[off] = blend(c.R, p.Buf[off])
	p.Buf[off+1] = blend(c.G, p.Buf[off+1])
	p.Buf[off+2] = blend(c.B, p.Buf[off+2])
	p.Buf[off+3] = uint8(a + uint32(p.Buf[off+3])*ia/255)
}

// Text paints s at (x, y) using the built-in 5×7 bitmap font (see
// font.go). Each glyph is 5 columns × 7 rows + 1 pixel of inter-
// glyph spacing (advance = 6).
func (p *PixelPainter) Text(x, y int, s string, ink RGBA) {
	for k := 0; k < len(s); k++ {
		bits, ok := font5x7[s[k]]
		if !ok {
			continue
		}
		gx := x + k*glyphAdvance
		for col := 0; col < 5; col++ {
			cb := bits[col]
			for row := 0; row < glyphHeight; row++ {
				if cb&(1<<row) != 0 {
					p.PutPixel(gx+col, y+row, ink)
				}
			}
		}
	}
}

// Size returns Width × Height in pixels.
func (p *PixelPainter) Size() (int, int) { return p.Width, p.Height }
