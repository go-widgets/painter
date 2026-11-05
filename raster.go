// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// RasterPainter is an optional Painter capability: the surface is a grid of
// pixels, and PutPixel lays down exactly the pixel it names, blended exactly as
// PixelPainter blends it.
//
// Code that composes something pixel by pixel asks for this rather than for
// *PixelPainter: a bitmap font that lights one block per glyph bit, a shear
// applied to a rendered run. Asking for the concrete type sent every OTHER
// painter down the fallback, a call to Text in the painter's own font, and so
// a painter that produces PixelPainter's pixels some other way, on a GPU for
// instance, drew different text from the same widgets. A CellPainter or a
// vector painter, whose PutPixel means something coarser, does not implement
// it, and keeps the fallback.
type RasterPainter interface {
	Painter
	// Raster marks the capability. It draws nothing.
	Raster()
}

// Raster marks PixelPainter as a RasterPainter.
func (p *PixelPainter) Raster() {}
