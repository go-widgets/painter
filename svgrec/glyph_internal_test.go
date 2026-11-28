// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-widgets/painter"
)

func square(n float64) func() *painter.Path {
	return func() *painter.Path {
		return painter.NewPath().MoveTo(0, 0).LineTo(n, 0).LineTo(n, n).Close()
	}
}

// Two different outlines whose hashes collide get two different symbols: the
// second takes a longer prefix instead of drawing with the first one's shape.
// When the longest prefix is taken too, the call is refused, not mis-drawn.
func TestAHashCollisionNeverDrawsTheWrongGlyph(t *testing.T) {
	old := symbolHash
	symbolHash = func([]byte) string { return strings.Repeat("ab", 32) }
	defer func() { symbolHash = old }()

	r, _ := New(100, 100)
	r.TextGlyphs("ab", []Glyph{{Key: "a", Outline: square(3)}, {Key: "b", X: 5, Outline: square(4)}}, painter.RGB(0, 0, 0))
	b, err := r.SVG()
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Count(s, "<symbol") != 2 || !strings.Contains(s, `<symbol id="gababababab"`) || !strings.Contains(s, `<symbol id="gabababababab"`) {
		t.Fatalf("collision not resolved\n%s", s)
	}
	// The same outline under the same hash is recognised, not defined again.
	r.TextGlyphs("a", []Glyph{{Key: "again", Outline: square(3)}}, painter.RGB(0, 0, 0))
	if b, _ := r.SVG(); strings.Count(string(b), "<symbol") != 2 {
		t.Fatal("an identical outline was defined twice")
	}
	// Prefixes 10, 12, ..., 32 are twelve ids; a thirteenth outline cannot be named.
	r, _ = New(100, 100)
	for i := 1; i <= 13; i++ {
		r.TextGlyphs("a", []Glyph{{Key: strings.Repeat("k", i), Outline: square(float64(i))}}, painter.RGB(0, 0, 0))
	}
	if _, err := r.SVG(); !errors.Is(err, ErrTooLarge) || r.Stats().Dropped != 1 {
		t.Fatalf("err %v dropped %d", err, r.Stats().Dropped)
	}
}

// A glyph with nothing to paint, among glyphs that paint, writes no <use>: an
// empty id would be an href to "#".
func TestAGlyphWithNothingToPaintIsSkippedInARun(t *testing.T) {
	r, _ := New(100, 100)
	r.TextGlyphs("abc", []Glyph{{Key: "a", Outline: square(3)}, {Key: "none"}, {Key: "nil", Outline: func() *painter.Path { return nil }}, {Key: "b", X: 4, Outline: square(3)}}, painter.RGB(0, 0, 0))
	b, err := r.SVG()
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); strings.Count(s, "<use ") != 2 || strings.Contains(s, `href="#"`) {
		t.Fatalf("%s", s)
	}
}

// The symbol itself is held to both limits, at the byte: what it refuses is not
// left half written in <defs>.
func TestASymbolIsHeldToTheLimits(t *testing.T) {
	r0, _ := New(100, 100)
	r0.TextGlyphs("a", []Glyph{{Key: "a", Outline: square(3)}}, painter.RGB(0, 0, 0))
	doc, err := r0.SVG()
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	el := len(s[strings.Index(s, "<symbol") : strings.Index(s, "</symbol>\n")+len("</symbol>\n")])
	run := []Glyph{{Key: "a", Outline: square(3)}}
	for name, tc := range map[string]struct {
		l    Limits
		want bool // the symbol is accepted
	}{
		"call at the bound": {Limits{MaxCallBytes: el}, true},
		"call one over":     {Limits{MaxCallBytes: el - 1}, false},
		"total one over":    {Limits{MaxTotalBytes: el - 1}, false},
	} {
		r, _ := New(100, 100)
		r.Limits = tc.l
		r.TextGlyphs("a", run, painter.RGB(0, 0, 0))
		if got := len(r.defs) > 0; got != tc.want {
			t.Errorf("%s: symbol written %v, want %v (err %v)", name, got, tc.want, r.Err())
		}
		if !tc.want && !errors.Is(r.Err(), ErrTooLarge) {
			t.Errorf("%s: err %v", name, r.Err())
		}
	}
}
