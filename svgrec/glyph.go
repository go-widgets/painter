// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"

	"github.com/go-widgets/painter"
)

// Glyphs as symbols (SVG 2, https://www.w3.org/TR/SVG2/struct.html)
//
// A text run set in a real font repeats the same few glyphs hundreds of times.
// Writing each as a full outline path costs the outline every time; here each
// DISTINCT outline is written once, as a <symbol> in <defs> (§5.5 The 'symbol'
// element, §5.3 'defs'), and every occurrence is a <use> (§5.6) that only says
// where. The symbol is declared overflow="visible": a symbol establishes a
// viewport (§5.5.1) that clips to the 'use' width and height by default, and a
// glyph reaches above and left of its origin.
//
// The symbol id is content-addressed, "g" and a hash of the path data, so the
// same outline gets the same id in every recorder: a set of frames recorded by
// different Recorders shares its symbols byte for byte, which is what lets an
// assembler of those frames emit each one once. The path data is
// https://www.w3.org/TR/SVG2/paths.html §9.3 absolute commands, relative to the
// glyph origin (the pen position on the baseline), three decimals, so a glyph
// placed at a fractional pen position keeps its shape and only x and y carry
// the fraction. The fill is set once, on the run's <g>, and inherited through
// <use> (§5.6.4): the symbol carries no colour, so one symbol serves any ink.

// maxGlyphKey bounds a Glyph.Key; a key is only a lookup, never written out.
const maxGlyphKey = 256

// Glyph is one glyph of a text run.
type Glyph struct {
	// Key says which outline this is: the same Key must always mean the same
	// outline (typically font, size and glyph id). It is a lookup only, so the
	// output does not depend on its spelling.
	Key string
	// X, Y is the pen position, where the glyph origin goes, in pixels, Y down.
	X, Y float64
	// Outline returns the glyph outline relative to its origin, in pixels, Y
	// down. It is called at most once per Key and Recorder, the first time the
	// Key is drawn; nil, or an outline with nothing to paint (a space), draws
	// nothing.
	Outline func() *painter.Path
}

// TextGlyphs draws a run of glyphs, in ink, as <use> references to shared
// symbols, inside a <g> that carries the string s as its <title> (the
// accessible name, as Text does). Only glyphs with an outline are drawn. s is
// validated like Text's: valid UTF-8, characters XML 1.0 can carry.
func (r *Recorder) TextGlyphs(s string, glyphs []Glyph, ink painter.RGBA) {
	if s == "" || ink.A == 0 || len(glyphs) == 0 {
		return
	}
	if len(s) > maxText {
		r.fail(ErrTooLarge)
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
	dx, dy := r.shift(0, 0)
	ids := make([]string, len(glyphs))
	used := false
	for i, g := range glyphs {
		if len(g.Key) > maxGlyphKey || !finite(g.X) || !finite(g.Y) || !inRange(dx, dy) {
			r.fail(ErrRange)
			return
		}
		id, err := r.glyphSymbol(g)
		if err != nil {
			r.fail(err)
			return
		}
		ids[i] = id
		used = used || id != ""
	}
	if !used {
		return
	}
	r.begin("<g")
	r.paint("fill", ink)
	r.str("><title>")
	r.escape(s)
	r.str("</title>")
	for i, g := range glyphs {
		if ids[i] == "" {
			continue
		}
		r.str(`<use href="#`)
		r.str(ids[i])
		r.str(`"`)
		if x := g.X + float64(dx); x != 0 {
			r.str(` x="`)
			r.flt(x)
			r.str(`"`)
		}
		if y := g.Y + float64(dy); y != 0 {
			r.str(` y="`)
			r.flt(y)
			r.str(`"`)
		}
		r.str("/>")
	}
	r.str("</g>\n")
	r.commit(false)
}

// glyphSymbol returns the id of the symbol for g, defining it on first use. An
// empty id with a nil error is a glyph with nothing to paint.
func (r *Recorder) glyphSymbol(g Glyph) (string, error) {
	if id, ok := r.glyphs[g.Key]; ok {
		return id, nil
	}
	var id string
	if g.Outline != nil {
		if pth := g.Outline(); pth != nil {
			d, drawn, err := r.encodePath(pth, 0, 0)
			if err != nil {
				return "", err
			}
			if drawn {
				if id, err = r.defineSymbol(d); err != nil {
					return "", err
				}
			}
		}
	}
	if r.glyphs == nil {
		r.glyphs = map[string]string{}
	}
	r.glyphs[g.Key] = id
	return id, nil
}

// symbolHash is the hex digest a symbol id is cut from. A variable so a test can
// force the collision that SHA-256 will not produce.
var symbolHash = func(d []byte) string {
	sum := sha256.Sum256(d)
	return hex.EncodeToString(sum[:])
}

// defineSymbol writes the symbol for path data d unless the same data is
// already defined, and returns its id.
func (r *Recorder) defineSymbol(d []byte) (string, error) {
	hexed := symbolHash(d)
	for n := 10; n <= 32; n += 2 { // a longer prefix on the (astronomically rare) collision
		id := "g" + hexed[:n]
		have, ok := r.defIDs[id]
		if ok && have == string(d) {
			return id, nil
		}
		if ok {
			continue
		}
		el := `<symbol id="` + id + `" overflow="visible"><path d="` + string(d) + "\"/></symbol>\n"
		if len(el) > r.Limits.callBytes() || len(r.body)+len(r.defs)+len(el) > r.Limits.totalBytes() {
			return "", ErrTooLarge
		}
		if r.defIDs == nil {
			r.defIDs = map[string]string{}
		}
		r.defIDs[id] = string(d)
		r.defs = append(r.defs, el...)
		return id, nil
	}
	return "", ErrTooLarge // 32 hex digits of SHA-256 collided: not reachable in practice
}
