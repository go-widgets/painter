// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCellPainterSize(t *testing.T) {
	p := NewCellPainter(5, 3)
	if w, h := p.Size(); w != 5 || h != 3 {
		t.Fatalf("Size = (%d,%d), want (5,3)", w, h)
	}
}

func TestCellPainterInitialisedToSpace(t *testing.T) {
	p := NewCellPainter(2, 2)
	for _, c := range p.Cells {
		if c.Rune != ' ' {
			t.Fatalf("cell not initialised to space: %q", c.Rune)
		}
	}
}

func TestCellPainterFillRect(t *testing.T) {
	p := NewCellPainter(4, 4)
	p.FillRect(Rect{1, 1, 2, 2}, RGB(0x10, 0x20, 0x30))
	// (0,0) untouched
	if p.Cells[0].Bg != (RGBA{}) {
		t.Fatalf("(0,0) bg mutated: %v", p.Cells[0].Bg)
	}
	// (1,1) filled
	got := p.Cells[1*4+1]
	want := RGBA{0x10, 0x20, 0x30, 0xFF}
	if got.Bg != want {
		t.Fatalf("(1,1) bg = %v, want %v", got.Bg, want)
	}
}

func TestCellPainterStrokeRect(t *testing.T) {
	p := NewCellPainter(6, 4)
	p.StrokeRect(Rect{1, 1, 4, 3}, RGB(0xFF, 0, 0), 1)
	// corners
	if p.Cells[1*6+1].Rune != '┌' {
		t.Fatalf("top-left corner = %q", p.Cells[1*6+1].Rune)
	}
	if p.Cells[1*6+4].Rune != '┐' {
		t.Fatalf("top-right corner = %q", p.Cells[1*6+4].Rune)
	}
	if p.Cells[3*6+1].Rune != '└' {
		t.Fatalf("bottom-left corner = %q", p.Cells[3*6+1].Rune)
	}
	if p.Cells[3*6+4].Rune != '┘' {
		t.Fatalf("bottom-right corner = %q", p.Cells[3*6+4].Rune)
	}
	// horizontals
	if p.Cells[1*6+2].Rune != '─' {
		t.Fatalf("top edge = %q", p.Cells[1*6+2].Rune)
	}
	// verticals
	if p.Cells[2*6+1].Rune != '│' {
		t.Fatalf("left edge = %q", p.Cells[2*6+1].Rune)
	}
}

func TestCellPainterStrokeRectZeroSizeNoOp(t *testing.T) {
	p := NewCellPainter(4, 4)
	p.StrokeRect(Rect{0, 0, 0, 0}, RGB(0xFF, 0, 0), 1)
	p.StrokeRect(Rect{0, 0, 4, 0}, RGB(0xFF, 0, 0), 1)
	p.StrokeRect(Rect{0, 0, 0, 4}, RGB(0xFF, 0, 0), 1)
	for i, c := range p.Cells {
		if c.Rune != ' ' {
			t.Fatalf("cell[%d] mutated by zero-size rect: %q", i, c.Rune)
		}
	}
}

func TestCellPainterPutPixel(t *testing.T) {
	p := NewCellPainter(4, 4)
	p.PutPixel(1, 1, RGB(0xFF, 0, 0))
	c := p.Cells[1*4+1]
	if c.Rune != '█' {
		t.Fatalf("PutPixel rune = %q, want '█'", c.Rune)
	}
	if c.Fg.R != 0xFF {
		t.Fatalf("PutPixel fg red = %d", c.Fg.R)
	}
}

func TestCellPainterText(t *testing.T) {
	p := NewCellPainter(10, 2)
	p.Text(2, 0, "OK", RGB(0xFF, 0xFF, 0xFF))
	if p.Cells[0*10+2].Rune != 'O' {
		t.Fatalf("first rune = %q", p.Cells[2].Rune)
	}
	if p.Cells[0*10+3].Rune != 'K' {
		t.Fatalf("second rune = %q", p.Cells[3].Rune)
	}
}

func TestCellPainterOutOfBoundsSetSkipped(t *testing.T) {
	p := NewCellPainter(2, 2)
	// each of these must be a no-op — must not panic and must not
	// mutate any cell.
	p.PutPixel(-1, 0, RGB(0xFF, 0, 0))
	p.PutPixel(0, -1, RGB(0xFF, 0, 0))
	p.PutPixel(2, 0, RGB(0xFF, 0, 0))
	p.PutPixel(0, 2, RGB(0xFF, 0, 0))
	p.FillRect(Rect{-5, -5, 1, 1}, RGB(0xFF, 0, 0))
	for i, c := range p.Cells {
		if c.Rune != ' ' {
			t.Fatalf("cell[%d] leaked ink: %q", i, c.Rune)
		}
	}
}

func TestCellPainterWriteANSI(t *testing.T) {
	p := NewCellPainter(2, 1)
	p.Text(0, 0, "AB", RGB(0xFF, 0, 0))
	var buf bytes.Buffer
	n, err := p.WriteANSI(&buf)
	if err != nil {
		t.Fatalf("WriteANSI err = %v", err)
	}
	if n != buf.Len() {
		t.Fatalf("byte count mismatch")
	}
	s := buf.String()
	if !strings.Contains(s, "A") || !strings.Contains(s, "B") {
		t.Fatalf("ANSI output missing runes: %q", s)
	}
	// truecolor sequence present
	if !strings.Contains(s, "\x1b[38;2;") || !strings.Contains(s, "\x1b[48;2;") {
		t.Fatalf("ANSI output missing truecolor sequences: %q", s)
	}
	// row terminator
	if !strings.Contains(s, "\x1b[0m\n") {
		t.Fatalf("ANSI output missing row reset")
	}
}

// errWriter is a Writer that always returns an error — used to
// exercise the error branch of WriteANSI.
type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) { return 0, errors.New("boom") }

func TestCellPainterWriteANSIReportsWriteError(t *testing.T) {
	p := NewCellPainter(1, 1)
	if _, err := p.WriteANSI(errWriter{}); err == nil {
		t.Fatalf("expected error from WriteANSI, got nil")
	}
}
