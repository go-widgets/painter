// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"math"
	"testing"
)

// The path builder + the anti-aliased scanline rasterizer moved to
// go-gfx/gfx/vector, where they are unit-tested white-box (flatten, coverage,
// clamp, span, disk, winding) and PROVEN pixel-identical to the pre-extraction
// painter by a byte-for-byte parity sweep. What remains to test HERE is painter's
// own layer: FillPath / StrokePath end-to-end through PixelPainter, the composite
// that blends coverage into the buffer under painter's clip, and the PathPainter
// capability wiring. These are black-box render assertions plus white-box tests
// of composite, which is painter's alone.

// alphaAt returns the destination alpha byte at (x, y) — the coverage the
// path rasterizer composited there over the (initially transparent) buffer.
func alphaAt(p *PixelPainter, x, y int) uint8 {
	return p.Buf[(y*p.Width+x)*4+3]
}

// coveredArea sums destination alpha / 255 over the whole buffer, i.e. the total
// painted coverage in pixel-units. For an opaque fill over a transparent ground
// this equals the shape's rendered area.
func coveredArea(p *PixelPainter) float64 {
	var sum float64
	for i := 3; i < len(p.Buf); i += 4 {
		sum += float64(p.Buf[i]) / 255
	}
	return sum
}

// --- fill: known shapes ----------------------------------------------------

// axis-aligned rectangle path, so the analytic area is exact and easy to check.
func TestFillPathRectangleExactCoverage(t *testing.T) {
	p := newPixel(20, 20)
	rect := NewPath().MoveTo(4, 4).LineTo(14, 4).LineTo(14, 12).LineTo(4, 12).Close()
	p.FillPath(rect, RGB(0x20, 0x40, 0x80), NonZero)

	// Interior fully covered.
	if a := alphaAt(p, 9, 8); a != 0xFF {
		t.Errorf("interior alpha = %d, want 255", a)
	}
	// Well outside untouched.
	if a := alphaAt(p, 1, 1); a != 0 {
		t.Errorf("outside alpha = %d, want 0", a)
	}
	// Interior colour is the fill colour (opaque).
	off := (8*p.Width + 9) * 4
	if p.Buf[off] != 0x20 || p.Buf[off+1] != 0x40 || p.Buf[off+2] != 0x80 {
		t.Errorf("interior colour = %v, want (32,64,128)", p.Buf[off:off+3])
	}
	// Total covered area == 10*8 = 80, exact for an integer-aligned rect.
	if a := coveredArea(p); math.Abs(a-80) > 0.5 {
		t.Errorf("covered area = %.3f, want ~80", a)
	}
}

func TestFillPathTriangleArea(t *testing.T) {
	p := newPixel(40, 40)
	// Right triangle, legs 20 and 20 -> area 200.
	tri := NewPath().MoveTo(5, 5).LineTo(25, 5).LineTo(5, 25).Close()
	p.FillPath(tri, RGB(0xFF, 0, 0), NonZero)

	if a := alphaAt(p, 8, 8); a != 0xFF { // near the right-angle corner, inside
		t.Errorf("interior alpha = %d, want 255", a)
	}
	if a := alphaAt(p, 22, 22); a != 0 { // beyond the hypotenuse
		t.Errorf("outside-hypotenuse alpha = %d, want 0", a)
	}
	if a := coveredArea(p); math.Abs(a-200) > 20 { // within ~perimeter tolerance
		t.Errorf("triangle area = %.2f, want ~200", a)
	}
}

func TestFillPathDiamondArea(t *testing.T) {
	p := newPixel(40, 40)
	// Diamond with diagonals 30 and 30 -> area = d1*d2/2 = 450.
	dia := NewPath().MoveTo(20, 5).LineTo(35, 20).LineTo(20, 35).LineTo(5, 20).Close()
	p.FillPath(dia, RGB(0, 0xFF, 0), NonZero)

	if a := alphaAt(p, 20, 20); a != 0xFF {
		t.Errorf("diamond centre alpha = %d, want 255", a)
	}
	if a := alphaAt(p, 6, 6); a != 0 { // outside a corner
		t.Errorf("diamond outside alpha = %d, want 0", a)
	}
	if a := coveredArea(p); math.Abs(a-450) > 30 {
		t.Errorf("diamond area = %.2f, want ~450", a)
	}
}

func TestFillPathCircleApproxArea(t *testing.T) {
	p := newPixel(60, 60)
	// Four cubic arcs approximate a circle of radius 25 centred at (30,30).
	const cx, cy, r = 30.0, 30.0, 25.0
	const k = 0.5522847498307936 * r // cubic circle constant * r
	circ := NewPath().MoveTo(cx+r, cy).
		CubicTo(cx+r, cy+k, cx+k, cy+r, cx, cy+r).
		CubicTo(cx-k, cy+r, cx-r, cy+k, cx-r, cy).
		CubicTo(cx-r, cy-k, cx-k, cy-r, cx, cy-r).
		CubicTo(cx+k, cy-r, cx+r, cy-k, cx+r, cy).
		Close()
	p.FillPath(circ, RGB(0x80, 0x80, 0x80), NonZero)

	if a := alphaAt(p, 30, 30); a != 0xFF {
		t.Errorf("circle centre alpha = %d, want 255", a)
	}
	if a := alphaAt(p, 2, 2); a != 0 {
		t.Errorf("circle corner alpha = %d, want 0", a)
	}
	want := math.Pi * r * r // ~1963
	if a := coveredArea(p); math.Abs(a-want) > 0.02*want {
		t.Errorf("circle area = %.1f, want ~%.1f (2%% tol)", a, want)
	}
}

func TestFillPathAntiAliasedEdge(t *testing.T) {
	// A non-axis-aligned edge produces partial-coverage (fractional-alpha) pixels.
	p := newPixel(40, 40)
	tri := NewPath().MoveTo(2, 2).LineTo(30, 6).LineTo(6, 30).Close()
	p.FillPath(tri, RGB(0xFF, 0xFF, 0xFF), NonZero)
	partial := false
	for i := 3; i < len(p.Buf); i += 4 {
		if a := p.Buf[i]; a > 0 && a < 0xFF {
			partial = true
			break
		}
	}
	if !partial {
		t.Error("no anti-aliased (partial-alpha) pixel on a slanted edge")
	}
}

// --- winding rules ---------------------------------------------------------

// star5 builds a classic 5-point self-overlapping star (pentagram) centred at
// (cx,cy). Drawn by connecting every second vertex, its centre pentagon is
// wound twice: NonZero fills it, EvenOdd cuts it out.
func star5(cx, cy, r float64) *Path {
	pth := NewPath()
	for i := 0; i <= 5; i++ {
		ang := -math.Pi/2 + float64(i)*4*math.Pi/5 // step by 2 vertices (144°)
		x := cx + r*math.Cos(ang)
		y := cy + r*math.Sin(ang)
		if i == 0 {
			pth.MoveTo(x, y)
		} else {
			pth.LineTo(x, y)
		}
	}
	return pth.Close()
}

func TestFillPathWindingNonZeroVsEvenOdd(t *testing.T) {
	mkNonZero := newPixel(60, 60)
	mkEvenOdd := newPixel(60, 60)
	star := star5(30, 32, 26)
	white := RGB(0xFF, 0xFF, 0xFF)
	mkNonZero.FillPath(star, white, NonZero)
	mkEvenOdd.FillPath(star, white, EvenOdd)

	// Centre pentagon: filled under NonZero, a hole under EvenOdd.
	if a := alphaAt(mkNonZero, 30, 32); a != 0xFF {
		t.Errorf("NonZero centre alpha = %d, want 255 (filled)", a)
	}
	if a := alphaAt(mkEvenOdd, 30, 32); a != 0 {
		t.Errorf("EvenOdd centre alpha = %d, want 0 (hole)", a)
	}
	// A point on an outer arm is filled under both rules.
	armX, armY := 30, 8 // near the top spike
	if a := alphaAt(mkNonZero, armX, armY); a == 0 {
		t.Error("NonZero: top arm should be filled")
	}
	if a := alphaAt(mkEvenOdd, armX, armY); a == 0 {
		t.Error("EvenOdd: top arm should be filled")
	}
	// EvenOdd removes the centre, so it paints strictly less area than NonZero.
	if ea, na := coveredArea(mkEvenOdd), coveredArea(mkNonZero); ea >= na {
		t.Errorf("EvenOdd area %.1f should be < NonZero area %.1f", ea, na)
	}
}

// --- stroke ----------------------------------------------------------------

func TestStrokePathOpenLine(t *testing.T) {
	p := newPixel(40, 12)
	line := NewPath().MoveTo(4, 6).LineTo(34, 6)
	p.StrokePath(line, RGB(0xFF, 0xFF, 0xFF), 4)
	// Centre of the stroke is opaque.
	if a := alphaAt(p, 19, 6); a != 0xFF {
		t.Errorf("stroke centre alpha = %d, want 255", a)
	}
	// A row well above the stroke band is untouched.
	if a := alphaAt(p, 19, 1); a != 0 {
		t.Errorf("above-stroke alpha = %d, want 0", a)
	}
	// The half-width-2 stroke reaches ~2px each side of the centre line.
	if a := alphaAt(p, 19, 4); a == 0 {
		t.Error("stroke should cover 2px above centre")
	}
}

func TestStrokePathClosedVsOpen(t *testing.T) {
	// A closed triangle strokes its closing edge; the open version leaves that
	// edge blank. Compare a pixel on the would-be closing segment.
	tri := func(closed bool) *Path {
		pth := NewPath().MoveTo(6, 6).LineTo(30, 6).LineTo(18, 28)
		if closed {
			pth.Close()
		}
		return pth
	}
	pc := newPixel(40, 40)
	po := newPixel(40, 40)
	pc.StrokePath(tri(true), RGB(0xFF, 0xFF, 0xFF), 3)
	po.StrokePath(tri(false), RGB(0xFF, 0xFF, 0xFF), 3)

	// Midpoint of the first edge (6,6)->(30,6) is stroked in both.
	if alphaAt(pc, 18, 6) == 0 || alphaAt(po, 18, 6) == 0 {
		t.Error("top edge should be stroked in both closed and open paths")
	}
	// The closing edge runs from (18,28) back to (6,6); sample its midpoint (12,17).
	if a := alphaAt(pc, 12, 17); a == 0 {
		t.Error("closed path should stroke the closing edge")
	}
	if a := alphaAt(po, 12, 17); a != 0 {
		t.Errorf("open path should NOT stroke the closing edge, got alpha %d", a)
	}
}

func TestStrokePathRoundCapNoDoubleDarken(t *testing.T) {
	// Two collinear segments share a vertex; the union (MAX) must not exceed full
	// coverage there — the shared-vertex pixel is exactly opaque, not over-blended.
	p := newPixel(40, 12)
	poly := NewPath().MoveTo(4, 6).LineTo(20, 6).LineTo(36, 6)
	p.StrokePath(poly, RGB(0xFF, 0xFF, 0xFF), 5)
	if a := alphaAt(p, 20, 6); a != 0xFF {
		t.Errorf("shared-vertex alpha = %d, want exactly 255 (no double-darken)", a)
	}
}

func TestStrokePathZeroLengthSegmentDot(t *testing.T) {
	// A degenerate LineTo to the same point yields a zero-length segment (nil
	// rectangle); the vertex disk still paints a round dot.
	p := newPixel(20, 20)
	dot := NewPath().MoveTo(10, 10).LineTo(10, 10)
	p.StrokePath(dot, RGB(0xFF, 0xFF, 0xFF), 6)
	if a := alphaAt(p, 10, 10); a == 0 {
		t.Error("zero-length segment should still paint a round dot via the join disk")
	}
}

// --- clip interaction ------------------------------------------------------

func TestFillPathHonoursClip(t *testing.T) {
	p := newPixel(30, 30)
	p.PushClip(Rect{10, 10, 10, 10})
	// A big rectangle covering the whole surface, clipped to the 10x10 window.
	big := NewPath().MoveTo(0, 0).LineTo(30, 0).LineTo(30, 30).LineTo(0, 30).Close()
	p.FillPath(big, RGB(0xFF, 0xFF, 0xFF), NonZero)
	p.PopClip()

	// Inside the clip: painted.
	if a := alphaAt(p, 14, 14); a != 0xFF {
		t.Errorf("inside-clip alpha = %d, want 255", a)
	}
	// Outside the clip: nothing, on every side.
	for _, pt := range [][2]int{{5, 5}, {25, 25}, {5, 14}, {25, 14}, {14, 5}, {14, 25}} {
		if a := alphaAt(p, pt[0], pt[1]); a != 0 {
			t.Errorf("outside-clip %v alpha = %d, want 0", pt, a)
		}
	}
	// Covered area is exactly the 10x10 clip window.
	if a := coveredArea(p); math.Abs(a-100) > 0.5 {
		t.Errorf("clipped area = %.2f, want 100", a)
	}
}

func TestStrokePathHonoursClip(t *testing.T) {
	p := newPixel(40, 40)
	p.PushClip(Rect{0, 0, 20, 40})
	line := NewPath().MoveTo(2, 20).LineTo(38, 20)
	p.StrokePath(line, RGB(0xFF, 0xFF, 0xFF), 6)
	p.PopClip()
	if a := alphaAt(p, 10, 20); a == 0 {
		t.Error("stroke inside clip should paint")
	}
	if a := alphaAt(p, 30, 20); a != 0 {
		t.Errorf("stroke outside clip should be blank, got %d", a)
	}
}

// --- degenerate / no-op branches ------------------------------------------

func TestFillPathNoOps(t *testing.T) {
	blank := func(p *PixelPainter) bool {
		for _, b := range p.Buf {
			if b != 0 {
				return false
			}
		}
		return true
	}
	cases := []struct {
		name string
		draw func(p *PixelPainter)
	}{
		{"nil path", func(p *PixelPainter) { p.FillPath(nil, RGB(1, 1, 1), NonZero) }},
		{"empty path", func(p *PixelPainter) { p.FillPath(NewPath(), RGB(1, 1, 1), NonZero) }},
		{"single point", func(p *PixelPainter) { p.FillPath(NewPath().MoveTo(5, 5), RGB(1, 1, 1), NonZero) }},
		{"transparent colour", func(p *PixelPainter) {
			p.FillPath(NewPath().MoveTo(0, 0).LineTo(8, 0).LineTo(0, 8).Close(), RGBA{9, 9, 9, 0}, NonZero)
		}},
		{"entirely off-screen", func(p *PixelPainter) {
			p.FillPath(NewPath().MoveTo(-20, -20).LineTo(-10, -20).LineTo(-15, -10).Close(), RGB(1, 1, 1), NonZero)
		}},
	}
	for _, tc := range cases {
		p := newPixel(16, 16)
		tc.draw(p)
		if !blank(p) {
			t.Errorf("%s: expected no-op, buffer was written", tc.name)
		}
	}
}

func TestStrokePathNoOps(t *testing.T) {
	blank := func(p *PixelPainter) bool {
		for _, b := range p.Buf {
			if b != 0 {
				return false
			}
		}
		return true
	}
	cases := []struct {
		name string
		draw func(p *PixelPainter)
	}{
		{"nil path", func(p *PixelPainter) { p.StrokePath(nil, RGB(1, 1, 1), 2) }},
		{"empty path", func(p *PixelPainter) { p.StrokePath(NewPath(), RGB(1, 1, 1), 2) }},
		{"zero width", func(p *PixelPainter) {
			p.StrokePath(NewPath().MoveTo(2, 2).LineTo(12, 2), RGB(1, 1, 1), 0)
		}},
		{"negative width", func(p *PixelPainter) {
			p.StrokePath(NewPath().MoveTo(2, 2).LineTo(12, 2), RGB(1, 1, 1), -3)
		}},
		{"transparent colour", func(p *PixelPainter) {
			p.StrokePath(NewPath().MoveTo(2, 2).LineTo(12, 2), RGBA{9, 9, 9, 0}, 3)
		}},
		{"single point (no segment)", func(p *PixelPainter) {
			p.StrokePath(NewPath().MoveTo(8, 8), RGB(1, 1, 1), 4)
		}},
		{"entirely off-screen", func(p *PixelPainter) {
			p.StrokePath(NewPath().MoveTo(-40, -40).LineTo(-30, -40), RGB(1, 1, 1), 3)
		}},
	}
	for _, tc := range cases {
		p := newPixel(16, 16)
		tc.draw(p)
		if !blank(p) {
			t.Errorf("%s: expected no-op, buffer was written", tc.name)
		}
	}
}

// --- capability wiring -----------------------------------------------------

func TestPathPainterCapabilityAssertion(t *testing.T) {
	// PixelPainter advertises PathPainter; CellPainter deliberately does not.
	var px Painter = newPixel(4, 4)
	if _, ok := px.(PathPainter); !ok {
		t.Error("PixelPainter should implement PathPainter")
	}
	var cell Painter = NewCellPainter(4, 4)
	if _, ok := cell.(PathPainter); ok {
		t.Error("CellPainter should NOT implement PathPainter (capability absent by design)")
	}
}

func TestFillPathClampsToSurface(t *testing.T) {
	// A shape spilling past the right + bottom edges is clamped to the surface
	// and still paints its on-screen part.
	p := newPixel(16, 16)
	big := NewPath().MoveTo(8, 8).LineTo(40, 8).LineTo(40, 40).LineTo(8, 40).Close()
	p.FillPath(big, RGB(0xFF, 0xFF, 0xFF), NonZero)
	if a := alphaAt(p, 12, 12); a != 0xFF {
		t.Errorf("on-screen interior alpha = %d, want 255", a)
	}
	if a := alphaAt(p, 15, 15); a != 0xFF {
		t.Errorf("bottom-right corner alpha = %d, want 255 (clamped, still painted)", a)
	}
}

func TestStrokePathOffSurfaceSegmentSkipped(t *testing.T) {
	// A path with a segment lying entirely off the surface exercises the
	// rasterizer's off-surface skip: that segment contributes nothing, but the
	// on-surface part still strokes.
	buf := make([]byte, 4*16*16)
	p := NewPixelPainter(buf, 16, 16)
	pth := NewPath().MoveTo(-100, 8).LineTo(-90, 8).LineTo(8, 8)
	p.StrokePath(pth, RGB(0xFF, 0, 0), 3)
	if coveredArea(p) == 0 {
		t.Fatal("on-surface part of the stroke painted nothing")
	}
	// The centre, on the visible segment, must be inked.
	if alphaAt(p, 8, 8) == 0 {
		t.Error("visible segment centre not painted")
	}
}

// --- composite (painter's own layer) --------------------------------------

func TestCompositeClampAndZeroAlpha(t *testing.T) {
	// White-box: a coverage > 1 clamps to full opacity; a coverage so small it
	// rounds the scaled alpha to 0 paints nothing.
	p := newPixel(4, 4)
	p.composite([]float64{1.5, 0.0009}, 0, 0, 2, 1, RGB(0xFF, 0xFF, 0xFF))
	if a := alphaAt(p, 0, 0); a != 0xFF {
		t.Errorf("coverage 1.5 should clamp to opaque, got %d", a)
	}
	if a := alphaAt(p, 1, 0); a != 0 {
		t.Errorf("coverage 0.0009 should round to nothing, got %d", a)
	}
}

func TestCompositeNegativeOriginClampsAndShortBuffer(t *testing.T) {
	// White-box for composite's up-front intersection. A grid whose origin is
	// off the top-left (ox,oy < 0) must clamp to the surface (exercises the
	// x0<0 / y0<0 branches), and an under-sized buffer must drop the pixels it
	// cannot hold (the off+3 >= len(Buf) guard) instead of panicking.
	p := NewPixelPainter(make([]byte, 4*3*3), 3, 3)
	cov := make([]float64, 9) // 3x3 grid at origin (-1,-1)
	for i := range cov {
		cov[i] = 1
	}
	p.composite(cov, -1, -1, 3, 3, RGB(0xFF, 0, 0))
	// Only the on-surface part (grid cells 1..2 in each axis -> pixels 0..1) inks.
	if alphaAt(p, 0, 0) != 0xFF {
		t.Errorf("clamped pixel (0,0) alpha = %d, want 255", alphaAt(p, 0, 0))
	}

	// Under-sized buffer: Width/Height claim 4x4 but the buffer holds one pixel.
	short := NewPixelPainter(make([]byte, 4), 4, 4)
	full := make([]float64, 16)
	for i := range full {
		full[i] = 1
	}
	short.composite(full, 0, 0, 4, 4, RGB(0, 0xFF, 0)) // must not panic
	if short.Buf[3] != 0xFF {
		t.Errorf("in-range pixel alpha = %d, want 255", short.Buf[3])
	}
}

func TestCompositeClipAndSurfaceEdges(t *testing.T) {
	// White-box: a clip strictly inside the grid on all four sides fires every
	// clip-clamp branch, and a grid extending past the right/bottom edges fires
	// the surface-clamp branches. Only the clip∩surface∩grid cell inks.
	p := NewPixelPainter(make([]byte, 4*8*8), 8, 8)
	p.PushClip(Rect{X: 2, Y: 2, W: 3, H: 3}) // inside on all sides
	cov := make([]float64, 100)              // 10x10 grid at (0,0) -> overruns 8x8
	for i := range cov {
		cov[i] = 1
	}
	p.composite(cov, 0, 0, 10, 10, RGB(0, 0, 0xFF))
	p.PopClip()
	// Inside the clip: inked. Outside the clip but inside surface: untouched.
	if alphaAt(p, 3, 3) != 0xFF {
		t.Errorf("clipped-in pixel alpha = %d, want 255", alphaAt(p, 3, 3))
	}
	if alphaAt(p, 6, 6) != 0 {
		t.Errorf("clipped-out pixel alpha = %d, want 0", alphaAt(p, 6, 6))
	}
}
