// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

func TestLightThemeDefined(t *testing.T) {
	th := LightTheme()
	if th.Background == (RGBA{}) {
		t.Fatalf("LightTheme Background not set")
	}
	// teal accent (0d9488)
	want := RGB(0x0D, 0x94, 0x88)
	if th.Accent != want {
		t.Fatalf("LightTheme Accent = %v, want %v", th.Accent, want)
	}
}

func TestDarkThemeDefined(t *testing.T) {
	th := DarkTheme()
	if th.OnSurface.R < 0xC0 {
		t.Fatalf("DarkTheme OnSurface should be light; got %v", th.OnSurface)
	}
}

func TestRGBHelperSetsFullAlpha(t *testing.T) {
	c := RGB(1, 2, 3)
	if c.A != 0xFF {
		t.Fatalf("RGB helper alpha = %d, want 0xFF", c.A)
	}
}
