// wordboxes: crengine's page and boxes for every word listed in WORDS.json (from experiment 03's spanwords), with
// the same settings as readalong, written as OUT.json ([{spine, id, page, rects}]), to compare line and page breaks.
//
//	wordboxes BOOK.epub WORDS.json OUT.json
package main

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"

	"einkcre/cre"
)

type word struct {
	Spine int    `json:"spine"`
	ID    string `json:"id"`
}

type box struct {
	Spine int               `json:"spine"`
	ID    string            `json:"id"`
	Page  int               `json:"page"`
	Rects []image.Rectangle `json:"rects"`
}

func main() {
	abs, _ := filepath.Abs(os.Args[1])
	raw, err := os.ReadFile(os.Args[2])
	check(err)
	var words []word
	check(json.Unmarshal(raw, &words))

	cre.Init("", 0, "/opt/crengine/data/hyph/")
	cre.RegisterFont("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	d := cre.New(1072, 1448)
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
	d.SetFontSize(34)
	for _, side := range []string{"left", "top", "right", "bottom"} {
		d.SetInt("crengine.page.margin."+side, 60)
	}
	if !d.Load(abs) {
		panic("crengine could not load " + abs)
	}
	d.Render()
	page := image.NewGray(image.Rect(0, 0, 1072, 1448))
	shown := -1
	var out []box
	for _, w := range words {
		b := box{Spine: w.Spine, ID: w.ID, Page: d.IDPage(w.Spine, w.ID)}
		if b.Page >= 0 {
			if b.Page != shown {
				d.DrawPage(b.Page, page) // the boxes are of the page shown
				shown = b.Page
			}
			b.Rects = d.IDRects(w.Spine, w.ID)
		}
		out = append(out, b)
	}
	j, _ := json.Marshal(out)
	check(os.WriteFile(os.Args[3], j, 0o644))
	d.Close()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
