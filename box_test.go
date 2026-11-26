// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"math"
	"math/rand"
	"testing"
)

func checker(w, h int) []byte {
	b := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := byte(0)
			if (x+y)%2 == 0 {
				v = 255
			}
			o := (y*w + x) * 4
			b[o], b[o+1], b[o+2], b[o+3] = v, v, v, 255
		}
	}
	return b
}

// A one pixel checkerboard shrunk by an integer factor n averages every n x n
// block: n even gives exactly half white, n odd gives (n*n+1)/2 of one colour,
// so the mean grey is 127.5 off by at most 255/(2*n*n) before rounding. The
// stated bound is that plus one level of rounding.
func TestBoxDownscaleCheckerboardIsMeanGrey(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 8, 10} {
		const out = 12
		src := checker(out*n, out*n)
		p := newPix(out, out)
		p.DrawImage(Rect{W: out, H: out}, src, out*n, out*n)
		bound := 255.0/(2*float64(n*n)) + 1
		for y := 0; y < out; y++ {
			for x := 0; x < out; x++ {
				c := pixAt(p, x, y)
				for _, v := range []byte{c.R, c.G, c.B} {
					if d := math.Abs(float64(v) - 127.5); d > bound {
						t.Fatalf("n=%d (%d,%d): %d is %.1f from 127.5, bound %.1f", n, x, y, v, d, bound)
					}
				}
				if c.A != 255 {
					t.Fatalf("n=%d: alpha %d, want 255", n, c.A)
				}
			}
		}
	}
}

// boxOracle is the area mean written the plain way (a coverage product per
// source pixel, in float64) to check the separable implementation against.
func boxOracle(src []byte, sw, sh, dw, dh, x, y int) (r, g, b, a float64) {
	sx0, sx1 := float64(x)*float64(sw)/float64(dw), float64(x+1)*float64(sw)/float64(dw)
	sy0, sy1 := float64(y)*float64(sh)/float64(dh), float64(y+1)*float64(sh)/float64(dh)
	var tot, pr, pg, pb, pa float64
	for j := int(sy0); float64(j) < sy1; j++ {
		for i := int(sx0); float64(i) < sx1; i++ {
			w := (math.Min(sx1, float64(i+1)) - math.Max(sx0, float64(i))) *
				(math.Min(sy1, float64(j+1)) - math.Max(sy0, float64(j)))
			o := (j*sw + i) * 4
			al := float64(src[o+3])
			tot += w
			pr += w * float64(src[o]) * al
			pg += w * float64(src[o+1]) * al
			pb += w * float64(src[o+2]) * al
			pa += w * al
		}
	}
	return pr / pa, pg / pa, pb / pa, pa / tot
}

// Non-integer ratios agree with the oracle to one level on every channel.
func TestBoxDownscaleMatchesAnOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, c := range [][4]int{{10, 10, 4, 4}, {37, 23, 11, 7}, {64, 64, 24, 24}, {100, 30, 100, 9}, {30, 100, 7, 100}} {
		sw, sh, dw, dh := c[0], c[1], c[2], c[3]
		src := make([]byte, sw*sh*4)
		for i := range src {
			src[i] = byte(rng.Intn(256))
		}
		for i := 3; i < len(src); i += 4 {
			src[i] = 255 // opaque: the result is then the plain colour mean
		}
		p := newPix(dw, dh)
		p.DrawImage(Rect{W: dw, H: dh}, src, sw, sh)
		for y := 0; y < dh; y++ {
			for x := 0; x < dw; x++ {
				r, g, b, _ := boxOracle(src, sw, sh, dw, dh, x, y)
				got := pixAt(p, x, y)
				for k, pair := range [][2]float64{{r, float64(got.R)}, {g, float64(got.G)}, {b, float64(got.B)}} {
					if math.Abs(pair[0]-pair[1]) > 1 {
						t.Fatalf("%v (%d,%d) ch%d: got %v, oracle %.2f", c, x, y, k, pair[1], pair[0])
					}
				}
			}
		}
	}
}

// A white shape on a transparent BLACK ground, shrunk onto a white ground, must
// come out white everywhere: averaging straight colour would drag the black of
// the invisible pixels in and leave a dark fringe on the edge.
func TestBoxDownscaleAlphaEdgeHasNoDarkFringe(t *testing.T) {
	const sw, sh, dw, dh = 40, 40, 7, 7 // 40/7: the edge falls mid-pixel
	src := make([]byte, sw*sh*4)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			o := (y*sw + x) * 4
			if x < 17 {
				src[o], src[o+1], src[o+2], src[o+3] = 255, 255, 255, 255
			} // else 0,0,0,0
		}
	}
	p := newPix(dw, dh)
	for i := range p.Buf {
		p.Buf[i] = 255
	}
	p.DrawImage(Rect{W: dw, H: dh}, src, sw, sh)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			if c := pixAt(p, x, y); c.R < 254 || c.G < 254 || c.B < 254 {
				t.Fatalf("(%d,%d) = %v: a dark fringe", x, y, c)
			}
		}
	}
	// And the soft edge really is partial alpha, not a hard cut: over a black
	// ground the edge column lies strictly between black and white.
	q := newPix(dw, dh)
	q.DrawImage(Rect{W: dw, H: dh}, src, sw, sh)
	edge := pixAt(q, 2, 3) // columns 14.3..20 of 40 straddle x=17
	if edge.R == 0 || edge.R == 255 {
		t.Fatalf("edge pixel %v is not a partial blend", edge)
	}
}

// Only a reduced axis is averaged; an enlarged one keeps nearest sampling.
func TestBoxOneAxisOnly(t *testing.T) {
	src := srcImage(8, 2) // red = column, green = row
	p := newPix(8, 4)     // width unchanged, height doubled: nothing shrinks
	p.DrawImage(Rect{W: 8, H: 4}, src, 8, 2)
	if got := pixAt(p, 5, 3); got.R != 5 || got.G != 1 {
		t.Fatalf("enlarge changed: %v", got)
	}
	q := newPix(4, 4) // width halves (averaged), height doubles (nearest)
	q.DrawImage(Rect{W: 4, H: 4}, src, 8, 2)
	// Destination column 1 covers source columns 2 and 3: red 2.5 -> 3 (round
	// half up); destination row 3 is source row 1, green exactly 1.
	if got := pixAt(q, 1, 3); got.R != 3 || got.G != 1 {
		t.Fatalf("got %v, want red 3 (mean of 2,3) and green 1 (nearest row)", got)
	}
}

// An axis enlarged by a NON-integer ratio keeps the exact nearest-neighbour
// mapping while the other axis is averaged: a window over 3 source rows shown on
// 4 would blend neighbouring rows if the enlarged axis were boxed too.
func TestBoxEnlargedAxisStaysNearestWhenTheOtherShrinks(t *testing.T) {
	// Height 3 -> 4, width 8 -> 4: green is the source row (0,1,2).
	// Bright values: a wrongly partial weight shows as a dimmer pixel.
	bright := func(w, h int) []byte {
		b := srcImage(w, h)
		for i := 0; i < len(b); i += 4 {
			b[i], b[i+1] = 100+b[i]*50, 100+b[i+1]*50
		}
		return b
	}
	p := newPix(4, 4)
	p.DrawImage(Rect{W: 4, H: 4}, bright(8, 3), 8, 3)
	for dy := 0; dy < 4; dy++ {
		if got, want := pixAt(p, 1, dy).G, byte(100+dy*3/4*50); got != want {
			t.Fatalf("row %d: green %d, want %d (nearest)", dy, got, want)
		}
	}
	// Width 3 -> 4, height 8 -> 4: red is the source column (0,1,2); the srcImage
	// green channel (the row) is averaged, so only red is the nearest column.
	q := newPix(4, 4)
	q.DrawImage(Rect{W: 4, H: 4}, bright(3, 8), 3, 8)
	for dx := 0; dx < 4; dx++ {
		if got, want := pixAt(q, dx, 1).R, byte(100+dx*3/4*50); got != want {
			t.Fatalf("column %d: red %d, want %d (nearest)", dx, got, want)
		}
	}
}

// A Buf shorter than Width*Height*4 is tolerated like PutPixel tolerates it: the
// rows that do not fit are skipped, not fatal.
func TestBoxDownscaleSkipsRowsBeyondAShortBuf(t *testing.T) {
	p := newPix(8, 8)
	p.Buf = p.Buf[:8*4*3] // three rows of eight
	p.DrawImage(Rect{W: 8, H: 8}, checker(16, 16), 16, 16)
	if got := pixAt(p, 0, 2); got.A != 255 {
		t.Fatalf("a row that fits was not drawn: %v", got)
	}
}

// FuzzDrawImageBox: any sizes, clip and translation: no panic, nothing written
// outside the clip, and a short buffer is ignored.
func FuzzDrawImageBox(f *testing.F) {
	f.Add(uint8(64), uint8(64), uint8(5), uint8(5), int8(0), int8(0), uint8(10), uint8(10), int64(1))
	f.Add(uint8(3), uint8(200), uint8(1), uint8(9), int8(-4), int8(7), uint8(3), uint8(30), int64(2))
	f.Fuzz(func(t *testing.T, sw, sh, dw, dh uint8, dx, dy int8, cw, ch uint8, seed int64) {
		w, h := int(sw), int(sh)
		if w == 0 || h == 0 {
			return
		}
		rng := rand.New(rand.NewSource(seed))
		src := make([]byte, w*h*4)
		rng.Read(src)
		const sz = 40
		p := newPix(sz, sz)
		before := append([]byte(nil), p.Buf...)
		clip := Rect{X: 5, Y: 5, W: int(cw) % 30, H: int(ch) % 30}
		p.PushClip(clip)
		p.DrawImage(Rect{X: int(dx), Y: int(dy), W: int(dw), H: int(dh)}, src, w, h)
		p.PopClip()
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				in := x >= clip.X && x < clip.X+clip.W && y >= clip.Y && y < clip.Y+clip.H
				o := (y*sz + x) * 4
				if !in && (p.Buf[o] != before[o] || p.Buf[o+1] != before[o+1] || p.Buf[o+2] != before[o+2] || p.Buf[o+3] != before[o+3]) {
					t.Fatalf("pixel (%d,%d) outside the clip %v was written", x, y, clip)
				}
			}
		}
		// A short source is ignored, never read past.
		p.DrawImage(Rect{W: int(dw), H: int(dh)}, src[:len(src)/2], w, h)
	})
}

func benchDown(b *testing.B, sw, sh, dw, dh int) {
	p := newPix(dw, dh)
	src := srcImage(sw, sh)
	dst := Rect{W: dw, H: dh}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.DrawImage(dst, src, sw, sh)
	}
}

func BenchmarkDrawImageDownscale256to24(b *testing.B)   { benchDown(b, 256, 256, 24, 24) }
func BenchmarkDrawImageDownscale64to16(b *testing.B)    { benchDown(b, 64, 64, 16, 16) }
func BenchmarkDrawImageDownscale1000to250(b *testing.B) { benchDown(b, 1000, 700, 250, 175) }

// boxBound is the STATED error bound of the box filter: the arithmetic is exact
// integer, so every byte is the real-number area mean rounded half up, at most
// one half away from it (the epsilon absorbs the float64 rounding of the oracle).
const boxBound = 0.5 + 1e-6

// boxCheck runs boxReduce on src and compares every destination pixel, colour
// and alpha, with the float64 oracle, translucent pixels included.
func boxCheck(t testing.TB, src []byte, sw, sh, dw, dh int) {
	t.Helper()
	seen := make([]bool, dw*dh)
	boxReduce(Rect{W: dw, H: dh}, Rect{W: dw, H: dh}, src, sw, sh, func(y, x int, c RGBA) {
		seen[y*dw+x] = true
		r, g, b, a := boxOracle(src, sw, sh, dw, dh, x, y)
		for k, pair := range [][2]float64{{r, float64(c.R)}, {g, float64(c.G)}, {b, float64(c.B)}, {a, float64(c.A)}} {
			if math.Abs(pair[0]-pair[1]) > boxBound {
				t.Fatalf("%dx%d->%dx%d (%d,%d) ch%d: got %v, oracle %.4f, bound %.1f", sw, sh, dw, dh, x, y, k, pair[1], pair[0], boxBound)
			}
		}
	})
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			if _, _, _, a := boxOracle(src, sw, sh, dw, dh, x, y); !seen[y*dw+x] && a >= 0.5+1e-6 {
				t.Fatalf("%dx%d->%dx%d (%d,%d): skipped, oracle alpha %.4f", sw, sh, dw, dh, x, y, a)
			}
		}
	}
}

// The bound holds on random TRANSLUCENT pictures, integer and fractional ratios,
// one axis or both, down to a single pixel.
func TestBoxExactWithinStatedBound(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for _, c := range [][4]int{{64, 64, 16, 16}, {256, 256, 24, 24}, {37, 23, 11, 7}, {10, 10, 1, 1},
		{9, 6, 6, 4}, {100, 30, 100, 9}, {30, 100, 7, 100}, {17, 17, 16, 16}, {300, 5, 299, 5},
		// integer ratio too heavy for the uint32 SumGroups path
		{300, 300, 1, 1}, {300, 4, 100, 2},
		// whole-number horizontal ratio: SumBlocks with a whole-number or no vertical
		// reduction, SumGroups when the vertical ratio is fractional
		{64, 32, 16, 8}, {64, 10, 16, 10}, {64, 30, 16, 7}, {12, 9, 4, 3},
		// a reduction taller than boxMaxWeight takes the uint64 path
		{3, 70000, 2, 3}, {1, 66052, 1, 5}} {
		src := make([]byte, c[0]*c[1]*4)
		rng.Read(src)
		boxCheck(t, src, c[0], c[1], c[2], c[3])
	}
}

// FuzzBoxVsOracle: any translucent picture and any reduced size agrees with the
// float64 oracle within the stated bound.
func FuzzBoxVsOracle(f *testing.F) {
	f.Add(uint8(40), uint8(30), uint8(7), uint8(5), int64(1))
	f.Add(uint8(64), uint8(64), uint8(16), uint8(16), int64(2))
	f.Fuzz(func(t *testing.T, sw, sh, dw, dh uint8, seed int64) {
		// An enlarged axis is nearest, not averaged, which the oracle does not model:
		// the destination is clamped to the source on both axes.
		w, h := int(sw)%97+1, int(sh)%97+1
		ow, oh := min(int(dw)%97+1, w), min(int(dh)%97+1, h)
		src := make([]byte, w*h*4)
		rand.New(rand.NewSource(seed)).Read(src)
		boxCheck(t, src, w, h, ow, oh)
	})
}

// noPanic fails the test, by assertion, if f panics.
func noPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked: %v", what, r)
		}
	}()
	f()
}

func TestBoxGrowReusesWhatFits(t *testing.T) {
	big := make([]int, 2, 8)
	got := grow(big, 5)
	if len(got) != 5 || &got[0] != &big[0] {
		t.Fatalf("grow within the capacity: len %d, reused %v", len(got), &got[0] == &big[0])
	}
	small := make([]int, 2)
	if got := grow(small, 5); len(got) != 5 || cap(got) < 5 {
		t.Fatalf("grow past the capacity: len %d cap %d", len(got), cap(got))
	}
	if got := grow(small, 2); len(got) != 2 || &got[0] != &small[0] {
		t.Fatal("grow to exactly the capacity must reuse")
	}
}

// The pooled scratch is resized between calls of different sizes.
func TestBoxScratchSurvivesSizeChanges(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for _, c := range [][4]int{{200, 200, 50, 50}, {20, 20, 5, 5}, {200, 100, 100, 50}, {8, 8, 4, 4}} {
		src := make([]byte, c[0]*c[1]*4)
		rng.Read(src)
		boxCheck(t, src, c[0], c[1], c[2], c[3])
	}
}

// A destination pixel whose alpha rounds to nothing is not drawn.
func TestBoxSkipsTransparentPixels(t *testing.T) {
	src := make([]byte, 8*8*4) // fully transparent
	n := 0
	noPanic(t, "transparent source", func() {
		boxReduce(Rect{W: 2, H: 2}, Rect{W: 2, H: 2}, src, 8, 8, func(int, int, RGBA) { n++ })
	})
	if n != 0 {
		t.Fatalf("%d pixels drawn from a transparent source", n)
	}
	src[3] = 1 // one source pixel of alpha 1 among 16: mean 1/16, rounds to 0
	noPanic(t, "faint source", func() {
		boxReduce(Rect{W: 2, H: 2}, Rect{W: 2, H: 2}, src, 8, 8, func(int, int, RGBA) { n++ })
	})
	if n != 0 {
		t.Fatalf("alpha 1/16 drew %d pixels", n)
	}
}

// A buffer shorter than Width*Height*4 is tolerated.
func TestBoxShortBufIsTolerated(t *testing.T) {
	p := newPix(8, 8)
	p.Buf = p.Buf[:10]
	src := make([]byte, 32*32*4)
	for i := range src {
		src[i] = 0xFF
	}
	noPanic(t, "short Buf", func() { p.DrawImage(Rect{W: 8, H: 8}, src, 32, 32) })
	if p.Buf[0] != 0xFF || p.Buf[4] != 0xFF {
		t.Fatalf("the pixels that fit were not drawn: %v", p.Buf[:8])
	}
}
