// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

func newRec(t *testing.T, w, h int) *svgrec.Recorder {
	t.Helper()
	r, err := svgrec.New(w, h)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// body is the document without its <svg> wrapper: the elements, one per line.
func body(t *testing.T, r *svgrec.Recorder) string {
	t.Helper()
	b, err := r.SVG()
	if err != nil {
		t.Fatalf("SVG: %v", err)
	}
	s := string(b)
	i := strings.Index(s, ">\n")
	s = s[i+2:]
	return strings.TrimSuffix(s, "</svg>\n")
}

var black = painter.RGB(0, 0, 0)

func TestNewAndSize(t *testing.T) {
	for _, sz := range [][2]int{{0, 5}, {5, 0}, {-1, 5}, {5, -1}, {svgrec.MaxDim + 1, 5}, {5, svgrec.MaxDim + 1}} {
		if r, err := svgrec.New(sz[0], sz[1]); err == nil || r != nil {
			t.Errorf("New(%d,%d) = %v, %v; want an error", sz[0], sz[1], r, err)
		}
	}
	r := newRec(t, svgrec.MaxDim, 1)
	if w, h := r.Size(); w != svgrec.MaxDim || h != 1 {
		t.Errorf("Size = %d,%d", w, h)
	}
}

func TestDocumentWrapper(t *testing.T) {
	r := newRec(t, 40, 30)
	b, err := r.SVG()
	if err != nil {
		t.Fatal(err)
	}
	want := "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"40\" height=\"30\" viewBox=\"0 0 40 30\">\n</svg>\n"
	if string(b) != want {
		t.Errorf("got %q want %q", b, want)
	}
}

func TestFillRectAndPixel(t *testing.T) {
	r := newRec(t, 40, 30)
	r.FillRect(painter.Rect{X: 1, Y: 2, W: 3, H: 4}, painter.RGBA{R: 255, G: 16, B: 1, A: 128})
	r.FillRect(painter.Rect{X: 5, Y: 6, W: 7, H: 8}, painter.RGB(0, 0xAB, 0xCD))
	r.PutPixel(9, 10, painter.RGB(1, 2, 3))
	// all of these draw nothing
	r.FillRect(painter.Rect{X: 1, Y: 1, W: 0, H: 4}, black)
	r.FillRect(painter.Rect{X: 1, Y: 1, W: 4, H: 0}, black)
	r.FillRect(painter.Rect{X: 1, Y: 1, W: -4, H: 4}, black)
	r.FillRect(painter.Rect{X: 1, Y: 1, W: 4, H: 4}, painter.RGBA{R: 9})
	r.PutPixel(1, 1, painter.RGBA{})
	want := `<rect x="1" y="2" width="3" height="4" fill="#ff1001" fill-opacity="0.50196"/>
<rect x="5" y="6" width="7" height="8" fill="#00abcd"/>
<rect x="9" y="10" width="1" height="1" fill="#010203"/>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if st := r.Stats(); st.Elements != 3 || st.Raster != 0 || st.Dropped != 0 || st.Bytes != len(want) {
		t.Errorf("stats %+v", st)
	}
}

func TestFillRoundRect(t *testing.T) {
	r := newRec(t, 40, 30)
	r.FillRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 20}, 4, black)
	r.FillRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 10}, 99, black) // clamped to 5
	r.FillRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 10}, 0, black)  // square
	r.FillRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 10}, -3, black) // square
	r.FillRoundRect(painter.Rect{X: 0, Y: 0, W: 0, H: 10}, 3, black)
	r.FillRoundRect(painter.Rect{X: 0, Y: 0, W: 5, H: 10}, 3, painter.RGBA{})
	want := `<rect x="0" y="0" width="20" height="20" rx="4" fill="#000000"/>
<rect x="0" y="0" width="20" height="10" rx="5" fill="#000000"/>
<rect x="0" y="0" width="20" height="10" fill="#000000"/>
<rect x="0" y="0" width="20" height="10" fill="#000000"/>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestStrokeRect(t *testing.T) {
	r := newRec(t, 40, 30)
	r.StrokeRect(painter.Rect{X: 0, Y: 0, W: 10, H: 8}, black, 1)
	r.StrokeRect(painter.Rect{X: 0, Y: 0, W: 10, H: 8}, black, 0)  // lineW < 1 is 1
	r.StrokeRect(painter.Rect{X: 0, Y: 0, W: 10, H: 8}, black, -5) // likewise
	r.StrokeRect(painter.Rect{X: 2, Y: 2, W: 10, H: 8}, painter.RGBA{R: 255, A: 64}, 2)
	r.StrokeRect(painter.Rect{X: 0, Y: 0, W: 10, H: 8}, black, 4) // rings meet: filled
	r.StrokeRect(painter.Rect{X: 0, Y: 0, W: 10, H: 8}, black, 3) // hole 4x2
	r.StrokeRect(painter.Rect{X: 0, Y: 0, W: 0, H: 8}, black, 1)
	r.StrokeRect(painter.Rect{X: 0, Y: 0, W: 8, H: 8}, painter.RGBA{}, 1)
	want := `<path fill-rule="evenodd" fill="#000000" d="M0 0h10v8h-10zM1 1h8v6h-8z"/>
<path fill-rule="evenodd" fill="#000000" d="M0 0h10v8h-10zM1 1h8v6h-8z"/>
<path fill-rule="evenodd" fill="#000000" d="M0 0h10v8h-10zM1 1h8v6h-8z"/>
<path fill-rule="evenodd" fill="#ff0000" fill-opacity="0.25098" d="M2 2h10v8h-10zM4 4h6v4h-6z"/>
<rect x="0" y="0" width="10" height="8" fill="#000000"/>
<path fill-rule="evenodd" fill="#000000" d="M0 0h10v8h-10zM3 3h4v2h-4z"/>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestStrokeRoundRect(t *testing.T) {
	r := newRec(t, 40, 30)
	r.StrokeRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 20}, 4, black, 1)
	r.StrokeRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 20}, 4, black, 6)  // 12 >= 20? no: hole 8x8, inner radius 0
	r.StrokeRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 10}, 99, black, 5) // rings meet: filled, radius clamped
	r.StrokeRoundRect(painter.Rect{X: 0, Y: 0, W: 20, H: 20}, 0, black, 1)  // square
	want := `<path fill-rule="evenodd" fill="#000000" d="M4 0L16 0C18.209 0 20 1.791 20 4L20 16C20 18.209 18.209 20 16 20L4 20C1.791 20 0 18.209 0 16L0 4C0 1.791 1.791 0 4 0zM4 1L16 1C17.657 1 19 2.343 19 4L19 16C19 17.657 17.657 19 16 19L4 19C2.343 19 1 17.657 1 16L1 4C1 2.343 2.343 1 4 1z"/>
<path fill-rule="evenodd" fill="#000000" d="M4 0L16 0C18.209 0 20 1.791 20 4L20 16C20 18.209 18.209 20 16 20L4 20C1.791 20 0 18.209 0 16L0 4C0 1.791 1.791 0 4 0zM6 6h8v8h-8z"/>
<rect x="0" y="0" width="20" height="10" rx="5" fill="#000000"/>
<path fill-rule="evenodd" fill="#000000" d="M0 0h20v20h-20zM1 1h18v18h-18z"/>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestText(t *testing.T) {
	r := newRec(t, 40, 30)
	r.Text(1, 2, "I-", painter.RGB(1, 2, 3))
	want := `<g><title>I-</title><path fill="#010203" d="M2 2h3v1h-3zM3 3h1v1h-1zM3 4h1v1h-1zM3 5h1v1h-1zM3 6h1v1h-1zM3 7h1v1h-1zM2 8h3v1h-3zM7 5h5v1h-5z"/></g>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTextScale(t *testing.T) {
	r := newRec(t, 40, 30)
	if r.TextScale() != 1 || r.TextWidth("AB") != 12 || r.TextHeight() != 7 {
		t.Errorf("default metrics: %d %d %d", r.TextScale(), r.TextWidth("AB"), r.TextHeight())
	}
	r.SetTextScale(0)
	if r.TextScale() != 1 {
		t.Errorf("scale 0 = %d, want 1", r.TextScale())
	}
	r.SetTextScale(3)
	if r.TextScale() != 3 || r.TextWidth("AB") != 36 || r.TextHeight() != 21 {
		t.Errorf("scale 3 metrics: %d %d %d", r.TextScale(), r.TextWidth("AB"), r.TextHeight())
	}
	r.Text(0, 0, "-", black)
	want := `<g><title>-</title><path fill="#000000" d="M0 9h15v3h-15z"/></g>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTextSkipsUndrawnBytesButAdvances(t *testing.T) {
	r := newRec(t, 40, 30)
	r.Text(0, 0, "a-", black) // 'a' is not in the font; '-' sits at the second cell
	want := `<g><title>a-</title><path fill="#000000" d="M6 3h5v1h-5z"/></g>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	r.Reset()
	r.Text(0, 0, "abc xyz", black) // nothing drawable: no element at all
	if got := body(t, r); got != "" {
		t.Errorf("undrawable text wrote %q", got)
	}
}

func TestTextEscapes(t *testing.T) {
	r := newRec(t, 40, 30)
	r.Text(0, 0, "A&B<C>\"D'\r\n\t-", black)
	b, err := r.SVG()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "<title>A&amp;B&lt;C&gt;&#34;D&#39;&#13;\n\t-</title>") {
		t.Errorf("title not escaped: %s", b)
	}
	p, err := parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.titles) != 1 || p.titles[0] != "A&B<C>\"D'\r\n\t-" {
		t.Errorf("round trip lost the text: %q", p.titles)
	}
}

func TestTextRefusals(t *testing.T) {
	cases := []struct {
		name string
		s    string
		x    int
		want error
	}{
		{"invalid utf8", "A\xffB", 0, svgrec.ErrInvalidUTF8},
		{"truncated rune", "A\xe2\x82", 0, svgrec.ErrInvalidUTF8},
		{"NUL", "A\x00B", 0, svgrec.ErrInvalidText},
		{"escape", "A\x1bB", 0, svgrec.ErrInvalidText},
		{"U+FFFE", "A￾B", 0, svgrec.ErrInvalidText},
		{"U+FFFF", "A￿B", 0, svgrec.ErrInvalidText},
		{"too long", strings.Repeat("A", 1<<16+1), 0, svgrec.ErrTooLarge},
		{"x out of range", "A", svgrec.MaxCoord + 1, svgrec.ErrRange},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRec(t, 40, 30)
			r.Text(c.x, 0, c.s, black)
			if !errors.Is(r.Err(), c.want) {
				t.Errorf("Err = %v, want %v", r.Err(), c.want)
			}
			if st := r.Stats(); st.Dropped != 1 || st.Elements != 0 {
				t.Errorf("stats %+v", st)
			}
			if _, err := r.SVG(); !errors.Is(err, c.want) {
				t.Errorf("SVG error = %v, want %v", err, c.want)
			}
		})
	}
	// Characters at the edge of XML's Char production are carried.
	r := newRec(t, 40, 30)
	r.Text(0, 0, "A\u0085퟿�\U00010000\U0010ffff ", black)
	if r.Err() != nil {
		t.Errorf("valid XML characters refused: %v", r.Err())
	}
	if _, err := parse(mustSVG(t, r)); err != nil {
		t.Error(err)
	}
	// Silent no-ops.
	r = newRec(t, 40, 30)
	r.Text(0, 0, "", black)
	r.Text(0, 0, "A", painter.RGBA{})
	if r.Err() != nil || r.Stats().Dropped != 0 {
		t.Errorf("no-op text recorded an error: %v", r.Err())
	}
}

// nrgba returns a decoded PNG pixel as straight (non-premultiplied) 8-bit
// channels.
func nrgba(c color.Color) (r, g, b, a uint8) {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return n.R, n.G, n.B, n.A
}

func mustSVG(t *testing.T, r *svgrec.Recorder) []byte {
	t.Helper()
	b, err := r.SVG()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFirstErrorWins(t *testing.T) {
	r := newRec(t, 40, 30)
	r.Text(0, 0, "\xff", black)
	r.FillRect(painter.Rect{X: svgrec.MaxCoord + 1, W: 1, H: 1}, black)
	if !errors.Is(r.Err(), svgrec.ErrInvalidUTF8) || r.Stats().Dropped != 2 {
		t.Errorf("Err %v dropped %d", r.Err(), r.Stats().Dropped)
	}
}

func TestRangeRefusals(t *testing.T) {
	big := svgrec.MaxCoord + 1
	calls := map[string]func(r *svgrec.Recorder){
		"fill x":       func(r *svgrec.Recorder) { r.FillRect(painter.Rect{X: big, W: 1, H: 1}, black) },
		"fill y neg":   func(r *svgrec.Recorder) { r.FillRect(painter.Rect{Y: -big, W: 1, H: 1}, black) },
		"fill w":       func(r *svgrec.Recorder) { r.FillRect(painter.Rect{W: big, H: 1}, black) },
		"fill h":       func(r *svgrec.Recorder) { r.FillRect(painter.Rect{W: 1, H: big}, black) },
		"round":        func(r *svgrec.Recorder) { r.FillRoundRect(painter.Rect{X: big, W: 1, H: 1}, 1, black) },
		"stroke":       func(r *svgrec.Recorder) { r.StrokeRect(painter.Rect{X: big, W: 1, H: 1}, black, 1) },
		"stroke round": func(r *svgrec.Recorder) { r.StrokeRoundRect(painter.Rect{X: big, W: 1, H: 1}, 1, black, 1) },
		"pixel":        func(r *svgrec.Recorder) { r.PutPixel(big, 0, black) },
		"text y":       func(r *svgrec.Recorder) { r.Text(0, big, "A", black) },
		"path NaN": func(r *svgrec.Recorder) {
			r.FillPath(painter.NewPath().MoveTo(math.NaN(), 0).LineTo(1, 1), black, painter.NonZero)
		},
		"path Inf": func(r *svgrec.Recorder) {
			r.FillPath(painter.NewPath().MoveTo(0, math.Inf(1)).LineTo(1, 1), black, painter.NonZero)
		},
		"path big": func(r *svgrec.Recorder) {
			r.FillPath(painter.NewPath().MoveTo(0, 0).LineTo(float64(big), 1), black, painter.NonZero)
		},
		"path big neg": func(r *svgrec.Recorder) {
			r.FillPath(painter.NewPath().MoveTo(0, 0).LineTo(1, -float64(big)), black, painter.NonZero)
		},
		"stroke width": func(r *svgrec.Recorder) {
			r.StrokePath(painter.NewPath().MoveTo(0, 0).LineTo(1, 1), black, float64(big))
		},
		"stroke +Inf": func(r *svgrec.Recorder) {
			r.StrokePath(painter.NewPath().MoveTo(0, 0).LineTo(1, 1), black, math.Inf(1))
		},
		"image srcW":   func(r *svgrec.Recorder) { r.DrawImage(painter.Rect{W: 1, H: 1}, nil, big, 1) },
		"image dst":    func(r *svgrec.Recorder) { r.DrawImage(painter.Rect{X: big, W: 1, H: 1}, []byte{0, 0, 0, 0}, 1, 1) },
		"mask dst w":   func(r *svgrec.Recorder) { r.DrawMask(painter.Rect{W: big, H: 1}, nil, 1, black) },
		"mask dst x":   func(r *svgrec.Recorder) { r.DrawMask(painter.Rect{X: big, W: 1, H: 1}, []byte{1}, 1, black) },
		"translate dx": func(r *svgrec.Recorder) { r.PushTranslate(big, 0); r.PopTranslate() },
		"translate sum": func(r *svgrec.Recorder) {
			r.PushTranslate(svgrec.MaxCoord, 0)
			r.PushTranslate(1, 0)
			r.PopTranslate()
			r.PopTranslate()
		},
		"clip": func(r *svgrec.Recorder) { r.PushClip(painter.Rect{X: big, W: 1, H: 1}); r.PopClip() },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			r := newRec(t, 40, 30)
			call(r)
			if !errors.Is(r.Err(), svgrec.ErrRange) {
				t.Errorf("Err = %v, want ErrRange", r.Err())
			}
			if _, err := parse(mustSVGAllowErr(r)); err != nil {
				t.Error(err)
			}
		})
	}
}

func mustSVGAllowErr(r *svgrec.Recorder) []byte {
	b, _ := r.SVG()
	return b
}

func TestAtTheRangeEdge(t *testing.T) {
	r := newRec(t, 40, 30)
	r.FillRect(painter.Rect{X: svgrec.MaxCoord, Y: -svgrec.MaxCoord, W: svgrec.MaxCoord, H: svgrec.MaxCoord}, black)
	r.Text(svgrec.MaxCoord, -svgrec.MaxCoord, "-", black)
	r.PushTranslate(svgrec.MaxCoord, -svgrec.MaxCoord)
	r.PopTranslate()
	if r.Err() != nil {
		t.Errorf("values at exactly the bound were refused: %v", r.Err())
	}
}

func TestClip(t *testing.T) {
	r := newRec(t, 40, 30)
	r.PopClip() // no-op with none pushed
	r.PushClip(painter.Rect{X: 1, Y: 2, W: 3, H: 4})
	r.FillRect(painter.Rect{X: 0, Y: 0, W: 9, H: 9}, black)
	r.PushClip(painter.Rect{X: 5, Y: 6, W: -3, H: -4}) // negative size clips everything
	r.PopClip()
	r.PopClip()
	want := `<clipPath id="c0"><rect x="1" y="2" width="3" height="4"/></clipPath><g clip-path="url(#c0)">
<rect x="0" y="0" width="9" height="9" fill="#000000"/>
<clipPath id="c1"><rect x="5" y="6" width="0" height="0"/></clipPath><g clip-path="url(#c1)">
</g>
</g>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if st := r.Stats(); st.Elements != 1 {
		t.Errorf("a clip group is structure, not an element: %+v", st)
	}
}

func TestOpenClipsAreClosedInTheDocument(t *testing.T) {
	r := newRec(t, 40, 30)
	r.PushClip(painter.Rect{W: 1, H: 1})
	r.PushClip(painter.Rect{W: 1, H: 1})
	b1 := mustSVG(t, r)
	if _, err := parse(b1); err != nil {
		t.Fatalf("an unbalanced recording must still be well formed: %v", err)
	}
	if !bytes.HasSuffix(b1, []byte("</g>\n</g>\n</svg>\n")) {
		t.Errorf("open groups not closed: %s", b1)
	}
	// SVG does not consume the recording: it can go on.
	r.PopClip()
	r.PopClip()
	b2 := mustSVG(t, r)
	if _, err := parse(b2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Error("popping the groups SVG had already closed changed the document")
	}
	r.FillRect(painter.Rect{W: 1, H: 1}, black) // now outside any clip
	b3 := mustSVG(t, r)
	if !bytes.HasSuffix(b3, []byte("</g>\n</g>\n<rect x=\"0\" y=\"0\" width=\"1\" height=\"1\" fill=\"#000000\"/>\n</svg>\n")) {
		t.Errorf("an element after the pops is still inside a group: %s", b3)
	}
}

func TestTranslate(t *testing.T) {
	r := newRec(t, 40, 30)
	r.PopTranslate() // no-op with none pushed
	r.PushTranslate(10, 20)
	r.FillRect(painter.Rect{X: 1, Y: 2, W: 3, H: 4}, black)
	r.PushTranslate(-4, 1)
	r.FillRect(painter.Rect{X: 1, Y: 2, W: 3, H: 4}, black)
	r.PushClip(painter.Rect{X: 0, Y: 0, W: 2, H: 2}) // the clip is translated too
	r.PopClip()
	r.PopTranslate()
	r.FillRect(painter.Rect{X: 1, Y: 2, W: 3, H: 4}, black)
	r.PopTranslate()
	r.FillRect(painter.Rect{X: 1, Y: 2, W: 3, H: 4}, black)
	want := `<rect x="11" y="22" width="3" height="4" fill="#000000"/>
<rect x="7" y="23" width="3" height="4" fill="#000000"/>
<clipPath id="c0"><rect x="6" y="21" width="2" height="2"/></clipPath><g clip-path="url(#c0)">
</g>
<rect x="11" y="22" width="3" height="4" fill="#000000"/>
<rect x="1" y="2" width="3" height="4" fill="#000000"/>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPaths(t *testing.T) {
	r := newRec(t, 40, 30)
	p := painter.NewPath()
	p.MoveTo(1.5, 2).LineTo(3, 4.0004).QuadTo(1, 1, 2, 2).CubicTo(1, 2, 3, 4, 5, 6).Close()
	r.FillPath(p, painter.RGB(9, 9, 9), painter.NonZero)
	r.FillPath(p, painter.RGB(9, 9, 9), painter.EvenOdd)
	r.StrokePath(p, painter.RGBA{A: 1}, 2.5)
	r.PushTranslate(10, 20)
	r.FillPath(painter.NewPath().MoveTo(-0.0001, 0).LineTo(0.0004, 1).LineTo(1, 1), black, painter.NonZero)
	r.PopTranslate()
	// nothing to paint
	r.FillPath(nil, black, painter.NonZero)
	r.FillPath(p, painter.RGBA{}, painter.NonZero)
	r.FillPath(painter.NewPath(), black, painter.NonZero)
	r.FillPath(painter.NewPath().MoveTo(1, 1).Close(), black, painter.NonZero)
	r.StrokePath(nil, black, 1)
	r.StrokePath(p, painter.RGBA{}, 1)
	r.StrokePath(p, black, 0)
	r.StrokePath(p, black, -1)
	r.StrokePath(p, black, math.NaN())
	want := `<path fill="#090909" d="M1.5 2L3 4Q1 1 2 2C1 2 3 4 5 6Z"/>
<path fill-rule="evenodd" fill="#090909" d="M1.5 2L3 4Q1 1 2 2C1 2 3 4 5 6Z"/>
<path fill="none" stroke="#000000" stroke-opacity="0.00392" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" d="M1.5 2L3 4Q1 1 2 2C1 2 3 4 5 6Z"/>
<path fill="#000000" d="M10 20L10 21L11 21"/>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if r.Err() != nil {
		t.Errorf("no-op paths recorded %v", r.Err())
	}
}

func TestAppendFloatShapes(t *testing.T) {
	// Observed through a path: trailing zeros are trimmed, -0 is 0.
	r := newRec(t, 40, 30)
	p := painter.NewPath().MoveTo(2, -0.0004).LineTo(1.2, 100.5).LineTo(-3.0006, 0.1)
	r.FillPath(p, black, painter.NonZero)
	want := `<path fill="#000000" d="M2 0L1.2 100.5L-3.001 0.1"/>
`
	if got := body(t, r); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestImage(t *testing.T) {
	src := []byte{
		255, 0, 0, 255, 0, 255, 0, 128,
		0, 0, 255, 0, 1, 2, 3, 4,
	}
	r := newRec(t, 40, 30)
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 4, H: 4}, src, 2, 2)      // enlargement
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 2, H: 2}, src, 2, 2)      // 1:1
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 1, H: 2}, src, 2, 2)      // reduction on x
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 2, H: 1}, src, 2, 2)      // reduction on y
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 4, H: 4}, src[:15], 2, 2) // short src: ignored
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 0, H: 4}, src, 2, 2)
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 4, H: 0}, src, 2, 2)
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 4, H: 4}, src, 0, 2)
	r.DrawImage(painter.Rect{X: 3, Y: 4, W: 4, H: 4}, src, 2, 0)
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
	b := mustSVG(t, r)
	if _, err := parse(b); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(body(t, r)), "\n")
	if len(lines) != 4 {
		t.Fatalf("%d images, want 4:\n%s", len(lines), b)
	}
	for i, wantPix := range []bool{true, true, false, false} {
		has := strings.Contains(lines[i], ` image-rendering="pixelated"`)
		if has != wantPix {
			t.Errorf("image %d pixelated = %v, want %v", i, has, wantPix)
		}
	}
	if !strings.HasPrefix(lines[0], `<image x="3" y="4" width="4" height="4" preserveAspectRatio="none" image-rendering="pixelated" href="data:image/png;base64,`) {
		t.Errorf("image element: %s", lines[0])
	}
	// The PNG decodes back to the source pixels.
	const pre = `href="data:image/png;base64,`
	i := strings.Index(lines[0], pre) + len(pre)
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(lines[0][i:], `"/>`))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if bd := img.Bounds(); bd.Dx() != 2 || bd.Dy() != 2 {
		t.Fatalf("decoded %v", bd)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			r8, g8, b8, a8 := nrgba(img.At(x, y))
			o := (y*2 + x) * 4
			if r8 != src[o] || g8 != src[o+1] || b8 != src[o+2] || a8 != src[o+3] {
				t.Errorf("pixel %d,%d = %d %d %d %d, want %v", x, y, r8, g8, b8, a8, src[o:o+4])
			}
		}
	}
	if st := r.Stats(); st.Raster != 4 || st.Elements != 4 {
		t.Errorf("stats %+v", st)
	}
}

func TestMask(t *testing.T) {
	r := newRec(t, 40, 30)
	// 3x2 mask in a stride-4 buffer; the second row is cut short.
	r.DrawMask(painter.Rect{X: 1, Y: 1, W: 3, H: 2}, []byte{0, 128, 255, 9, 7, 7}, 4, painter.RGBA{R: 10, G: 20, B: 30, A: 200})
	if r.Err() != nil {
		t.Fatal(r.Err())
	}
	lines := strings.Split(strings.TrimSpace(body(t, r)), "\n")
	l := lines[0]
	if !strings.HasPrefix(l, `<image x="1" y="1" width="3" height="2" preserveAspectRatio="none" image-rendering="pixelated" href="data:image/png;base64,`) {
		t.Fatalf("mask element: %s", l)
	}
	const pre = `href="data:image/png;base64,`
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimSuffix(l[strings.Index(l, pre)+len(pre):], `"/>`))
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	for x, wantA := range []uint8{0, 100, 200} {
		r8, g8, b8, a8 := nrgba(img.At(x, 0))
		if wantA == 0 {
			if a8 != 0 {
				t.Errorf("pixel %d alpha %d", x, a8)
			}
			continue
		}
		if r8 != 10 || g8 != 20 || b8 != 30 || a8 != wantA {
			t.Errorf("pixel %d = %d %d %d %d, want 10 20 30 %d", x, r8, g8, b8, a8, wantA)
		}
	}
	for x := 0; x < 3; x++ { // the unreadable row is transparent
		if _, _, _, a8 := nrgba(img.At(x, 1)); a8 != 0 {
			t.Errorf("short row pixel %d alpha %d", x, a8)
		}
	}
	// refused or ignored
	r = newRec(t, 40, 30)
	r.DrawMask(painter.Rect{W: 0, H: 1}, []byte{1}, 1, black)
	r.DrawMask(painter.Rect{W: 1, H: 0}, []byte{1}, 1, black)
	r.DrawMask(painter.Rect{W: 1, H: 1}, []byte{1}, 0, black)
	r.DrawMask(painter.Rect{W: 1, H: 1}, []byte{1}, 1, painter.RGBA{})
	if r.Err() != nil || body(t, r) != "" {
		t.Errorf("no-op masks wrote %q / %v", body(t, r), r.Err())
	}
}

func TestLimits(t *testing.T) {
	t.Run("per call", func(t *testing.T) {
		r := newRec(t, 40, 30)
		r.Limits.MaxCallBytes = 60
		r.FillRect(painter.Rect{W: 1, H: 1}, black) // 59 bytes: fits
		r.FillRect(painter.Rect{W: 100, H: 100, X: 100, Y: 100}, black)
		if !errors.Is(r.Err(), svgrec.ErrTooLarge) || r.Stats().Elements != 1 || r.Stats().Dropped != 1 {
			t.Errorf("Err %v stats %+v", r.Err(), r.Stats())
		}
	})
	t.Run("per call exact", func(t *testing.T) {
		r := newRec(t, 40, 30)
		want := `<rect x="0" y="0" width="1" height="1" fill="#000000"/>` + "\n"
		r.Limits.MaxCallBytes = len(want)
		r.FillRect(painter.Rect{W: 1, H: 1}, black)
		if r.Err() != nil {
			t.Errorf("an element of exactly the limit was refused: %v", r.Err())
		}
	})
	t.Run("total", func(t *testing.T) {
		r := newRec(t, 40, 30)
		one := len(`<rect x="0" y="0" width="1" height="1" fill="#000000"/>` + "\n")
		r.Limits.MaxTotalBytes = 2 * one
		r.FillRect(painter.Rect{W: 1, H: 1}, black)
		r.FillRect(painter.Rect{W: 1, H: 1}, black)
		if r.Err() != nil {
			t.Fatalf("two elements of exactly the total were refused: %v", r.Err())
		}
		r.FillRect(painter.Rect{W: 1, H: 1}, black)
		if !errors.Is(r.Err(), svgrec.ErrTooLarge) {
			t.Errorf("Err = %v", r.Err())
		}
	})
	t.Run("document larger than total by its wrapper", func(t *testing.T) {
		r := newRec(t, 40, 30)
		r.Limits.MaxTotalBytes = len(`<rect x="0" y="0" width="1" height="1" fill="#000000"/>` + "\n")
		r.FillRect(painter.Rect{W: 1, H: 1}, black)
		if r.Err() != nil {
			t.Fatal(r.Err())
		}
		b, err := r.SVG()
		if !errors.Is(err, svgrec.ErrTooLarge) || len(b) == 0 {
			t.Errorf("SVG = %d bytes, %v", len(b), err)
		}
	})
	t.Run("path", func(t *testing.T) {
		r := newRec(t, 40, 30)
		r.Limits.MaxCallBytes = 50
		p := painter.NewPath().MoveTo(0, 0)
		for i := 0; i < 50; i++ {
			p.LineTo(float64(i), 1)
		}
		r.FillPath(p, black, painter.NonZero)
		if !errors.Is(r.Err(), svgrec.ErrTooLarge) {
			t.Errorf("Err = %v", r.Err())
		}
	})
	t.Run("text", func(t *testing.T) {
		r := newRec(t, 40, 30)
		r.Limits.MaxCallBytes = 100
		r.Text(0, 0, "HELLO", black)
		if !errors.Is(r.Err(), svgrec.ErrTooLarge) {
			t.Errorf("Err = %v", r.Err())
		}
	})
	t.Run("image pixels", func(t *testing.T) {
		r := newRec(t, 40, 30)
		r.Limits.MaxImagePixels = 4
		r.DrawImage(painter.Rect{W: 2, H: 2}, make([]byte, 16), 2, 2) // exactly the limit
		r.DrawMask(painter.Rect{W: 2, H: 2}, make([]byte, 4), 2, black)
		if r.Err() != nil {
			t.Fatalf("exactly the limit was refused: %v", r.Err())
		}
		r.DrawImage(painter.Rect{W: 2, H: 3}, make([]byte, 24), 2, 3)
		if !errors.Is(r.Err(), svgrec.ErrTooLarge) {
			t.Errorf("image: Err = %v", r.Err())
		}
		r2 := newRec(t, 40, 30)
		r2.Limits.MaxImagePixels = 4
		r2.DrawMask(painter.Rect{W: 3, H: 2}, make([]byte, 6), 3, black)
		if !errors.Is(r2.Err(), svgrec.ErrTooLarge) {
			t.Errorf("mask: Err = %v", r2.Err())
		}
	})
	t.Run("encoded image", func(t *testing.T) {
		r := newRec(t, 40, 30)
		r.Limits.MaxCallBytes = 500
		r.DrawImage(painter.Rect{W: 8, H: 8}, make([]byte, 8*8*4), 8, 8)
		if r.Err() != nil {
			t.Fatalf("a small PNG was refused: %v", r.Err())
		}
		pix := make([]byte, 64*64*4)
		s := uint32(2463534242) // xorshift32: incompressible enough
		for i := range pix {
			s ^= s << 13
			s ^= s >> 17
			s ^= s << 5
			pix[i] = byte(s)
		}
		r.DrawImage(painter.Rect{W: 64, H: 64}, pix, 64, 64)
		if !errors.Is(r.Err(), svgrec.ErrTooLarge) {
			t.Errorf("Err = %v", r.Err())
		}
	})
}

func TestResetAndDeterminism(t *testing.T) {
	record := func(r *svgrec.Recorder) {
		r.PushClip(painter.Rect{X: 1, Y: 1, W: 9, H: 9})
		r.FillRoundRect(painter.Rect{X: 2, Y: 2, W: 8, H: 8}, 3, painter.RGBA{R: 1, G: 2, B: 3, A: 77})
		r.Text(2, 12, "DET", black)
		r.StrokePath(painter.NewPath().MoveTo(0.1, 0.2).QuadTo(3.3333, 4, 5, 6), black, 1.25)
		r.DrawMask(painter.Rect{W: 2, H: 2}, []byte{1, 2, 3, 4}, 2, black)
		r.PopClip()
	}
	a, b := newRec(t, 40, 30), newRec(t, 40, 30)
	record(a)
	record(b)
	if !bytes.Equal(mustSVG(t, a), mustSVG(t, b)) {
		t.Error("the same calls gave different bytes")
	}
	first := mustSVG(t, a)
	if !bytes.Equal(first, mustSVG(t, a)) {
		t.Error("SVG is not repeatable")
	}
	a.Text(0, 0, "\xff", black)
	a.PushTranslate(5, 5)
	a.Reset()
	if a.Err() != nil || a.Stats() != (svgrec.Stats{}) {
		t.Errorf("Reset left %v %+v", a.Err(), a.Stats())
	}
	a.SetTextScale(1)
	record(a)
	if !bytes.Equal(first, mustSVG(t, a)) {
		t.Error("after Reset (ids, translation, clips) the same calls gave different bytes")
	}
}

func TestCapabilities(t *testing.T) {
	var p painter.Painter = newRec(t, 4, 4)
	if _, ok := p.(painter.Clipper); !ok {
		t.Error("not a Clipper")
	}
	if _, ok := p.(painter.Translator); !ok {
		t.Error("not a Translator")
	}
	if _, ok := p.(painter.PathPainter); !ok {
		t.Error("not a PathPainter")
	}
	if _, ok := p.(painter.ImagePainter); !ok {
		t.Error("not an ImagePainter")
	}
	if _, ok := p.(painter.MaskPainter); !ok {
		t.Error("not a MaskPainter")
	}
	if _, ok := p.(painter.FacePainter); ok {
		t.Error("claims FacePainter, which it cannot honour")
	}
	if _, ok := p.(painter.RasterPainter); ok {
		t.Error("claims RasterPainter, which says PutPixel is a pixel")
	}
}
