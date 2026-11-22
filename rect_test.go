package painter

import "testing"

func TestRectIntersectEmptyArea(t *testing.T) {
	frame := Rect{0, 0, 200, 160}
	cases := []struct {
		name string
		r    Rect
		want Rect
	}{
		{"inside", Rect{10, 20, 30, 40}, Rect{10, 20, 30, 40}},
		{"top-left overhang", Rect{-5, -5, 15, 15}, Rect{0, 0, 10, 10}},
		{"bottom-right overhang", Rect{196, 156, 20, 20}, Rect{196, 156, 4, 4}},
		{"whole frame", frame, frame},
		{"off right", Rect{210, 0, 5, 5}, Rect{}},
		{"touching right edge", Rect{200, 0, 5, 5}, Rect{}},
		{"touching bottom edge", Rect{0, 160, 5, 5}, Rect{}},
		{"above", Rect{0, -20, 5, 10}, Rect{}},
		{"empty width", Rect{3, 3, 0, 7}, Rect{}},
		{"negative height", Rect{3, 3, 7, -2}, Rect{}},
		{"one pixel corner", Rect{199, 159, 9, 9}, Rect{199, 159, 1, 1}},
	}
	for _, c := range cases {
		got := c.r.Intersect(frame)
		if got != c.want {
			t.Errorf("%s: %v.Intersect(frame) = %v, want %v", c.name, c.r, got, c.want)
		}
		if got.Empty() != (c.want == Rect{}) {
			t.Errorf("%s: Empty() = %v", c.name, got.Empty())
		}
		if got.Area() != c.want.W*c.want.H {
			t.Errorf("%s: Area() = %d, want %d", c.name, got.Area(), c.want.W*c.want.H)
		}
	}
	if (Rect{0, 0, -3, 4}).Area() != 0 || (Rect{0, 0, 3, 0}).Area() != 0 || (Rect{0, 0, 3, 4}).Area() != 12 {
		t.Error("Area of empty / non-empty rectangles wrong")
	}
	if !(Rect{0, 0, 0, 5}).Empty() || !(Rect{0, 0, 5, -1}).Empty() || (Rect{0, 0, 1, 1}).Empty() {
		t.Error("Empty wrong")
	}
	// Symmetry.
	a, b := Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10}
	if a.Intersect(b) != b.Intersect(a) || a.Intersect(b) != (Rect{5, 5, 5, 5}) {
		t.Error("Intersect is not symmetric or wrong")
	}
}
