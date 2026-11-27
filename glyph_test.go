// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

func TestGlyphMetrics(t *testing.T) {
	if GlyphWidth != 5 || GlyphHeight != 7 || GlyphAdvance != 6 {
		t.Errorf("metrics %d %d %d", GlyphWidth, GlyphHeight, GlyphAdvance)
	}
}

func TestGlyph(t *testing.T) {
	cols, ok := Glyph('-')
	if !ok {
		t.Fatal("'-' is in the font")
	}
	want := [GlyphWidth]byte{0x08, 0x08, 0x08, 0x08, 0x08}
	if cols != want {
		t.Errorf("'-' = %v, want %v", cols, want)
	}
	if cols, ok := Glyph(0); ok || cols != ([GlyphWidth]byte{}) {
		t.Errorf("NUL = %v, %v; want empty and not ok", cols, ok)
	}
	if _, ok := Glyph('a'); ok {
		t.Error("lower case is not in the font")
	}
	if _, ok := Glyph(0xff); ok {
		t.Error("0xff is not in the font")
	}
}
