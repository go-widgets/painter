// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

// both painters satisfy Clipper.
var (
	_ Clipper = (*PixelPainter)(nil)
	_ Clipper = (*CellPainter)(nil)
)

// pxAt reads the RGBA at (x, y) from a PixelPainter's buffer.
func pxAt(p *PixelPainter, x, y int) RGBA {
	o := (y*p.Width + x) * 4
	return RGBA{R: p.Buf[o], G: p.Buf[o+1], B: p.Buf[o+2], A: p.Buf[o+3]}
}

func TestPixelPainterClipConfinesDrawing(t *testing.T) {
	p := NewPixelPainter(make([]byte, 10*10*4), 10, 10)
	ink := RGBA{R: 1, G: 2, B: 3, A: 255}
	p.PushClip(Rect{X: 2, Y: 2, W: 3, H: 3}) // allow only [2,5)×[2,5)
	p.FillRect(Rect{X: 0, Y: 0, W: 10, H: 10}, ink)
	p.PopClip()

	if pxAt(p, 3, 3) != ink {
		t.Errorf("inside clip not drawn: %+v", pxAt(p, 3, 3))
	}
	for _, pt := range [][2]int{{0, 0}, {1, 1}, {5, 5}, {9, 9}, {5, 3}} {
		if got := pxAt(p, pt[0], pt[1]); got != (RGBA{}) {
			t.Errorf("outside clip drawn at %v: %+v", pt, got)
		}
	}
	// After PopClip, drawing is unconfined again.
	p.FillRect(Rect{X: 0, Y: 0, W: 1, H: 1}, ink)
	if pxAt(p, 0, 0) != ink {
		t.Error("PopClip did not restore full-surface drawing")
	}
}

func TestPixelPainterClipNestingIntersects(t *testing.T) {
	p := NewPixelPainter(make([]byte, 10*10*4), 10, 10)
	ink := RGBA{R: 9, A: 255}
	p.PushClip(Rect{X: 0, Y: 0, W: 6, H: 6})
	p.PushClip(Rect{X: 4, Y: 4, W: 6, H: 6}) // ∩ = [4,6)×[4,6)
	p.FillRect(Rect{X: 0, Y: 0, W: 10, H: 10}, ink)
	if pxAt(p, 5, 5) != ink {
		t.Error("nested-clip intersection not drawn at (5,5)")
	}
	if pxAt(p, 2, 2) != (RGBA{}) || pxAt(p, 7, 7) != (RGBA{}) {
		t.Error("drawing escaped the intersection")
	}
	p.PopClip()
	p.PopClip()
}

func TestPixelPainterClipEmptyIntersectionDrawsNothing(t *testing.T) {
	p := NewPixelPainter(make([]byte, 8*8*4), 8, 8)
	p.PushClip(Rect{X: 0, Y: 0, W: 3, H: 3})
	p.PushClip(Rect{X: 5, Y: 5, W: 3, H: 3}) // disjoint → empty
	p.FillRect(Rect{X: 0, Y: 0, W: 8, H: 8}, RGBA{R: 1, A: 255})
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if pxAt(p, x, y) != (RGBA{}) {
				t.Fatalf("disjoint clip drew at (%d,%d)", x, y)
			}
		}
	}
}

func TestPopClipEmptyIsNoOp(t *testing.T) {
	p := NewPixelPainter(make([]byte, 4*4*4), 4, 4)
	p.PopClip() // must not panic on an empty stack
	p.FillRect(Rect{X: 0, Y: 0, W: 4, H: 4}, RGBA{R: 5, A: 255})
	if pxAt(p, 0, 0) != (RGBA{R: 5, A: 255}) {
		t.Error("PopClip on empty stack disturbed drawing")
	}
}

func TestCellPainterClipConfinesDrawing(t *testing.T) {
	p := NewCellPainter(6, 6)
	p.PushClip(Rect{X: 1, Y: 1, W: 2, H: 2}) // allow [1,3)×[1,3)
	p.FillRect(Rect{X: 0, Y: 0, W: 6, H: 6}, RGBA{R: 10})
	p.Text(0, 0, "Z", RGBA{}) // outside the clip → dropped
	p.PopClip()
	if p.Cells[1*6+1].Bg != (RGBA{R: 10}) {
		t.Error("cell inside clip not filled")
	}
	if p.Cells[0].Bg != (RGBA{}) || p.Cells[0].Rune == 'Z' {
		t.Errorf("cell outside clip written: %+v", p.Cells[0])
	}
}

func TestStrokeRectCornersPaintedOnce(t *testing.T) {
	// A translucent stroke over a solid ground: with the old double-corner paint
	// a corner composited twice and read darker than the edges. It must now equal.
	buf := make([]byte, 6*6*4)
	for i := range buf {
		buf[i] = 0xFF // white, opaque
	}
	p := NewPixelPainter(buf, 6, 6)
	p.StrokeRect(Rect{X: 0, Y: 0, W: 5, H: 5}, RGBA{R: 0, G: 0, B: 0, A: 128}, 1)
	corner := pxAt(p, 0, 0)
	edge := pxAt(p, 2, 0) // a top-edge pixel, painted exactly once
	if corner != edge {
		t.Fatalf("corner %+v != edge %+v — corner double-composited", corner, edge)
	}
}

func TestIntersectDisjointIsEmpty(t *testing.T) {
	if got := intersect(Rect{X: 0, Y: 0, W: 2, H: 2}, Rect{X: 5, Y: 5, W: 2, H: 2}); got != (Rect{}) {
		t.Errorf("disjoint intersect = %+v, want empty", got)
	}
	if got := intersect(Rect{X: 0, Y: 0, W: 4, H: 4}, Rect{X: 2, Y: 2, W: 4, H: 4}); got != (Rect{X: 2, Y: 2, W: 2, H: 2}) {
		t.Errorf("overlap intersect = %+v, want {2,2,2,2}", got)
	}
}
