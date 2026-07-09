// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// Widget is what every UI element implements. A widget's Draw only
// touches the Painter primitives; it never allocates a buffer, never
// knows whether it's rendering pixels or cells.
type Widget interface {
	Draw(p Painter, theme *Theme)
}

// Button is the prototype's canonical widget — solid fill + border +
// centred label. Every real widget in the toolkit follows the same
// four-line pattern.
type Button struct {
	Bounds  Rect
	Label   string
	Pressed bool
}

// Draw paints the button. A pressed button swaps Surface + Accent to
// visualise the state.
func (b *Button) Draw(p Painter, theme *Theme) {
	fill := theme.Surface
	ink := theme.OnSurface
	if b.Pressed {
		fill = theme.Accent
		ink = theme.Surface
	}
	p.FillRect(b.Bounds, fill)
	p.StrokeRect(b.Bounds, theme.Border, 1)
	// Centre the label on the button's middle row. (H-1)/2 is the centre
	// row in cell mode (e.g. row 1 of a 3-tall button, not the bottom
	// border at Y+2) and an acceptable vertical centre in pixel mode; a
	// single expression keeps the backend-agnostic "4-line widget" shape.
	p.Text(b.Bounds.X+2, b.Bounds.Y+(b.Bounds.H-1)/2, b.Label, ink)
}

// Label is a static text widget. No fill, no border, no state.
type Label struct {
	Bounds Rect
	Text   string
}

// Draw paints the label ink on the surface's own background — the
// host is expected to have filled the parent Bounds first.
func (l *Label) Draw(p Painter, theme *Theme) {
	p.Text(l.Bounds.X, l.Bounds.Y, l.Text, theme.OnSurface)
}

// ProgressBar visualises a 0.0..1.0 value as a filled ratio of its
// bounds. Values outside the range clamp.
type ProgressBar struct {
	Bounds Rect
	Value  float64
}

// Draw paints the empty track + a filled portion sized to Value.
func (b *ProgressBar) Draw(p Painter, theme *Theme) {
	v := b.Value
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.FillRect(b.Bounds, theme.Surface)
	p.StrokeRect(b.Bounds, theme.Border, 1)
	fill := Rect{
		X: b.Bounds.X,
		Y: b.Bounds.Y,
		W: int(float64(b.Bounds.W) * v),
		H: b.Bounds.H,
	}
	if fill.W > 0 {
		p.FillRect(fill, theme.Accent)
	}
}
