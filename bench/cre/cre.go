//go:build cre

// Package cre is experiment 02's crengine (KOReader's engine, C++ through cgo) as an engine.Document, with the
// settings experiment 01 compared against Chromium: KOReader's defaults for a new book, CSS word spacing. It needs
// the eink-cre image (crengine and its libraries) and -tags cre.
package cre

import (
	"fmt"
	"image"
	"sync"

	"einkcre/cre"
	engine "einkengine"
	"einkengine/epub"
)

func init() { engine.Register("cre", Open) }

var initOnce sync.Once

type doc struct {
	d       *cre.Doc
	shown   int         // the page crengine has shown last: Find's boxes are of that page
	scratch *image.Gray // for showing a page Find needs that the reader has not drawn
}

func Open(b *epub.Book, o engine.Options) (engine.Document, error) {
	initOnce.Do(func() {
		cre.Init("", 0, "/opt/crengine/data/hyph/")
		cre.RegisterFont(o.Fallback) // crengine needs a font registered before it opens a view
	})
	d := cre.New(o.Width, o.Height)
	d.SetString("crengine.font.fallback.faces", "DejaVu Sans")
	d.SetInt("crengine.render.dpi", 96)
	d.SetInt("crengine.render.block.rendering.flags", 0x7FFFFFFF)
	d.SetStyleSheet("/opt/crengine/data/epub.css")
	d.SetInt("crengine.doc.embedded.styles.enabled", 1)
	d.SetInt("crengine.doc.embedded.fonts.enabled", 1)
	d.SetInt("font.kerning.mode", 3)
	d.SetInt("font.hinting.mode", 2)
	d.SetInt("crengine.style.space.width.scale.percent", 100)
	d.SetInt("crengine.style.space.condensing.percent", 100)
	d.SetInterlineSpace(100)
	d.SetString("window.status.line", "1")
	d.SetString("crengine.textlang.main.lang", "fa")
	d.SetString("crengine.textlang.embedded.langs.enabled", "1")
	d.SetString("crengine.textlang.hyphenation.enabled", "1")
	d.SetFontSize(o.FontSize)
	for _, side := range []string{"left", "top", "right", "bottom"} {
		d.SetInt("crengine.page.margin."+side, o.Margin)
	}
	if !d.Load(b.Path) {
		d.Close()
		return nil, fmt.Errorf("crengine could not load %s", b.Path)
	}
	d.Render()
	return &doc{d: d, shown: -1, scratch: image.NewGray(image.Rect(0, 0, o.Width, o.Height))}, nil
}

func (d *doc) Pages() int { return d.d.PageCount() }
func (d *doc) Close()     { d.d.Close() }

func (d *doc) Draw(n int, img *image.Gray) {
	d.d.DrawPage(n, img)
	d.shown = n
}

func (d *doc) Find(spine int, id string) (int, []image.Rectangle) {
	p := d.d.IDPage(spine, id)
	if p < 0 {
		return -1, nil
	}
	if p != d.shown {
		d.Draw(p, d.scratch)
	}
	return p, d.d.IDRects(spine, id)
}
