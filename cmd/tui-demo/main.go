// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// tui-demo renders the same three widgets as wui-demo, but into a
// CellPainter, and writes the resulting 24-bit-ANSI stream to
// stdout. Point your terminal at the output to see the widgets
// rendered on a cell grid — same widget source, cell back-end.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/go-widgets/painter"
)

// runFunc / osExit are dependency-injection seams so tests can drive
// main()'s success and error branches without spawning a subprocess.
var (
	runFunc = run
	osExit  = os.Exit
)

func main() {
	osExit(runFunc(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tui-demo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	theme := fs.String("theme", "light", "theme (light|dark)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	th := painter.LightTheme()
	if *theme == "dark" {
		th = painter.DarkTheme()
	}

	const W, H = 60, 14
	cp := painter.NewCellPainter(W, H)
	cp.FillRect(painter.Rect{X: 0, Y: 0, W: W, H: H}, th.Background)

	widgets := []painter.Widget{
		&painter.Label{Bounds: painter.Rect{X: 2, Y: 1, W: 40, H: 1}, Text: "GO WIDGETS PAINTER"},
		&painter.Button{Bounds: painter.Rect{X: 2, Y: 3, W: 12, H: 3}, Label: "OK"},
		&painter.Button{Bounds: painter.Rect{X: 16, Y: 3, W: 14, H: 3}, Label: "CANCEL", Pressed: true},
		&painter.ProgressBar{Bounds: painter.Rect{X: 2, Y: 8, W: 40, H: 3}, Value: 0.72},
	}
	for _, w := range widgets {
		w.Draw(cp, th)
	}

	if _, err := cp.WriteANSI(stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
