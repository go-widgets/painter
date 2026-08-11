// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// Translator is an optional Painter capability: while a translation is pushed,
// every coordinate handed to the painter is shifted by it before anything is
// clipped or written.
//
// Together with [Clipper] it is a VIEWPORT — the pair a scrolling or panning
// widget needs. Clipping alone was not enough, and the gap had a real cost:
// with no way to say "draw my child 250 pixels higher", [Clipper]'s only
// customer, ScrollView, moved the child's BOUNDS instead, drew, and put them
// back. Geometry that changes for the duration of a paint is invisible to
// anything reading it from outside — a screen reader was told a control sat a
// quarter of a window below where it was painted.
//
// A translation shifts the PAINT, not the widget: a child still lays out and
// reports its bounds wherever it genuinely is, and the viewport decides where
// those pixels land. Nothing has to be moved and restored.
//
//	if t, ok := p.(painter.Translator); ok {
//		t.PushTranslate(-offsetX, -offsetY)
//		defer t.PopTranslate()
//	}
//	child.Draw(p, theme)
//
// Translations nest: each push adds to the enclosing one, so a scrolled list
// inside a scrolled panel behaves as a reader would expect. A clip pushed while
// a translation is active is translated too, since the caller expresses it in
// the same coordinates as everything else it draws.
//
// Both PixelPainter and CellPainter implement Translator; a back-end that
// cannot translate simply does not, and the assertion is skipped — the same
// contract [Clipper] uses.
type Translator interface {
	PushTranslate(dx, dy int)
	PopTranslate()
}

// offset is one entry of a translation stack: the ACCUMULATED shift at that
// depth, so reading the current translation is a look at the top rather than a
// walk down the stack.
type offset struct{ dx, dy int }

// pushOffset adds dx,dy to the enclosing translation and returns the new stack.
func pushOffset(s []offset, dx, dy int) []offset {
	cur := currentOffset(s)
	return append(s, offset{dx: cur.dx + dx, dy: cur.dy + dy})
}

// popOffset removes the innermost translation. Popping an empty stack is a
// no-op rather than a panic: a widget that pops without pushing is confused,
// not dangerous, and a painter is the wrong place to enforce that.
func popOffset(s []offset) []offset {
	if len(s) == 0 {
		return s
	}
	return s[:len(s)-1]
}

// currentOffset is the shift in force, or zero when nothing is pushed.
func currentOffset(s []offset) offset {
	if len(s) == 0 {
		return offset{}
	}
	return s[len(s)-1]
}

// shiftRect and shiftPoint move a caller's coordinates into surface space.
func shiftRect(s []offset, r Rect) Rect {
	o := currentOffset(s)
	r.X += o.dx
	r.Y += o.dy
	return r
}

func shiftPoint(s []offset, x, y int) (int, int) {
	o := currentOffset(s)
	return x + o.dx, y + o.dy
}
