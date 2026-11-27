// Copyright (c) 2026 the go-widgets/painter authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package svgrec_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// allowed is the whole vocabulary the writer may emit: element name to the
// attribute names it may carry. Anything else in an output is an injection.
var allowed = map[string]map[string]bool{
	"svg":      set("xmlns", "width", "height", "viewBox"),
	"rect":     set("x", "y", "width", "height", "rx", "fill", "fill-opacity"),
	"path":     set("d", "fill", "fill-opacity", "fill-rule", "stroke", "stroke-opacity", "stroke-width", "stroke-linecap", "stroke-linejoin"),
	"g":        set("clip-path"),
	"title":    set(),
	"clipPath": set("id"),
	"image":    set("x", "y", "width", "height", "preserveAspectRatio", "image-rendering", "href"),
}

func set(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

// parsed is what a well-formedness pass found.
type parsed struct {
	titles   []string       // character data of every <title>, decoded
	elements map[string]int // count per element name
}

// parse reads doc back with encoding/xml and fails on anything that is not
// well formed, or that is not in the vocabulary: an unknown element or
// attribute, a processing instruction, a directive, a comment, or character
// data outside a <title>.
func parse(doc []byte) (parsed, error) {
	p := parsed{elements: map[string]int{}}
	d := xml.NewDecoder(bytes.NewReader(doc))
	d.Strict = true
	var stack []string
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return p, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			attrs, ok := allowed[t.Name.Local]
			if !ok {
				return p, fmt.Errorf("element %q is not in the vocabulary", t.Name.Local)
			}
			p.elements[t.Name.Local]++
			for _, a := range t.Attr {
				if !attrs[a.Name.Local] {
					return p, fmt.Errorf("attribute %q on <%s> is not in the vocabulary", a.Name.Local, t.Name.Local)
				}
			}
			stack = append(stack, t.Name.Local)
			if t.Name.Local == "title" {
				p.titles = append(p.titles, "")
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1] == "title" {
				p.titles[len(p.titles)-1] += string(t)
			} else if strings.TrimSpace(string(t)) != "" {
				return p, fmt.Errorf("character data %q outside a title", t)
			}
		default:
			return p, fmt.Errorf("unexpected token %T", tok)
		}
	}
	if len(stack) != 0 {
		return p, fmt.Errorf("unclosed elements %v", stack)
	}
	return p, nil
}
