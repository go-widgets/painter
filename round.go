// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "math"

// FillRoundRect fills r with corners rounded to radius (in pixels), anti-
// aliasing the corner edge. radius is clamped to half the smaller side; a
// radius <= 0 degrades to a plain FillRect. Corner-edge pixels are plotted
// with fractional alpha, which PutPixel composites onto the destination -- so
// a rounded button/pill reads as a smooth macOS-style shape rather than a
// jagged one.
func (p *PixelPainter) FillRoundRect(r Rect, radius int, c RGBA) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	rad := clampRadius(radius, r)
	if rad <= 0 {
		p.FillRect(r, c)
		return
	}
	rf := float64(rad)
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			cov := cornerFillCoverage(x, y, r, rad, rf)
			if cov <= 0 {
				continue
			}
			col := c
			if cov < 1 {
				col.A = uint8(float64(c.A)*cov + 0.5)
			}
			p.PutPixel(x, y, col)
		}
	}
}

// StrokeRoundRect paints a 1-pixel rounded border around r. The straight runs
// are crisp 1-px lines; the four corners are an anti-aliased quarter-ring.
func (p *PixelPainter) StrokeRoundRect(r Rect, radius int, c RGBA, lineW int) {
	_ = lineW // 1-px hint; matches StrokeRect
	if r.W <= 0 || r.H <= 0 {
		return
	}
	rad := clampRadius(radius, r)
	if rad <= 0 {
		p.StrokeRect(r, c, lineW)
		return
	}
	// Straight edges (between the corner arcs).
	for x := r.X + rad; x < r.X+r.W-rad; x++ {
		p.PutPixel(x, r.Y, c)
		p.PutPixel(x, r.Y+r.H-1, c)
	}
	for y := r.Y + rad; y < r.Y+r.H-rad; y++ {
		p.PutPixel(r.X, y, c)
		p.PutPixel(r.X+r.W-1, y, c)
	}
	// Anti-aliased corner arcs: plot pixels within ~1px of the radius ring.
	rf := float64(rad)
	for _, cn := range corners(r, rad) {
		for y := cn.y0; y < cn.y0+rad; y++ {
			for x := cn.x0; x < cn.x0+rad; x++ {
				d := math.Hypot(float64(x)+0.5-cn.cx, float64(y)+0.5-cn.cy)
				// Ring coverage peaks at 1 on the radius and falls off either
				// side; never exceeds 1, so no upper clamp is needed.
				cov := 1 - math.Abs(d-(rf-0.5))
				if cov <= 0 {
					continue
				}
				col := c
				col.A = uint8(float64(c.A)*cov + 0.5)
				p.PutPixel(x, y, col)
			}
		}
	}
}

// clampRadius caps radius at half the smaller side of r.
func clampRadius(radius int, r Rect) int {
	if m := min2(r.W, r.H) / 2; radius > m {
		return m
	}
	return radius
}

// cornerFillCoverage returns 1 for a pixel outside every corner box, and the
// anti-aliased inside-coverage (0..1) for a pixel within a corner box.
func cornerFillCoverage(x, y int, r Rect, rad int, rf float64) float64 {
	cx, cy, in := cornerCenter(x, y, r, rad)
	if !in {
		return 1
	}
	d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
	cov := rf + 0.5 - d
	if cov < 0 {
		return 0
	}
	if cov > 1 {
		return 1
	}
	return cov
}

// cornerCenter returns the circle centre for whichever corner box (x,y) is in.
func cornerCenter(x, y int, r Rect, rad int) (cx, cy float64, in bool) {
	left := x < r.X+rad
	right := x >= r.X+r.W-rad
	top := y < r.Y+rad
	bottom := y >= r.Y+r.H-rad
	switch {
	case left && top:
		return float64(r.X + rad), float64(r.Y + rad), true
	case right && top:
		return float64(r.X + r.W - rad), float64(r.Y + rad), true
	case left && bottom:
		return float64(r.X + rad), float64(r.Y + r.H - rad), true
	case right && bottom:
		return float64(r.X + r.W - rad), float64(r.Y + r.H - rad), true
	}
	return 0, 0, false
}

type cornerBox struct {
	x0, y0 int
	cx, cy float64
}

// corners returns the four rad×rad corner boxes + their circle centres.
func corners(r Rect, rad int) [4]cornerBox {
	return [4]cornerBox{
		{r.X, r.Y, float64(r.X + rad), float64(r.Y + rad)},
		{r.X + r.W - rad, r.Y, float64(r.X + r.W - rad), float64(r.Y + rad)},
		{r.X, r.Y + r.H - rad, float64(r.X + rad), float64(r.Y + r.H - rad)},
		{r.X + r.W - rad, r.Y + r.H - rad, float64(r.X + r.W - rad), float64(r.Y + r.H - rad)},
	}
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
