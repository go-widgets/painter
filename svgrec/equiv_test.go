// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"testing"

	"github.com/go-gfx/gfx/svg"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

// Pixel equivalence: the same calls go to a painter.PixelPainter (the raster
// reference) and to a Recorder whose SVG is rasterised by go-gfx/gfx/svg, the
// fleet's pure-Go SVG rasteriser (Scale 1, so one user unit is one pixel). Both
// start from the same opaque white ground so translucent blends are comparable.
//
// The tolerance is STATED per scene as the largest per-channel difference
// (0..255) any pixel may show, plus the share of pixels allowed to differ at
// all. Axis-aligned rectangles and bitmap-font text sit on the pixel grid and
// must match to the rounding of the alpha blend (1). Anything with a curved
// edge is anti-aliased differently by the two (distance-to-centre versus true
// coverage), so its bound is wider; see the table below.

const bg = 0xFF // the ground is opaque white

type scene struct {
	name string
	w, h int
	draw func(p painter.Painter)
	// maxDiff is the largest allowed per-channel difference; fracDiff the
	// largest allowed share of pixels that differ by more than 1.
	maxDiff  int
	fracDiff float64
}

func ground(p painter.Painter) {
	w, h := p.Size()
	p.FillRect(painter.Rect{W: w, H: h}, painter.RGB(bg, bg, bg))
}

func renderRaster(s scene) []byte {
	buf := make([]byte, s.w*s.h*4)
	pp := painter.NewPixelPainter(buf, s.w, s.h)
	ground(pp)
	s.draw(pp)
	return buf
}

func renderSVG(t *testing.T, s scene) ([]byte, []byte) {
	t.Helper()
	rec, err := svgrec.New(s.w, s.h)
	if err != nil {
		t.Fatal(err)
	}
	ground(rec)
	s.draw(rec)
	doc, err := rec.SVG()
	if err != nil {
		t.Fatalf("%s: %v", s.name, err)
	}
	res, err := svg.Rasterize(string(doc), svg.Options{Scale: 1})
	if err != nil {
		t.Fatalf("%s: rasterise: %v\n%s", s.name, err, doc)
	}
	if res.Image.W != s.w || res.Image.H != s.h {
		t.Fatalf("%s: rasterised %dx%d, want %dx%d", s.name, res.Image.W, res.Image.H, s.w, s.h)
	}
	return res.Image.Pix, doc
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// diff returns the largest per-channel difference and the share of pixels whose
// largest channel difference exceeds 1.
func diff(a, b []byte) (maxD int, frac float64) {
	n, bad := len(a)/4, 0
	for i := 0; i < len(a); i += 4 {
		m := 0
		for c := 0; c < 3; c++ {
			m = max(m, abs(int(a[i+c])-int(b[i+c])))
		}
		maxD = max(maxD, m)
		if m > 1 {
			bad++
		}
	}
	return maxD, float64(bad) / float64(n)
}

// gradient is a w x h straight-alpha RGBA test picture, opaque unless
// translucent is set.
func gradient(w, h int, translucent bool) []byte {
	src := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		a := byte(255)
		if translucent {
			a = byte(60 + i*3)
		}
		src[i*4], src[i*4+1], src[i*4+2], src[i*4+3] = byte(i*4), byte(255-i*4), byte(i*3), a
	}
	return src
}

func ring() *painter.Path {
	p := painter.NewPath()
	p.MoveTo(10, 10).LineTo(54, 10).LineTo(54, 54).LineTo(10, 54).Close()
	p.MoveTo(20, 20).LineTo(44, 20).LineTo(44, 44).LineTo(20, 44).Close()
	return p
}

func curve() *painter.Path {
	p := painter.NewPath()
	p.MoveTo(8, 50).QuadTo(32, 0, 56, 50).CubicTo(40, 62, 24, 62, 8, 50).Close()
	return p
}

var scenes = []scene{
	{"rects", 64, 64, func(p painter.Painter) {
		p.FillRect(painter.Rect{X: 3, Y: 4, W: 30, H: 20}, painter.RGB(200, 30, 30))
		p.FillRect(painter.Rect{X: 20, Y: 14, W: 30, H: 30}, painter.RGBA{R: 20, G: 60, B: 220, A: 128})
		p.PutPixel(1, 1, painter.RGB(0, 0, 0))
		p.PutPixel(62, 62, painter.RGBA{A: 77})
	}, 1, 0},
	{"strokerect", 64, 64, func(p painter.Painter) {
		p.StrokeRect(painter.Rect{X: 2, Y: 2, W: 30, H: 20}, painter.RGB(0, 0, 0), 1)
		p.StrokeRect(painter.Rect{X: 8, Y: 8, W: 40, H: 40}, painter.RGBA{R: 200, A: 100}, 3)
		p.StrokeRect(painter.Rect{X: 40, Y: 40, W: 8, H: 8}, painter.RGB(0, 120, 0), 9)
	}, 1, 0},
	{"text", 120, 40, func(p painter.Painter) {
		p.Text(2, 2, "HELLO 123", painter.RGB(10, 10, 10))
		p.Text(2, 12, "AB:C", painter.RGBA{R: 200, A: 140})
		if s, ok := p.(interface{ SetTextScale(int) }); ok {
			s.SetTextScale(2)
		}
		p.Text(2, 22, "OK", painter.RGB(0, 0, 160))
	}, 1, 0},
	{"clip+translate", 64, 64, func(p painter.Painter) {
		c := p.(painter.Clipper)
		tr := p.(painter.Translator)
		tr.PushTranslate(10, 6)
		c.PushClip(painter.Rect{X: 4, Y: 4, W: 30, H: 30})
		p.FillRect(painter.Rect{X: 0, Y: 0, W: 80, H: 80}, painter.RGB(30, 160, 60))
		c.PushClip(painter.Rect{X: 20, Y: 0, W: 40, H: 12})
		p.FillRect(painter.Rect{X: 0, Y: 0, W: 80, H: 80}, painter.RGB(250, 150, 0))
		c.PopClip()
		tr.PushTranslate(-4, -4)
		p.Text(0, 20, "CLIP", painter.RGB(0, 0, 0))
		tr.PopTranslate()
		c.PopClip()
		tr.PopTranslate()
		p.FillRect(painter.Rect{X: 0, Y: 60, W: 64, H: 4}, painter.RGB(90, 0, 90))
	}, 1, 0},
	{"roundrect", 64, 64, func(p painter.Painter) {
		p.FillRoundRect(painter.Rect{X: 4, Y: 4, W: 40, H: 24}, 8, painter.RGB(30, 90, 200))
		p.FillRoundRect(painter.Rect{X: 10, Y: 30, W: 40, H: 30}, 100, painter.RGBA{R: 200, A: 140})
		p.FillRoundRect(painter.Rect{X: 50, Y: 4, W: 10, H: 10}, 0, painter.RGB(0, 0, 0))
	}, 40, 0.04}, // measured 36 / 3.1%
	{"strokeroundrect", 64, 64, func(p painter.Painter) {
		p.StrokeRoundRect(painter.Rect{X: 4, Y: 4, W: 50, H: 30}, 8, painter.RGB(0, 0, 0), 1)
		p.StrokeRoundRect(painter.Rect{X: 6, Y: 36, W: 50, H: 24}, 10, painter.RGB(180, 0, 0), 3)
	}, 65, 0.09}, // measured 61 / 7.5%
	{"fillpath", 64, 64, func(p painter.Painter) {
		pp := p.(painter.PathPainter)
		pp.FillPath(ring(), painter.RGB(20, 120, 60), painter.EvenOdd)
		pp.FillPath(curve(), painter.RGBA{R: 200, A: 120}, painter.NonZero)
	}, 2, 0}, // measured 1: both sides use go-gfx's rasteriser
	{"strokepath", 64, 64, func(p painter.Painter) {
		pp := p.(painter.PathPainter)
		pp.StrokePath(curve(), painter.RGB(0, 0, 0), 3)
		pp.StrokePath(ring(), painter.RGBA{B: 200, A: 150}, 1.5)
	}, 2, 0}, // measured 1
	// Images and masks: the position and opaque colours are exact. A translucent
	// pixel comes out of go-gfx/gfx/svg too dark in its colour channels (a
	// source G of 255 at alpha 60 composites to 209, not 255: the colour is
	// weighted by alpha twice on that rasteriser's image path; the alpha
	// itself is right), so the translucent scenes carry a wider stated bound
	// that measures that rasteriser quirk, not the recorder.
	{"image opaque 1:1", 32, 32, func(p painter.Painter) {
		p.(painter.ImagePainter).DrawImage(painter.Rect{X: 5, Y: 6, W: 8, H: 8}, gradient(8, 8, false), 8, 8)
	}, 1, 0},
	{"image translucent 1:1", 32, 32, func(p painter.Painter) {
		p.(painter.ImagePainter).DrawImage(painter.Rect{X: 5, Y: 6, W: 8, H: 8}, gradient(8, 8, true), 8, 8)
	}, 55, 0.07}, // measured 50 / 6.25%
	{"mask binary 1:1", 32, 32, func(p painter.Painter) {
		m := make([]byte, 10*6)
		for i := range m {
			m[i] = byte(255 * (i / 3 % 2))
		}
		p.(painter.MaskPainter).DrawMask(painter.Rect{X: 3, Y: 4, W: 10, H: 6}, m, 10, painter.RGB(200, 20, 40))
	}, 1, 0},
	{"mask translucent 1:1", 32, 32, func(p painter.Painter) {
		m := make([]byte, 10*6)
		for i := range m {
			m[i] = byte(i * 4)
		}
		p.(painter.MaskPainter).DrawMask(painter.Rect{X: 3, Y: 4, W: 10, H: 6}, m, 10, painter.RGB(200, 20, 40))
	}, 55, 0.07}, // measured 51 / 5.8%
	{"widgets", 120, 60, func(p painter.Painter) {
		th := painter.LightTheme()
		(&painter.Label{Bounds: painter.Rect{X: 4, Y: 4, W: 60, H: 8}, Text: "STATUS"}).Draw(p, th)
		(&painter.Button{Bounds: painter.Rect{X: 4, Y: 16, W: 50, H: 16}, Label: "OK"}).Draw(p, th)
		(&painter.Button{Bounds: painter.Rect{X: 60, Y: 16, W: 50, H: 16}, Label: "NO", Pressed: true}).Draw(p, th)
		(&painter.ProgressBar{Bounds: painter.Rect{X: 4, Y: 40, W: 106, H: 10}, Value: 0.6}).Draw(p, th)
	}, 1, 0},
}

func TestPixelEquivalence(t *testing.T) {
	for _, s := range scenes {
		t.Run(s.name, func(t *testing.T) {
			want := renderRaster(s)
			got, _ := renderSVG(t, s)
			maxD, frac := diff(want, got)
			t.Logf("max channel diff %d, %.2f%% of pixels differ by more than 1", maxD, frac*100)
			if maxD > s.maxDiff || frac > s.fracDiff {
				t.Errorf("max diff %d (allowed %d), differing share %.4f (allowed %.4f)", maxD, s.maxDiff, frac, s.fracDiff)
			}
		})
	}
}
