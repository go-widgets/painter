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
| `RasterPainter` | `Raster` (a marker) | `raster.go` |

`svgrec.Recorder` (`svgrec/`) implements `Clipper`, `Translator`, `PathPainter`,
`ImagePainter` and `MaskPainter`, and records instead of drawing (below).

`IconBGRAOver` (`icon.go`) renders a decoded RGBA picture, shrunk to fit a box
and composited over a background colour, as the top-down BGRA a native button
(a Windows DIB section) takes; `IconSize` is its aspect-preserving fit. A
`DrawImage` on a BGRA painter swaps opaque pixels too (before v0.20.0 it copied
them as they were).

A `DrawImage` shown smaller than the picture averages the source under each
destination pixel (box filter on premultiplied alpha; an enlarged axis stays
nearest). The arithmetic is exact integer, so every byte is within 0.5 of the
real-number area mean (asserted against a float64 oracle), and the two passes
run in the NEON / AVX2 kernels of
[go-simd/boxfilter](https://github.com/go-simd/boxfilter); a call allocates
nothing. Apple M4 Max, one core, `go test -bench Downscale -cpu 1`, load 8:

| size | v0.20.0 (nearest) | v0.20.1 (float) | now |
| --- | ---: | ---: | ---: |
| 256x256 to 24x24 | 3.0 us | 108 us, 2 allocs | 20.7 us, 0 allocs |
| 64x64 to 16x16 | 0.55 us | 8.4 us, 2 allocs | 2.3 us, 0 allocs |
| 1000x700 to 250x175 | 95 us | 1.41 ms, 2 allocs | 315 us, 0 allocs |

`RasterPainter` says that `PutPixel` lays down exactly the pixel PixelPainter
would. Code that composes pixel by pixel, such as a bitmap font lighting one
block per glyph bit, asks for it rather than for `*PixelPainter`. A painter
that draws the same pixels another way, such as go-widgets/gpupainter on the
GPU, then gets the same request. `CellPainter` does not implement it.

```go
if c, ok := p.(painter.Clipper); ok {
	c.PushClip(bounds)
	defer c.PopClip()
}
```

`PixelPainter` implements all of them. Its anti-aliased paths are rasterised
by [go-gfx/gfx/vector](https://github.com/go-gfx/gfx), in Go; that is the
module's one dependency.

## A recording backend: `svgrec`

`svgrec.Recorder` turns the same calls into a **native SVG document**: `<rect>`,
`<path>`, `<clipPath>`, and an `<image>` only where a raster is really drawn.
The output is lossless, resizable and byte-for-byte deterministic: the same
calls give the same bytes, with ids `c0`, `c1`, ... and fixed attribute order.

```go
r, _ := svgrec.New(1000, 700)
widget.Draw(r, painter.LightTheme())
doc, err := r.SVG() // err is the first call that was refused, if any
```

It implements `Clipper`, `Translator`, `PathPainter`, `ImagePainter` and
`MaskPainter`. It does not implement `FacePainter` (below) or `RasterPainter`.

| Painter call | SVG | Faithful? |
| --- | --- | --- |
| `FillRect`, `PutPixel` | `<rect>` with `fill`, `fill-opacity` | yes |
| `FillRoundRect` | `<rect rx>` | radius clamped like PixelPainter; edge AA differs |
| `StrokeRect` | an `evenodd` `<path>`: outer rectangle plus the inner one, rings drawn inward like PixelPainter; a plain `<rect>` once the rings meet | yes |
| `StrokeRoundRect` | the same frame with Bézier corners (the rasteriser we test with has no arc command) | edge AA differs |
| `Text` | glyph outlines, see below | yes, on the pixel grid |
| `TextGlyphs` (recorder method) | one `<symbol>` per distinct glyph in `<defs>`, one `<use href x y>` per occurrence, inside `<g fill><title>text</title>` | yes |
| `PushClip` / `PopClip` | `<clipPath><rect/></clipPath>` and a nested `<g clip-path>` | yes |
| `PushTranslate` / `PopTranslate` | applied to the coordinates, no `<g transform>` | yes |
| `FillPath` | `<path d fill-rule>` (`M L Q C Z`) | yes |
| `StrokePath` | `<path fill="none" stroke stroke-width>` with round caps and joins | the width is in user units, so it **scales with the viewBox**; a zero-length subpath paints a dot in SVG and nothing in PixelPainter |
| `DrawImage`, `DrawMask` | `<image href="data:image/png;base64,...">`, `image-rendering="pixelated"` when enlarged | raster by nature; counted in `Stats.Raster` |

**Text is outlines, not `<text>`.** An embedded subset font would need a font
writer, a TrueType face in the file, and a viewer that honours it (a viewer
that does not falls back to another face and the layout moves). The 5x7 bitmap
font is a pixel grid, so each string becomes one `<path>` of merged horizontal
runs inside `<g><title>text</title>...</g>`: it looks the same everywhere and
scales without blur. The price is that the run is not selectable text, and it
is larger than a `<text>` element; the `<title>` carries the string, escaped,
for assistive technology (SVG 2 §5.7, the `title` element). Text drawn through a
TrueType `Face` is not implemented (`FacePainter` is not offered), so such text
reaches the recorder as `DrawMask`, a raster.

**Glyph symbols.** A caller that owns the outlines (a text shaper) draws a run
with `Recorder.TextGlyphs(text, []svgrec.Glyph{{Key, X, Y, Outline}}, ink)`.
Each distinct glyph is written once, origin-relative, as a
`<symbol overflow="visible"><path/></symbol>` in `<defs>` (SVG 2, structure,
5.3 to 5.6) with a content-addressed id, and each occurrence is a `<use>` at
its pen position: a page of text costs its alphabet plus one short element per
letter, not one path per letter. `Key` names the outline; `Outline` is called at
most once per key and recorder. `go-widgets/svg` `Assemble` merges the symbols
of different frames, and expands them in its still frames. On a 40 line, 1600
glyph page of 26 distinct outlines (`go test -bench TextPage ./svgrec`): 1 099 644
bytes (257 757 gzip) drawn as one path per glyph, 87 436 bytes (11 699 gzip)
as symbols.

**Limits.** Painter methods return no error, so a refused call is dropped, the
first error is kept, and `Err()` and `SVG()` report it. Text must be valid
UTF-8 and made of XML 1.0 characters; coordinates are bounded by `MaxCoord`
(1<<24) and the canvas by `MaxDim` (1<<14); `Limits` bounds one call
(`MaxCallBytes`, 4 MiB), the whole document (`MaxTotalBytes`, 64 MiB) and an
image (`MaxImagePixels`, 1 Mi). The fuzz targets parse the output back with
`encoding/xml` and fail on any element or attribute outside the writer's own
vocabulary; the corpus is committed in `svgrec/testdata/fuzz`.

**Equivalence.** The tests render the same scenes with `PixelPainter` and with
the recorder followed by the pure-Go rasteriser `go-gfx/gfx/svg`, scale 1, over
an opaque ground. Stated tolerance, per channel (0..255): rectangles, stroked
rectangles, bitmap text, clips, translation and the sample widgets **0**;
filled and stroked paths **1**; opaque images and binary masks **0**; rounded
rectangles **36** and stroked rounded ones **61** (on 3 % and 7.5 % of the
pixels: the raster painter covers a corner by distance to the pixel centre,
the SVG rasteriser by true coverage); translucent images and masks **50**, a
quirk of that rasteriser's image path (the colour of a translucent pixel is
weighted by alpha twice) rather than of the recorder. Goldens for four
widgets live in `svgrec/testdata`; regenerate with `go test ./svgrec -run
TestGolden -update`.

**Cost (a baseline, not a gate).** One 1000x700 frame (background, 24 buttons,
12 progress bars, 30 labels), Apple M4 Max, one core, `go test -bench Frame
-benchmem -cpu 1 ./svgrec`, best of 4. The machine was loaded (1-minute load
9 to 14, above the 8 we measure under), so read the times as indicative only;
bytes and allocations are exact.

| | time | allocated | allocs | output |
| --- | ---: | ---: | ---: | ---: |
| `PixelPainter` (reusable RGBA buffer) | 177 us | 1.3 KB | 55 | 2.8 MB of pixels |
| `Recorder`, calls only | 300 us | 397 KB | 455 | 138 elements, 130 KB of SVG |
| `Recorder` plus `SVG()` | 329 us | 528 KB | 458 | the same document |

The recording is the larger object to build, and the smaller one to keep: 21
times fewer bytes than the pixels, at any zoom.

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
