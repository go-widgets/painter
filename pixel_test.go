// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"testing"
)

func newPixel(w, h int) *PixelPainter {
	return NewPixelPainter(make([]byte, 4*w*h), w, h)
}

func TestPixelPainterSize(t *testing.T) {
	p := newPixel(4, 3)
	if w, h := p.Size(); w != 4 || h != 3 {
		t.Fatalf("Size = (%d,%d), want (4,3)", w, h)
	}
}

func TestPixelPainterPutPixel(t *testing.T) {
	p := newPixel(2, 2)
	p.PutPixel(0, 0, RGB(0x10, 0x20, 0x30))
	p.PutPixel(1, 1, RGB(0x40, 0x50, 0x60))
	if p.Buf[0] != 0x10 || p.Buf[1] != 0x20 || p.Buf[2] != 0x30 || p.Buf[3] != 0xFF {
		t.Fatalf("pixel (0,0) = %v", p.Buf[0:4])
	}
	off := (1*2 + 1) * 4
	if p.Buf[off] != 0x40 || p.Buf[off+1] != 0x50 || p.Buf[off+2] != 0x60 || p.Buf[off+3] != 0xFF {
		t.Fatalf("pixel (1,1) = %v", p.Buf[off:off+4])
	}
}

func TestPixelPainterPutPixelOutOfBoundsDropped(t *testing.T) {
	p := newPixel(2, 2)
	// each of these must be a no-op — no panic, no write outside buf
	p.PutPixel(-1, 0, RGB(1, 1, 1))
	p.PutPixel(0, -1, RGB(1, 1, 1))
	p.PutPixel(2, 0, RGB(1, 1, 1))
	p.PutPixel(0, 2, RGB(1, 1, 1))
	for i, b := range p.Buf {
		if b != 0 {
			t.Fatalf("buf[%d] = %d, want 0 (nothing should have been written)", i, b)
		}
	}
}

func TestPixelPainterPutPixelShortBufferDropped(t *testing.T) {
	p := &PixelPainter{Buf: make([]byte, 4), Width: 2, Height: 2}
	// (1,1) computes off=12 which is >= len(buf); must no-op, not panic
	p.PutPixel(1, 1, RGB(1, 2, 3))
	for i, b := range p.Buf {
		if b != 0 {
			t.Fatalf("buf[%d] = %d, want 0 (short buf must drop write)", i, b)
		}
	}
}

func TestPixelPainterFillRect(t *testing.T) {
	p := newPixel(4, 4)
	p.FillRect(Rect{1, 1, 2, 2}, RGB(0xAA, 0xBB, 0xCC))
	// corner (0,0) untouched
	if p.Buf[0] != 0 {
		t.Fatalf("expected (0,0) untouched")
	}
	// (1,1) filled
	off := (1*4 + 1) * 4
	if p.Buf[off] != 0xAA || p.Buf[off+1] != 0xBB || p.Buf[off+2] != 0xCC {
		t.Fatalf("(1,1) = %v", p.Buf[off:off+3])
	}
}

func TestPixelPainterStrokeRect(t *testing.T) {
	p := newPixel(6, 6)
	p.StrokeRect(Rect{1, 1, 4, 4}, RGB(0xFF, 0, 0), 1)
	// (1,1) — corner drawn
	off := (1*6 + 1) * 4
	if p.Buf[off] != 0xFF {
		t.Fatalf("corner (1,1) not drawn")
	}
	// (2,2) — inside stroke, should be untouched
	off = (2*6 + 2) * 4
	if p.Buf[off] != 0 {
		t.Fatalf("interior (2,2) leaked into stroke")
	}
	// (4,4) — bottom-right corner drawn
	off = (4*6 + 4) * 4
	if p.Buf[off] != 0xFF {
		t.Fatalf("corner (4,4) not drawn")
	}
}

func TestPixelPainterStrokeRectZeroSizeNoOp(t *testing.T) {
	p := newPixel(4, 4)
	p.StrokeRect(Rect{0, 0, 0, 0}, RGB(0xFF, 0, 0), 1)
	p.StrokeRect(Rect{0, 0, 4, 0}, RGB(0xFF, 0, 0), 1)
	p.StrokeRect(Rect{0, 0, 0, 4}, RGB(0xFF, 0, 0), 1)
	for i, b := range p.Buf {
		if b != 0 {
			t.Fatalf("buf[%d] = %d, want 0 (zero-size rect must no-op)", i, b)
		}
	}
}

func TestPixelPainterText(t *testing.T) {
	// draw "A" and make sure at least one pixel was set
	p := newPixel(20, 12)
	p.Text(2, 2, "A", RGB(0xFF, 0xFF, 0xFF))
	written := 0
	for i := 0; i < len(p.Buf); i += 4 {
		if p.Buf[i] == 0xFF {
			written++
		}
	}
	if written == 0 {
		t.Fatalf("expected 'A' to produce ink pixels; got 0")
	}
}

func TestPixelPainterTextUnknownGlyphSkipped(t *testing.T) {
	// tilde '~' is not in the font table — the loop must skip it silently
	p := newPixel(20, 12)
	p.Text(2, 2, "~", RGB(0xFF, 0xFF, 0xFF))
	for i := 0; i < len(p.Buf); i += 4 {
		if p.Buf[i] != 0 {
			t.Fatalf("expected zero ink for unknown glyph; got byte %d", p.Buf[i])
		}
	}
}
