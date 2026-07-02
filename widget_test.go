// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

// Compile-time assertion: every prototype widget satisfies Widget.
var (
	_ Widget = (*Button)(nil)
	_ Widget = (*Label)(nil)
	_ Widget = (*ProgressBar)(nil)
)

func TestButtonDrawsInkOnBothBackends(t *testing.T) {
	b := &Button{Bounds: Rect{0, 0, 40, 16}, Label: "OK"}
	th := LightTheme()

	pp := newPixel(40, 16)
	b.Draw(pp, th)
	if !hasInk(pp.Buf) {
		t.Fatalf("Button on PixelPainter: expected ink pixels")
	}

	cp := NewCellPainter(20, 4)
	b.Bounds = Rect{0, 0, 20, 4}
	b.Draw(cp, th)
	if !hasCellRune(cp, '┌') {
		t.Fatalf("Button on CellPainter: expected box-draw corner")
	}
	if !hasCellRune(cp, 'O') || !hasCellRune(cp, 'K') {
		t.Fatalf("Button on CellPainter: expected label runes")
	}
}

func TestButtonPressedSwapsInk(t *testing.T) {
	// pressed → fill = Accent, ink = Surface (inverted). The centre
	// of the button should carry the Accent colour.
	pp := newPixel(40, 16)
	b := &Button{Bounds: Rect{0, 0, 40, 16}, Label: "OK", Pressed: true}
	b.Draw(pp, LightTheme())
	// probe (2, 2) — inside the fill, outside the border/label
	off := (2*40 + 2) * 4
	if pp.Buf[off] != 0x0D || pp.Buf[off+1] != 0x94 || pp.Buf[off+2] != 0x88 {
		t.Fatalf("pressed button fill = %v, want teal", pp.Buf[off:off+3])
	}
}

func TestLabelDrawsInk(t *testing.T) {
	pp := newPixel(30, 12)
	l := &Label{Bounds: Rect{2, 2, 20, 8}, Text: "GO"}
	l.Draw(pp, LightTheme())
	if !hasInk(pp.Buf) {
		t.Fatalf("Label on PixelPainter: expected ink pixels")
	}
}

func TestProgressBarFillClamps(t *testing.T) {
	th := LightTheme()

	// value clamps below 0 → no accent-filled column
	pp := newPixel(20, 6)
	pb := &ProgressBar{Bounds: Rect{0, 0, 20, 6}, Value: -1}
	pb.Draw(pp, th)
	// interior pixel (2, 2) shouldn't be Accent
	off := (2*20 + 2) * 4
	if pp.Buf[off] == 0x0D && pp.Buf[off+1] == 0x94 {
		t.Fatalf("value=-1: interior should not be Accent, got teal")
	}

	// value clamps above 1 → whole bar Accent-filled
	pp = newPixel(20, 6)
	pb = &ProgressBar{Bounds: Rect{0, 0, 20, 6}, Value: 2}
	pb.Draw(pp, th)
	off = (2*20 + 10) * 4
	if !(pp.Buf[off] == 0x0D && pp.Buf[off+1] == 0x94 && pp.Buf[off+2] == 0x88) {
		t.Fatalf("value=2: expected Accent-filled centre, got %v", pp.Buf[off:off+3])
	}
}

func TestProgressBarMidValue(t *testing.T) {
	pp := newPixel(20, 6)
	pb := &ProgressBar{Bounds: Rect{0, 0, 20, 6}, Value: 0.5}
	pb.Draw(pp, LightTheme())
	// left of midpoint (x=3) should be Accent
	off := (2*20 + 3) * 4
	if !(pp.Buf[off] == 0x0D && pp.Buf[off+1] == 0x94) {
		t.Fatalf("value=0.5: pixel at x=3 should be Accent, got %v", pp.Buf[off:off+3])
	}
	// right of midpoint (x=15) should be Surface (white)
	off = (2*20 + 15) * 4
	if !(pp.Buf[off] == 0xFF && pp.Buf[off+1] == 0xFF && pp.Buf[off+2] == 0xFF) {
		t.Fatalf("value=0.5: pixel at x=15 should be Surface, got %v", pp.Buf[off:off+3])
	}
}

// helpers ---------------------------------------------------------

func hasInk(buf []byte) bool {
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 0 {
			return true
		}
	}
	return false
}

func hasCellRune(cp *CellPainter, r rune) bool {
	for _, c := range cp.Cells {
		if c.Rune == r {
			return true
		}
	}
	return false
}
