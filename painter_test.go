// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import "testing"

func TestRectContains(t *testing.T) {
	r := Rect{X: 10, Y: 20, W: 5, H: 5}
	cases := []struct {
		px, py int
		want   bool
	}{
		{10, 20, true},  // top-left corner is inside
		{14, 24, true},  // last inside pixel
		{15, 20, false}, // right edge is exclusive
		{10, 25, false}, // bottom edge is exclusive
		{9, 22, false},  // left of rect
		{12, 19, false}, // above rect
	}
	for _, tc := range cases {
		if got := r.Contains(tc.px, tc.py); got != tc.want {
			t.Errorf("Rect{10,20,5,5}.Contains(%d,%d) = %v, want %v", tc.px, tc.py, got, tc.want)
		}
	}
}
