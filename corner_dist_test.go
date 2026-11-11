package painter

import (
	"math"
	"math/big"
	"testing"
)

// cornerDist is the f64 nearest the true distance for every pixel of every
// corner box up to radius 96, which math/big (256 bits) decides, and it differs
// from math.Hypot on some of them: the count is the number of bytes the
// definition can move, logged.
func TestCornerDistIsTheNearestFloat(t *testing.T) {
	hypotOff, cases := 0, 0
	for rad := 1; rad <= 96; rad++ {
		for j := range rad {
			for i := range rad {
				// the corner box's top-left pixel is (0, 0); the centre (rad, rad)
				got := cornerDist(i, j, float64(rad), float64(rad))
				dx, dy := int64(2*(i-rad)+1), int64(2*(j-rad)+1)
				d4 := new(big.Float).SetPrec(256).SetInt64(dx*dx + dy*dy)
				want, _ := new(big.Float).SetPrec(256).Quo(new(big.Float).SetPrec(256).Sqrt(d4), big.NewFloat(2)).Float64()
				if got != want {
					t.Fatalf("radius %d pixel (%d,%d): %v, want %v", rad, i, j, got, want)
				}
				if h := math.Hypot(float64(i)+0.5-float64(rad), float64(j)+0.5-float64(rad)); h != got {
					hypotOff++
				}
				cases++
			}
		}
	}
	t.Logf("%d corner pixels: math.Hypot is not the nearest float on %d", cases, hypotOff)
}
