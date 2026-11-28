// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

// boxGlyph is a 6x10 box and a triangle, glyph-origin relative, the way a font
// outline is: it reaches left of and above its origin.
func boxGlyph(w float64) func() *painter.Path {
	return func() *painter.Path {
		return painter.NewPath().MoveTo(-1, -9).LineTo(w, -9).LineTo(w, 1).LineTo(-1, 1).Close().
			MoveTo(1.25, -3.5).QuadTo(2.5, -7, 3.75, -3.5).Close()
	}
}

func rec(t *testing.T) *svgrec.Recorder {
	t.Helper()
	r, err := svgrec.New(300, 100)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func doc(t *testing.T, r *svgrec.Recorder) []byte {
	t.Helper()
	b, err := r.SVG()
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	if _, perr := parse(b); perr != nil {
		t.Fatalf("not a clean SVG: %v\n%s", perr, b)
	}
	return b
}

// A glyph drawn many times is defined once; a repeat costs a <use>.
func TestTextGlyphsDefinesEachDistinctGlyphOnce(t *testing.T) {
	r := rec(t)
	run := []svgrec.Glyph{
		{Key: "a", X: 10, Y: 20, Outline: boxGlyph(6)},
		{Key: "b", X: 17.5, Y: 20, Outline: boxGlyph(5)},
		{Key: "a", X: 25, Y: 20, Outline: boxGlyph(6)},
		{Key: "a", X: 32.125, Y: 20, Outline: boxGlyph(6)},
	}
	r.TextGlyphs("abaa", run, painter.RGB(1, 2, 3))
	b := doc(t, r)
	p, _ := parse(b)
	if p.elements["symbol"] != 2 || p.elements["use"] != 4 || p.elements["defs"] != 1 {
		t.Fatalf("symbols %d uses %d defs %d, want 2, 4, 1\n%s", p.elements["symbol"], p.elements["use"], p.elements["defs"], b)
	}
	if len(p.titles) != 1 || p.titles[0] != "abaa" {
		t.Fatalf("titles %q", p.titles)
	}
	if !bytes.Contains(b, []byte(`fill="#010203"`)) || bytes.Contains(b, []byte(`<symbol id="`+"g")) == false {
		t.Fatalf("missing the ink on the run or the symbols\n%s", b)
	}
	if strings.Count(string(b), "fill=") != 1 {
		t.Fatalf("the ink must be set once, on the run\n%s", b)
	}
}

// The same outline gets the same symbol, byte for byte, in another recorder and
// under another key and another ink: that is what an assembler shares.
func TestGlyphSymbolsAreContentAddressed(t *testing.T) {
	sym := func(key string, ink painter.RGBA, x float64) string {
		r := rec(t)
		r.TextGlyphs("a", []svgrec.Glyph{{Key: key, X: x, Y: 5, Outline: boxGlyph(6)}}, ink)
		b := doc(t, r)
		i := bytes.Index(b, []byte("<symbol"))
		j := bytes.Index(b, []byte("</symbol>"))
		if i < 0 || j < i {
			t.Fatalf("no symbol\n%s", b)
		}
		return string(b[i:j])
	}
	if a, b := sym("k1", painter.RGB(1, 2, 3), 1), sym("other", painter.RGB(200, 0, 9), 99.5); a != b {
		t.Fatalf("symbols differ:\n%s\n%s", a, b)
	}
	r := rec(t)
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "x", Outline: boxGlyph(6)}, {Key: "y", X: 9, Outline: boxGlyph(6)}}, painter.RGB(1, 2, 3))
	if p, _ := parse(doc(t, r)); p.elements["symbol"] != 1 {
		t.Fatal("two keys with one outline wrote two symbols")
	}
}

// Placement: x, y, the translation in force, and a translucent ink.
func TestTextGlyphsPlacement(t *testing.T) {
	r := rec(t)
	r.PushTranslate(100, 40)
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "a", X: 0, Y: 0, Outline: boxGlyph(6)}, {Key: "a", X: 2.5, Y: 7, Outline: boxGlyph(6)}}, painter.RGBA{R: 9, G: 9, B: 9, A: 128})
	r.PopTranslate()
	b := string(doc(t, r))
	for _, want := range []string{`x="100" y="40"/>`, `x="102.5" y="47"/>`, `fill-opacity="0.50196"`} {
		if !strings.Contains(b, want) {
			t.Errorf("missing %q in\n%s", want, b)
		}
	}
}

// A glyph with no outline (a space) writes nothing; a run of only such glyphs
// writes no element at all; a transparent ink or an empty run is a no-op.
func TestTextGlyphsDrawsNothingForNothing(t *testing.T) {
	r := rec(t)
	r.TextGlyphs(" ", []svgrec.Glyph{{Key: "sp"}, {Key: "e", Outline: func() *painter.Path { return nil }},
		{Key: "m", Outline: func() *painter.Path { return painter.NewPath().MoveTo(1, 1).Close() }}}, painter.RGB(0, 0, 0))
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}}, painter.RGBA{})
	r.TextGlyphs("", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}}, painter.RGB(0, 0, 0))
	r.TextGlyphs("a", nil, painter.RGB(0, 0, 0))
	if b := string(doc(t, r)); strings.Contains(b, "<g") || strings.Contains(b, "<defs") {
		t.Fatalf("drew something\n%s", b)
	}
	if r.Stats().Elements != 0 {
		t.Fatal("elements counted for nothing")
	}
}

// Outline is asked once per key, however often the key is drawn.
func TestOutlineIsCalledOncePerKey(t *testing.T) {
	r := rec(t)
	n := 0
	g := svgrec.Glyph{Key: "a", Outline: func() *painter.Path { n++; return boxGlyph(6)() }}
	r.TextGlyphs("aaa", []svgrec.Glyph{g, g, g}, painter.RGB(0, 0, 0))
	r.TextGlyphs("a", []svgrec.Glyph{g}, painter.RGB(0, 0, 0))
	if n != 1 {
		t.Fatalf("Outline called %d times", n)
	}
}

// The refusals Text has, plus a bad coordinate, a bad outline and the limits;
// a refused call writes nothing and the first error is kept.
func TestTextGlyphsRefusals(t *testing.T) {
	g := func(x float64) []svgrec.Glyph { return []svgrec.Glyph{{Key: "a", X: x, Outline: boxGlyph(6)}} }
	nan := math.NaN()
	for name, c := range map[string]struct {
		s    string
		g    []svgrec.Glyph
		want error
	}{
		"invalid utf8":  {"\xff", g(0), svgrec.ErrInvalidUTF8},
		"xml char":      {"a\x00", g(0), svgrec.ErrInvalidText},
		"nan x":         {"a", g(nan), svgrec.ErrRange},
		"huge x":        {"a", g(1 << 30), svgrec.ErrRange},
		"too long text": {strings.Repeat("a", 1<<16+1), g(0), svgrec.ErrTooLarge},
		"long key":      {"a", []svgrec.Glyph{{Key: strings.Repeat("k", 257), Outline: boxGlyph(6)}}, svgrec.ErrRange},
		"bad outline": {"a", []svgrec.Glyph{{Key: "n", Outline: func() *painter.Path {
			return painter.NewPath().MoveTo(0, 0).LineTo(nan, 1)
		}}}, svgrec.ErrRange},
	} {
		r := rec(t)
		r.TextGlyphs(c.s, c.g, painter.RGB(0, 0, 0))
		b, err := r.SVG()
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err %v, want %v", name, err, c.want)
		}
		if r.Stats().Dropped != 1 || bytes.Contains(b, []byte("<g")) || bytes.Contains(b, []byte("<use")) {
			t.Errorf("%s: a refused call left output\n%s", name, b)
		}
	}
	// A bad glyph in the middle of a run drops the whole run.
	r := rec(t)
	r.TextGlyphs("ab", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}, {Key: "b", X: nan, Outline: boxGlyph(5)}}, painter.RGB(0, 0, 0))
	if _, err := r.SVG(); !errors.Is(err, svgrec.ErrRange) {
		t.Fatal(err)
	}
}

// The limits count the symbols: a document whose symbols alone exceed the total
// is refused, and so is one symbol larger than a call may be.
func TestGlyphSymbolsRespectLimits(t *testing.T) {
	r := rec(t)
	r.Limits = svgrec.Limits{MaxCallBytes: 60}
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}}, painter.RGB(0, 0, 0))
	if _, err := r.SVG(); !errors.Is(err, svgrec.ErrTooLarge) {
		t.Fatalf("call limit: %v", err)
	}
	r = rec(t)
	r.Limits = svgrec.Limits{MaxTotalBytes: 150}
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}}, painter.RGB(0, 0, 0))
	if _, err := r.SVG(); !errors.Is(err, svgrec.ErrTooLarge) {
		t.Fatalf("total limit: %v", err)
	}
	// Bytes counts the symbols too.
	r = rec(t)
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}}, painter.RGB(0, 0, 0))
	b := doc(t, r)
	if got := r.Stats().Bytes; got < 100 || got >= len(b) {
		t.Fatalf("Stats.Bytes %d for a %d byte document", got, len(b))
	}
}

// Reset forgets the symbols: the next document defines them again.
func TestResetForgetsGlyphs(t *testing.T) {
	r := rec(t)
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}}, painter.RGB(0, 0, 0))
	first := doc(t, r)
	r.Reset()
	if b := doc(t, r); bytes.Contains(b, []byte("<symbol")) {
		t.Fatalf("Reset kept a symbol\n%s", b)
	}
	r.TextGlyphs("a", []svgrec.Glyph{{Key: "a", Outline: boxGlyph(6)}}, painter.RGB(0, 0, 0))
	if !bytes.Equal(first, doc(t, r)) {
		t.Fatal("the same call after Reset gave other bytes")
	}
}

// Determinism: the same calls, the same bytes.
func TestTextGlyphsIsDeterministic(t *testing.T) {
	run := func() []byte {
		r := rec(t)
		for i := range 20 {
			r.TextGlyphs("ab", []svgrec.Glyph{{Key: "a", X: float64(i), Y: 3, Outline: boxGlyph(6)}, {Key: "b", X: float64(i) + 7.3, Y: 3, Outline: boxGlyph(5)}}, painter.RGB(1, 2, 3))
		}
		return doc(t, r)
	}
	if !bytes.Equal(run(), run()) {
		t.Fatal("two runs differ")
	}
}

// The numeric equivalence: <use x y> of a symbol draws the outline FillPath
// writes at that position, to the three decimals the format keeps. Each is
// parsed back from the document and compared coordinate by coordinate.
func TestSymbolUseEqualsTheDirectPath(t *testing.T) {
	spots := [][2]float64{{0, 0}, {10.5, 20.25}, {299.999, 99.001}, {-3.3333, 7.7777}, {123.4567, 8.9}}
	r := rec(t)
	var run []svgrec.Glyph
	for _, s := range spots {
		run = append(run, svgrec.Glyph{Key: "a", X: s[0], Y: s[1], Outline: boxGlyph(6)})
	}
	r.TextGlyphs("a", run, painter.RGB(1, 2, 3))
	got := expandUses(t, doc(t, r))
	if len(got) != len(spots) {
		t.Fatalf("%d uses", len(got))
	}
	for i, s := range spots {
		d := rec(t)
		// the direct path: the outline moved by (x, y) in float, then written
		d.FillPath(translate(s[0], s[1]), painter.RGB(1, 2, 3), painter.NonZero)
		want := pathNumbers(t, doc(t, d))
		if len(want) != len(got[i]) {
			t.Fatalf("spot %d: %d numbers, want %d", i, len(got[i]), len(want))
		}
		for k := range want {
			if math.Abs(want[k]-got[i][k]) > 0.0011 {
				t.Fatalf("spot %d number %d: %v vs %v", i, k, got[i][k], want[k])
			}
		}
	}
}

// translate is boxGlyph(6) moved by (dx, dy) before it is written.
func translate(dx, dy float64) *painter.Path {
	return painter.NewPath().MoveTo(-1+dx, -9+dy).LineTo(6+dx, -9+dy).LineTo(6+dx, 1+dy).LineTo(-1+dx, 1+dy).Close().
		MoveTo(1.25+dx, -3.5+dy).QuadTo(2.5+dx, -7+dy, 3.75+dx, -3.5+dy).Close()
}

// pathNumbers returns every number of the first <path d> of a document.
func pathNumbers(t *testing.T, b []byte) []float64 {
	t.Helper()
	i := bytes.Index(b, []byte(` d="`))
	if i < 0 {
		t.Fatalf("no path\n%s", b)
	}
	rest := b[i+4:]
	return numbers(t, string(rest[:bytes.IndexByte(rest, '"')]))
}

func numbers(t *testing.T, d string) []float64 {
	t.Helper()
	var out []float64
	for _, f := range strings.FieldsFunc(d, func(r rune) bool { return r == ' ' || strings.ContainsRune("MLQCZ", r) }) {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			t.Fatalf("number %q: %v", f, err)
		}
		out = append(out, v)
	}
	return out
}

// expandUses returns, per <use>, the symbol's path numbers moved by its x, y.
func expandUses(t *testing.T, b []byte) [][]float64 {
	t.Helper()
	syms := map[string][]float64{}
	var out [][]float64
	d := xml.NewDecoder(bytes.NewReader(b))
	var cur string
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		attr := func(n string) string {
			for _, a := range se.Attr {
				if a.Name.Local == n {
					return a.Value
				}
			}
			return ""
		}
		switch se.Name.Local {
		case "symbol":
			cur = attr("id")
		case "path":
			if cur != "" {
				syms[cur] = numbers(t, attr("d"))
				cur = ""
			}
		case "use":
			x, _ := strconv.ParseFloat(attr("x"), 64)
			y, _ := strconv.ParseFloat(attr("y"), 64)
			base := syms[strings.TrimPrefix(attr("href"), "#")]
			if base == nil {
				t.Fatalf("use of an undefined symbol %q", attr("href"))
			}
			n := make([]float64, len(base))
			for i, v := range base {
				if i%2 == 0 {
					n[i] = v + x
				} else {
					n[i] = v + y
				}
			}
			out = append(out, n)
		}
	}
}

// FuzzTextGlyphs: whatever the text, key and coordinates, the output is clean,
// inside the vocabulary, every <use> resolves to a defined symbol, the title is
// the text, and a refusal is one of the named errors.
func FuzzTextGlyphs(f *testing.F) {
	f.Add("HELLO", "a", 1.5, 2.5, 6.0)
	f.Add("A&B<C>\"D'", "k", -3.0, 1e9, 1.0)
	f.Add("</title><script>", "\"/><x", 0.0, 0.0, 1e300)
	f.Add("\xff", "", math.NaN(), math.Inf(1), math.NaN())
	f.Fuzz(func(t *testing.T, s, key string, x, y, w float64) {
		r, err := svgrec.New(200, 100)
		if err != nil {
			t.Fatal(err)
		}
		r.PushTranslate(3, 4)
		r.TextGlyphs(s, []svgrec.Glyph{
			{Key: key, X: x, Y: y, Outline: func() *painter.Path {
				return painter.NewPath().MoveTo(0, 0).LineTo(w, 0).LineTo(w, w).Close()
			}},
			{Key: key + "2", X: y, Y: x, Outline: boxGlyph(6)},
		}, painter.RGB(1, 2, 3))
		b, serr := r.SVG()
		p, perr := parse(b)
		if perr != nil {
			t.Fatalf("output is not a clean SVG: %v\n%s", perr, b)
		}
		if serr != nil {
			if !errors.Is(serr, svgrec.ErrInvalidUTF8) && !errors.Is(serr, svgrec.ErrInvalidText) && !errors.Is(serr, svgrec.ErrRange) && !errors.Is(serr, svgrec.ErrTooLarge) {
				t.Fatalf("unexpected error %v", serr)
			}
			if p.elements["use"] != 0 || len(p.titles) != 0 {
				t.Fatalf("a refused call left output\n%s", b)
			}
			return
		}
		if len(p.titles) == 1 && p.titles[0] != s {
			t.Fatalf("title %q != text %q", p.titles[0], s)
		}
		if !utf8.ValidString(s) {
			t.Fatal("invalid text accepted")
		}
		// every use resolves
		defined := map[string]bool{}
		dec := xml.NewDecoder(bytes.NewReader(b))
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			if se, ok := tok.(xml.StartElement); ok {
				for _, a := range se.Attr {
					if se.Name.Local == "symbol" && a.Name.Local == "id" {
						defined[a.Value] = true
					}
					if se.Name.Local == "use" && a.Name.Local == "href" && !defined[strings.TrimPrefix(a.Value, "#")] {
						t.Fatalf("use of %q before or without its symbol\n%s", a.Value, b)
					}
				}
			}
		}
	})
}
