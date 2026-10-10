# go-widgets/painter

[![CI](https://github.com/go-widgets/painter/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/painter/actions/workflows/ci.yml)
[![pages](https://github.com/go-widgets/painter/actions/workflows/pages.yml/badge.svg)](https://go-widgets.github.io/painter/)
[![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-painter-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-widgets/painter)
![coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)
![go](https://img.shields.io/badge/Go-1.27.1%2B-00ADD8?logo=go&logoColor=white)
[![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)](./LICENSE)

**▶ Live demo: https://go-widgets.github.io/painter/**


The drawing seam of [go-widgets](https://github.com/go-widgets): one
`Painter` interface that every widget draws through, and two painters that
turn the same calls into different things:

- **`PixelPainter`** writes into a caller-owned RGBA `[]byte`, which a native
  window, a browser `<canvas>`, an Android surface or an image encoder then
  presents;
- **`CellPainter`** writes into a grid of terminal cells (rune, foreground,
  background) with a 24-bit ANSI serialiser.

A widget never learns which one it was handed. That is what lets
[go-widgets/toolkit](https://github.com/go-widgets/toolkit), whose
`Widget.Draw` has taken a `painter.Painter` since its v0.6, run unchanged in a
window, a browser and a terminal.

## The interface

```go
type Painter interface {
	FillRect(r Rect, c RGBA)
	StrokeRect(r Rect, c RGBA, lineW int)
	FillRoundRect(r Rect, radius int, c RGBA)
	StrokeRoundRect(r Rect, radius int, c RGBA, lineW int)
	PutPixel(x, y int, c RGBA)
	Text(x, y int, s string, ink RGBA)
	Size() (w, h int)
}
```

Seven methods, all of which every back-end must implement — so the contract
says what happens where a back-end cannot: a cell grid ignores `lineW`, and
draws a rounded rectangle square.

## Optional capabilities

What not every surface can do is not in the base interface. A widget that
needs it type-asserts, and draws something sensible when the answer is no:

| Interface | Methods | File |
| --- | --- | --- |
| `Clipper` | `PushClip`, `PopClip` | `clip.go` |
| `Translator` | `PushTranslate`, `PopTranslate` | `translate.go` |
| `PathPainter` | `FillPath`, `StrokePath` | `path.go`, `pathpaint.go` |
| `ImagePainter` | `DrawImage` | `image.go` |
| `MaskPainter` | `DrawMask` | `mask.go` |
| `FacePainter` | `TextFace` | `face.go` |

```go
if c, ok := p.(painter.Clipper); ok {
	c.PushClip(bounds)
	defer c.PopClip()
}
```

`PixelPainter` implements all of them. Its anti-aliased paths are rasterised
by [go-gfx/gfx/vector](https://github.com/go-gfx/gfx), in Go; that is the
module's one dependency.

## Try it

```bash
# pixels, written as a PNG — what a browser canvas or a window would get
go run ./cmd/wui-demo --out demo.png
go run ./cmd/wui-demo --out demo.png --theme dark

# cells, written to stdout as 24-bit ANSI
go run ./cmd/tui-demo
go run ./cmd/tui-demo --theme dark

# pixels in a browser
task serve                                      # http://localhost:8091/
```

The three demos draw the same three sample widgets (`widget.go`: `Label`, two
`Button`s, a `ProgressBar`) through the same code; only the painter changes.
Those widgets and the 5×7 font in `font.go` exist for the demos. The real
widget set, and its full font support, is
[go-widgets/toolkit](https://github.com/go-widgets/toolkit).

## Status

The production seam of go-widgets/toolkit and go-widgets/tui. 100% statement
coverage, `CGO_ENABLED=0`, cross-built for amd64, arm64, riscv64, loong64,
ppc64le and s390x, and `GOOS=js GOARCH=wasm`.

## License

BSD 3-Clause. See [LICENSE](./LICENSE).
