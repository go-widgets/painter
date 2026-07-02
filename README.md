# go-widgets/painter

[![CI](https://github.com/go-widgets/painter/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/painter/actions/workflows/ci.yml)
[![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-painter-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-widgets/painter)
![coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)
![go](https://img.shields.io/badge/Go-1.26.4%2B-00ADD8?logo=go&logoColor=white)
![status](https://img.shields.io/badge/status-prototype-9a6700)
[![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)](./LICENSE)

**Prototype** of a `Painter` abstraction that lets a single widget
render into three deployment families with the *same code*:

- **WUI** — browser wasm + `<canvas>` + `putImageData` (via `PixelPainter`)
- **GUI** — native window (SDL / Ebitengine / image files) (via `PixelPainter`)
- **TUI** — terminal cell grid + 24-bit ANSI (via `CellPainter`)

Both `PixelPainter` and `CellPainter` implement the same 5-primitive
`Painter` interface — widgets never see the back-end.

## Why this exists

Today the go-widgets/toolkit widget's `Draw` takes a `([]byte, w int,
h int)` — a hard binding to a pixel back-end. Great for the browser
and native canvases; unworkable for a terminal grid, where the atom
is a cell (rune + fg + bg), not a pixel.

This repo prototypes the redesign so we can answer:

> Can we write the same widget once and render it to WUI, GUI, *and*
> TUI without conditional-compilation gymnastics?

The 5-primitive interface says yes:

```go
type Painter interface {
    FillRect(r Rect, c RGBA)
    StrokeRect(r Rect, c RGBA, lineW int)
    PutPixel(x, y int, c RGBA)
    Text(x, y int, s string, ink RGBA)
    Size() (w, h int)
}

type Widget interface {
    Draw(p Painter, theme *Theme)
}
```

A widget's `Draw` composes only those five calls. The back-end
decides whether they land as pixels or cells.

## Try it

```bash
# WUI/GUI proxy — renders to a PNG the same way a browser canvas
# would consume the RGBA buffer.
go run ./cmd/wui-demo --out demo.png            # light theme
go run ./cmd/wui-demo --out demo.png --theme dark

# TUI — writes 24-bit-ANSI to stdout; point your terminal at it.
go run ./cmd/tui-demo
go run ./cmd/tui-demo --theme dark
```

Both commands render **the exact same three widgets** (`Label`,
two `Button`s, `ProgressBar`) through the exact same widget code
in `widget.go`.

## What's in the box

| File           | Contents                                             |
| -------------- | ---------------------------------------------------- |
| `painter.go`   | `Painter` interface, `Rect`, `RGBA`, `RGB` helper    |
| `pixel.go`     | `PixelPainter` — writes into an RGBA `[]byte` buffer |
| `cell.go`     | `CellPainter` — writes into a `[]Cell` grid + 24-bit-ANSI serializer |
| `font.go`      | Minimal 5×7 bitmap font (uppercase + digits + punct.) |
| `theme.go`     | `Theme` struct + `LightTheme` / `DarkTheme` palettes  |
| `widget.go`    | Sample widgets: `Button`, `Label`, `ProgressBar`      |
| `cmd/wui-demo` | Renders widgets to a PNG (WUI / GUI back-end proxy)   |
| `cmd/tui-demo` | Renders widgets to stdout as 24-bit ANSI (TUI)        |

## Status

**Prototype — design validation.** ~5-primitive API, 3 widgets,
100 % coverage on library packages, `CGO_ENABLED=0`, builds on all
6 supported 64-bit Go targets (amd64, arm64, riscv64, loong64,
ppc64le, s390x) + `GOOS=js GOARCH=wasm`.

The API is deliberately small. The next step, if the design proves
out, is a wholesale migration of go-widgets/toolkit's widget set to
this interface + folding this repo's implementation into the toolkit
as its v1.0 rendering path.

## Non-goals for the prototype

- Not a full toolkit — 3 widgets only.
- No event dispatch — `HitTest` / `OnEvent` are out of scope. They
  come back once the render side is validated.
- No full font — 5×7 bitmap covers uppercase + digits + a handful of
  punctuation. A production merge reuses go-widgets/toolkit's full
  font table.
- No terminal host loop — `cmd/tui-demo` just writes ANSI to stdout;
  it doesn't put the terminal into raw mode or dispatch input.

## License

BSD 3-Clause. See [LICENSE](./LICENSE).
