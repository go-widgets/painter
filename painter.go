// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package painter is a prototype of the "Painter" abstraction that
// lets a single widget's Draw method target three deployment
// families:
//
//   - WUI (browser wasm + <canvas> + putImageData) → PixelPainter
//   - GUI (native window; SDL2, Ebitengine, image/png export…) → PixelPainter
//   - TUI (terminal cell grid + ANSI escape codes) → CellPainter
//
// The current go-widgets/toolkit widgets are hard-bound to a
// []byte + surfaceW pair — great for pixel back-ends, incompatible
// with a cell grid. This repo prototypes a redesign around a
// primitive-set interface:
//
//	type Painter interface {
//	    FillRect(r Rect, c RGBA)
//	    StrokeRect(r Rect, c RGBA, lineW int)
//	    Text(x, y int, s string, ink RGBA)
//	    PutPixel(x, y int, c RGBA)
//	}
//
// A widget's Draw becomes:
//
//	func (b *Button) Draw(p Painter, theme *Theme) {
//	    r := b.Bounds
//	    p.FillRect(r, theme.Surface)
//	    p.StrokeRect(r, theme.Border, 1)
//	    p.Text(r.X+8, r.Y+8, b.Label, theme.OnSurface)
//	}
//
// The same widget code renders identically in a browser canvas
// (PixelPainter), a native window (PixelPainter again — the host
// consumes the buffer differently), a terminal (CellPainter maps
// RGBA to ANSI 16-colour, snaps rects to cells + uses box-draw
// glyphs for strokes), or an SVG snapshot (not shipped in this
// prototype — see go-widgets/svg).
//
// Status: PROTOTYPE. The API surface is deliberately small (5
// primitives). Once validated the full toolkit widget set migrates
// + the prototype folds into go-widgets/toolkit as its v1.0
// rendering path.
package painter

// Rect is a rectangle in the painter's coordinate system. Same
// shape as toolkit.Rect so future migration is a simple type-alias.
type Rect struct{ X, Y, W, H int }

// RGBA is a 32-bit colour value. Painters that can't represent a
// given RGBA (a cell grid limited to 16 colours, for instance)
// pick the closest supported value.
type RGBA struct{ R, G, B, A uint8 }

// RGB constructs an opaque colour with A=0xFF.
func RGB(r, g, b uint8) RGBA { return RGBA{r, g, b, 0xFF} }

// Painter is the primitive-set every back-end implements. A widget
// composes only these calls; the back-end decides how they land on
// the actual output.
//
// Coordinates are in the painter's OWN unit — pixel for PixelPainter,
// cell for CellPainter. The widget doesn't need to know which; the
// host sets the widget's Bounds in the right units before Draw is
// called.
type Painter interface {
	// FillRect paints a solid rectangle.
	FillRect(r Rect, c RGBA)

	// StrokeRect paints a 1-line-wide border around r (no fill).
	// lineW is a hint; back-ends that can't do variable strokes
	// (a cell grid, for instance) ignore it.
	StrokeRect(r Rect, c RGBA, lineW int)

	// PutPixel paints a single pixel at (x, y). On a CellPainter
	// this promotes to a filled cell.
	PutPixel(x, y int, c RGBA)

	// Text paints ink text starting at (x, y). Font metrics come
	// from the painter's own bitmap (PixelPainter's 5×7 font) or
	// the terminal's own font (CellPainter — 1 cell per rune).
	Text(x, y int, s string, ink RGBA)

	// Size returns the painter's canvas dimensions in painter units
	// (pixels for PixelPainter, cells for CellPainter). A widget
	// that wants to fill the whole surface reads this instead of
	// hard-coding a size.
	Size() (w, h int)
}
