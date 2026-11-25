// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

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

// boxAxis is the source span one destination index covers on one axis.
type boxAxis struct {
	srcN, dstN int64
	down       bool
}

func newBoxAxis(srcN, dstN int) boxAxis {
	return boxAxis{srcN: int64(srcN), dstN: int64(dstN), down: dstN < srcN}
}

// span returns the source indices [i0, i1) destination index x covers. int64
// throughout: x*srcN reaches 2^58 for a 2^29 pixel side and would wrap a 32-bit
// int.
func (a boxAxis) span(x int) (i0, i1 int) {
	if !a.down {
		i := int(int64(x) * a.srcN / a.dstN)
		return i, i + 1
	}
	i0 = int(int64(x) * a.srcN / a.dstN)
	i1 = int((int64(x+1)*a.srcN + a.dstN - 1) / a.dstN)
	return i0, i1
}

// weight is source index i's share of destination index x, the shares of one x
// summing to 1.
func (a boxAxis) weight(x, i int) float64 {
	if !a.down {
		return 1
	}
	lo, hi := int64(i)*a.dstN, int64(i+1)*a.dstN
	if l := int64(x) * a.srcN; l > lo {
		lo = l
	}
	if h := int64(x+1) * a.srcN; h < hi {
		hi = h
	}
	return float64(hi-lo) / float64(a.srcN)
}

// drawImageBox draws the part eff of dst (the whole scaled picture) averaging
// the source under every destination pixel. The caller has checked the sizes
// and that src holds srcW*srcH*4 bytes.
func (p *PixelPainter) drawImageBox(dst, eff Rect, src []byte, srcW, srcH int) {
	ax, ay := newBoxAxis(srcW, dst.W), newBoxAxis(srcH, dst.H)
	n := eff.W * 4
	hrow := make([]float64, n)
	vacc := make([]float64, n)
	lastSY := -1

	// hpass reduces source row sy to eff.W premultiplied sums in hrow.
	hpass := func(sy int) {
		row := src[sy*srcW*4 : (sy+1)*srcW*4]
		for xi := 0; xi < eff.W; xi++ {
			j0, j1 := ax.span(eff.X - dst.X + xi)
			var pr, pg, pb, pa float64
			for j := j0; j < j1; j++ {
				w := ax.weight(eff.X-dst.X+xi, j)
				a := float64(row[j*4+3])
				pr += w * float64(row[j*4]) * a
				pg += w * float64(row[j*4+1]) * a
				pb += w * float64(row[j*4+2]) * a
				pa += w * a
			}
			o := xi * 4
			hrow[o], hrow[o+1], hrow[o+2], hrow[o+3] = pr, pg, pb, pa
		}
	}

	for y := eff.Y; y < eff.Y+eff.H; y++ {
		dy := y - dst.Y
		dstRow := y * p.Width * 4
		if dstRow+(eff.X+eff.W)*4 > len(p.Buf) {
			continue
		}
		i0, i1 := ay.span(dy)
		for k := range vacc {
			vacc[k] = 0
		}
		for i := i0; i < i1; i++ {
			if i != lastSY {
				hpass(i)
				lastSY = i
			}
			w := ay.weight(dy, i)
			for k := range vacc {
				vacc[k] += w * hrow[k]
			}
		}
		for xi := 0; xi < eff.W; xi++ {
			o := xi * 4
			pa := vacc[o+3]
			if pa < 0.5 {
				continue // alpha rounds to 0: nothing to draw
			}
			c := RGBA{
				R: boxByte(vacc[o] / pa),
				G: boxByte(vacc[o+1] / pa),
				B: boxByte(vacc[o+2] / pa),
				A: boxByte(pa),
			}
			dOff := dstRow + (eff.X+xi)*4
			if c.A == 0xFF && !p.swap {
				p.Buf[dOff], p.Buf[dOff+1], p.Buf[dOff+2], p.Buf[dOff+3] = c.R, c.G, c.B, 0xFF
			} else {
				p.blendInto(dOff, c)
			}
		}
	}
}

// boxByte rounds v to the nearest byte, clamped to 0..255.
func boxByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
