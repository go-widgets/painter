// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// Clipper is an optional Painter capability: while a clip rect is pushed, every
// drawing primitive is confined to it (intersected with any enclosing clip).
// It is the seam a scrollable/overflowing widget uses to keep a child inside
// its own bounds — the base Painter interface deliberately cannot draw-clip, so
// widgets that need it type-assert:
//
//	if c, ok := p.(painter.Clipper); ok {
//		c.PushClip(bounds)
//		defer c.PopClip()
//	}
//	child.Draw(p, theme)
//
// Both PixelPainter and CellPainter implement Clipper; a back-end that cannot
// clip simply does not, and the assertion is skipped.
type Clipper interface {
	PushClip(r Rect)
	PopClip()
}

// intersect returns the overlap of a and b, or an empty Rect (W or H == 0) when
// they do not overlap — an empty clip draws nothing, which is the correct result
// for a child scrolled entirely out of view.
func intersect(a, b Rect) Rect {
	x0 := max(a.X, b.X)
	y0 := max(a.Y, b.Y)
	x1 := min(a.X+a.W, b.X+b.W)
	y1 := min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// pushClip appends r (intersected with the current top) to a clip stack.
func pushClip(stack []Rect, r Rect) []Rect {
	if len(stack) > 0 {
		r = intersect(r, stack[len(stack)-1])
	}
	return append(stack, r)
}

// popClip removes the top of a clip stack (a no-op on an empty stack).
func popClip(stack []Rect) []Rect {
	if len(stack) == 0 {
		return stack
	}
	return stack[:len(stack)-1]
}

// clipAllows reports whether (x, y) may be drawn given the clip stack: true when
// no clip is set, else the current (top) clip must contain the point.
func clipAllows(stack []Rect, x, y int) bool {
	if n := len(stack); n > 0 {
		return stack[n-1].Contains(x, y)
	}
	return true
}
