// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

// The goldens are the exact bytes a few widgets record. They contain no
// image (PNG bytes depend on the Go toolchain's compress/flate), so they are
// stable across toolchains. Regenerate after an intended change with
//
//	go test ./svgrec -run TestGolden -update
//
// and review the diff: a golden that moves is a changed output format.
var update = flag.Bool("update", false, "rewrite the golden files")

var goldens = []struct {
	name string
	w, h int
	draw func(p painter.Painter)
}{
	{"label", 80, 16, func(p painter.Painter) {
		(&painter.Label{Bounds: painter.Rect{X: 4, Y: 4, W: 70, H: 8}, Text: "STATUS: OK"}).Draw(p, painter.LightTheme())
	}},
	{"button", 120, 40, func(p painter.Painter) {
		th := painter.LightTheme()
		(&painter.Button{Bounds: painter.Rect{X: 4, Y: 4, W: 50, H: 16}, Label: "OK"}).Draw(p, th)
		(&painter.Button{Bounds: painter.Rect{X: 60, Y: 4, W: 50, H: 16}, Label: "NO", Pressed: true}).Draw(p, th)
	}},
	{"progress-dark", 120, 24, func(p painter.Painter) {
		th := painter.DarkTheme()
		p.FillRect(painter.Rect{W: 120, H: 24}, th.Background)
		(&painter.ProgressBar{Bounds: painter.Rect{X: 4, Y: 6, W: 112, H: 12}, Value: 0.6}).Draw(p, th)
	}},
	{"clipped-panel", 100, 60, func(p painter.Painter) {
		p.(painter.Translator).PushTranslate(10, 10)
		p.(painter.Clipper).PushClip(painter.Rect{W: 60, H: 30})
		p.FillRoundRect(painter.Rect{X: -5, Y: -5, W: 80, H: 40}, 6, painter.RGB(30, 90, 200))
		p.StrokeRoundRect(painter.Rect{X: 2, Y: 2, W: 50, H: 20}, 4, painter.RGBA{R: 255, G: 255, B: 255, A: 200}, 1)
		p.Text(6, 8, "A&B <C>", painter.RGB(255, 255, 255))
		p.(painter.Clipper).PopClip()
		p.(painter.Translator).PopTranslate()
	}},
}

func TestGolden(t *testing.T) {
	for _, g := range goldens {
		t.Run(g.name, func(t *testing.T) {
			r, err := svgrec.New(g.w, g.h)
			if err != nil {
				t.Fatal(err)
			}
			g.draw(r)
			got, err := r.SVG()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parse(got); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", g.name+".svg")
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s differs from its golden (run with -update to regenerate)\n got: %s\nwant: %s", g.name, got, want)
			}
		})
	}
}
