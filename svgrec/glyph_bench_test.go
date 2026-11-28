// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"math"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/painter/svgrec"
)

// A text page: 40 lines of 40 letters out of an alphabet of 26 outlines of 24
// curves each, the shape of a UI drawn with a TrueType face. "Before" is what a
// recorder had to do without TextGlyphs: one absolute <path> per glyph
// occurrence. "After" is one <symbol> per distinct glyph and one <use> per
// occurrence. The bytes are exact; the times are indicative.
const (
	textLines    = 40
	textLineLen  = 40
	textAlphabet = 26
)

func letter(k int) *painter.Path {
	p := painter.NewPath().MoveTo(0.5, 0)
	for i := 0; i < 24; i++ {
		a := float64(i+k) * 0.37
		p.QuadTo(3+4*math.Sin(a), -7+5*math.Cos(a), 0.25*float64(i%9)+1.125*float64(k%5), -6.5+0.7*float64(i%7))
	}
	return p.Close()
}

func glyphRun(line int) []svgrec.Glyph {
	run := make([]svgrec.Glyph, textLineLen)
	for i := range run {
		k := (line*7 + i*3) % textAlphabet
		run[i] = svgrec.Glyph{Key: fmt.Sprint(k), X: 8 + float64(i)*7.25, Y: 14 + float64(line)*12, Outline: func() *painter.Path { return letter(k) }}
	}
	return run
}

func textPage(b *testing.B, symbols bool) (raw, gz int) {
	r, err := svgrec.New(400, 520)
	if err != nil {
		b.Fatal(err)
	}
	ink := painter.RGB(20, 20, 30)
	for l := 0; l < textLines; l++ {
		run := glyphRun(l)
		if symbols {
			r.TextGlyphs("a line", run, ink)
			continue
		}
		for _, g := range run {
			p := g.Outline()
			r.PushTranslate(int(g.X), int(g.Y))
			r.FillPath(p, ink, painter.NonZero)
			r.PopTranslate()
		}
	}
	doc, err := r.SVG()
	if err != nil {
		b.Fatal(err)
	}
	var z bytes.Buffer
	w, _ := gzip.NewWriterLevel(&z, 9)
	_, _ = w.Write(doc)
	_ = w.Close()
	return len(doc), z.Len()
}

func benchTextPage(b *testing.B, symbols bool) {
	b.ReportAllocs()
	var raw, gz int
	for i := 0; i < b.N; i++ {
		raw, gz = textPage(b, symbols)
	}
	b.ReportMetric(float64(raw), "raw-bytes")
	b.ReportMetric(float64(gz), "gzip-bytes")
}

// BenchmarkTextPagePerGlyphPaths is the "before" of TextGlyphs.
func BenchmarkTextPagePerGlyphPaths(b *testing.B) { benchTextPage(b, false) }

// BenchmarkTextPageSymbols is the "after".
func BenchmarkTextPageSymbols(b *testing.B) { benchTextPage(b, true) }
