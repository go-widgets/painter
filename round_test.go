// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

func pxAlpha(p *PixelPainter, x, y int) uint8 {
	return p.Buf[(y*p.Width+x)*4+3]
}

func TestFillRoundRectRadiusZeroIsFillRect(t *testing.T) {
	p := newPixel(6, 6)
	p.FillRoundRect(Rect{0, 0, 6, 6}, 0, RGB(0x11, 0x22, 0x33))
	// Every pixel opaque -- identical to FillRect.
	for i := 3; i < len(p.Buf); i += 4 {
		if p.Buf[i] != 0xFF {
			t.Fatalf("radius 0 should fill every pixel opaque; buf[%d]=%d", i, p.Buf[i])
		}
	}
}

func TestFillRoundRectRoundsCorners(t *testing.T) {
	p := newPixel(20, 20)
	white := RGB(0xFF, 0xFF, 0xFF)
	p.FillRoundRect(Rect{0, 0, 20, 20}, 6, white)
	// The very corner is outside the corner circle -> untouched (alpha 0).
	if a := pxAlpha(p, 0, 0); a != 0 {
		t.Errorf("corner (0,0) alpha = %d, want 0 (rounded away)", a)
	}
	// The centre is fully filled.
	if a := pxAlpha(p, 10, 10); a != 0xFF {
		t.Errorf("centre alpha = %d, want 255", a)
	}
	// The top-left corner box carries at least one anti-aliased pixel
	// (0 < alpha < 255) on the arc.
	partial := false
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			if a := pxAlpha(p, x, y); a > 0 && a < 0xFF {
				partial = true
			}
		}
	}
	if !partial {
		t.Error("no anti-aliased (partial-alpha) pixel found on the corner arc")
	}
	// All four corners are rounded (symmetry).
	for _, c := range [][2]int{{19, 0}, {0, 19}, {19, 19}} {
		if a := pxAlpha(p, c[0], c[1]); a != 0 {
			t.Errorf("corner %v alpha = %d, want 0", c, a)
		}
	}
}

func TestFillRoundRectRadiusClamped(t *testing.T) {
	// radius far larger than the box -> clamped to half the smaller side; the
	// centre column still fills and it does not panic.
	p := newPixel(10, 8)
	p.FillRoundRect(Rect{0, 0, 10, 8}, 999, RGB(1, 2, 3))
	if a := pxAlpha(p, 5, 4); a != 0xFF {
		t.Errorf("centre alpha = %d, want 255", a)
	}
}

func TestFillRoundRectZeroSizeNoOp(t *testing.T) {
	p := newPixel(4, 4)
	p.FillRoundRect(Rect{0, 0, 0, 4}, 2, RGB(1, 1, 1))
	p.FillRoundRect(Rect{0, 0, 4, 0}, 2, RGB(1, 1, 1))
	for i, b := range p.Buf {
		if b != 0 {
			t.Fatalf("zero-size FillRoundRect wrote buf[%d]=%d", i, b)
		}
	}
}

func TestStrokeRoundRectRadiusZeroIsStrokeRect(t *testing.T) {
	p := newPixel(6, 6)
	p.StrokeRoundRect(Rect{0, 0, 6, 6}, 0, RGB(0xFF, 0, 0), 1)
	// Border present, interior empty (same shape as StrokeRect).
	if pxAlpha(p, 0, 0) != 0xFF {
		t.Error("radius 0: corner should be a crisp stroked pixel")
	}
	if pxAlpha(p, 3, 3) != 0 {
		t.Error("radius 0: interior should be empty")
	}
}

func TestStrokeRoundRectRoundsCorners(t *testing.T) {
	p := newPixel(20, 20)
	p.StrokeRoundRect(Rect{0, 0, 20, 20}, 6, RGB(0xFF, 0xFF, 0xFF), 1)
	// A straight-edge midpoint is stroked crisply.
	if pxAlpha(p, 10, 0) != 0xFF {
		t.Error("top edge midpoint should be a crisp border pixel")
	}
	// The square corner is rounded away (not stroked).
	if pxAlpha(p, 0, 0) != 0 {
		t.Error("square corner (0,0) should be rounded away")
	}
	// Some pixel on the corner arc is stroked (AA).
	found := false
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			if pxAlpha(p, x, y) > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Error("no corner-arc stroke pixels found")
	}
}

func TestStrokeRoundRectZeroSizeNoOp(t *testing.T) {
	p := newPixel(4, 4)
	p.StrokeRoundRect(Rect{0, 0, 0, 0}, 2, RGB(1, 1, 1), 1)
	for i, b := range p.Buf {
		if b != 0 {
			t.Fatalf("zero-size StrokeRoundRect wrote buf[%d]=%d", i, b)
		}
	}
}

func TestCellPainterRoundFallsBackToSquare(t *testing.T) {
	p := NewCellPainter(6, 6)
	p.FillRoundRect(Rect{1, 1, 3, 3}, 2, RGB(0x10, 0x20, 0x30))
	// Rounding is a no-op on a cell grid: the corner cell is still filled.
	if p.Cells[1*6+1].Bg != (RGBA{0x10, 0x20, 0x30, 0xFF}) {
		t.Errorf("FillRoundRect corner cell = %v, want the square fill", p.Cells[1*6+1].Bg)
	}
	p2 := NewCellPainter(6, 6)
	p2.StrokeRoundRect(Rect{1, 1, 4, 4}, 2, RGB(0xFF, 0, 0), 1)
	if p2.Cells[1*6+1].Rune != '┌' {
		t.Errorf("StrokeRoundRect corner rune = %q, want the square box corner", p2.Cells[1*6+1].Rune)
	}
}

func TestMin2(t *testing.T) {
	if min2(3, 5) != 3 || min2(5, 3) != 3 {
		t.Fatal("min2 wrong")
	}
}
