// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"math"
	"testing"
)

// benchSurface returns a fresh 512x512 painter over a zeroed RGBA buffer, the
// realistic icon/scene surface the rasterizer fills. The buffer is reused across
// iterations (the caller re-zeroes only when a run's output must be clean); for a
// pure hot-path benchmark the accumulating composite is representative.
func benchSurface() *PixelPainter {
	const s = 512
	return NewPixelPainter(make([]byte, 4*s*s), s, s)
}

// circlePath approximates a circle of radius r centred at (cx, cy) with four
// cubic quadrants — the canonical curved fill (many flattened edges).
func circlePath(cx, cy, r float64) *Path {
	const k = 0.5522847498307936 // 4/3 * (sqrt(2)-1): quadrant control offset
	o := r * k
	return NewPath().
		MoveTo(cx+r, cy).
		CubicTo(cx+r, cy+o, cx+o, cy+r, cx, cy+r).
		CubicTo(cx-o, cy+r, cx-r, cy+o, cx-r, cy).
		CubicTo(cx-r, cy-o, cx-o, cy-r, cx, cy-r).
		CubicTo(cx+o, cy-r, cx+r, cy-o, cx+r, cy).
		Close()
}

// manyVertexPolygon builds an n-gon inscribed in radius r — a large filled path
// with n straight edges and no curves.
func manyVertexPolygon(cx, cy, r float64, n int) *Path {
	p := NewPath()
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		x, y := cx+r*math.Cos(a), cy+r*math.Sin(a)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	return p.Close()
}

// starPath builds a self-overlapping n-point star (exercises the winding rules:
// NonZero fills it solid, EvenOdd cuts the centre out).
func starPath(cx, cy, rOuter, rInner float64, points int) *Path {
	p := NewPath()
	for i := 0; i < points*2; i++ {
		r := rOuter
		if i%2 == 1 {
			r = rInner
		}
		a := math.Pi * float64(i) / float64(points)
		x, y := cx+r*math.Sin(a), cy-r*math.Cos(a)
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	return p.Close()
}

// polyline builds an open zig-zag polyline with n vertices spanning the surface
// — the canonical long stroked path.
func polyline(n int, w, h float64) *Path {
	p := NewPath()
	for i := 0; i < n; i++ {
		x := w * float64(i) / float64(n-1)
		y := h / 2
		if i%2 == 1 {
			y = h / 4
		}
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	return p
}

func BenchmarkFillPathCircleNonZero(b *testing.B) {
	p := benchSurface()
	pth := circlePath(256, 256, 240)
	ink := RGBA{0x33, 0x66, 0xCC, 0xFF}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.FillPath(pth, ink, NonZero)
	}
}

func BenchmarkFillPathPolygonNonZero(b *testing.B) {
	p := benchSurface()
	pth := manyVertexPolygon(256, 256, 240, 256)
	ink := RGBA{0x33, 0x66, 0xCC, 0xFF}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.FillPath(pth, ink, NonZero)
	}
}

func BenchmarkFillPathStarNonZero(b *testing.B) {
	p := benchSurface()
	pth := starPath(256, 256, 240, 96, 8)
	ink := RGBA{0x33, 0x66, 0xCC, 0xFF}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.FillPath(pth, ink, NonZero)
	}
}

func BenchmarkFillPathStarEvenOdd(b *testing.B) {
	p := benchSurface()
	pth := starPath(256, 256, 240, 96, 8)
	ink := RGBA{0x33, 0x66, 0xCC, 0xFF}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.FillPath(pth, ink, EvenOdd)
	}
}

func BenchmarkFillPathClipped(b *testing.B) {
	p := benchSurface()
	pth := circlePath(256, 256, 240)
	ink := RGBA{0x33, 0x66, 0xCC, 0xFF}
	p.PushClip(Rect{X: 128, Y: 128, W: 256, H: 256})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.FillPath(pth, ink, NonZero)
	}
	p.PopClip()
}

func BenchmarkStrokePathPolyline(b *testing.B) {
	p := benchSurface()
	pth := polyline(64, 512, 512)
	ink := RGBA{0x33, 0x66, 0xCC, 0xFF}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.StrokePath(pth, ink, 6)
	}
}

func BenchmarkStrokePathCircle(b *testing.B) {
	p := benchSurface()
	pth := circlePath(256, 256, 200)
	ink := RGBA{0x33, 0x66, 0xCC, 0xFF}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.StrokePath(pth, ink, 4)
	}
}
