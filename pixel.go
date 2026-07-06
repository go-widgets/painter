// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

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
}

// NewPixelPainter builds a fresh painter over an already-allocated
// buffer. The buffer must be exactly `4*width*height` bytes; a
// mismatch is not policed here (the primitive calls just no-op on
// out-of-bounds writes).
func NewPixelPainter(buf []byte, width, height int) *PixelPainter {
	return &PixelPainter{Buf: buf, Width: width, Height: height}
}

// FillRect fills r with c. Out-of-bounds bytes are dropped so a
// widget that ranges past the edge doesn't panic.
func (p *PixelPainter) FillRect(r Rect, c RGBA) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			p.PutPixel(x, y, c)
		}
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
	for x := r.X; x < r.X+r.W; x++ {
		p.PutPixel(x, r.Y, c)
		p.PutPixel(x, r.Y+r.H-1, c)
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		p.PutPixel(r.X, y, c)
		p.PutPixel(r.X+r.W-1, y, c)
	}
	_ = lineW // hint; not used at 1 px
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
// Compositing over an opaque destination yields an opaque result, so a
// surface stays fully opaque for the host compositor.
func (p *PixelPainter) PutPixel(x, y int, c RGBA) {
	if x < 0 || y < 0 || x >= p.Width || y >= p.Height {
		return
	}
	off := (y*p.Width + x) * 4
	if off < 0 || off+3 >= len(p.Buf) {
		return
	}
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
	// src-over: out = src*a + dst*(1-a), rounded. Alpha byte too so the
	// result over an opaque ground stays opaque.
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
