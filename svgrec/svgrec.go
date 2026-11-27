// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package svgrec is a recording painter.Painter that writes the draw calls of
// a frame as native SVG elements instead of pixels, so the capture of a widget
// tree can be scaled to any size without loss.
//
// # Mapping (SVG 2, https://www.w3.org/TR/SVG2/)
//
//	FillRect          <rect> (§10.2 The 'rect' element)
//	StrokeRect        <path fill-rule="evenodd"> frame: outer rect minus inner rect
//	FillRoundRect     <rect rx ry>
//	StrokeRoundRect   <path fill-rule="evenodd"> ring of cubic Béziers
//	PutPixel          <rect width="1" height="1">
//	Text              <g><title>text</title><path/></g>: glyph OUTLINES (below)
//	FillPath          <path d fill-rule> (§9.3 Path data; §13.4 Fill properties)
//	StrokePath        <path fill="none" stroke-width stroke-linecap/linejoin="round">
//	PushClip/PopClip  <clipPath> + <g clip-path> (the clipPath element, Masking chapter)
//	PushTranslate     applied to the coordinates (see below), no <g transform>
//	DrawImage         <image href="data:image/png;base64,…"> (§12.3 The 'image' element; §13.10.3 'image-rendering')
//	DrawMask          <image> of the ink tinted by the coverage mask
//
// Colour is sRGB, straight alpha, src-over: the same blend PixelPainter makes
// on bytes. Alpha 255 emits no opacity attribute.
//
// # Where the mapping is not exact (nothing is approximated silently)
//
//   - Pixel painters anti-alias a rounded corner from the distance of each
//     pixel centre to the corner circle; SVG integrates true coverage. Same
//     geometry (radius r circle), differences are sub-pixel, bounded by the
//     tolerance the equivalence test states.
//   - StrokeRect/StrokeRoundRect are drawn INWARD by lineW pixels like
//     PixelPainter (a stroke centred on the edge would spill outside the rect),
//     so they are emitted as a filled frame, not a stroke. When lineW is so
//     large that no hole is left, a filled rect/rounded rect is emitted.
//   - StrokePath keeps the width in user units, so it scales with the image
//     (SVG's default; vector-effect="non-scaling-stroke" is NOT used). A zero
//     length sub-path with round caps paints a dot in SVG; PixelPainter paints
//     nothing. Paths with no segment other than move/close are dropped.
//   - Clip: PixelPainter clips with the INTERSECTION of the pushed rects, which
//     is what nested <g clip-path> groups compute. Clip rects are in absolute
//     (already translated) coordinates, so a translation pushed after a clip
//     does not move it, as in PixelPainter.
//   - Blend modes: the Painter interface has none; every call is src-over.
//   - DrawImage/DrawMask are RASTER: they embed a PNG, and only a widget that
//     really draws a raster image or a glyph mask reaches them. They are counted
//     in [Stats].Raster so a capture can tell how vector it is. An enlargement
//     is image-rendering:pixelated (nearest, like PixelPainter); a reduction is
//     left to the viewer's filter (PixelPainter box-filters), which is the
//     closest correct choice, not an identical one. PNG bytes follow the Go
//     release's compress/flate, so they are deterministic for a given toolchain
//     but not guaranteed across Go versions.
//   - FacePainter is NOT implemented: this package has no TrueType outliner, so
//     a toolkit font falls back to DrawMask (raster). Real outlines for a Face
//     are a follow-up; see the repository README.
//
// # Text: glyph outlines, not an embedded font
//
// The built-in font is a 5x7 bitmap. Text emits its lit cells as ONE <path> of
// merged horizontal runs, on the pixel grid. That is the font's exact outline,
// resolution independent, needs no font to be installed, and cannot reflow or
// substitute. An embedded subsetted font would need an sfnt writer and a
// TrueType face the bitmap does not have, would make the viewer's hinting and
// rasteriser decide the look, and could fall back to another face when the
// font fails to load. The trade-off: the run is not selectable or searchable
// text. To keep the text accessible the string is carried in a <title> child of
// the <g> (SVG 2 §5.7, the 'title' element), escaped. Bytes the font does not draw
// are skipped but still advance the pen, exactly as PixelPainter.Text.
//
// # Determinism and limits
//
// The same calls give byte-identical output: ids are c0, c1, … in call order,
// integers are decimal, floats are fixed to three decimals with trailing zeros
// trimmed, attributes have a fixed order. Painter methods cannot return an
// error, so a refused call is dropped and the first error is kept: [Recorder.Err]
// and [Recorder.SVG] report it, so a capture can never be silently incomplete.
// A call is refused when its text is not valid UTF-8 or holds a character XML
// 1.0 cannot carry, when a coordinate is NaN, infinite or beyond ±2^24, or when
// the element it would write exceeds [Limits].
package svgrec

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"math"
	mbits "math/bits"
	"strconv"
	"unicode/utf8"

	"github.com/go-gfx/gfx/vector"
	"github.com/go-widgets/painter"
)

// Errors reported by [Recorder.Err]. They are wrapped with the call that failed.
var (
	// ErrInvalidUTF8 is a text argument that is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("svgrec: text is not valid UTF-8")
	// ErrInvalidText is a text argument holding a character XML 1.0 cannot carry.
	ErrInvalidText = errors.New("svgrec: text holds a character XML cannot carry")
	// ErrRange is a coordinate or size that is NaN, infinite or too large.
	ErrRange = errors.New("svgrec: coordinate out of range")
	// ErrTooLarge is a call whose element, or a document whose total, exceeds
	// [Limits].
	ErrTooLarge = errors.New("svgrec: output exceeds its limit")
)

const (
	// MaxCoord bounds every coordinate, size and offset (2^24): beyond it an
	// int64 sum could not overflow, and no real surface is larger.
	MaxCoord = 1 << 24
	// MaxDim bounds the surface given to [New].
	MaxDim = 1 << 14
	// maxText bounds the bytes of one Text call before any glyph is laid out.
	maxText = 1 << 16
)

// Limits bound what a recording may write. The zero value of a field means its
// default.
type Limits struct {
	// MaxCallBytes bounds the bytes ONE call may add (default 4 MiB).
	MaxCallBytes int
	// MaxTotalBytes bounds the whole document (default 64 MiB).
	MaxTotalBytes int
	// MaxImagePixels bounds DrawImage's source and DrawMask's rectangle, in
	// pixels (default 1 Mi).
	MaxImagePixels int
}

func (l Limits) callBytes() int  { return orDefault(l.MaxCallBytes, 4<<20) }
func (l Limits) totalBytes() int { return orDefault(l.MaxTotalBytes, 64<<20) }
func (l Limits) imagePx() int    { return orDefault(l.MaxImagePixels, 1<<20) }

func orDefault(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

// Stats counts what a recording holds.
type Stats struct {
	Elements int // drawing elements written
	Raster   int // of which embed a PNG (DrawImage, DrawMask)
	Dropped  int // calls refused; the first reason is in Err
	Bytes    int // size of the body so far
}

// Recorder is a painter.Painter that records SVG. Its methods are not safe for
// concurrent use.
type Recorder struct {
	// Limits bound the output; set before drawing.
	Limits Limits

	w, h  int
	body  []byte
	buf   []byte
	off   [][2]int
	open  int // open clip groups
	nclip int
	scale int
	err   error
	stats Stats
}

var (
	_ painter.Painter      = (*Recorder)(nil)
	_ painter.Clipper      = (*Recorder)(nil)
	_ painter.Translator   = (*Recorder)(nil)
	_ painter.PathPainter  = (*Recorder)(nil)
	_ painter.ImagePainter = (*Recorder)(nil)
	_ painter.MaskPainter  = (*Recorder)(nil)
)

// New returns a recorder for a w x h surface, 1 <= w, h <= [MaxDim].
func New(w, h int) (*Recorder, error) {
	if w < 1 || h < 1 || w > MaxDim || h > MaxDim {
		return nil, errors.New("svgrec: surface size out of range")
	}
	return &Recorder{w: w, h: h}, nil
}

// Size returns the surface size in pixels.
func (r *Recorder) Size() (int, int) { return r.w, r.h }

// Err is the first reason a call was dropped, nil when none was.
func (r *Recorder) Err() error { return r.err }

// Stats reports what has been recorded.
func (r *Recorder) Stats() Stats {
	s := r.stats
	s.Bytes = len(r.body)
	return s
}

// Reset forgets every call, keeping the size, the limits and the text scale.
func (r *Recorder) Reset() {
	r.body, r.off, r.open, r.nclip, r.err, r.stats = r.body[:0], nil, 0, 0, nil, Stats{}
}

// SVG returns the document. Groups still open are closed in the copy returned,
// so it can be called again and keep recording. The error is [Recorder.Err], or
// ErrTooLarge when the document exceeds [Limits].MaxTotalBytes; the bytes are
// returned either way.
func (r *Recorder) SVG() ([]byte, error) {
	var b []byte
	b = append(b, `<svg xmlns="http://www.w3.org/2000/svg" width="`...)
	b = strconv.AppendInt(b, int64(r.w), 10)
	b = append(b, `" height="`...)
	b = strconv.AppendInt(b, int64(r.h), 10)
	b = append(b, `" viewBox="0 0 `...)
	b = strconv.AppendInt(b, int64(r.w), 10)
	b = append(b, ' ')
	b = strconv.AppendInt(b, int64(r.h), 10)
	b = append(b, "\">\n"...)
	b = append(b, r.body...)
	for i := 0; i < r.open; i++ {
		b = append(b, "</g>\n"...)
	}
	b = append(b, "</svg>\n"...)
	if r.err == nil && len(b) > r.Limits.totalBytes() {
		return b, ErrTooLarge
	}
	return b, r.err
}

// fail records a dropped call.
func (r *Recorder) fail(err error) {
	r.stats.Dropped++
	if r.err == nil {
		r.err = err
	}
}

// begin starts an element in the scratch buffer.
func (r *Recorder) begin(s string) { r.buf = append(r.buf[:0], s...) }

func (r *Recorder) str(s string)  { r.buf = append(r.buf, s...) }
func (r *Recorder) num(v int)     { r.buf = strconv.AppendInt(r.buf, int64(v), 10) }
func (r *Recorder) flt(v float64) { r.buf = appendFloat(r.buf, v) }
func (r *Recorder) attr(n string, v int) {
	r.str(" ")
	r.str(n)
	r.str(`="`)
	r.num(v)
	r.str(`"`)
}

// commit appends the scratch element to the body when it fits the limits.
func (r *Recorder) commit(raster bool) {
	if len(r.buf) > r.Limits.callBytes() || len(r.body)+len(r.buf) > r.Limits.totalBytes() {
		r.fail(ErrTooLarge)
		return
	}
	r.body = append(r.body, r.buf...)
	r.stats.Elements++
	if raster {
		r.stats.Raster++
	}
}

// appendFloat writes v with three decimals, trailing zeros trimmed, "-0" as "0".
func appendFloat(b []byte, v float64) []byte {
	n := len(b)
	b = strconv.AppendFloat(b, v, 'f', 3, 64)
	for b[len(b)-1] == '0' {
		b = b[:len(b)-1]
	}
	if b[len(b)-1] == '.' {
		b = b[:len(b)-1]
	}
	if len(b)-n == 2 && b[n] == '-' && b[n+1] == '0' {
		b = append(b[:n], '0')
	}
	return b
}

var alphaTab = func() (t [256]string) {
	for a := range t {
		b := strconv.AppendFloat(nil, float64(a)/255, 'f', 5, 64)
		for b[len(b)-1] == '0' && b[len(b)-2] != '.' {
			b = b[:len(b)-1]
		}
		t[a] = string(b)
	}
	return t
}()

const hexdigits = "0123456789abcdef"

// paint writes ` <name>="#rrggbb"` and, for a translucent colour, the opacity.
func (r *Recorder) paint(name string, c painter.RGBA) {
	r.str(" ")
	r.str(name)
	r.str(`="#`)
	r.buf = append(r.buf, hexdigits[c.R>>4], hexdigits[c.R&15], hexdigits[c.G>>4], hexdigits[c.G&15], hexdigits[c.B>>4], hexdigits[c.B&15])
	r.str(`"`)
	if c.A != 0xFF {
		r.str(" ")
		r.str(name)
		r.str(`-opacity="`)
		r.str(alphaTab[c.A])
		r.str(`"`)
	}
}

// shift applies the current translation to a point.
func (r *Recorder) shift(x, y int) (int, int) {
	n := len(r.off)
	if n == 0 {
		return x, y
	}
	return x + r.off[n-1][0], y + r.off[n-1][1]
}

func inRange(vs ...int) bool {
	for _, v := range vs {
		if v > MaxCoord || v < -MaxCoord {
			return false
		}
	}
	return true
}

// rect checks and translates rc; ok is false (and the call recorded as
// dropped) when a value is out of range. An empty rect is ok with empty true.
func (r *Recorder) rect(rc painter.Rect) (out painter.Rect, ok bool) {
	if !inRange(rc.X, rc.Y, rc.W, rc.H) {
		r.fail(ErrRange)
		return rc, false
	}
	out.X, out.Y = r.shift(rc.X, rc.Y)
	out.W, out.H = rc.W, rc.H
	return out, true
}

// FillRect paints a solid rectangle. Implements painter.Painter.
func (r *Recorder) FillRect(rc painter.Rect, c painter.RGBA) {
	if rc.W <= 0 || rc.H <= 0 || c.A == 0 {
		return
	}
	rc, ok := r.rect(rc)
	if !ok {
		return
	}
	r.fillRectEl(rc, 0, c)
}

// fillRectEl writes a <rect>, rounded when rad > 0, and commits it.
func (r *Recorder) fillRectEl(rc painter.Rect, rad int, c painter.RGBA) {
	r.begin("<rect")
	r.attr("x", rc.X)
	r.attr("y", rc.Y)
	r.attr("width", rc.W)
	r.attr("height", rc.H)
	if rad > 0 {
		r.attr("rx", rad)
	}
	r.paint("fill", c)
	r.str("/>\n")
	r.commit(false)
}

// PutPixel paints one pixel. Implements painter.Painter.
func (r *Recorder) PutPixel(x, y int, c painter.RGBA) {
	r.FillRect(painter.Rect{X: x, Y: y, W: 1, H: 1}, c)
}

// FillRoundRect fills rc with corners rounded to radius, clamped to half the
// smaller side; radius <= 0 is a FillRect. Implements painter.Painter.
func (r *Recorder) FillRoundRect(rc painter.Rect, radius int, c painter.RGBA) {
	if rc.W <= 0 || rc.H <= 0 || c.A == 0 {
		return
	}
	rc, ok := r.rect(rc)
	if !ok {
		return
	}
	r.fillRectEl(rc, clampRadius(radius, rc), c)
}

func clampRadius(radius int, rc painter.Rect) int {
	return max(min(radius, min(rc.W, rc.H)/2), 0)
}

// StrokeRect paints a frame lineW pixels wide INSIDE rc. Implements
// painter.Painter.
func (r *Recorder) StrokeRect(rc painter.Rect, c painter.RGBA, lineW int) {
	r.strokeFrame(rc, 0, c, lineW)
}

// StrokeRoundRect is StrokeRect with rounded corners. Implements
// painter.Painter.
func (r *Recorder) StrokeRoundRect(rc painter.Rect, radius int, c painter.RGBA, lineW int) {
	r.strokeFrame(rc, radius, c, lineW)
}

func (r *Recorder) strokeFrame(rc painter.Rect, radius int, c painter.RGBA, lineW int) {
	if rc.W <= 0 || rc.H <= 0 || c.A == 0 {
		return
	}
	rc, ok := r.rect(rc)
	if !ok {
		return
	}
	lw := max(lineW, 1)
	rad := clampRadius(radius, rc)
	if 2*lw >= min(rc.W, rc.H) { // the rings meet: no hole is left
		r.fillRectEl(rc, rad, c)
		return
	}
	in := painter.Rect{X: rc.X + lw, Y: rc.Y + lw, W: rc.W - 2*lw, H: rc.H - 2*lw}
	r.begin(`<path fill-rule="evenodd"`)
	r.paint("fill", c)
	r.str(` d="`)
	r.roundRectPath(rc, rad)
	r.roundRectPath(in, clampRadius(rad-lw, in))
	r.str("\"/>\n")
	r.commit(false)
}

// roundRectPath writes a closed rounded-rectangle sub-path with cubic Béziers
// (kappa for a quarter circle), a plain rectangle when rad is 0.
func (r *Recorder) roundRectPath(rc painter.Rect, rad int) {
	x, y, w, h := rc.X, rc.Y, rc.W, rc.H
	if rad == 0 {
		r.str("M")
		r.num(x)
		r.str(" ")
		r.num(y)
		r.str("h")
		r.num(w)
		r.str("v")
		r.num(h)
		r.str("h")
		r.num(-w)
		r.str("z")
		return
	}
	const kappa = 0.5522847498307936
	rr, k := float64(rad), kappa*float64(rad)
	fx, fy, fw, fh := float64(x), float64(y), float64(w), float64(h)
	pt := func(cmd string, v ...float64) {
		r.str(cmd)
		for i, f := range v {
			if i > 0 {
				r.str(" ")
			}
			r.flt(f)
		}
	}
	pt("M", fx+rr, fy)
	pt("L", fx+fw-rr, fy)
	pt("C", fx+fw-rr+k, fy, fx+fw, fy+rr-k, fx+fw, fy+rr)
	pt("L", fx+fw, fy+fh-rr)
	pt("C", fx+fw, fy+fh-rr+k, fx+fw-rr+k, fy+fh, fx+fw-rr, fy+fh)
	pt("L", fx+rr, fy+fh)
	pt("C", fx+rr-k, fy+fh, fx, fy+fh-rr+k, fx, fy+fh-rr)
	pt("L", fx, fy+rr)
	pt("C", fx, fy+rr-k, fx+rr-k, fy, fx+rr, fy)
	r.str("z")
}

// SetTextScale magnifies the built-in font by a whole number (below 1 is 1),
// as PixelPainter.SetTextScale.
func (r *Recorder) SetTextScale(n int) { r.scale = max(n, 1) }

// TextScale is the magnification Text is drawn at.
func (r *Recorder) TextScale() int { return max(r.scale, 1) }

// TextWidth is how wide s is drawn.
func (r *Recorder) TextWidth(s string) int { return len(s) * painter.GlyphAdvance * r.TextScale() }

// TextHeight is how tall a line is drawn.
func (r *Recorder) TextHeight() int { return painter.GlyphHeight * r.TextScale() }

// Text draws s with the built-in 5x7 font as glyph outlines, (x, y) the
// top-left corner. See the package comment. Implements painter.Painter.
func (r *Recorder) Text(x, y int, s string, ink painter.RGBA) {
	if s == "" || ink.A == 0 {
		return
	}
	if len(s) > maxText {
		r.fail(ErrTooLarge)
		return
	}
	if !inRange(x, y) {
		r.fail(ErrRange)
		return
	}
	if !utf8.ValidString(s) {
		r.fail(ErrInvalidUTF8)
		return
	}
	for _, c := range s {
		if !xmlChar(c) {
			r.fail(ErrInvalidText)
			return
		}
	}
	x, y = r.shift(x, y)
	n := r.TextScale()
	var d []byte
	for k := range len(s) {
		cols, ok := painter.Glyph(s[k])
		if !ok {
			continue
		}
		gx := x + k*painter.GlyphAdvance*n
		for row := range painter.GlyphHeight {
			// bits has bit col set where this row of the glyph is inked; each
			// maximal run of set bits becomes one rectangle.
			var bits uint
			for col := range painter.GlyphWidth {
				bits |= uint(cols[col]>>row&1) << col
			}
			for bits != 0 {
				col := mbits.TrailingZeros(bits)
				run := mbits.TrailingZeros(^(bits >> col))
				bits &^= (1<<run - 1) << col
				d = append(d, 'M')
				d = strconv.AppendInt(d, int64(gx+col*n), 10)
				d = append(d, ' ')
				d = strconv.AppendInt(d, int64(y+row*n), 10)
				d = append(d, 'h')
				d = strconv.AppendInt(d, int64(run*n), 10)
				d = append(d, 'v')
				d = strconv.AppendInt(d, int64(n), 10)
				d = append(d, 'h')
				d = strconv.AppendInt(d, int64(-run*n), 10)
				d = append(d, 'z')
			}
		}
	}
	if len(d) == 0 {
		return // nothing the font draws
	}
	r.begin("<g><title>")
	r.escape(s)
	r.str("</title><path")
	r.paint("fill", ink)
	r.str(` d="`)
	r.buf = append(r.buf, d...)
	r.str("\"/></g>\n")
	r.commit(false)
}

// xmlChar reports whether c is a character of XML 1.0 §2.2 Char.
func xmlChar(c rune) bool {
	return c == 0x9 || c == 0xA || c == 0xD ||
		(c >= 0x20 && c <= 0xD7FF) ||
		(c >= 0xE000 && c <= 0xFFFD) ||
		(c >= 0x10000 && c <= 0x10FFFF)
}

// escape writes s, already validated, as XML character data.
func (r *Recorder) escape(s string) {
	for i := range len(s) {
		switch s[i] {
		case '&':
			r.str("&amp;")
		case '<':
			r.str("&lt;")
		case '>':
			r.str("&gt;")
		case '"':
			r.str("&#34;")
		case '\'':
			r.str("&#39;")
		case '\r':
			r.str("&#13;")
		default:
			r.buf = append(r.buf, s[i])
		}
	}
}

// PushClip confines later drawing to rc (intersected with the enclosing clip,
// which nesting computes). Implements painter.Clipper.
func (r *Recorder) PushClip(rc painter.Rect) {
	rc, ok := r.rect(rc)
	if !ok {
		// Without the group, PopClip would close one that is not ours: keep the
		// stack balanced by pushing an empty clip, which draws nothing.
		rc = painter.Rect{}
	}
	rc.W, rc.H = max(rc.W, 0), max(rc.H, 0)
	r.begin(`<clipPath id="c`)
	r.num(r.nclip)
	r.str(`"><rect`)
	r.attr("x", rc.X)
	r.attr("y", rc.Y)
	r.attr("width", rc.W)
	r.attr("height", rc.H)
	r.str(`/></clipPath><g clip-path="url(#c`)
	r.num(r.nclip)
	r.str(")\">\n")
	// A clip group is structure, not a drawing element: it is not counted, and
	// it is exempt from the per-call limit that guards payloads.
	r.body = append(r.body, r.buf...)
	r.nclip++
	r.open++
}

// PopClip removes the most recent PushClip (a no-op with none). Implements
// painter.Clipper.
func (r *Recorder) PopClip() {
	if r.open == 0 {
		return
	}
	r.open--
	r.body = append(r.body, "</g>\n"...)
}

// PushTranslate shifts later drawing by dx, dy on top of the enclosing
// translation. Implements painter.Translator.
func (r *Recorder) PushTranslate(dx, dy int) {
	cx, cy := r.shift(0, 0)
	if !inRange(dx, dy) || !inRange(cx+dx, cy+dy) {
		r.fail(ErrRange)
		// Keep push/pop balanced: an out-of-range push shifts by nothing.
		dx, dy = 0, 0
	}
	r.off = append(r.off, [2]int{cx + dx, cy + dy})
}

// PopTranslate removes the most recent PushTranslate (a no-op with none).
// Implements painter.Translator.
func (r *Recorder) PopTranslate() {
	if n := len(r.off); n > 0 {
		r.off = r.off[:n-1]
	}
}

// FillPath fills pth under rule. Implements painter.PathPainter.
func (r *Recorder) FillPath(pth *painter.Path, c painter.RGBA, rule painter.FillRule) {
	if pth == nil || c.A == 0 {
		return
	}
	r.begin("<path")
	if rule == painter.EvenOdd {
		r.str(` fill-rule="evenodd"`)
	}
	r.paint("fill", c)
	r.pathData(pth)
}

// StrokePath strokes pth, centred on it, width wide, round joins and caps.
// Implements painter.PathPainter.
func (r *Recorder) StrokePath(pth *painter.Path, c painter.RGBA, width float64) {
	if pth == nil || c.A == 0 || !(width > 0) {
		return
	}
	if width > MaxCoord {
		r.fail(ErrRange)
		return
	}
	r.begin(`<path fill="none"`)
	r.paint("stroke", c)
	r.str(` stroke-width="`)
	r.flt(width)
	r.str(`" stroke-linecap="round" stroke-linejoin="round"`)
	r.pathData(pth)
}

// pathData finishes a path element whose attributes are in the scratch buffer.
func (r *Recorder) pathData(pth *painter.Path) {
	var d []byte
	var perr error
	drawn := false
	dx, dy := r.shift(0, 0)
	ox, oy := float64(dx), float64(dy)
	pth.Each(func(op vector.PathOp, pts ...[2]float64) {
		letter := "MLQCZ"[op]
		if op != vector.OpMove && op != vector.OpClose {
			drawn = true
		}
		d = append(d, letter)
		for i, p := range pts {
			if !finite(p[0]) || !finite(p[1]) {
				perr = ErrRange
				return
			}
			if i > 0 {
				d = append(d, ' ')
			}
			d = appendFloat(d, p[0]+ox)
			d = append(d, ' ')
			d = appendFloat(d, p[1]+oy)
		}
		if len(d) > r.Limits.callBytes() {
			perr = ErrTooLarge
		}
	})
	switch {
	case perr != nil:
		r.fail(perr)
	case !drawn:
		// nothing to paint
	default:
		r.str(` d="`)
		r.buf = append(r.buf, d...)
		r.str("\"/>\n")
		r.commit(false)
	}
}

func finite(v float64) bool { return !math.IsNaN(v) && math.Abs(v) <= MaxCoord }

// DrawImage embeds src (srcW x srcH, 4 bytes per pixel, straight RGBA) as a
// PNG scaled into dst. A src too short for its size is ignored, like
// PixelPainter. Implements painter.ImagePainter.
func (r *Recorder) DrawImage(dst painter.Rect, src []byte, srcW, srcH int) {
	if srcW <= 0 || srcH <= 0 || dst.W <= 0 || dst.H <= 0 {
		return
	}
	if !inRange(srcW, srcH) {
		r.fail(ErrRange)
		return
	}
	if srcW*srcH > r.Limits.imagePx() {
		r.fail(ErrTooLarge)
		return
	}
	if len(src) < srcW*srcH*4 {
		return
	}
	dst, ok := r.rect(dst)
	if !ok {
		return
	}
	pix := &image.NRGBA{Pix: src[:srcW*srcH*4], Stride: srcW * 4, Rect: image.Rect(0, 0, srcW, srcH)}
	r.imageEl(dst, pix, dst.W >= srcW && dst.H >= srcH)
}

// DrawMask paints ink through an 8-bit coverage mask, embedded as a PNG of the
// tinted coverage at 1:1. Implements painter.MaskPainter.
func (r *Recorder) DrawMask(dst painter.Rect, mask []byte, stride int, ink painter.RGBA) {
	if dst.W <= 0 || dst.H <= 0 || stride <= 0 || ink.A == 0 {
		return
	}
	if !inRange(dst.W, dst.H) {
		r.fail(ErrRange)
		return
	}
	if dst.W*dst.H > r.Limits.imagePx() {
		r.fail(ErrTooLarge)
		return
	}
	dst, ok := r.rect(dst)
	if !ok {
		return
	}
	pix := image.NewNRGBA(image.Rect(0, 0, dst.W, dst.H))
	for y := range dst.H {
		row := y * stride
		if row+dst.W > len(mask) {
			continue // unreadable rows stay transparent, as PixelPainter skips them
		}
		for x := range dst.W {
			o := pix.PixOffset(x, y)
			pix.Pix[o], pix.Pix[o+1], pix.Pix[o+2] = ink.R, ink.G, ink.B
			pix.Pix[o+3] = uint8((uint32(mask[row+x])*uint32(ink.A) + 127) / 255)
		}
	}
	r.imageEl(dst, pix, true)
}

// imageEl writes the <image> element for pix placed at dst.
func (r *Recorder) imageEl(dst painter.Rect, pix *image.NRGBA, pixelated bool) {
	var png_ bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	// Encode fails only when its writer does or the image is empty; a
	// bytes.Buffer never fails and callers pass a non-empty NRGBA.
	_ = enc.Encode(&png_, pix)
	// The size bound is commit's; MaxImagePixels already bounds the PNG.
	r.begin("<image")
	r.attr("x", dst.X)
	r.attr("y", dst.Y)
	r.attr("width", dst.W)
	r.attr("height", dst.H)
	r.str(` preserveAspectRatio="none"`)
	if pixelated {
		r.str(` image-rendering="pixelated"`)
	}
	r.str(` href="data:image/png;base64,`)
	r.buf = base64.StdEncoding.AppendEncode(r.buf, png_.Bytes())
	r.str("\"/>\n")
	r.commit(true)
}
