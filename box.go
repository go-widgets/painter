// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"sync"

	"github.com/go-simd/boxfilter"
)

// A picture shown smaller than it is must be AVERAGED, not sampled: taking one
// source pixel in every n makes a 256 px icon shown at 24 px alias visibly (a
// one pixel line vanishes or doubles). The box filter below gives every
// destination pixel the exact area-weighted mean of the source pixels it
// covers, with fractional edge weights when the ratio is not an integer.
//
// The mean is taken on PREMULTIPLIED colour (r*a, g*a, b*a) and alpha, then
// divided back out. Averaging straight colour would pull the colour of fully
// transparent pixels (usually black) into a soft edge and leave a dark fringe;
// weighting by alpha keeps the colour of what is actually visible.
//
// Each axis is treated alone: an axis that is shown larger than, or as large as,
// the source keeps the nearest-neighbour mapping (one source pixel, weight 1),
// so an enlargement is unchanged and a picture squeezed in one direction only
// is averaged in that direction only.
//
// The arithmetic is INTEGER and EXACT. Reduced by their gcd, the source is srcN
// units wide and a destination pixel dstN units; the share of source pixel i in
// destination pixel x is the integer overlap of [i*dstN, (i+1)*dstN) with
// [x*srcN, (x+1)*srcN), the shares of one x summing to srcN. A source pixel is
// premultiplied to at most 255*255, so the weighted sums are exact integers and
// the only rounding left is the final division, rounded half up: every byte
// differs from the real-number mean by at most one half, which the tests assert
// against a float64 oracle.
//
// The work is two separable passes, VERTICAL FIRST. The rows a destination row
// covers are premultiplied and accumulated, at their weights, into one row of
// uint32 by the go-simd/boxfilter kernels (NEON or AVX2 where the CPU has them);
// then each destination pixel sums its span of that row, the interior pixels of a
// span carrying one common weight so they are added first and multiplied once.
// When the horizontal ratio is an integer (every weight 1) the second pass is
// the SumGroups kernel as well, and when the vertical ratio is a whole number (or
// there is no vertical reduction) both passes are one SumBlocks call that keeps
// the sums in registers. uint32 holds the sums while the total vertical
// weight is at most boxMaxWeight (65025 times it stays below 2^32); a taller
// reduction than that takes a uint64 scalar path with the same results. The
// scratch is pooled: a steady-state call does not allocate.

// boxMaxWeight is the largest total weight whose sum of 255*255 products fits a
// uint32: 65025 * 66051 < 2^32.
const boxMaxWeight = 66051

// boxAxis is the source span one destination index covers on one axis.
type boxAxis struct {
	srcN, dstN int64 // reduced by their gcd
	down       bool
}

func newBoxAxis(srcN, dstN int) boxAxis {
	s, d := int64(srcN), int64(dstN)
	if dstN < srcN {
		g := gcd64(s, d)
		return boxAxis{srcN: s / g, dstN: d / g, down: true}
	}
	return boxAxis{srcN: s, dstN: d}
}

func gcd64(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// norm is the sum of the weights of one destination index.
func (a boxAxis) norm() uint64 {
	if !a.down {
		return 1
	}
	return uint64(a.srcN)
}

// boxSpan is the source indices [i0, i1) one destination index covers and their
// integer weights: w0 for i0, w1 for i1-1 (unused when the span is one index) and
// wm for every index between. x*srcN is int64: it reaches 2^58 for a 2^29 pixel
// side and would wrap a 32-bit int.
type boxSpan struct {
	i0, i1     int
	w0, wm, w1 uint64
}

func (a boxAxis) span(x int) boxSpan {
	if !a.down {
		i := int(int64(x) * a.srcN / a.dstN)
		return boxSpan{i0: i, i1: i + 1, w0: 1, wm: 1, w1: 1}
	}
	lo, hi := int64(x)*a.srcN, int64(x+1)*a.srcN
	i0 := int(lo / a.dstN)
	i1 := int((hi + a.dstN - 1) / a.dstN)
	s := boxSpan{i0: i0, i1: i1, wm: uint64(a.dstN)}
	s.w0 = uint64(min(hi, int64(i0+1)*a.dstN) - lo)
	if i1-i0 > 1 {
		s.w1 = uint64(hi - max(lo, int64(i1-1)*a.dstN))
	}
	return s
}

// boxScratch is the per-call working memory, pooled across calls.
type boxScratch struct {
	xs  []boxSpan
	v32 []uint32 // the vertical row, narrow path
	v64 []uint64 // the vertical row, wide path
	g32 []uint32 // the SumGroups result
}

var boxPool = sync.Pool{New: func() any { return new(boxScratch) }}

// grow returns s resized to n, reallocating only when it is too small.
func grow[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	return s[:n]
}

// boxVWide adds w times the premultiplied pixels of row to v, in uint64: the
// scalar path for a reduction too tall for uint32.
func boxVWide(v []uint64, row []byte, w uint64) {
	for j := 0; j+4 <= len(row); j += 4 {
		a := uint64(row[j+3])
		v[j] += uint64(row[j]) * a * w
		v[j+1] += uint64(row[j+1]) * a * w
		v[j+2] += uint64(row[j+2]) * a * w
		v[j+3] += a * w
	}
}

// boxHSpan sums the premultiplied pixels of one span of v (whose index 0 is
// source column lo) at the span's weights: first pixel, the interior at one
// common weight, last pixel.
func boxHSpan[T uint32 | uint64](v []T, s *boxSpan, lo int) (r, g, b, a uint64) {
	o := (s.i0 - lo) * 4
	r, g, b, a = uint64(v[o])*s.w0, uint64(v[o+1])*s.w0, uint64(v[o+2])*s.w0, uint64(v[o+3])*s.w0
	last := s.i1 - 1
	var mr, mg, mb, ma uint64
	for j := s.i0 + 1; j < last; j++ {
		p := v[(j-lo)*4 : (j-lo)*4+4 : (j-lo)*4+4]
		mr += uint64(p[0])
		mg += uint64(p[1])
		mb += uint64(p[2])
		ma += uint64(p[3])
	}
	r, g, b, a = r+mr*s.wm, g+mg*s.wm, b+mb*s.wm, a+ma*s.wm
	if last > s.i0 {
		p := v[(last-lo)*4 : (last-lo)*4+4 : (last-lo)*4+4]
		r += uint64(p[0]) * s.w1
		g += uint64(p[1]) * s.w1
		b += uint64(p[2]) * s.w1
		a += uint64(p[3]) * s.w1
	}
	return r, g, b, a
}

// drawImageBox draws the part eff of dst (the whole scaled picture) averaging
// the source under every destination pixel. The caller has checked the sizes
// and that src holds srcW*srcH*4 bytes.
func (p *PixelPainter) drawImageBox(dst, eff Rect, src []byte, srcW, srcH int) {
	boxReduce(dst, eff, src, srcW, srcH, func(y, xi int, c RGBA) {
		dOff := y*p.Width*4 + (eff.X+xi)*4
		if dOff+4 > len(p.Buf) {
			return // a Buf shorter than Width*Height*4 is tolerated, like PutPixel
		}
		if c.A == 0xFF && !p.swap {
			p.Buf[dOff], p.Buf[dOff+1], p.Buf[dOff+2], p.Buf[dOff+3] = c.R, c.G, c.B, 0xFF
		} else {
			p.blendInto(dOff, c)
		}
	})
}

// boxReduce averages the source under every pixel of eff (not empty) and hands
// each result with alpha >= 1 to put as (surface row y, column xi within eff).
func boxReduce(dst, eff Rect, src []byte, srcW, srcH int, put func(y, xi int, c RGBA)) {
	ax, ay := newBoxAxis(srcW, dst.W), newBoxAxis(srcH, dst.H)
	sc := boxPool.Get().(*boxScratch)
	defer boxPool.Put(sc)
	sc.xs = grow(sc.xs, eff.W)
	for xi := range sc.xs {
		sc.xs[xi] = ax.span(eff.X - dst.X + xi)
	}
	// Spans only move right: the columns any of them reads are [lo, hi).
	lo, hi := sc.xs[0].i0, sc.xs[eff.W-1].i1
	norm := ax.norm() * ay.norm()
	wide := ay.norm() > boxMaxWeight
	// Whole groups of srcN source pixels, every weight 1, and the sums still
	// fit a uint32: the horizontal pass is one SumGroups call.
	groups := !wide && ax.down && ax.dstN == 1 && norm <= boxMaxWeight
	// ... and the rows too: a whole-number vertical ratio (or no reduction, one
	// row) is the SumBlocks kernel, which sums the n x m blocks in registers.
	blocks := groups && (ay.dstN == 1 || !ay.down)
	if wide {
		sc.v64 = grow(sc.v64, 4*(hi-lo))
	} else {
		sc.v32 = grow(sc.v32, 4*(hi-lo))
		sc.g32 = grow(sc.g32, 4*eff.W)
	}

	emit := func(y, xi int, r, g, b, a uint64) {
		if 2*a < norm {
			return // alpha rounds to 0: nothing to draw
		}
		put(y, xi, RGBA{
			R: boxDiv(r, a),
			G: boxDiv(g, a),
			B: boxDiv(b, a),
			A: uint8((2*a + norm) / (2 * norm)),
		})
	}

	for y := eff.Y; y < eff.Y+eff.H; y++ {
		vs := ay.span(y - dst.Y)
		last := vs.i1 - 1
		weight := func(i int) uint64 {
			switch i {
			case vs.i0:
				return vs.w0
			case last:
				return vs.w1
			}
			return vs.wm
		}
		row := func(i int) []byte { return src[(i*srcW+lo)*4 : (i*srcW+hi)*4] }
		if wide {
			clear(sc.v64)
			for i := vs.i0; i < vs.i1; i++ {
				boxVWide(sc.v64, row(i), weight(i))
			}
			for xi := range sc.xs {
				r, g, b, a := boxHSpan(sc.v64, &sc.xs[xi], lo)
				emit(y, xi, r, g, b, a)
			}
			continue
		}
		if blocks {
			// Every weight 1 on both axes: one pass, no accumulator row.
			boxfilter.SumBlocks(sc.g32, src[(vs.i0*srcW+lo)*4:], srcW*4, int(ax.srcN), vs.i1-vs.i0)
			for xi := range sc.xs {
				o := xi * 4
				emit(y, xi, uint64(sc.g32[o]), uint64(sc.g32[o+1]), uint64(sc.g32[o+2]), uint64(sc.g32[o+3]))
			}
			continue
		}
		clear(sc.v32)
		for i := vs.i0; i < vs.i1; i++ {
			boxfilter.AccumPremul(sc.v32, row(i), uint32(weight(i)))
		}
		if groups {
			boxfilter.SumGroups(sc.g32, sc.v32, int(ax.srcN))
			for xi := range sc.xs {
				o := xi * 4
				emit(y, xi, uint64(sc.g32[o]), uint64(sc.g32[o+1]), uint64(sc.g32[o+2]), uint64(sc.g32[o+3]))
			}
			continue
		}
		for xi := range sc.xs {
			r, g, b, a := boxHSpan(sc.v32, &sc.xs[xi], lo)
			emit(y, xi, r, g, b, a)
		}
	}
}

// boxDiv is num/den rounded half up; num <= 255*den by construction.
func boxDiv(num, den uint64) uint8 {
	return uint8((2*num + den) / (2 * den))
}
