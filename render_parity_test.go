// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"math"
	"testing"
)

// TestWidgetSceneRenderParity is the migration gate: a large widget-like scene
// composed of EVERY PixelPainter primitive — rounded buttons, a framed panel, a
// bar chart, a line chart (vector StrokePath), a filled star icon (vector
// FillPath, both winding rules), an image thumbnail, a glyph coverage mask, and a
// table grid, under clips and a translation, over a translucent wash — is
// rendered TWICE onto identical fresh buffers: once through the migrated public
// FillPath / StrokePath (which consume go-gfx/gfx/vector), and once through a
// hermetic, verbatim copy of the PRE-migration rasterizer + composite
// (render_refimpl_test.go). Every non-path primitive is unchanged code called
// identically in both passes, so any byte difference can only come from the
// rasterizer swap.
//
// It asked for BYTE EQUALITY until go-gfx/gfx v0.10.0, and that was right while
// the migration was the thing being proved. It is not right now: gfx has since
// CORRECTED its stroke rasteriser, and a gate demanding the old bytes forbids
// the correction. gfx's own comment names the defect it fixed --
//
//	Rasterising each piece on its own and keeping the greater coverage would
//	look like the same thing and is not. Two pieces that meet along a shared
//	edge each cover part of the pixel that edge cuts, and the greater of two
//	halves is a half. A finely cut curve -- the way every plotting program
//	writes one -- is nothing but such seams, and would come out at half its
//	colour, combed through with lighter notches at every vertex.
//
// -- so the reference here is a frozen private copy of a rasteriser that has
// been improved upstream, and holding gfx to it means holding painter at gfx
// v0.1.0 forever. Measured against v0.20.0 over this scene:
//
//	276 bytes of 307200 differ (0.090%), peak 23, mean -1.39
//	172 bytes darker, 104 lighter
//
// The mean being NEGATIVE is the correction showing: the old union under-darkened
// the seams, so the new one is darker there, and the lighter bytes are the other
// side of the same edges.
//
// What the gate still catches is a call wired wrongly -- a wrong colour, a wrong
// winding rule, a path off by a pixel -- because none of those move 0.09% of the
// buffer by 23 levels. They move all of it, or a whole shape of it.
//
// Rendering both on the same machine (rather than pinning a committed golden)
// makes the proof immune to cross-architecture floating-point (FMA) differences:
// both passes run the same arithmetic on the same CPU, so the only thing under
// test is old-rasterizer vs new-rasterizer. See feedback-prove-against-replaced-
// code (compare against the replaced implementation, reproduced verbatim) and
// feedback-autonomous-visual-verification (programmatic capture, not eyes).

// fillFn / strokeFn abstract the two path entry points so one scene renders both
// ways.
type fillFn func(cmds []refCmd, c RGBA, rule FillRule)
type strokeFn func(cmds []refCmd, c RGBA, width float64)

// lineChartCmds is a sine polyline across the right of the surface.
func lineChartCmds() []refCmd {
	var cmds []refCmd
	for i := 0; i <= 12; i++ {
		x := 200.0 + float64(i)*9
		y := 60.0 + 30*math.Sin(float64(i)*0.7)
		op := rLine
		if i == 0 {
			op = rMove
		}
		cmds = append(cmds, refCmd{op: op, x: x, y: y})
	}
	return cmds
}

// starIconCmds is a self-overlapping 5-point star (exercises both winding rules).
func starIconCmds() []refCmd {
	var cmds []refCmd
	for i := 0; i <= 5; i++ {
		ang := -math.Pi/2 + float64(i)*4*math.Pi/5
		sx := 130 + 30*math.Cos(ang)
		sy := 150 + 30*math.Sin(ang)
		op := rLine
		if i == 0 {
			op = rMove
		}
		cmds = append(cmds, refCmd{op: op, x: sx, y: sy})
	}
	cmds = append(cmds, refCmd{op: rClose})
	return cmds
}

// renderWidgetScene draws the full scene into p, taking the path entry points as
// callbacks so the same scene renders through the migrated painter or the ref.
func renderWidgetScene(p *PixelPainter, fillP fillFn, strokeP strokeFn) {
	const W, H = 320, 240

	// Ground: a translucent diagonal wash so every later blend lands on a
	// non-trivial background (a blend that wrongly behaves as a copy can't hide).
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			p.PutPixel(x, y, RGBA{uint8(x % 256), uint8(y % 256), uint8((x + y) % 256), 0x40})
		}
	}
	p.FillRect(Rect{0, 0, W, H}, RGBA{0x1e, 0x1e, 0x28, 0xC0})

	// Framed panel.
	panel := Rect{8, 8, 180, 96}
	p.FillRect(panel, RGBA{0x2a, 0x2e, 0x3a, 0xE0})
	p.StrokeRect(panel, RGB(0x50, 0x58, 0x70), 1)

	// Two rounded buttons (fill + stroke + text), opaque + translucent.
	b1 := Rect{16, 16, 76, 26}
	p.FillRoundRect(b1, 8, RGB(0x30, 0x6a, 0xc4))
	p.StrokeRoundRect(b1, 8, RGBA{0xff, 0xff, 0xff, 0x30}, 1)
	p.Text(b1.X+10, b1.Y+10, "OK", RGB(0xff, 0xff, 0xff))
	b2 := Rect{100, 16, 84, 26}
	p.FillRoundRect(b2, 12, RGBA{0xc4, 0x50, 0x40, 0xB0})
	p.StrokeRoundRect(b2, 12, RGBA{0x00, 0x00, 0x00, 0x40}, 1)
	p.Text(b2.X+8, b2.Y+10, "Cancel", RGB(0xff, 0xee, 0xee))

	// Bar chart clipped to the panel.
	p.PushClip(panel)
	for i, bh := range []int{18, 40, 28, 60, 34, 52, 22} {
		bx := 16 + i*24
		p.FillRect(Rect{bx, 96 - bh + 8, 16, bh}, RGB(uint8(0x40+i*20), 0xb0, uint8(0xd0-i*10)))
	}
	p.PopClip()

	// Line chart via StrokePath (the vector stroker).
	strokeP(lineChartCmds(), RGB(0x00, 0xd0, 0x90), 2.5)

	// Filled star icon via FillPath (EvenOdd) + StrokePath, under a translation.
	// Paths do not honour the translation (they never did), but pushing it proves
	// that stays true through the migration.
	p.PushTranslate(210, 96)
	star := starIconCmds()
	fillP(star, RGBA{0xf0, 0xc0, 0x20, 0xE0}, EvenOdd)
	strokeP(star, RGB(0x80, 0x60, 0x00), 1.5)
	p.PopTranslate()

	// Image thumbnail via DrawImage (a small checker), scaled up.
	const sw, shh = 8, 8
	checker := make([]byte, 4*sw*shh)
	for y := 0; y < shh; y++ {
		for x := 0; x < sw; x++ {
			o := (y*sw + x) * 4
			v := byte(0x30)
			if (x+y)%2 == 0 {
				v = 0xe0
			}
			checker[o], checker[o+1], checker[o+2], checker[o+3] = v, byte(0xff-int(v)), 0x80, 0xC0
		}
	}
	p.DrawImage(Rect{16, 120, 64, 64}, checker, sw, shh)

	// Glyph coverage mask via DrawMask.
	const mw, mh = 24, 12
	mask := make([]byte, mw*mh)
	for y := 0; y < mh; y++ {
		for x := 0; x < mw; x++ {
			c := (x*10 + y*6) % 256
			if x > 8 && x < 16 && y > 3 && y < 8 {
				c = 0
			}
			mask[y*mw+x] = byte(c)
		}
	}
	p.DrawMask(Rect{96, 132, mw, mh}, mask, mw, RGB(0x90, 0xf0, 0xff))

	// Table: a grid of tinted rows + text, clipped.
	tbl := Rect{96, 150, 160, 80}
	p.PushClip(tbl)
	for r := 0; r < 4; r++ {
		rowY := tbl.Y + r*18
		tint := RGBA{0x30, 0x34, 0x40, 0xA0}
		if r%2 == 1 {
			tint = RGBA{0x24, 0x28, 0x34, 0xA0}
		}
		p.FillRect(Rect{tbl.X, rowY, tbl.W, 18}, tint)
		for c := 0; c < 3; c++ {
			p.Text(tbl.X+6+c*52, rowY+6, "Cell", RGB(0xcc, 0xd0, 0xe0))
			p.PutPixel(tbl.X+c*52, rowY, RGB(0x60, 0x66, 0x80))
		}
	}
	p.PopClip()
}

func TestWidgetSceneRenderParity(t *testing.T) {
	const W, H = 320, 240

	// Migrated painter: FillPath / StrokePath consume go-gfx/gfx/vector.
	pNew := NewPixelPainter(make([]byte, 4*W*H), W, H)
	renderWidgetScene(pNew,
		func(cmds []refCmd, c RGBA, rule FillRule) { pNew.FillPath(buildPath(cmds), c, rule) },
		func(cmds []refCmd, c RGBA, width float64) { pNew.StrokePath(buildPath(cmds), c, width) },
	)

	// Reference painter: the verbatim pre-migration rasterizer + composite.
	pRef := NewPixelPainter(make([]byte, 4*W*H), W, H)
	renderWidgetScene(pRef,
		func(cmds []refCmd, c RGBA, rule FillRule) { refFillPathInto(pRef, cmds, c, rule) },
		func(cmds []refCmd, c RGBA, width float64) { refStrokePathInto(pRef, cmds, c, width) },
	)

	if len(pNew.Buf) != len(pRef.Buf) {
		t.Fatalf("buffer sizes differ: new %d, ref %d", len(pNew.Buf), len(pRef.Buf))
	}
	diffs, peak, sum, firstIdx := 0, 0, 0, -1
	for i := range pNew.Buf {
		d := int(pNew.Buf[i]) - int(pRef.Buf[i])
		if d == 0 {
			continue
		}
		if firstIdx < 0 {
			firstIdx = i
		}
		diffs++
		sum += d
		if d < 0 {
			d = -d
		}
		if d > peak {
			peak = d
		}
	}
	// The bounds sit far above what the correction costs and far below what a
	// wiring mistake costs, which is the only reason a number here can be
	// defended. Against gfx v0.20.0 this scene measures 0.090% and 23.
	const maxShare, maxPeak = 0.01, 64
	share := float64(diffs) / float64(len(pNew.Buf))
	if share > maxShare || peak > maxPeak {
		px := firstIdx / 4
		t.Fatalf("migrated render differs from the pre-migration reference in %d bytes "+
			"(%.3f%%, peak %d, mean %+.2f); first at byte %d (pixel %d, x=%d y=%d, "+
			"channel %d): new %d ref %d",
			diffs, 100*share, peak, float64(sum)/float64(diffs), firstIdx, px,
			px%W, px/W, firstIdx%4, pNew.Buf[firstIdx], pRef.Buf[firstIdx])
	}
	if diffs > 0 {
		t.Logf("within bounds of the pre-migration reference: %d bytes differ (%.3f%%), peak %d, mean %+.2f", diffs, 100*share, peak, float64(sum)/float64(diffs))
	}

	// Control the instrument: the scene must actually have painted vector paths,
	// so the comparison is not vacuously equal on two blank buffers. Assert the
	// star icon and line chart left ink.
	painted := 0
	for i := 3; i < len(pNew.Buf); i += 4 {
		if pNew.Buf[i] != 0 {
			painted++
		}
	}
	if painted < W*H/2 {
		t.Fatalf("scene painted only %d/%d pixels; too sparse to be a meaningful parity check", painted, W*H)
	}
	t.Logf("widget scene compared over %d painted pixels", painted)
}

// TestRenderParityControl confirms the byte comparison is a live instrument: a
// scene rendered with a deliberately wrong stroke width diverges from the
// reference, so a passing parity test cannot be a false positive.
func TestRenderParityControl(t *testing.T) {
	const W, H = 64, 64
	pNew := NewPixelPainter(make([]byte, 4*W*H), W, H)
	pRef := NewPixelPainter(make([]byte, 4*W*H), W, H)
	// An in-bounds diagonal line so both sides actually paint on this 64x64.
	line := []refCmd{{op: rMove, x: 6, y: 20}, {op: rLine, x: 58, y: 44}}
	// Perturb: the "new" side strokes one unit wider than the reference.
	pNew.StrokePath(buildPath(line), RGB(0, 0xd0, 0x90), 3.5)
	refStrokePathInto(pRef, line, RGB(0, 0xd0, 0x90), 2.5)
	same := true
	for i := range pNew.Buf {
		if pNew.Buf[i] != pRef.Buf[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("control: a wrong stroke width should make the buffers differ, but they matched")
	}
}
