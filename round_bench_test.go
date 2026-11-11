package painter

import "testing"

// BenchmarkRoundRects fills and strokes rounded rectangles of radii 4 to 64
// on a 1024 x 768 buffer: the corner arcs are most of the work.
func BenchmarkRoundRects(b *testing.B) {
	const w, h = 1024, 768
	buf := make([]byte, 4*w*h)
	p := NewPixelPainter(buf, w, h)
	b.ReportAllocs()
	for b.Loop() {
		for i := range 64 {
			rad := 4 + i
			r := Rect{X: (i * 37) % 800, Y: (i * 53) % 600, W: 2*rad + 60, H: 2*rad + 40}
			p.FillRoundRect(r, rad, RGBA{R: 200, G: 100, B: 50, A: 180})
			p.StrokeRoundRect(r, rad, RGBA{R: 10, G: 20, B: 250, A: 255}, 2)
		}
	}
}
