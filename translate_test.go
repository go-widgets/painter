// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

func newPix(w, h int) *PixelPainter {
	return &PixelPainter{Buf: make([]byte, w*h*4), Width: w, Height: h}
}

func lit(p *PixelPainter, x, y int) bool {
	off := (y*p.Width + x) * 4
	return p.Buf[off+3] != 0
}

var white = RGBA{R: 255, G: 255, B: 255, A: 255}

// A translation moves the PAINT, not the caller's rectangle: the same
// FillRect lands somewhere else without the caller changing a coordinate.
func TestPushTranslateMovesThePaint(t *testing.T) {
	p := newPix(20, 20)
	p.PushTranslate(5, 7)
	p.FillRect(Rect{X: 0, Y: 0, W: 2, H: 2}, white)
	p.PopTranslate()

	if lit(p, 0, 0) {
		t.Error("painted at the untranslated origin")
	}
	if !lit(p, 5, 7) || !lit(p, 6, 8) {
		t.Error("the 2x2 fill did not land at 5,7")
	}
	if lit(p, 7, 9) {
		t.Error("painted beyond the 2x2 fill")
	}
}

// Popping restores the enclosing translation, so a widget cannot leak its
// offset onto its siblings.
func TestPopTranslateRestores(t *testing.T) {
	p := newPix(20, 20)
	p.PushTranslate(5, 5)
	p.PopTranslate()
	p.PutPixel(1, 1, white)
	if !lit(p, 1, 1) {
		t.Error("the translation outlived its Pop")
	}
}

// Translations nest by adding, so a scrolled list inside a scrolled panel
// behaves the way a reader expects.
func TestTranslationsNest(t *testing.T) {
	p := newPix(20, 20)
	p.PushTranslate(3, 0)
	p.PushTranslate(4, 2)
	p.PutPixel(0, 0, white)
	p.PopTranslate()
	p.PutPixel(0, 0, white)
	p.PopTranslate()

	if !lit(p, 7, 2) {
		t.Error("the inner translation did not accumulate onto the outer one")
	}
	if !lit(p, 3, 0) {
		t.Error("popping the inner translation did not return to the outer one")
	}
}

// The offset must be applied ONCE however many primitives compose to reach
// the surface: FillRoundRect falls through to FillRect, which calls PutPixel.
// Shifting in each public method instead of at the write would move a rounded
// rectangle three times as far as a pixel.
func TestTranslationAppliedOncePerWrite(t *testing.T) {
	p := newPix(30, 30)
	p.PushTranslate(10, 10)
	p.FillRoundRect(Rect{X: 0, Y: 0, W: 4, H: 4}, 0, white) // radius 0 -> FillRect
	p.PopTranslate()

	if !lit(p, 10, 10) {
		t.Error("the rounded fill did not land at the translated origin")
	}
	if lit(p, 20, 20) {
		t.Error("the offset was applied more than once")
	}
}

// A clip pushed while translated is expressed in the caller's coordinates,
// like everything else it draws.
func TestClipIsTranslatedToo(t *testing.T) {
	p := newPix(20, 20)
	p.PushTranslate(5, 5)
	p.PushClip(Rect{X: 0, Y: 0, W: 2, H: 2})
	p.FillRect(Rect{X: 0, Y: 0, W: 10, H: 10}, white)
	p.PopClip()
	p.PopTranslate()

	if !lit(p, 5, 5) || !lit(p, 6, 6) {
		t.Error("the clipped area was not painted")
	}
	if lit(p, 7, 7) {
		t.Error("painting escaped a clip that should have moved with the translation")
	}
}

// Popping more than was pushed is a confused caller, not a crash.
func TestPopTranslateOnEmptyIsSafe(t *testing.T) {
	p := newPix(4, 4)
	p.PopTranslate()
	p.PutPixel(0, 0, white)
	if !lit(p, 0, 0) {
		t.Error("an unbalanced Pop disturbed the painter")
	}
}

// The cell back-end carries the same capability, so a terminal UI scrolls the
// same way a pixel one does.
func TestCellPainterTranslates(t *testing.T) {
	c := &CellPainter{Cells: make([]Cell, 10*10), W: 10, H: 10}
	c.PushTranslate(2, 3)
	c.FillRect(Rect{X: 0, Y: 0, W: 1, H: 1}, white)
	c.PopTranslate()

	if c.Cells[3*10+2].Bg != white {
		t.Errorf("cell 2,3 = %+v, want the fill", c.Cells[3*10+2].Bg)
	}
	if c.Cells[0].Bg == white {
		t.Error("painted at the untranslated origin")
	}
}

// Both painters satisfy the capability, which is what widgets type-assert for.
func TestPaintersImplementTranslator(t *testing.T) {
	var _ Translator = (*PixelPainter)(nil)
	var _ Translator = (*CellPainter)(nil)
}
