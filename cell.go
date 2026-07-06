// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package painter

import (
	"bytes"
	"fmt"
	"io"
)

// Cell is one terminal cell — a rune plus a foreground and background
// colour. Painters serialize cell grids into ANSI escape sequences at
// render time.
type Cell struct {
	Rune rune
	Fg   RGBA
	Bg   RGBA
}

// CellPainter maps the primitive set onto a fixed-size cell grid.
// Coordinates are in cells; a rectangle of size (10, 3) is 10 cells
// wide and 3 cells tall regardless of the terminal's font.
//
// Colour: written to a 24-bit-ANSI-truecolor terminal at flush time.
// The primitive set carries full RGBA; the terminal handles quantiz-
// ation. This keeps the widget code identical between pixel + cell
// back-ends.
type CellPainter struct {
	W     int
	H     int
	Cells []Cell
}

// NewCellPainter builds a fresh painter over an allocated grid. The
// grid is initialized to space + black on black — the widget draws
// its own background.
func NewCellPainter(w, h int) *CellPainter {
	cells := make([]Cell, w*h)
	for i := range cells {
		cells[i].Rune = ' '
	}
	return &CellPainter{W: w, H: h, Cells: cells}
}

// FillRect paints a solid block of cells. The rune stays ' '; only
// the background colour is set. StrokeRect overlays box characters
// on top.
func (p *CellPainter) FillRect(r Rect, c RGBA) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			p.set(x, y, ' ', RGBA{}, c)
		}
	}
}

// StrokeRect draws a 1-cell-wide box using Unicode box-draw runes.
// lineW is ignored — a terminal cell is atomic.
func (p *CellPainter) StrokeRect(r Rect, c RGBA, lineW int) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	x0, y0 := r.X, r.Y
	x1, y1 := r.X+r.W-1, r.Y+r.H-1
	// corners
	p.setFg(x0, y0, '┌', c)
	p.setFg(x1, y0, '┐', c)
	p.setFg(x0, y1, '└', c)
	p.setFg(x1, y1, '┘', c)
	// horizontals
	for x := x0 + 1; x < x1; x++ {
		p.setFg(x, y0, '─', c)
		p.setFg(x, y1, '─', c)
	}
	// verticals
	for y := y0 + 1; y < y1; y++ {
		p.setFg(x0, y, '│', c)
		p.setFg(x1, y, '│', c)
	}
	_ = lineW
}

// FillRoundRect can't round on a cell grid (a cell is atomic), so it
// falls back to a square FillRect -- the rounding is a no-op here.
func (p *CellPainter) FillRoundRect(r Rect, radius int, c RGBA) {
	_ = radius
	p.FillRect(r, c)
}

// StrokeRoundRect falls back to the square box-draw StrokeRect on a cell grid.
func (p *CellPainter) StrokeRoundRect(r Rect, radius int, c RGBA, lineW int) {
	_ = radius
	p.StrokeRect(r, c, lineW)
}

// PutPixel paints a single cell as a filled block character. Useful
// for pixel-precise widgets that want to render "dots" on a terminal.
func (p *CellPainter) PutPixel(x, y int, c RGBA) {
	p.setFg(x, y, '█', c)
}

// Text writes s as-is starting at (x, y) — one rune per cell. UTF-8
// wide characters are not policed at this prototype stage; a produc-
// tion CellPainter would use golang.org/x/text/width.
func (p *CellPainter) Text(x, y int, s string, ink RGBA) {
	i := 0
	for _, r := range s {
		p.setFg(x+i, y, r, ink)
		i++
	}
}

// Size returns width × height in cells.
func (p *CellPainter) Size() (int, int) { return p.W, p.H }

// set writes a full cell (rune + fg + bg) at (x, y), skipping any
// out-of-bounds coordinate.
func (p *CellPainter) set(x, y int, r rune, fg, bg RGBA) {
	if x < 0 || y < 0 || x >= p.W || y >= p.H {
		return
	}
	p.Cells[y*p.W+x] = Cell{Rune: r, Fg: fg, Bg: bg}
}

// setFg writes a rune + fg without touching the existing bg.
func (p *CellPainter) setFg(x, y int, r rune, fg RGBA) {
	if x < 0 || y < 0 || x >= p.W || y >= p.H {
		return
	}
	c := &p.Cells[y*p.W+x]
	c.Rune = r
	c.Fg = fg
}

// WriteANSI serializes the grid as a single ANSI-encoded string
// (24-bit truecolor). One escape sequence per cell; the terminal
// renders it verbatim. A trailing reset (`\x1b[0m`) is emitted at
// the end + a newline is emitted between rows.
func (p *CellPainter) WriteANSI(w io.Writer) (int, error) {
	var buf bytes.Buffer
	for y := 0; y < p.H; y++ {
		for x := 0; x < p.W; x++ {
			c := p.Cells[y*p.W+x]
			// truecolor fg + bg + rune
			fmt.Fprintf(&buf,
				"\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm%c",
				c.Fg.R, c.Fg.G, c.Fg.B,
				c.Bg.R, c.Bg.G, c.Bg.B,
				c.Rune,
			)
		}
		buf.WriteString("\x1b[0m\n")
	}
	return w.Write(buf.Bytes())
}
