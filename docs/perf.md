# Performance — the `go-gfx/gfx/vector` migration

`FillPath` / `StrokePath` used to run painter's own hand-rolled anti-aliased
rasterizer. They now consume the shared **`go-gfx/gfx/vector`** rasterizer, which
is the exact same code moved out verbatim. The rule for that move was: **prove it
is pixel-identical, and prove it does not regress performance.**

## Pixel identity

`render_parity_test.go` renders one large widget scene — rounded buttons, a
framed panel, a bar chart, a line chart (`StrokePath`), a filled star icon
(`FillPath`, even-odd), an image thumbnail, a glyph coverage mask, and a table
grid, under clips and a translation, over a translucent wash — **twice** on the
same machine: once through the migrated `FillPath` / `StrokePath`, once through a
hermetic verbatim copy of the pre-migration rasterizer + composite
(`render_refimpl_test.go`). The two RGBA buffers are asserted **byte-identical**
(76 800 painted pixels). A control test perturbs one stroke width and confirms
the comparison flags the difference, so the pass cannot be vacuous. (The
coverage-stage identity is proven independently, byte-for-byte over 754
shape/rule/width/clamp combinations, in go-gfx's own `parity_test.go`.)

## No performance regression (benchstat, arm64, Go 1.26.4, `-count=6`)

`before` = pristine `origin/main` (painter's inline rasterizer); `after` = this
branch (consuming `go-gfx/gfx/vector`). Same benchmarks, same machine.

```
                          │   before    │             after                  │
                          │   sec/op    │   sec/op     vs base               │
FillPathCircleNonZero-16     1.260m ± 9%   1.161m ± 2%  -7.81% (p=0.002 n=6)
FillPathPolygonNonZero-16    1.409m ± 1%   1.304m ± 1%  -7.46% (p=0.002 n=6)
FillPathStarNonZero-16       572.0µ ± 1%   547.9µ ± 0%  -4.21% (p=0.002 n=6)
FillPathStarEvenOdd-16       569.5µ ± 1%   548.4µ ± 1%  -3.70% (p=0.002 n=6)
FillPathClipped-16           906.0µ ± 1%   871.9µ ± 0%  -3.76% (p=0.002 n=6)
StrokePathPolyline-16        771.7µ ± 3%   747.4µ ± 1%  -3.15% (p=0.002 n=6)
StrokePathCircle-16          239.1µ ± 2%   217.9µ ± 1%  -8.88% (p=0.002 n=6)
geomean                      716.2µ        676.2µ       -5.59%

                          │ allocs/op │ allocs/op vs base            │
FillPathCircleNonZero-16     17.00 ± 0%   17.00 ± 0%  ~ (all equal)
FillPathPolygonNonZero-16    18.00 ± 0%   18.00 ± 0%  ~ (all equal)
FillPathStarNonZero-16       10.00 ± 0%   10.00 ± 0%  ~ (all equal)
FillPathStarEvenOdd-16       10.00 ± 0%   10.00 ± 0%  ~ (all equal)
FillPathClipped-16           17.00 ± 0%   17.00 ± 0%  ~ (all equal)
StrokePathPolyline-16        78.00 ± 0%   78.00 ± 0%  ~ (all equal)
StrokePathCircle-16          146.0 ± 0%   146.0 ± 0%  ~ (all equal)
```

- **Allocations are byte-identical** (every sample equal) — the reused
  `Rasterizer` scratch amortises exactly as painter's old on-`PixelPainter`
  scratch did; the residual allocations (path flattening, edge list, per-segment
  stroke slices) were there before and are unchanged.
- **Bytes/op** are within noise (geomean −0.77%).
- **Time is ~5.6% FASTER** geomean, not slower — the extracted `Rasterizer`
  is a tighter struct and inlines at least as well. The requirement was "no
  regression"; the move is a small net win.

The rasterizer is scalar `float64` scanline coverage — there is **no SIMD /
go-asmgen path** to preserve (unlike `go-gfx/gfx/resample`). painter's
`StrokePath` fast paths (per-segment sub-box rasterisation, coverage-MAX union,
disk-span clamping) all moved intact, so the large speedups those bought over a
naïve per-pixel stroker stand.
