// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"bytes"
	"testing"
)

func TestIconSize(t *testing.T) {
	for _, c := range []struct{ w, h, mw, mh, pad, ew, eh int }{
		{16, 16, 32, 32, 0, 16, 16}, // fits: unchanged, never scaled up
		{64, 32, 32, 32, 0, 32, 16}, // wide
		{32, 64, 32, 32, 0, 16, 32}, // tall
		{1000, 1, 10, 10, 0, 10, 1}, // never below 1
		{16, 16, 0, 0, 0, 16, 16},   // no room given: unchanged
		{0, 5, 10, 10, 0, 0, 0},     // nothing to show
		{64, 32, 40, 40, 4, 32, 16}, // a margin of 4 leaves 32
		{16, 16, 20, 20, 4, 12, 12}, // the margin makes it shrink
		{16, 16, 8, 8, 4, 16, 16},   // the margin eats the box: unchanged
		{16, 16, 100, 100, -5, 16, 16},
		{16, 16, 1 << 62, 1 << 62, 1 << 62, 16, 16}, // no overflow
	} {
		if w, h := IconSize(c.w, c.h, c.mw, c.mh, c.pad); w != c.ew || h != c.eh {
			t.Errorf("IconSize(%d,%d,%d,%d,%d) = %d,%d want %d,%d", c.w, c.h, c.mw, c.mh, c.pad, w, h, c.ew, c.eh)
		}
	}
}

func TestIconBGRAOver(t *testing.T) {
	bg := RGB(240, 230, 220)
	for _, c := range []struct {
		name string
		px   []byte
		want []byte
	}{
		{"opaque is kept, in BGRA order", []byte{200, 100, 50, 255}, []byte{50, 100, 200, 255}},
		{"transparent is the background", []byte{200, 100, 50, 0}, []byte{220, 230, 240, 255}},
	} {
		w, h, got := IconBGRAOver(c.px, 1, 1, 8, 8, 0, bg)
		if w != 1 || h != 1 || !bytes.Equal(got, c.want) {
			t.Errorf("%s: %dx%d %v, want %v", c.name, w, h, got, c.want)
		}
	}
	// Half red over white: red stays, green and blue fall to about half.
	_, _, got := IconBGRAOver([]byte{255, 0, 0, 128}, 1, 1, 8, 8, 0, RGB(255, 255, 255))
	if got[3] != 255 || got[2] < 250 || got[0] < 100 || got[0] > 160 || got[0] != got[1] {
		t.Errorf("half red over white = %v", got)
	}
	// Shrunk: a 4x4 of opaque red onto 2x2, every pixel red, in BGRA order.
	w, h, got := IconBGRAOver(bytes.Repeat([]byte{255, 0, 0, 255}, 16), 4, 4, 6, 6, 2, bg)
	if w != 2 || h != 2 || !bytes.Equal(got, bytes.Repeat([]byte{0, 0, 255, 255}, 4)) {
		t.Errorf("shrunk: %dx%d %v", w, h, got)
	}
}

func TestIconBGRAOverRefusesWhatItCannotRead(t *testing.T) {
	for _, c := range []struct{ n, w, h int }{{4, 2, 2}, {0, 1, 1}, {4, 0, 1}, {4, 1, -1}, {16, 1 << 30, 1 << 30}} {
		if w, h, b := IconBGRAOver(make([]byte, c.n), c.w, c.h, 8, 8, 0, RGB(0, 0, 0)); w != 0 || h != 0 || b != nil {
			t.Errorf("%+v accepted: %dx%d", c, w, h)
		}
	}
}

// DrawImage on a BGRA painter must swap the colour bytes of OPAQUE pixels too:
// the verbatim row paths used to copy them as they were.
func TestDrawImageOnABGRAPainterSwapsOpaquePixels(t *testing.T) {
	src := bytes.Repeat([]byte{10, 20, 30, 255}, 4) // 2x2 opaque
	for _, dst := range []Rect{{W: 2, H: 2}, {W: 4, H: 4}} {
		buf := make([]byte, dst.W*dst.H*4)
		NewPixelPainterBGRA(buf, dst.W, dst.H).DrawImage(dst, src, 2, 2)
		if !bytes.Equal(buf[:4], []byte{30, 20, 10, 255}) {
			t.Errorf("%v: first pixel %v, want 30 20 10 255", dst, buf[:4])
		}
	}
}

func FuzzIconBGRAOver(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4}, 1, 1, 8, 8, 0)
	f.Add(bytes.Repeat([]byte{9, 9, 9, 200}, 64), 8, 8, 9, 11, 3)
	f.Add([]byte{}, 1<<30, 1<<30, 1, 1, 1)
	f.Add(bytes.Repeat([]byte{9, 9, 9, 200}, 64), 8, 8, -1<<62, 1<<62, 1<<62)
	f.Fuzz(func(t *testing.T, src []byte, sw, sh, mw, mh, pad int) {
		w, h, b := IconBGRAOver(src, sw, sh, mw, mh, pad, RGB(1, 2, 3))
		if b == nil {
			if w != 0 || h != 0 {
				t.Fatalf("no pixels but %dx%d", w, h)
			}
			return
		}
		if w < 1 || h < 1 || len(b) != w*h*4 || len(b) > len(src) && len(b) > 4 {
			t.Fatalf("%dx%d len %d from %d source bytes", w, h, len(b), len(src))
		}
		for i := 3; i < len(b); i += 4 {
			if b[i] != 255 {
				t.Fatal("not opaque")
			}
		}
	})
}
