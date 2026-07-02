// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command wui-wasm is the browser wasm live-demo of the go-widgets/painter
// prototype. It renders the same three widgets that cmd/wui-demo prints
// to a PNG and cmd/tui-demo prints as ANSI — but into a browser <canvas>
// via a PixelPainter. Same widget code, same theme, same Painter
// interface. Only the host consumer of the RGBA buffer changes.
//
// The host page (see index.html) exposes a theme picker + a "press"
// toggle for the buttons; a JS shim calls into the exported
// `setTheme` / `setPressed` funcs and re-renders.
//
//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/go-widgets/painter"
)

const (
	surfaceW = 480
	surfaceH = 320
)

type state struct {
	theme   *painter.Theme
	pressed bool
	buf     []byte
	pp      *painter.PixelPainter
}

func newState() *state {
	buf := make([]byte, 4*surfaceW*surfaceH)
	return &state{
		theme: painter.LightTheme(),
		buf:   buf,
		pp:    painter.NewPixelPainter(buf, surfaceW, surfaceH),
	}
}

// draw paints the fixed 3-widget scene at 2× the wui-demo size so it
// looks decent in a browser.
func (s *state) draw() {
	th := s.theme
	pp := s.pp
	pp.FillRect(painter.Rect{X: 0, Y: 0, W: surfaceW, H: surfaceH}, th.Background)

	widgets := []painter.Widget{
		&painter.Label{Bounds: painter.Rect{X: 32, Y: 24, W: 400, H: 24}, Text: "GO WIDGETS PAINTER"},
		&painter.Button{Bounds: painter.Rect{X: 32, Y: 72, W: 192, H: 48}, Label: "OK", Pressed: s.pressed},
		&painter.Button{Bounds: painter.Rect{X: 256, Y: 72, W: 192, H: 48}, Label: "CANCEL", Pressed: !s.pressed},
		&painter.ProgressBar{Bounds: painter.Rect{X: 32, Y: 168, W: 416, H: 40}, Value: 0.72},
		&painter.Label{Bounds: painter.Rect{X: 32, Y: 232, W: 416, H: 12}, Text: "SAME CODE. WUI PIXEL BACKEND."},
	}
	for _, w := range widgets {
		w.Draw(pp, th)
	}
}

func main() {
	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", "screen")
	if canvas.IsUndefined() || canvas.IsNull() {
		println("wui-wasm: no #screen canvas in the host page")
		return
	}
	canvas.Set("width", surfaceW)
	canvas.Set("height", surfaceH)
	ctx := canvas.Call("getContext", "2d")

	imageData := ctx.Call("createImageData", surfaceW, surfaceH)
	dst := imageData.Get("data")

	s := newState()

	render := func() {
		s.draw()
		js.CopyBytesToJS(dst, s.buf)
		ctx.Call("putImageData", imageData, 0, 0)
	}

	// Export a JS-callable knob to set the theme from the host page.
	js.Global().Set("painterSetTheme", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		if args[0].String() == "dark" {
			s.theme = painter.DarkTheme()
		} else {
			s.theme = painter.LightTheme()
		}
		render()
		return nil
	}))

	// Export a JS-callable knob to swap which button is pressed.
	js.Global().Set("painterTogglePressed", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		s.pressed = !s.pressed
		render()
		return nil
	}))

	// Initial render.
	render()

	// Signal ready — the host page hides the loading text on this.
	js.Global().Get("document").Call("dispatchEvent", js.Global().Get("CustomEvent").New("painter:ready"))

	// Park forever so exported callbacks stay live.
	select {}
}
