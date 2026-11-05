// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"bytes"
	"testing"
)

// A path drawn under PushTranslate(dx, dy) lands exactly where the same path
// drawn with every point moved by (dx, dy) lands, filled or stroked, lines
// and curves alike, as every other primitive does. Until this was fixed the
// translation was ignored for paths: a chart inside a scrolled view was drawn
// where it would have been unscrolled.
func TestPathsFollowTheTranslation(t *testing.T) {
	const w, h = 48, 40
	shape := func(dx, dy float64) *Path {
		return NewPath().
			MoveTo(4+dx, 4+dy).LineTo(20+dx, 6+dy).
			QuadTo(26+dx, 14+dy, 18+dx, 20+dy).
			CubicTo(12+dx, 24+dy, 6+dx, 18+dy, 5+dx, 12+dy).
			Close()
	}
	ink := RGBA{R: 10, G: 120, B: 230, A: 200}
	for _, d := range [][2]int{{13, 9}, {-3, 7}, {0, 0}, {30, -10}} {
		for _, stroke := range []bool{false, true} {
			draw := func(translated bool) []byte {
				buf := make([]byte, 4*w*h)
				p := NewPixelPainter(buf, w, h)
				pth := shape(float64(d[0]), float64(d[1]))
				if translated {
					p.PushTranslate(d[0], d[1])
					pth = shape(0, 0)
				}
				if stroke {
					p.StrokePath(pth, ink, 2.5)
				} else {
					p.FillPath(pth, ink, NonZero)
				}
				return buf
			}
			want, got := draw(false), draw(true)
			if !bytes.Equal(want, got) {
				t.Errorf("translate %v, stroke %v: the translated path is not where the moved path is", d, stroke)
			}
			if bytes.Equal(got, make([]byte, len(got))) && d != [2]int{30, -10} {
				t.Errorf("translate %v, stroke %v: nothing drawn", d, stroke)
			}
		}
	}
}

// The translation applies to the path, not to its clip a second time: a clip
// pushed under a translation is already in surface pixels.
func TestTranslatedPathUnderTranslatedClip(t *testing.T) {
	const w, h = 32, 32
	draw := func(translated bool) []byte {
		buf := make([]byte, 4*w*h)
		p := NewPixelPainter(buf, w, h)
		off := 0.0
		if translated {
			p.PushTranslate(8, 8)
		} else {
			off = 8
		}
		p.PushClip(Rect{X: 2, Y: 2, W: 10, H: 10}) // (10,10)-(20,20) when translated
		if !translated {
			p.PopClip()
			p.PushClip(Rect{X: 10, Y: 10, W: 10, H: 10})
		}
		// A 6-pixel square at (off+3, off+3): it overlaps the clip only once
		// moved by 8, so an ignored translation draws nothing at all.
		p.FillPath(NewPath().MoveTo(off+3, off+3).LineTo(off+9, off+3).LineTo(off+9, off+9).LineTo(off+3, off+9).Close(), RGBA{A: 255}, NonZero)
		return buf
	}
	want, got := draw(false), draw(true)
	if bytes.Equal(want, make([]byte, len(want))) {
		t.Fatal("the moved path drew nothing under the clip: the test proves nothing")
	}
	if !bytes.Equal(want, got) {
		t.Fatal("a translated path under a translated clip differs from the moved path under the moved clip")
	}
}
