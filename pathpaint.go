// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// PixelPainter is a PathPainter: it asks go-gfx/gfx/vector for the anti-aliased
// coverage of an outline, then composites that coverage into its RGBA buffer
// under its own clip. The rasterisation (flatten + scanline coverage + round
// join/cap stroker) is the exact code painter used to hand-roll, moved verbatim
// into the shared foundation; only the composite below — which is painter's own,
// because it touches painter's buffer and clip — stays here.
var _ PathPainter = (*PixelPainter)(nil)

// FillPath fills pth with c under rule. See PathPainter.FillPath.
func (p *PixelPainter) FillPath(pth *Path, c RGBA, rule FillRule) {
	if pth == nil || c.A == 0 {
		return
	}
	cov, ox, oy, w, h, ok := p.rz.Fill(pth, rule, p.Width, p.Height)
	if !ok {
		return
	}
	p.composite(cov, ox, oy, w, h, c)
}

// StrokePath paints pth's outline with c, width units wide. See
// PathPainter.StrokePath. The stroke coverage (the union of a rectangle per
// segment and a round join/cap disk at every vertex) is computed by the vector
// rasterizer and composited once.
func (p *PixelPainter) StrokePath(pth *Path, c RGBA, width float64) {
	if pth == nil || c.A == 0 {
		return
	}
	cov, ox, oy, w, h, ok := p.rz.Stroke(pth, width, p.Width, p.Height)
	if !ok {
		return
	}
	p.composite(cov, ox, oy, w, h, c)
}

// composite writes a coverage grid (w*h, values 0..1) at origin (ox, oy) into
// the buffer, scaling c.A by coverage. The clip + surface intersection is taken
// ONCE up front so the inner loop carries no per-pixel bounds/clip branch and no
// PutPixel call — every visited pixel is already known drawable; the result is
// byte-identical to PutPixel-per-pixel (pixels PutPixel would have dropped fall
// outside the intersected region).
func (p *PixelPainter) composite(cov []float64, ox, oy, w, h int, c RGBA) {
	// Visible box = grid ∩ surface ∩ top clip, in absolute pixel coordinates.
	x0, y0, x1, y1 := ox, oy, ox+w, oy+h
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > p.Width {
		x1 = p.Width
	}
	if y1 > p.Height {
		y1 = p.Height
	}
	if n := len(p.clip); n > 0 {
		r := p.clip[n-1]
		if r.X > x0 {
			x0 = r.X
		}
		if r.Y > y0 {
			y0 = r.Y
		}
		if r.X+r.W < x1 {
			x1 = r.X + r.W
		}
		if r.Y+r.H < y1 {
			y1 = r.Y + r.H
		}
	}
	for y := y0; y < y1; y++ {
		crow := cov[(y-oy)*w : (y-oy)*w+w] // this row's coverage; crow[x-ox] = col x
		for x := x0; x < x1; x++ {
			a := crow[x-ox]
			if a <= 0 {
				continue
			}
			if a > 1 {
				a = 1
			}
			col := c
			col.A = uint8(float64(c.A)*a + 0.5)
			if col.A == 0 {
				continue
			}
			off := (y*p.Width + x) * 4
			if off+3 >= len(p.Buf) {
				continue // degenerate under-sized buffer; PutPixel drops these too
			}
			p.blendInto(off, col)
		}
	}
}
