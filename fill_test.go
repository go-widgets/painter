// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

// fillReference is the loop FillRect used to be: a PutPixel per pixel, which
// applies the translation, the surface bounds, the clip and the blend. It is
// the yardstick, because a fill that is fast and different is not an
// optimisation.
func fillReference(p *PixelPainter, r Rect, c RGBA) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			p.PutPixel(x, y, c)
		}
	}
}

func TestFillRectMatchesTheLoopItReplaced(t *testing.T) {
	opaque := RGBA{R: 200, G: 100, B: 50, A: 255}
	half := RGBA{R: 200, G: 100, B: 50, A: 128}

	for _, tc := range []struct {
		name   string
		r      Rect
		c      RGBA
		clip   *Rect
		tx, ty int
	}{
		{name: "opaque, whole surface", r: Rect{X: 0, Y: 0, W: 20, H: 20}, c: opaque},
		{name: "opaque, inset", r: Rect{X: 3, Y: 4, W: 6, H: 5}, c: opaque},
		{name: "opaque, single pixel", r: Rect{X: 7, Y: 7, W: 1, H: 1}, c: opaque},
		{name: "opaque, single column", r: Rect{X: 2, Y: 2, W: 1, H: 9}, c: opaque},
		{name: "opaque, single row", r: Rect{X: 2, Y: 2, W: 9, H: 1}, c: opaque},
		{name: "opaque, odd width", r: Rect{X: 1, Y: 1, W: 7, H: 3}, c: opaque},
		{name: "off the left and top", r: Rect{X: -4, Y: -3, W: 9, H: 8}, c: opaque},
		{name: "off the right and bottom", r: Rect{X: 15, Y: 16, W: 9, H: 8}, c: opaque},
		{name: "entirely off the surface", r: Rect{X: 40, Y: 40, W: 5, H: 5}, c: opaque},
		{name: "empty", r: Rect{X: 2, Y: 2, W: 0, H: 5}, c: opaque},
		{name: "negative height", r: Rect{X: 2, Y: 2, W: 5, H: -1}, c: opaque},
		{name: "fully transparent", r: Rect{X: 0, Y: 0, W: 10, H: 10}, c: RGBA{R: 9, G: 9, B: 9}},
		{name: "translucent", r: Rect{X: 2, Y: 2, W: 8, H: 6}, c: half},
		{name: "opaque, clipped", r: Rect{X: 0, Y: 0, W: 20, H: 20}, c: opaque, clip: &Rect{X: 3, Y: 5, W: 6, H: 4}},
		{name: "translucent, clipped", r: Rect{X: 0, Y: 0, W: 20, H: 20}, c: half, clip: &Rect{X: 3, Y: 5, W: 6, H: 4}},
		{name: "opaque, clipped away", r: Rect{X: 0, Y: 0, W: 5, H: 5}, c: opaque, clip: &Rect{X: 15, Y: 15, W: 2, H: 2}},
		{name: "translated", r: Rect{X: 1, Y: 1, W: 5, H: 4}, c: opaque, tx: 6, ty: 7},
		{name: "translated and clipped", r: Rect{X: 0, Y: 0, W: 12, H: 12}, c: opaque, tx: 3, ty: 3, clip: &Rect{X: 1, Y: 1, W: 6, H: 6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(f func(p *PixelPainter)) *PixelPainter {
				p := newPix(20, 20)
				// A non-black ground, so a blend that wrongly behaves as a copy
				// shows up instead of hiding in zeroes.
				for i := 0; i < len(p.Buf); i += 4 {
					p.Buf[i], p.Buf[i+1], p.Buf[i+2], p.Buf[i+3] = 30, 60, 90, 255
				}
				if tc.tx != 0 || tc.ty != 0 {
					p.PushTranslate(tc.tx, tc.ty)
				}
				if tc.clip != nil {
					p.PushClip(*tc.clip)
				}
				f(p)
				if tc.clip != nil {
					p.PopClip()
				}
				if tc.tx != 0 || tc.ty != 0 {
					p.PopTranslate()
				}
				return p
			}

			got := run(func(p *PixelPainter) { p.FillRect(tc.r, tc.c) })
			want := run(func(p *PixelPainter) { fillReference(p, tc.r, tc.c) })

			for i := range got.Buf {
				if got.Buf[i] != want.Buf[i] {
					px := i / 4
					t.Fatalf("pixel %d,%d byte %d: FillRect %d, the loop it replaced %d",
						px%20, px/20, i%4, got.Buf[i], want.Buf[i])
				}
			}
		})
	}
}

// A Buf shorter than Width*Height*4 is tolerated rather than fatal, the same
// way PutPixel tolerates it.
func TestFillRectShortBuffer(t *testing.T) {
	p := &PixelPainter{Buf: make([]byte, 4*2*4), Width: 4, Height: 4}
	p.FillRect(Rect{X: 0, Y: 0, W: 4, H: 4}, RGBA{R: 1, G: 2, B: 3, A: 255})
	if p.Buf[3] == 0 {
		t.Error("the rows that did fit were not filled")
	}

	// The translucent path walks the same rows and must tolerate it too.
	q := &PixelPainter{Buf: make([]byte, 4*2*4), Width: 4, Height: 4}
	q.FillRect(Rect{X: 0, Y: 0, W: 4, H: 4}, RGBA{R: 1, G: 2, B: 3, A: 128})
	if q.Buf[3] == 0 {
		t.Error("the rows that did fit were not blended")
	}
}

// The fill that every widget makes: a window-sized opaque background.
func BenchmarkFillRect(b *testing.B) {
	p := newPix(1000, 700)
	r := Rect{X: 0, Y: 0, W: 1000, H: 700}
	c := RGBA{R: 30, G: 60, B: 90, A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.FillRect(r, c)
	}
}

func BenchmarkFillRectPerPixel(b *testing.B) {
	p := newPix(1000, 700)
	r := Rect{X: 0, Y: 0, W: 1000, H: 700}
	c := RGBA{R: 30, G: 60, B: 90, A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fillReference(p, r, c)
	}
}

// Small fills are what a table of rows and a row of buttons actually issue, and
// there the per-row set-up has to earn its keep too.
func BenchmarkFillRectSmall(b *testing.B) {
	p := newPix(1000, 700)
	c := RGBA{R: 30, G: 60, B: 90, A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for y := 0; y < 700; y += 20 {
			p.FillRect(Rect{X: 4, Y: y, W: 180, H: 18}, c)
		}
	}
}

func BenchmarkFillRectSmallPerPixel(b *testing.B) {
	p := newPix(1000, 700)
	c := RGBA{R: 30, G: 60, B: 90, A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for y := 0; y < 700; y += 20 {
			fillReference(p, Rect{X: 4, Y: y, W: 180, H: 18}, c)
		}
	}
}
