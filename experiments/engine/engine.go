// Package engine is the seam between the reader and a layout engine: the reader (Go) opens the book with
// readium/go-toolkit, owns the screen and the narration; an engine lays the book out once, keeps it, draws pages into
// buffers the reader owns and says where an element is. Engines register by name: "gotext" (experiment 03, pure Go)
// always, "cre" (crengine through cgo, experiment 02) when built with -tags cre. See docs/engine-design.md.
package engine

import (
	"fmt"
	"image"
	"sort"

	"einkengine/epub"
)

// Options are the page and text settings every engine takes.
type Options struct {
	Width, Height int    // page, px
	Margin        int    // on all four sides, px
	FontSize      int    // the root font size, px
	Fallback      string // a font file for characters the book's own fonts lack
}

// Document is one book laid out at one page size. It is stateful: laid out once, at open.
type Document interface {
	// Pages is the number of pages.
	Pages() int
	// Draw draws page n (from 0) into img, which must be the page size: black text on white, 8-bit grey.
	Draw(n int, img *image.Gray)
	// Find is where the element with this id in spine item spine is: the page it starts on (-1 if not found) and
	// the boxes of its text on that page, one per line.
	Find(spine int, id string) (page int, boxes []image.Rectangle)
	Close()
}

// PageLines is optional: the spine item each page starts in and its lines of text (images not counted), for
// comparing pages with Chromium's.
type PageLines interface {
	PageLines() []PageInfo
}

type PageInfo struct {
	Spine int `json:"spine"`
	Lines int `json:"lines"`
}

// Opener lays a book out.
type Opener func(b *epub.Book, o Options) (Document, error)

var engines = map[string]Opener{}

// Register makes an engine available by name (from its package's init).
func Register(name string, open Opener) { engines[name] = open }

// Names are the engines in this build.
func Names() []string {
	var ns []string
	for n := range engines {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// Open lays the book out with the named engine.
func Open(name string, b *epub.Book, o Options) (Document, error) {
	open, ok := engines[name]
	if !ok {
		return nil, fmt.Errorf("no engine %q in this build (have %v)", name, Names())
	}
	return open(b, o)
}
