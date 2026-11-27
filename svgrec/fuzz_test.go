// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

// FuzzText feeds arbitrary strings to Text. Whatever the bytes, the output
// must parse back with encoding/xml, contain only the writer's own vocabulary
// (no injected element or attribute), and, when the text was accepted, carry
// it back unchanged in its <title>. A refused text must have been invalid.
func FuzzText(f *testing.F) {
	for _, s := range []string{
		"", "HELLO", "A&B<C>\"D'", "</title><script>alert(1)</script>",
		"]]><svg onload=x>", "<!-- c -->", "<?pi?>", "\" onload=\"x", "\xff\xfe",
		"\x00", "￾", "tab\tnl\ncr\r", "‮", "日本語",
	} {
		f.Add(s, 0, 0, 1)
	}
	f.Fuzz(func(t *testing.T, s string, x, y, scale int) {
		r, err := svgrec.New(200, 100)
		if err != nil {
			t.Fatal(err)
		}
		r.SetTextScale(scale)
		r.Text(x, y, s, painter.RGB(1, 2, 3))
		doc, serr := r.SVG()
		p, perr := parse(doc)
		if perr != nil {
			t.Fatalf("output is not a clean SVG: %v\n%s", perr, doc)
		}
		if serr != nil {
			if utf8.ValidString(s) && !errors.Is(serr, svgrec.ErrInvalidText) && !errors.Is(serr, svgrec.ErrRange) && !errors.Is(serr, svgrec.ErrTooLarge) {
				t.Fatalf("unexpected error %v for %q", serr, s)
			}
			if len(p.titles) != 0 {
				t.Fatalf("a refused text still produced a title: %q", p.titles)
			}
			return
		}
		if len(p.titles) > 1 {
			t.Fatalf("one call produced %d titles", len(p.titles))
		}
		// Text that draws nothing writes no element; otherwise the title is the
		// text exactly (the writer escapes CR as a character reference, so an
		// XML parser's line-end folding cannot alter it).
		if len(p.titles) == 1 && p.titles[0] != s {
			t.Fatalf("title %q != text %q", p.titles[0], s)
		}
	})
}

// FuzzCalls drives every numeric entry point with arbitrary values. The output
// must always be well formed and inside the vocabulary, and a refusal must be
// one of the named errors.
func FuzzCalls(f *testing.F) {
	f.Add(1, 2, 3, 4, 5, 255, 0, 0, 255, 1.5, 2.5)
	f.Add(-1, -2, -3, -4, 100, 1, 2, 3, 4, 0.0, -1.0)
	f.Add(1<<24, 1<<24+1, -(1 << 24), 1, 1, 0, 0, 0, 0, 1e300, 1e-300)
	f.Fuzz(func(t *testing.T, x, y, w, h, rad, cr, cg, cb, ca int, fx, fy float64) {
		r, err := svgrec.New(64, 64)
		if err != nil {
			t.Fatal(err)
		}
		c := painter.RGBA{R: uint8(cr), G: uint8(cg), B: uint8(cb), A: uint8(ca)}
		rc := painter.Rect{X: x, Y: y, W: w, H: h}
		r.PushClip(rc)
		r.PushTranslate(x, y)
		r.FillRect(rc, c)
		r.StrokeRect(rc, c, rad)
		r.FillRoundRect(rc, rad, c)
		r.StrokeRoundRect(rc, rad, c, rad/3)
		r.PutPixel(x, y, c)
		p := painter.NewPath().MoveTo(fx, fy).LineTo(fy, fx).QuadTo(fx, fx, fy, fy).CubicTo(fx, fy, fy, fx, fx, fx).Close()
		r.FillPath(p, c, painter.EvenOdd)
		r.StrokePath(p, c, fx)
		if w > 0 && h > 0 && w < 16 && h < 16 {
			r.DrawImage(rc, make([]byte, w*h*4), w, h)
			r.DrawMask(rc, make([]byte, w*h), w, c)
		}
		r.PopTranslate()
		r.PopClip()
		doc, serr := r.SVG()
		if _, perr := parse(doc); perr != nil {
			t.Fatalf("output is not a clean SVG: %v\n%s", perr, doc)
		}
		if serr != nil && !errors.Is(serr, svgrec.ErrRange) && !errors.Is(serr, svgrec.ErrTooLarge) {
			t.Fatalf("unexpected error %v", serr)
		}
	})
}
