// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

// srcImage builds a w*h RGBA block whose red channel encodes the column and
// green the row, so a test can tell exactly which source pixel landed where.
func srcImage(w, h int) []byte {
	b := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			b[o], b[o+1], b[o+2], b[o+3] = uint8(x), uint8(y), 0, 0xFF
		}
	}
	return b
}

func pixAt(p *PixelPainter, x, y int) RGBA {
	o := (y*p.Width + x) * 4
	return RGBA{R: p.Buf[o], G: p.Buf[o+1], B: p.Buf[o+2], A: p.Buf[o+3]}
}

// A 1:1 blit puts every source pixel exactly where it belongs.
func TestDrawImageOneToOne(t *testing.T) {
	p := newPix(8, 8)
	p.DrawImage(Rect{X: 2, Y: 3, W: 4, H: 4}, srcImage(4, 4), 4, 4)

	if got := pixAt(p, 2, 3); got.R != 0 || got.G != 0 {
		t.Errorf("top-left = %+v, want source 0,0", got)
	}
	if got := pixAt(p, 5, 6); got.R != 3 || got.G != 3 {
		t.Errorf("bottom-right = %+v, want source 3,3", got)
	}
	if pixAt(p, 1, 3).A != 0 {
		t.Error("painted left of the destination")
	}
}

// Scaling uses the same nearest-neighbour mapping the hand-written loops did,
// so widgets that move onto this look identical.
func TestDrawImageScales(t *testing.T) {
	p := newPix(8, 8)
	p.DrawImage(Rect{X: 0, Y: 0, W: 4, H: 4}, srcImage(2, 2), 2, 2)

	if got := pixAt(p, 0, 0); got.R != 0 {
		t.Errorf("0,0 = %+v, want source column 0", got)
	}
	if got := pixAt(p, 3, 3); got.R != 1 || got.G != 1 {
		t.Errorf("3,3 = %+v, want source 1,1", got)
	}
}

// The clip and the translation apply to a blit exactly as to every other
// primitive — a viewport must be able to hold an image.
func TestDrawImageHonoursClipAndTranslation(t *testing.T) {
	p := newPix(10, 10)
	p.PushTranslate(2, 2)
	p.PushClip(Rect{X: 0, Y: 0, W: 2, H: 2})
	p.DrawImage(Rect{X: 0, Y: 0, W: 4, H: 4}, srcImage(4, 4), 4, 4)
	p.PopClip()
	p.PopTranslate()

	if pixAt(p, 2, 2).A == 0 {
		t.Error("the clipped area was not painted")
	}
	if pixAt(p, 5, 5).A != 0 {
		t.Error("the blit escaped a clip that moved with the translation")
	}
	if pixAt(p, 0, 0).A != 0 {
		t.Error("painted at the untranslated origin")
	}
}

// Translucent pixels composite through the same blend the rest of the painter
// uses, so an image over a background looks the way it did pixel by pixel.
func TestDrawImageBlendsAlpha(t *testing.T) {
	p := newPix(2, 2)
	p.FillRect(Rect{X: 0, Y: 0, W: 2, H: 2}, RGBA{R: 0, G: 0, B: 0, A: 255})
	half := []byte{255, 255, 255, 128}
	p.DrawImage(Rect{X: 0, Y: 0, W: 1, H: 1}, half, 1, 1)

	if got := pixAt(p, 0, 0); got.R == 0 || got.R == 255 {
		t.Errorf("blended pixel = %+v, want a mid grey", got)
	}
}

// A caller with a bug must not take the painter down with it, and must not read
// past the end of its own slice.
func TestDrawImageRejectsNonsense(t *testing.T) {
	p := newPix(4, 4)
	p.DrawImage(Rect{X: 0, Y: 0, W: 2, H: 2}, srcImage(2, 2), 0, 2)  // no source width
	p.DrawImage(Rect{X: 0, Y: 0, W: 0, H: 2}, srcImage(2, 2), 2, 2)  // empty destination
	p.DrawImage(Rect{X: 0, Y: 0, W: 2, H: 2}, []byte{1, 2, 3}, 2, 2) // source too short
	for i := range p.Buf {
		if p.Buf[i] != 0 {
			t.Fatal("a malformed call painted something")
		}
	}
}

// Off-surface destinations are clipped rather than dropped whole: the visible
// part still lands.
func TestDrawImagePartiallyOffSurface(t *testing.T) {
	p := newPix(4, 4)
	p.DrawImage(Rect{X: -1, Y: -1, W: 3, H: 3}, srcImage(3, 3), 3, 3)
	if pixAt(p, 0, 0).A == 0 {
		t.Error("the on-surface part of the blit was dropped")
	}
}

// The cell back-end degrades to coloured cells, the same promotion PutPixel
// already makes, so a terminal shows something rather than nothing.
func TestCellPainterDrawImage(t *testing.T) {
	// A pixel on a grid is a full-block glyph in the FOREGROUND — that is what
	// CellPainter.PutPixel already means by one — so that is what to assert.
	c := &CellPainter{Cells: make([]Cell, 4*4), W: 4, H: 4}
	c.DrawImage(Rect{X: 1, Y: 1, W: 2, H: 2}, srcImage(2, 2), 2, 2)
	if c.Cells[1*4+1].Fg.A == 0 {
		t.Errorf("cell 1,1 = %+v, want a filled block", c.Cells[1*4+1])
	}
	if c.Cells[0].Fg.A != 0 {
		t.Error("painted outside the destination")
	}
	// Fully transparent source pixels leave the grid alone.
	c2 := &CellPainter{Cells: make([]Cell, 4*4), W: 4, H: 4}
	c2.DrawImage(Rect{X: 0, Y: 0, W: 1, H: 1}, []byte{9, 9, 9, 0}, 1, 1)
	if c2.Cells[0].Fg.A != 0 {
		t.Error("a transparent pixel filled a cell")
	}
	c2.DrawImage(Rect{X: 0, Y: 0, W: 1, H: 1}, []byte{1, 2, 3}, 1, 1) // too short
	c2.DrawImage(Rect{X: 0, Y: 0, W: 0, H: 1}, srcImage(1, 1), 1, 1)  // empty
}

// reference is the loop DrawImage replaced, written out. PutPixel applies the
// clip, the translation and the blend, so this honours everything DrawImage
// must honour -- which makes it the yardstick. Comparing the primitive against
// a clip-forced variant of ITSELF would only prove it agrees with itself, and
// stops proving even that once the fast paths learn to run under a clip.
func reference(p *PixelPainter, dst Rect, src []byte, srcW, srcH int) {
	for dy := 0; dy < dst.H; dy++ {
		sy := dy * srcH / dst.H
		for dx := 0; dx < dst.W; dx++ {
			o := (sy*srcW + dx*srcW/dst.W) * 4
			p.PutPixel(dst.X+dx, dst.Y+dy, RGBA{
				R: src[o], G: src[o+1], B: src[o+2], A: src[o+3],
			})
		}
	}
}

// translucent copies an image and knocks a band of it down to partial alpha, so
// the blend path is exercised on rows that the opaque fast path would take.
func translucent(src []byte, w, h int) []byte {
	out := append([]byte(nil), src...)
	for y := h / 3; y < 2*h/3; y++ {
		for x := 0; x < w; x++ {
			out[(y*w+x)*4+3] = 128
		}
	}
	return out
}

// Every route through DrawImage -- row copy, row repeat, scaled row, clipped
// span, per-pixel blend -- must land the same pixels as the loop it replaced.
func TestDrawImageMatchesTheLoopItReplaced(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		srcW, srcH, dstW, dstH int
		dstX, dstY             int
		clip                   *Rect
		tx, ty                 int
		alpha                  bool
	}{
		{name: "1:1", srcW: 16, srcH: 6, dstW: 16, dstH: 6},
		{name: "1:1 offset", srcW: 8, srcH: 4, dstW: 8, dstH: 4, dstX: 3, dstY: 2},
		{name: "enlarged 3x", srcW: 5, srcH: 4, dstW: 15, dstH: 12},
		{name: "enlarged unevenly", srcW: 5, srcH: 4, dstW: 13, dstH: 9},
		{name: "shrunk", srcW: 12, srcH: 10, dstW: 5, dstH: 3},
		{name: "wider, shorter", srcW: 4, srcH: 9, dstW: 17, dstH: 3},
		{name: "off the left and top", srcW: 8, srcH: 6, dstW: 8, dstH: 6, dstX: -3, dstY: -2},
		{name: "off the right and bottom", srcW: 8, srcH: 6, dstW: 8, dstH: 6, dstX: 15, dstY: 16},
		{name: "clipped in x only", srcW: 8, srcH: 6, dstW: 16, dstH: 12, clip: &Rect{X: 4, Y: 0, W: 6, H: 20}},
		{name: "clipped in y only", srcW: 8, srcH: 6, dstW: 16, dstH: 12, clip: &Rect{X: 0, Y: 3, W: 20, H: 5}},
		{name: "clipped in both", srcW: 8, srcH: 6, dstW: 16, dstH: 12, clip: &Rect{X: 2, Y: 3, W: 7, H: 5}},
		{name: "clipped away entirely", srcW: 8, srcH: 6, dstW: 8, dstH: 6, clip: &Rect{X: 40, Y: 40, W: 2, H: 2}},
		{name: "translated", srcW: 6, srcH: 5, dstW: 12, dstH: 10, tx: 4, ty: 3},
		{name: "translated and clipped", srcW: 6, srcH: 5, dstW: 12, dstH: 10, tx: 4, ty: 3, clip: &Rect{X: 1, Y: 1, W: 6, H: 6}},
		{name: "translucent band", srcW: 8, srcH: 9, dstW: 8, dstH: 9, alpha: true},
		{name: "translucent band enlarged", srcW: 8, srcH: 9, dstW: 16, dstH: 18, alpha: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := srcImage(tc.srcW, tc.srcH)
			if tc.alpha {
				src = translucent(src, tc.srcW, tc.srcH)
			}
			dst := Rect{X: tc.dstX, Y: tc.dstY, W: tc.dstW, H: tc.dstH}

			run := func(f func(p *PixelPainter)) *PixelPainter {
				p := newPix(20, 20)
				// A non-black ground, so a blend that wrongly behaves as a copy
				// shows up instead of hiding in zeroes.
				p.FillRect(Rect{X: 0, Y: 0, W: 20, H: 20}, RGBA{R: 40, G: 80, B: 120, A: 255})
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

			got := run(func(p *PixelPainter) { p.DrawImage(dst, src, tc.srcW, tc.srcH) })
			want := run(func(p *PixelPainter) { reference(p, dst, src, tc.srcW, tc.srcH) })

			for i := range got.Buf {
				if got.Buf[i] != want.Buf[i] {
					px := i / 4
					t.Fatalf("pixel %d,%d byte %d: DrawImage %d, the loop it replaced %d",
						px%20, px/20, i%4, got.Buf[i], want.Buf[i])
				}
			}
		})
	}
}

// A row that is not fully opaque falls out of the fast path and composites,
// so a translucent band over a background still blends.
func TestDrawImageRowWithAlphaFallsOutOfTheFastPath(t *testing.T) {
	p := newPix(2, 1)
	p.FillRect(Rect{X: 0, Y: 0, W: 2, H: 1}, RGBA{A: 255})
	src := []byte{255, 255, 255, 255, 255, 255, 255, 128}
	p.DrawImage(Rect{X: 0, Y: 0, W: 2, H: 1}, src, 2, 1)

	if got := pixAt(p, 0, 0); got.R != 255 {
		t.Errorf("opaque pixel = %+v, want white", got)
	}
	if got := pixAt(p, 1, 0); got.R == 0 || got.R == 255 {
		t.Errorf("translucent pixel = %+v, want a blend", got)
	}
}

// A Buf shorter than Width*Height*4 is tolerated rather than fatal, the same
// way PutPixel tolerates it: the blit fills what exists and stops. Both the
// row-copy path and the per-pixel path have to survive it.
func TestDrawImageShortBuffer(t *testing.T) {
	src := srcImage(4, 4)

	// Row copy: the last rows fall outside the truncated buffer.
	short := &PixelPainter{Buf: make([]byte, 4*2*4), Width: 4, Height: 4}
	short.DrawImage(Rect{X: 0, Y: 0, W: 4, H: 4}, src, 4, 4)
	if short.Buf[3] == 0 {
		t.Error("the rows that did fit were not painted")
	}

	// Per-pixel: a clip forces the slow path over the same short buffer.
	short2 := &PixelPainter{Buf: make([]byte, 4*2*4), Width: 4, Height: 4}
	short2.PushClip(Rect{X: 0, Y: 0, W: 4, H: 4})
	short2.DrawImage(Rect{X: 0, Y: 0, W: 4, H: 4}, src, 4, 4)
	short2.PopClip()
	if short2.Buf[3] == 0 {
		t.Error("the rows that did fit were not painted on the clipped path")
	}
}

// Both painters satisfy the capability, which is what widgets type-assert for.
func TestPaintersImplementImagePainter(t *testing.T) {
	var _ ImagePainter = (*PixelPainter)(nil)
	var _ ImagePainter = (*CellPainter)(nil)
}

// The measurement that justifies the primitive: the same blit, once through
// DrawImage and once the way every widget had to write it before.
func BenchmarkDrawImage(b *testing.B) {
	p := newPix(1000, 700)
	src := srcImage(1000, 700)
	dst := Rect{X: 0, Y: 0, W: 1000, H: 700}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.DrawImage(dst, src, 1000, 700)
	}
}

// The enlarging case, which is what a wallpaper or a photo in a panel actually
// does: one source row feeds several destination rows.
func BenchmarkDrawImageScaled(b *testing.B) {
	p := newPix(1000, 700)
	src := srcImage(500, 350)
	dst := Rect{X: 0, Y: 0, W: 1000, H: 700}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.DrawImage(dst, src, 500, 350)
	}
}

func BenchmarkPerPixelBlitScaled(b *testing.B) {
	p := newPix(1000, 700)
	src := srcImage(500, 350)
	var q Painter = p
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for dy := 0; dy < 700; dy++ {
			sy := dy * 350 / 700
			for dx := 0; dx < 1000; dx++ {
				o := (sy*500 + dx*500/1000) * 4
				q.PutPixel(dx, dy, RGBA{R: src[o], G: src[o+1], B: src[o+2], A: src[o+3]})
			}
		}
	}
}

// A clipped blit -- what a wallpaper cropped to its bounds, or a page scrolled
// inside a viewport, actually is. It used to fall through to the per-pixel loop
// because the clip was tested per pixel.
func BenchmarkDrawImageClipped(b *testing.B) {
	p := newPix(1000, 700)
	src := srcImage(500, 350)
	dst := Rect{X: -100, Y: -50, W: 1200, H: 800}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.PushClip(Rect{X: 0, Y: 0, W: 1000, H: 700})
		p.DrawImage(dst, src, 500, 350)
		p.PopClip()
	}
}

func BenchmarkPerPixelBlitClipped(b *testing.B) {
	p := newPix(1000, 700)
	src := srcImage(500, 350)
	var q Painter = p
	dst := Rect{X: -100, Y: -50, W: 1200, H: 800}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.PushClip(Rect{X: 0, Y: 0, W: 1000, H: 700})
		for dy := 0; dy < dst.H; dy++ {
			sy := dy * 350 / dst.H
			for dx := 0; dx < dst.W; dx++ {
				o := (sy*500 + dx*500/dst.W) * 4
				q.PutPixel(dst.X+dx, dst.Y+dy, RGBA{R: src[o], G: src[o+1], B: src[o+2], A: src[o+3]})
			}
		}
		p.PopClip()
	}
}

func BenchmarkPerPixelBlit(b *testing.B) {
	p := newPix(1000, 700)
	src := srcImage(1000, 700)
	var q Painter = p
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for dy := 0; dy < 700; dy++ {
			for dx := 0; dx < 1000; dx++ {
				o := (dy*1000 + dx) * 4
				q.PutPixel(dx, dy, RGBA{R: src[o], G: src[o+1], B: src[o+2], A: src[o+3]})
			}
		}
	}
}
