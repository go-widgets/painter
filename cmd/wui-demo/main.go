// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// wui-demo renders the prototype's three widgets into a PixelPainter
// and writes the resulting RGBA buffer as a PNG. The buffer is the
// SAME output shape a WUI host (browser wasm + canvas.putImageData)
// or a GUI host (native SDL / Ebitengine / etc.) would consume — the
// PNG is only a convenient way to snapshot the pixel output on the
// command line.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
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

// run splits from main so tests can drive it deterministically —
// see cmd/wui-demo tests. Exit code 0 on success, 1 on error.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("wui-demo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "wui-demo.png", "output PNG path (use '-' for stdout)")
	theme := fs.String("theme", "light", "theme (light|dark)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	th := painter.LightTheme()
	if *theme == "dark" {
		th = painter.DarkTheme()
	}

	const W, H = 240, 160
	buf := make([]byte, 4*W*H)
	// paint background
	pp := painter.NewPixelPainter(buf, W, H)
	pp.FillRect(painter.Rect{X: 0, Y: 0, W: W, H: H}, th.Background)

	// three widgets identical to the tui-demo — same code, same theme
	widgets := []painter.Widget{
		&painter.Label{Bounds: painter.Rect{X: 16, Y: 12, W: 200, H: 12}, Text: "GO WIDGETS PAINTER"},
		&painter.Button{Bounds: painter.Rect{X: 16, Y: 36, W: 96, H: 24}, Label: "OK"},
		&painter.Button{Bounds: painter.Rect{X: 128, Y: 36, W: 96, H: 24}, Label: "CANCEL", Pressed: true},
		&painter.ProgressBar{Bounds: painter.Rect{X: 16, Y: 84, W: 208, H: 20}, Value: 0.72},
	}
	for _, w := range widgets {
		w.Draw(pp, th)
	}

	img := &image.NRGBA{
		Pix:    buf,
		Stride: 4 * W,
		Rect:   image.Rect(0, 0, W, H),
	}

	var w io.Writer = stdout
	if *out != "-" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer f.Close()
		w = f
	}
	if err := png.Encode(w, img); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *out != "-" {
		fmt.Fprintf(stdout, "wrote %s (%dx%d, %s theme)\n", *out, W, H, *theme)
	}
	return 0
}
