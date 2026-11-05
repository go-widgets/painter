// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

// PixelPainter is a raster painter; the coarser painters are not, so code
// that composes pixel by pixel leaves them on their own Text.
func TestWhoIsARasterPainter(t *testing.T) {
	var p Painter = NewPixelPainter(make([]byte, 4), 1, 1)
	r, ok := p.(RasterPainter)
	if !ok {
		t.Fatal("PixelPainter is not a RasterPainter")
	}
	r.Raster() // draws nothing
	if got := p.(*PixelPainter).Buf; got[0]|got[1]|got[2]|got[3] != 0 {
		t.Fatalf("Raster drew: %v", got)
	}
	if _, ok := Painter(NewCellPainter(4, 2)).(RasterPainter); ok {
		t.Fatal("CellPainter claims to be a RasterPainter")
	}
}
