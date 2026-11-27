// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"fmt"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

// A typical frame: a 1000x700 window with a background, 24 buttons, 12
// progress bars and 30 labels. The same drawing code feeds the raster
// reference (PixelPainter, which writes into a pre-allocated 2.8 MB buffer) and
// the recorder (which produces an SVG document). The numbers are a baseline,
// not a gate: the two do different work (one touches every pixel, the other
// writes a document that a viewer will rasterise later at any size).
const frameW, frameH = 1000, 700

func frame(p painter.Painter) {
	th := painter.LightTheme()
	p.FillRect(painter.Rect{W: frameW, H: frameH}, th.Background)
	for i := 0; i < 24; i++ {
		x, y := 10+(i%6)*160, 10+(i/6)*40
		(&painter.Button{Bounds: painter.Rect{X: x, Y: y, W: 150, H: 30}, Label: fmt.Sprintf("ACTION %d", i), Pressed: i%5 == 0}).Draw(p, th)
	}
	for i := 0; i < 12; i++ {
		x, y := 10+(i%3)*320, 190+(i/3)*24
		(&painter.ProgressBar{Bounds: painter.Rect{X: x, Y: y, W: 300, H: 14}, Value: float64(i) / 12}).Draw(p, th)
	}
	for i := 0; i < 30; i++ {
		x, y := 10+(i%3)*320, 300+(i/3)*20
		(&painter.Label{Bounds: painter.Rect{X: x, Y: y, W: 300, H: 8}, Text: fmt.Sprintf("ROW %02d  STATUS OK  LOAD %d", i, i*3)}).Draw(p, th)
	}
}

func BenchmarkFrameRaster(b *testing.B) {
	buf := make([]byte, frameW*frameH*4)
	p := painter.NewPixelPainter(buf, frameW, frameH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame(p)
	}
	b.ReportMetric(float64(len(buf)), "out-bytes")
}

func BenchmarkFrameRecorder(b *testing.B) {
	r, err := svgrec.New(frameW, frameH)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset()
		frame(r)
	}
	b.StopTimer()
	if r.Err() != nil {
		b.Fatal(r.Err())
	}
	b.ReportMetric(float64(r.Stats().Bytes), "out-bytes")
	b.ReportMetric(float64(r.Stats().Elements), "elements")
}

// BenchmarkFrameRecorderSVG includes assembling the document (SVG()).
func BenchmarkFrameRecorderSVG(b *testing.B) {
	r, err := svgrec.New(frameW, frameH)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset()
		frame(r)
		if _, err := r.SVG(); err != nil {
			b.Fatal(err)
		}
	}
}
