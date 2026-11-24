package painter

import (
	"math/rand"
	"testing"
)

// randRect draws small rectangles, empty ones (zero or negative size) included,
// so the identity law is exercised as often as the overlapping case.
func randRect(r *rand.Rand) Rect {
	return Rect{X: r.Intn(41) - 20, Y: r.Intn(41) - 20, W: r.Intn(21) - 4, H: r.Intn(21) - 4}
}

func TestRectUnionExamples(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b Rect
		want Rect
	}{
		{"overlap", Rect{0, 0, 10, 10}, Rect{5, 5, 10, 10}, Rect{0, 0, 15, 15}},
		{"disjoint", Rect{0, 0, 2, 2}, Rect{10, 20, 3, 4}, Rect{0, 0, 13, 24}},
		{"nested", Rect{0, 0, 10, 10}, Rect{2, 2, 3, 3}, Rect{0, 0, 10, 10}},
		{"left identity", Rect{}, Rect{3, 4, 5, 6}, Rect{3, 4, 5, 6}},
		{"right identity", Rect{3, 4, 5, 6}, Rect{9, 9, 0, 7}, Rect{3, 4, 5, 6}},
		{"negative size is empty", Rect{3, 4, 5, 6}, Rect{-50, -50, -3, 8}, Rect{3, 4, 5, 6}},
		{"both empty", Rect{5, 5, 0, 0}, Rect{9, 9, -1, 2}, Rect{}},
	} {
		if got := c.a.Union(c.b); got != c.want {
			t.Errorf("%s: %v.Union(%v) = %v, want %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

// TestRectUnionProperties checks, over random rectangles (empty ones included):
// commutative, associative, an empty rectangle is the identity, the result
// contains every unit of both operands, and no smaller rectangle would (its
// edges are the operands' extremes).
func TestRectUnionProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		a, b, c := randRect(rng), randRect(rng), randRect(rng)
		u := a.Union(b)
		if u != b.Union(a) {
			t.Fatalf("not commutative: %v.Union(%v) = %v but reversed %v", a, b, u, b.Union(a))
		}
		if l, r := a.Union(b).Union(c), a.Union(b.Union(c)); l != r {
			t.Fatalf("not associative: (%v U %v) U %v = %v, but %v U (%v U %v) = %v", a, b, c, l, a, b, c, r)
		}
		if got := a.Union(Rect{}); !a.Empty() && got != a {
			t.Fatalf("zero Rect is not the identity: %v U {} = %v", a, got)
		}
		if got := a.Union(a); !a.Empty() && got != a {
			t.Fatalf("not idempotent: %v U %v = %v", a, a, got)
		}
		for _, o := range []Rect{a, b} {
			if o.Empty() {
				continue
			}
			// Every corner unit of the operand lies inside the union.
			for _, p := range [][2]int{{o.X, o.Y}, {o.X + o.W - 1, o.Y}, {o.X, o.Y + o.H - 1}, {o.X + o.W - 1, o.Y + o.H - 1}} {
				if !u.Contains(p[0], p[1]) {
					t.Fatalf("%v.Union(%v) = %v does not contain %v's unit %v", a, b, u, o, p)
				}
			}
		}
		if !a.Empty() || !b.Empty() {
			// Tight: each edge of the union is an edge of an operand.
			minX, minY, maxX, maxY := 1<<30, 1<<30, -1<<30, -1<<30
			for _, o := range []Rect{a, b} {
				if o.Empty() {
					continue
				}
				minX, minY = min(minX, o.X), min(minY, o.Y)
				maxX, maxY = max(maxX, o.X+o.W), max(maxY, o.Y+o.H)
			}
			if u != (Rect{minX, minY, maxX - minX, maxY - minY}) {
				t.Fatalf("%v.Union(%v) = %v, not the bounding box", a, b, u)
			}
		} else if u != (Rect{}) {
			t.Fatalf("union of two empty rects = %v, want the zero Rect", u)
		}
	}
}
