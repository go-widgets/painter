// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "github.com/go-gfx/gfx/vector"

// The 2-D outline model and the anti-aliased scanline rasterizer that used to
// live here now live in the shared foundation, go-gfx/gfx/vector, so painter,
// go-webengine and any other renderer share ONE exact rasterizer. The types are
// re-exported as aliases, so a consumer keeps writing painter.NewPath(),
// painter.NonZero, *painter.Path exactly as before — the identifiers are the same
// values, and *painter.Path IS *vector.Path. The pixel-buffer composite (blend
// under this painter's clip) stays here; see pathpaint.go.

// FillRule selects how a path's winding count decides which side of the outline
// is filled: NonZero (non-zero winding) or EvenOdd. It is an alias of
// vector.FillRule.
type FillRule = vector.FillRule

const (
	// NonZero fills a point when the signed edge-crossing count around it is
	// non-zero — the intuitive rule for most icon shapes.
	NonZero = vector.NonZero
	// EvenOdd fills a point when the crossing count is odd, so a shape drawn over
	// itself punches a hole.
	EvenOdd = vector.EvenOdd
)

// Path is a mutable 2-D outline built from move / line / quadratic / cubic /
// close commands. It is an alias of vector.Path, so the builder methods and any
// path a consumer holds are unchanged by the extraction.
type Path = vector.Path

// NewPath returns an empty Path ready to build.
func NewPath() *Path { return vector.NewPath() }

// PathPainter is an optional Painter capability: a back-end that can rasterise
// arbitrary 2-D outlines implements it. Like Clipper and FacePainter it is
// type-asserted, so the base Painter interface stays a fixed-primitive set and a
// back-end that cannot rasterise paths (a cell grid) simply does not implement
// it:
//
//	if pp, ok := p.(painter.PathPainter); ok {
//		pp.FillPath(icon, ink, painter.NonZero)
//	} else {
//		// coarse fallback in the base primitives, or skip the vector part
//	}
//
// PixelPainter implements PathPainter with the go-gfx/gfx/vector anti-aliased
// scanline coverage rasterizer, compositing the coverage through its own pixel
// write so the active clip is honoured. CellPainter and any FacePainter-only
// back-end deliberately do NOT: a terminal cell is atomic and cannot carry
// sub-cell vector coverage, so the capability is reported absent and the consumer
// falls back.
type PathPainter interface {
	// FillPath fills pth with colour c under the given winding rule. Curves are
	// flattened; corner and edge pixels get fractional coverage (anti-aliased),
	// composited through the painter's own pixel write so the active clip is
	// honoured. An empty path, or one enclosing no area, paints nothing.
	FillPath(pth *Path, c RGBA, rule FillRule)

	// StrokePath paints pth's outline with colour c, centred on the path and
	// width units wide, with round joins and caps. A closed sub-path strokes its
	// closing segment too. width <= 0, a nil/empty path, or an isolated point
	// paint nothing.
	StrokePath(pth *Path, c RGBA, width float64)
}
