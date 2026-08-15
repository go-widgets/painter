// Copyright (c) the go-widgets/painter authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package painter

import "testing"

// StrokeRect and StrokeRoundRect took a lineW and threw it away: the pixel
// back-end always painted a one-pixel outline, so a caller asking for a thicker
// border got a hairline and no error. It is the reason a toolkit could not
// thicken its chrome for a HiDPI screen -- the request had nowhere to land.
//
// These check the thickness where it is visible: how far in from the edge the
// border still paints, and where the interior begins.

// strokeProbe fills a surface with a sentinel, strokes it and reports the run
// of border pixels inward from the left edge along the middle row.
func strokeProbe(t *testing.T, w, h, lineW, radius int, round bool) (border int, buf []byte) {
	t.Helper()
	buf = make([]byte, 4*w*h)
	p := NewPixelPainter(buf, w, h)
	ink := RGBA{R: 255, G: 255, B: 255, A: 255}
	r := Rect{X: 0, Y: 0, W: w, H: h}
	if round {
		p.StrokeRoundRect(r, radius, ink, lineW)
	} else {
		p.StrokeRect(r, ink, lineW)
	}
	y := h / 2
	for x := 0; x < w; x++ {
		o := 4 * (y*w + x)
		if buf[o] == 0 && buf[o+1] == 0 && buf[o+2] == 0 {
			break
		}
		border++
	}
	return border, buf
}

func TestStrokeRectHonoursLineWidth(t *testing.T) {
	for _, tc := range []struct {
		lineW, want int
	}{
		{0, 1},  // a border of no pixels is not a border
		{-3, 1}, // nor is one of negative pixels
		{1, 1},
		{2, 2},
		{3, 3},
		{5, 5},
	} {
		got, _ := strokeProbe(t, 40, 20, tc.lineW, 0, false)
		if got != tc.want {
			t.Errorf("StrokeRect lineW=%d painted %d pixels inward, want %d", tc.lineW, got, tc.want)
		}
	}
}

// A stroke thicker than the rect fills it rather than painting outside it or
// running off the end of the buffer.
func TestStrokeRectThickerThanTheRect(t *testing.T) {
	const w, h = 9, 7
	got, buf := strokeProbe(t, w, h, 20, 0, false)
	if got != w {
		t.Errorf("an over-thick stroke painted %d pixels of a %d-wide rect, want all of it", got, w)
	}
	// Every pixel, and nothing beyond: the rings stop when they meet.
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] != 255 {
			t.Fatalf("pixel %d was left unpainted by an over-thick stroke", i/4)
		}
	}
}

func TestStrokeRoundRectHonoursLineWidth(t *testing.T) {
	// Measured on the middle row, where a rounded rect's edge is straight, so
	// the thickness is the same question as for a square one.
	for _, tc := range []struct {
		lineW, want int
	}{
		{0, 1},  // a border of no pixels is not a border
		{-3, 1}, // nor is one of negative pixels
		{1, 1},
		{2, 2},
		{4, 4},
	} {
		got, _ := strokeProbe(t, 60, 40, tc.lineW, 8, true)
		if got != tc.want {
			t.Errorf("StrokeRoundRect lineW=%d painted %d pixels inward, want %d", tc.lineW, got, tc.want)
		}
	}
}

// A radius of zero is a square rect, and must be as thick as it was asked for
// through either entry point.
func TestStrokeRoundRectWithNoRadius(t *testing.T) {
	got, _ := strokeProbe(t, 30, 20, 3, 0, true)
	if got != 3 {
		t.Errorf("a rounded stroke with radius 0 painted %d pixels inward, want 3", got)
	}
}

// The cell back-end has nothing to vary: a terminal cell is atomic, and its
// documented behaviour is to ignore the hint. Asserted so a future change to
// the pixel side does not quietly take the cell side with it.
func TestCellPainterStillIgnoresLineWidth(t *testing.T) {
	p := NewCellPainter(10, 4)
	p.StrokeRect(Rect{X: 0, Y: 0, W: 10, H: 4}, RGBA{R: 255, A: 255}, 3)
	// The interior is untouched: a 3-cell-thick border would have swallowed it.
	if got := p.Cells[1*10+5]; got.Bg.A != 0 {
		t.Errorf("the cell interior was painted (%+v), so lineW was not ignored", got)
	}
}

// A rounded stroke thicker than the rect stops when the rings meet, rather than
// walking outside it.
func TestStrokeRoundRectThickerThanTheRect(t *testing.T) {
	const w, h = 12, 10
	buf := make([]byte, 4*w*h)
	p := NewPixelPainter(buf, w, h)
	p.StrokeRoundRect(Rect{X: 0, Y: 0, W: w, H: h}, 4, RGBA{R: 255, G: 255, B: 255, A: 255}, 40)
	// The centre is inside every ring, so it is painted; nothing crashed and
	// nothing was written past the surface.
	o := 4 * ((h/2)*w + w/2)
	if buf[o] == 0 && buf[o+1] == 0 && buf[o+2] == 0 {
		t.Error("the centre of an over-thick rounded stroke was left unpainted")
	}
}
