// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

// maskReference is the loop DrawMask replaced: the way a font back-end had to
// paint a glyph, scaling the coverage by the ink's alpha itself and handing
// each pixel to PutPixel, which applies the translation, the surface bounds,
// the clip and the blend.
func maskReference(p *PixelPainter, dst Rect, mask []byte, stride int, ink RGBA) {
	for dy := 0; dy < dst.H; dy++ {
		for dx := 0; dx < dst.W; dx++ {
			cov := mask[dy*stride+dx]
			if cov == 0 {
				continue
			}
			a := uint8(uint32(cov) * uint32(ink.A) / 255)
			p.PutPixel(dst.X+dx, dst.Y+dy, RGBA{R: ink.R, G: ink.G, B: ink.B, A: a})
		}
	}
}

// glyphMask builds a w*h coverage mask with a hole, a solid core and soft
// edges — the shape of a real glyph, so no case is a uniform block.
func glyphMask(w, h int) []byte {
	m := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x == 0 || y == 0:
				m[y*w+x] = 0
			case x < w/3:
				m[y*w+x] = uint8(x * 255 / max(w/3, 1))
			default:
				m[y*w+x] = 255
			}
		}
	}
	return m
}

func TestDrawMaskMatchesTheLoopItReplaced(t *testing.T) {
	mask := glyphMask(9, 7)
	opaque := RGBA{R: 240, G: 30, B: 90, A: 255}
	half := RGBA{R: 240, G: 30, B: 90, A: 128}

	for _, tc := range []struct {
		name   string
		dst    Rect
		ink    RGBA
		clip   *Rect
		tx, ty int
	}{
		{name: "opaque ink", dst: Rect{X: 3, Y: 3, W: 9, H: 7}, ink: opaque},
		{name: "translucent ink", dst: Rect{X: 3, Y: 3, W: 9, H: 7}, ink: half},
		{name: "off the left and top", dst: Rect{X: -4, Y: -3, W: 9, H: 7}, ink: opaque},
		{name: "off the right and bottom", dst: Rect{X: 14, Y: 15, W: 9, H: 7}, ink: opaque},
		{name: "entirely off the surface", dst: Rect{X: 40, Y: 40, W: 9, H: 7}, ink: opaque},
		{name: "clipped", dst: Rect{X: 2, Y: 2, W: 9, H: 7}, ink: opaque, clip: &Rect{X: 4, Y: 4, W: 3, H: 3}},
		{name: "clipped away", dst: Rect{X: 2, Y: 2, W: 9, H: 7}, ink: opaque, clip: &Rect{X: 30, Y: 30, W: 2, H: 2}},
		{name: "translated", dst: Rect{X: 1, Y: 1, W: 9, H: 7}, ink: opaque, tx: 5, ty: 4},
		{name: "translated and clipped", dst: Rect{X: 0, Y: 0, W: 9, H: 7}, ink: half, tx: 3, ty: 3, clip: &Rect{X: 2, Y: 2, W: 6, H: 6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(f func(p *PixelPainter)) *PixelPainter {
				p := newPix(24, 24)
				// A non-black ground, so a blend that wrongly behaves as a copy
				// shows up instead of hiding in zeroes.
				p.FillRect(Rect{X: 0, Y: 0, W: 24, H: 24}, RGBA{R: 20, G: 70, B: 40, A: 255})
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

			got := run(func(p *PixelPainter) { p.DrawMask(tc.dst, mask, 9, tc.ink) })
			want := run(func(p *PixelPainter) { maskReference(p, tc.dst, mask, 9, tc.ink) })

			for i := range got.Buf {
				if got.Buf[i] != want.Buf[i] {
					px := i / 4
					t.Fatalf("pixel %d,%d byte %d: DrawMask %d, the loop it replaced %d",
						px%24, px/24, i%4, got.Buf[i], want.Buf[i])
				}
			}
		})
	}
}

// DrawMask writes the blend itself instead of calling blendInto, which is
// worth 2x and worth exactly nothing if the arithmetic drifts. Every coverage
// value, against several ink alphas and several grounds -- including
// translucent ones, where the alpha byte truncates while the colour bytes
// round, and where a plausible-looking formula is wrong by one.
func TestDrawMaskBlendsExactlyLikePutPixel(t *testing.T) {
	mask := make([]byte, 256)
	for i := range mask {
		mask[i] = uint8(i)
	}
	for _, inkA := range []uint8{255, 200, 128, 64, 1} {
		for _, ground := range []RGBA{
			{R: 0, G: 0, B: 0, A: 255},
			{R: 255, G: 255, B: 255, A: 255},
			{R: 90, G: 140, B: 200, A: 255},
			{R: 90, G: 140, B: 200, A: 128},
			{R: 3, G: 5, B: 7, A: 1},
			{},
		} {
			ink := RGBA{R: 240, G: 30, B: 90, A: inkA}
			run := func(f func(p *PixelPainter)) *PixelPainter {
				p := newPix(256, 1)
				for i := 0; i < len(p.Buf); i += 4 {
					p.Buf[i], p.Buf[i+1], p.Buf[i+2], p.Buf[i+3] = ground.R, ground.G, ground.B, ground.A
				}
				f(p)
				return p
			}
			dst := Rect{X: 0, Y: 0, W: 256, H: 1}
			got := run(func(p *PixelPainter) { p.DrawMask(dst, mask, 256, ink) })
			want := run(func(p *PixelPainter) { maskReference(p, dst, mask, 256, ink) })
			for i := range got.Buf {
				if got.Buf[i] != want.Buf[i] {
					t.Fatalf("ink alpha %d over ground %+v: coverage %d byte %d = %d, PutPixel gives %d",
						inkA, ground, i/4, i%4, got.Buf[i], want.Buf[i])
				}
			}
		}
	}
}

// A stride wider than the rectangle is the normal case: a glyph is a window
// into a bigger mask atlas, and only its own columns may be read.
func TestDrawMaskHonoursTheStride(t *testing.T) {
	// 8 wide, but only the first 3 columns of each row belong to the glyph.
	const stride = 8
	mask := make([]byte, stride*3)
	for y := 0; y < 3; y++ {
		mask[y*stride], mask[y*stride+1], mask[y*stride+2] = 255, 255, 255
		for x := 3; x < stride; x++ {
			mask[y*stride+x] = 255 // would show if the stride were ignored
		}
	}
	p := newPix(12, 6)
	p.DrawMask(Rect{X: 0, Y: 0, W: 3, H: 3}, mask, stride, RGBA{R: 255, G: 255, B: 255, A: 255})

	if p.Buf[(0*12+2)*4+3] == 0 {
		t.Error("the glyph's own last column was not painted")
	}
	if p.Buf[(0*12+3)*4+3] != 0 {
		t.Error("painted past the rectangle, reading the neighbouring glyph")
	}
	if p.Buf[(1*12+0)*4+3] == 0 {
		t.Error("the second row started at the wrong offset")
	}
}

// Nonsense arguments leave the surface alone; a mask shorter than the rectangle
// costs the rows that are missing, not a panic.
func TestDrawMaskRejectsNonsense(t *testing.T) {
	p := newPix(8, 8)
	full := Rect{X: 0, Y: 0, W: 4, H: 4}
	white := RGBA{R: 255, G: 255, B: 255, A: 255}
	m := glyphMask(4, 4)

	p.DrawMask(Rect{X: 0, Y: 0, W: 0, H: 4}, m, 4, white) // empty destination
	p.DrawMask(Rect{X: 0, Y: 0, W: 4, H: 0}, m, 4, white) // empty destination
	p.DrawMask(full, m, 0, white)                         // no stride
	p.DrawMask(full, m, 4, RGBA{R: 255, G: 255, B: 255})  // invisible ink
	for i := range p.Buf {
		if p.Buf[i] != 0 {
			t.Fatal("a malformed call painted something")
		}
	}

	// Short mask: the rows that exist land, the rest are dropped.
	p.DrawMask(full, m[:8], 4, white)
	if p.Buf[3] == 0 && p.Buf[(1*8+1)*4+3] == 0 {
		t.Error("a short mask painted nothing at all")
	}
}

// A short Buf is tolerated the way PutPixel tolerates it.
func TestDrawMaskShortBuffer(t *testing.T) {
	p := &PixelPainter{Buf: make([]byte, 4*2*4), Width: 4, Height: 4}
	m := make([]byte, 16)
	for i := range m {
		m[i] = 255
	}
	p.DrawMask(Rect{X: 0, Y: 0, W: 4, H: 4}, m, 4, RGBA{R: 1, G: 2, B: 3, A: 255})
	if p.Buf[3] == 0 {
		t.Error("the rows that did fit were not painted")
	}
}

// The cell back-end resolves coverage to a decision: a cell is the glyph's
// colour or it is not.
func TestCellPainterDrawMask(t *testing.T) {
	c := &CellPainter{Cells: make([]Cell, 6*6), W: 6, H: 6}
	m := []byte{
		255, 10, 200,
		0, 130, 127,
	}
	white := RGBA{R: 255, G: 255, B: 255, A: 255}
	c.DrawMask(Rect{X: 0, Y: 0, W: 3, H: 2}, m, 3, white)

	if c.Cells[0].Fg.A == 0 {
		t.Error("full coverage left the cell empty")
	}
	if c.Cells[1].Fg.A != 0 {
		t.Error("a tenth of coverage filled a cell")
	}
	if c.Cells[6+1].Fg.A == 0 {
		t.Error("just over half coverage left the cell empty")
	}
	if c.Cells[6+2].Fg.A != 0 {
		t.Error("just under half coverage filled a cell")
	}

	// Degenerate calls are ignored rather than fatal.
	c.DrawMask(Rect{X: 0, Y: 0, W: 0, H: 2}, m, 3, white)
	c.DrawMask(Rect{X: 0, Y: 0, W: 3, H: 2}, m, 0, white)
	c.DrawMask(Rect{X: 0, Y: 0, W: 3, H: 2}, m, 3, RGBA{})
	c.DrawMask(Rect{X: 0, Y: 0, W: 3, H: 4}, m, 3, white) // mask ends early
}

func TestPaintersImplementMaskPainter(t *testing.T) {
	var _ MaskPainter = (*PixelPainter)(nil)
	var _ MaskPainter = (*CellPainter)(nil)
}

// A page of text: 2000 glyph-sized masks, which is the order a full window of
// labels issues.
func BenchmarkDrawMask(b *testing.B) {
	p := newPix(1000, 700)
	m := glyphMask(9, 14)
	ink := RGBA{R: 220, G: 220, B: 220, A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for g := 0; g < 2000; g++ {
			p.DrawMask(Rect{X: (g * 11) % 980, Y: (g / 89) * 16 % 680, W: 9, H: 14}, m, 9, ink)
		}
	}
}

func BenchmarkMaskPerPixel(b *testing.B) {
	p := newPix(1000, 700)
	m := glyphMask(9, 14)
	ink := RGBA{R: 220, G: 220, B: 220, A: 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for g := 0; g < 2000; g++ {
			maskReference(p, Rect{X: (g * 11) % 980, Y: (g / 89) * 16 % 680, W: 9, H: 14}, m, 9, ink)
		}
	}
}
