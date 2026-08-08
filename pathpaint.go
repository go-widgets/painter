// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"math"
	"sort"
)

// PixelPainter is a PathPainter: it rasterises arbitrary outlines with an
// anti-aliased scanline coverage accumulator, the same fractional-coverage idea
// round.go uses for rounded corners, generalised to any polygon.
var _ PathPainter = (*PixelPainter)(nil)

// pathSS is the number of vertical sub-scanlines sampled per pixel row. The
// horizontal coverage within a row is computed analytically (exact span overlap),
// so this only sets the vertical anti-aliasing resolution. 4 gives 4 alpha
// levels vertically — enough for crisp icon edges without a heavy inner loop.
const pathSS = 4

// edge is one flattened line segment of a contour, in painter units.
type edge struct{ x0, y0, x1, y1 float64 }

// crossing is where an edge cuts a scanline: the x coordinate and the winding
// direction (+1 for a downward edge, -1 for an upward one).
type crossing struct {
	x   float64
	dir int
}

// FillPath fills pth with c under rule. See PathPainter.FillPath.
func (p *PixelPainter) FillPath(pth *Path, c RGBA, rule FillRule) {
	if pth == nil || c.A == 0 {
		return
	}
	edges := fillEdges(pth.flatten(flattenTol))
	if len(edges) == 0 {
		return
	}
	ox, oy, w, h, ok := p.pathBox(edgeBounds(edges))
	if !ok {
		return
	}
	cov := coverGrid(edges, rule, ox, oy, w, h, pathSS)
	p.composite(cov, ox, oy, w, h, c)
}

// StrokePath paints pth's outline with c, width units wide. See
// PathPainter.StrokePath. The stroke is the union of a rectangle per segment and
// a disk (round join / cap) at every vertex; the union is taken as a per-pixel
// coverage MAX so overlapping pieces never double-darken, then composited once.
func (p *PixelPainter) StrokePath(pth *Path, c RGBA, width float64) {
	if pth == nil || c.A == 0 || width <= 0 {
		return
	}
	hw := width / 2
	var segs []edge
	var verts []point
	for _, s := range pth.flatten(flattenTol) {
		n := len(s.pts)
		if n < 2 {
			continue // a lone point is not strokeable
		}
		for i := 0; i+1 < n; i++ {
			segs = append(segs, edge{s.pts[i].x, s.pts[i].y, s.pts[i+1].x, s.pts[i+1].y})
		}
		if s.closed && s.pts[n-1] != s.pts[0] {
			segs = append(segs, edge{s.pts[n-1].x, s.pts[n-1].y, s.pts[0].x, s.pts[0].y})
		}
		verts = append(verts, s.pts...)
	}
	if len(verts) == 0 {
		return
	}
	minX, minY, maxX, maxY := pointBounds(verts)
	ox, oy, w, h, ok := p.pathBox(minX-hw, minY-hw, maxX+hw, maxY+hw)
	if !ok {
		return
	}
	cov := make([]float64, w*h)
	for _, s := range segs {
		if re := segRectEdges(s.x0, s.y0, s.x1, s.y1, hw); re != nil {
			maxInto(cov, coverGrid(re, NonZero, ox, oy, w, h, pathSS))
		}
	}
	for _, v := range verts {
		diskMax(cov, ox, oy, w, h, v.x, v.y, hw)
	}
	p.composite(cov, ox, oy, w, h, c)
}

// pathBox turns a float bounding box into an integer sub-box clamped to the
// surface, returning ok=false when nothing of it is on-screen.
func (p *PixelPainter) pathBox(minX, minY, maxX, maxY float64) (ox, oy, w, h int, ok bool) {
	ox = int(math.Floor(minX))
	oy = int(math.Floor(minY))
	x1 := int(math.Ceil(maxX))
	y1 := int(math.Ceil(maxY))
	if ox < 0 {
		ox = 0
	}
	if oy < 0 {
		oy = 0
	}
	if x1 > p.Width {
		x1 = p.Width
	}
	if y1 > p.Height {
		y1 = p.Height
	}
	w, h = x1-ox, y1-oy
	if w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	return ox, oy, w, h, true
}

// composite writes a coverage grid (w*h, values 0..1) at origin (ox, oy) into
// the buffer, scaling c.A by coverage. Clip + bounds are honoured by PutPixel.
func (p *PixelPainter) composite(cov []float64, ox, oy, w, h int, c RGBA) {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			a := cov[j*w+i]
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
			p.PutPixel(ox+i, oy+j, col)
		}
	}
}

// fillEdges builds the fill edge list from flattened contours: consecutive
// vertices plus an implicit closing edge per sub-path. Sub-paths with fewer than
// two points enclose no area and are dropped.
func fillEdges(subs []subpath) []edge {
	var e []edge
	for _, s := range subs {
		n := len(s.pts)
		if n < 2 {
			continue
		}
		for i := 0; i+1 < n; i++ {
			e = append(e, edge{s.pts[i].x, s.pts[i].y, s.pts[i+1].x, s.pts[i+1].y})
		}
		if last, first := s.pts[n-1], s.pts[0]; last != first {
			e = append(e, edge{last.x, last.y, first.x, first.y})
		}
	}
	return e
}

// coverGrid returns per-pixel coverage (0..1) of the region enclosed by edges
// under rule, over the integer box [ox,oy,w,h], sampling ss vertical
// sub-scanlines per row with analytic horizontal coverage.
func coverGrid(edges []edge, rule FillRule, ox, oy, w, h, ss int) []float64 {
	cov := make([]float64, w*h)
	inv := 1.0 / float64(ss)
	oxf := float64(ox)
	var xs []crossing
	for py := 0; py < h; py++ {
		row := cov[py*w : py*w+w]
		for s := 0; s < ss; s++ {
			sy := float64(oy+py) + (float64(s)+0.5)*inv
			xs = crossingsAt(edges, sy, xs[:0])
			if len(xs) < 2 {
				continue
			}
			sort.Slice(xs, func(i, j int) bool { return xs[i].x < xs[j].x })
			wind := 0
			for i := 0; i < len(xs)-1; i++ {
				wind += xs[i].dir
				if insideRule(wind, rule) {
					addSpan(row, xs[i].x, xs[i+1].x, oxf, w, inv)
				}
			}
		}
	}
	return cov
}

// crossingsAt collects the x-crossings of every edge with the horizontal line
// y=sy into dst (which the caller resets), using the half-open [ymin, ymax)
// convention so a vertex shared by two edges is counted exactly once.
func crossingsAt(edges []edge, sy float64, dst []crossing) []crossing {
	for _, e := range edges {
		if e.y0 == e.y1 {
			continue // horizontal edge never crosses a scanline
		}
		var dir int
		if e.y0 < e.y1 {
			if sy < e.y0 || sy >= e.y1 {
				continue
			}
			dir = 1
		} else {
			if sy < e.y1 || sy >= e.y0 {
				continue
			}
			dir = -1
		}
		t := (sy - e.y0) / (e.y1 - e.y0)
		dst = append(dst, crossing{x: e.x0 + t*(e.x1-e.x0), dir: dir})
	}
	return dst
}

// insideRule reports whether a winding count means "inside" under rule.
func insideRule(wind int, rule FillRule) bool {
	if rule == EvenOdd {
		return wind&1 != 0
	}
	return wind != 0
}

// addSpan adds the horizontal coverage of the inside interval [xa, xb] to row,
// scaled by weight. row[i] maps to pixel column ox+i; a pixel gets the length of
// its overlap with [xa, xb] (0..1), giving analytic horizontal anti-aliasing.
func addSpan(row []float64, xa, xb, ox float64, w int, weight float64) {
	if xa < ox {
		xa = ox
	}
	if hi := ox + float64(w); xb > hi {
		xb = hi
	}
	if xb <= xa {
		return
	}
	ixa := int(math.Floor(xa - ox))
	ixb := int(math.Ceil(xb - ox))
	for ix := ixa; ix < ixb; ix++ {
		left := xa
		if l := ox + float64(ix); l > left {
			left = l
		}
		right := xb
		if r := ox + float64(ix+1); r < right {
			right = r
		}
		if c := right - left; c > 0 {
			row[ix] += c * weight
		}
	}
}

// segRectEdges returns the four edges of the width-2*hw rectangle centred on the
// segment (x0,y0)->(x1,y1). A zero-length segment has no rectangle (nil): the
// vertex disk covers that spot.
func segRectEdges(x0, y0, x1, y1, hw float64) []edge {
	dx, dy := x1-x0, y1-y0
	l := math.Hypot(dx, dy)
	if l == 0 {
		return nil
	}
	nx, ny := -dy/l*hw, dx/l*hw
	ax, ay := x0+nx, y0+ny
	bx, by := x1+nx, y1+ny
	cx, cy := x1-nx, y1-ny
	ex, ey := x0-nx, y0-ny
	return []edge{{ax, ay, bx, by}, {bx, by, cx, cy}, {cx, cy, ex, ey}, {ex, ey, ax, ay}}
}

// diskMax unions (per-pixel MAX) a filled disk of radius r centred at (cx, cy)
// into cov, anti-aliasing the rim with the round.go distance formula.
func diskMax(cov []float64, ox, oy, w, h int, cx, cy, r float64) {
	if r <= 0 {
		return
	}
	for j := 0; j < h; j++ {
		py := float64(oy+j) + 0.5
		for i := 0; i < w; i++ {
			px := float64(ox+i) + 0.5
			c := r + 0.5 - math.Hypot(px-cx, py-cy)
			if c <= 0 {
				continue
			}
			if c > 1 {
				c = 1
			}
			if k := j*w + i; c > cov[k] {
				cov[k] = c
			}
		}
	}
}

// maxInto merges src into dst by per-pixel maximum (a coverage union).
func maxInto(dst, src []float64) {
	for i, v := range src {
		if v > dst[i] {
			dst[i] = v
		}
	}
}

// edgeBounds returns the bounding box of an edge list (len >= 1 assumed).
func edgeBounds(edges []edge) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, e := range edges {
		minX = math.Min(minX, math.Min(e.x0, e.x1))
		minY = math.Min(minY, math.Min(e.y0, e.y1))
		maxX = math.Max(maxX, math.Max(e.x0, e.x1))
		maxY = math.Max(maxY, math.Max(e.y0, e.y1))
	}
	return
}

// pointBounds returns the bounding box of a point list (len >= 1 assumed).
func pointBounds(pts []point) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, pt := range pts {
		minX = math.Min(minX, pt.x)
		minY = math.Min(minY, pt.y)
		maxX = math.Max(maxX, pt.x)
		maxY = math.Max(maxY, pt.y)
	}
	return
}
