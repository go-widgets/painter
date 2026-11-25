// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// IconSize is the size a srcW x srcH picture is shown at inside a boxW x boxH
// box with pad pixels of margin on every side: its own size when it fits,
// otherwise the largest size of its aspect that fits. It never scales up and
// never returns less than 1 on a side. A box with no room left (boxW or boxH
// not larger than twice pad) leaves the picture's own size.
func IconSize(srcW, srcH, boxW, boxH, pad int) (w, h int) {
	if srcW <= 0 || srcH <= 0 {
		return 0, 0
	}
	// Clamp first so the arithmetic below cannot overflow, whatever the caller
	// passes; a box larger than the picture changes nothing (never scales up).
	const limit = 1 << 30
	pad = min(max(pad, 0), limit)
	boxW, boxH = min(max(boxW, 0), limit), min(max(boxH, 0), limit)
	maxW, maxH := min(boxW-2*pad, srcW), min(boxH-2*pad, srcH)
	if maxW <= 0 || maxH <= 0 || (srcW <= maxW && srcH <= maxH) {
		return srcW, srcH
	}
	w = maxW
	h = srcH * w / srcW
	if h > maxH {
		h = maxH
		w = srcW * h / srcH
	}
	return max(w, 1), max(h, 1)
}

// IconBGRAOver renders a straight-alpha RGBA picture (srcW x srcH, 4 bytes per
// pixel) at [IconSize] over the opaque background bg, and returns it as
// top-down BGRA with alpha 255: the layout of a 32-bit Windows DIB section, a
// native button's picture. Sampling is [PixelPainter.DrawImage]'s
// (nearest-neighbour) and the blend is the painter's, so there is one
// implementation of both.
//
// Because the result is never larger than the source, the output is bounded by
// the input: a src shorter than srcW*srcH*4, or a non-positive size, answers
// (0, 0, nil) rather than reading past its end.
func IconBGRAOver(src []byte, srcW, srcH, boxW, boxH, pad int, bg RGBA) (w, h int, bgra []byte) {
	if srcW <= 0 || srcH <= 0 || srcW > len(src)/4/srcH {
		return 0, 0, nil
	}
	w, h = IconSize(srcW, srcH, boxW, boxH, pad)
	bgra = make([]byte, w*h*4)
	p := NewPixelPainterBGRA(bgra, w, h)
	whole := Rect{W: w, H: h}
	p.FillRect(whole, RGB(bg.R, bg.G, bg.B))
	p.DrawImage(whole, src, srcW, srcH)
	return w, h, bgra
}
