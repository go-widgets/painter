// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// Face is a resolved font face a run of text is drawn with. The base Text
// primitive carries only a string + ink, so a non-pixel painter (a vector or
// recording back-end) renders it in the painter's OWN fallback font. When the
// text is actually set in a TrueType/OpenType face — a widget running under
// go-widgets/toolkit's NewTrueTypeFont, say — that fallback loses both the real
// glyph shapes and the real advances, so the run mis-sizes.
//
// Face is the bridge: it exposes just enough of the face for a vector back-end
// to embed the true font and place the text at the true size, WITHOUT the
// painter package taking on a font-engine dependency (it stays stdlib-only).
// The font's own layout (per-glyph advances, kerning) is reproduced by the
// consumer re-reading these same sfnt bytes, so the vector text lines up with
// the on-screen raster the widget laid itself out against.
//
// A Face is handed to a FacePainter (see below); a painter that cannot use one
// never sees it.
type Face interface {
	// FontData returns the original TrueType/OpenType sfnt bytes of the face.
	// A consumer embeds or subsets these to render real, selectable text. The
	// slice is read-only; callers must not mutate it.
	FontData() []byte

	// SizePx is the face's em size in painter units (pixels) — the size the
	// widget laid its text out at, which the consumer reproduces so the run
	// occupies the same box.
	SizePx() int

	// Ascent is the baseline offset from the text's top edge, in painter units.
	// The Text/TextFace convention places (x, y) at the run's TOP-LEFT corner,
	// so a baseline-origin back-end (PDF, PostScript) drops the pen by Ascent.
	Ascent() int
}

// FacePainter is an optional Painter capability: a back-end that can render real
// text in a specific font face implements it. It is the shaped-text seam a
// proportional/TrueType font uses so a vector or recording painter emits genuine
// selectable text (a PDF text-show operator, an SVG <text>, …) in the true face
// rather than the painter's built-in fallback font.
//
// A font that owns a Face type-asserts, exactly like Clipper:
//
//	if fp, ok := p.(painter.FacePainter); ok {
//		fp.TextFace(x, y, s, face, ink)
//		return
//	}
//	p.Text(x, y, s, ink) // fallback: painter's own font
//
// PixelPainter deliberately does NOT implement FacePainter: a raster font
// scan-converts its own glyph coverage and blits through PutPixel, so it never
// needs the seam. Only non-pixel back-ends (which cannot rasterise glyph masks)
// consume it.
type FacePainter interface {
	// TextFace draws s in face with (x, y) the run's top-left corner and ink the
	// fill colour. Coordinates are painter units (pixels). Text laid out in
	// visual order is passed as-is; the back-end maps each rune through the
	// face's own cmap.
	TextFace(x, y int, s string, face Face, ink RGBA)
}
