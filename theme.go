// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

// Theme is the palette every widget consults. The prototype ships a
// minimum viable set — a production merge into toolkit reuses the
// full go-widgets/toolkit theme struct.
type Theme struct {
	Background RGBA
	Surface    RGBA
	OnSurface  RGBA
	Border     RGBA
	Accent     RGBA
}

// LightTheme mirrors the go-widgets/toolkit default light palette.
func LightTheme() *Theme {
	return &Theme{
		Background: RGB(0xF7, 0xF7, 0xF7),
		Surface:    RGB(0xFF, 0xFF, 0xFF),
		OnSurface:  RGB(0x1A, 0x1A, 0x1A),
		Border:     RGB(0xC0, 0xC0, 0xC0),
		Accent:     RGB(0x0D, 0x94, 0x88), // teal — go-widgets brand
	}
}

// DarkTheme mirrors the go-widgets/toolkit default dark palette.
func DarkTheme() *Theme {
	return &Theme{
		Background: RGB(0x12, 0x12, 0x12),
		Surface:    RGB(0x1E, 0x1E, 0x1E),
		OnSurface:  RGB(0xE8, 0xE8, 0xE8),
		Border:     RGB(0x44, 0x44, 0x44),
		Accent:     RGB(0x2D, 0xD4, 0xBF),
	}
}
