//go:build blitz

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	blitz "indiumengine"
)

// Every panel drawn in several states, to compare before and after a change to how they are built. PANELS: a folder
// to write the pictures into (the test is skipped without it); FONTS: a fallback fonts folder.
func TestPanels(t *testing.T) {
	dir := os.Getenv("PANELS")
	if dir == "" {
		t.Skip("PANELS not set")
	}
	os.MkdirAll(dir, 0o755)
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	const w, h = 1236, 1648
	save := func(name string, img image.Image) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		png.Encode(f, img)
	}
	for _, narrated := range []bool{true, false} {
		for _, hide := range []bool{false, true} {
			for _, dir := range []string{"", "rtl", "ltr"} {
				for _, lh := range []float32{0, 1.35} {
					s := &settings{FontSize: 50, LineHeight: lh, Margin: 60, HideClock: hide, Direction: map[string]string{"k": dir}}
					m := drawMenu(s, "k", 1.16, narrated, true, fonts, w)
					save(fmt.Sprintf("menu-narrated%v-hide%v-dir%s-lh%g", narrated, hide, dir, lh), m.img)
				}
			}
		}
	}
	save("remove", drawRemove(fonts, w).img)
	save("splash", splash(t.TempDir(), fonts, w, h))
	save("splash-kobo", splash(t.TempDir(), fonts, 1264, 1680))
	for _, rtl := range []bool{true, false} {
		for _, back := range []int{0, 7} {
			for _, share := range []float64{0, 0.25, 1} {
				p := progress{chapter: "Tom & Jerry <the first> chapter", at: 3, of: 12, page: 40, total: 300, share: share, back: back, rtl: rtl}
				save(fmt.Sprintf("nav-rtl%v-back%d-share%g", rtl, back, share), drawNav(p, fonts, w, h).img)
			}
		}
	}
	var es []entry
	for i := range 40 {
		es = append(es, entry{title: fmt.Sprintf("Chapter %d of the part", i), depth: i % 3})
	}
	es[3].title = "Salt & pepper"
	es[4].title = "Less < more"
	es[5].title = ""
	es[6].title = "a > b"
	for _, rtl := range []bool{true, false} {
		v := newContentsView(es, 7, rtl, fonts, w, h)
		if v.pages < 2 {
			t.Errorf("contents: %d pages, want at least 2", v.pages)
		}
		for page := range 2 {
			v.page = page
			img := image.NewGray(image.Rect(0, 0, w, h))
			v.draw(img)
			save(fmt.Sprintf("contents-rtl%v-page%d", rtl, page), img)
		}
		v.close()
	}
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	newClockFace(fonts).draw(img, time.Date(2026, 10, 29, 20, 48, 0, 0, time.UTC), 60, w/2, false, false)
	save("clock", img.SubImage(image.Rect(0, 0, w, 200)))
}

// Every button's bottom edge is drawn on its panel: a page ends at its last line of text, so a box whose edge lies
// below its text can be cut off when the panel is laid out too short for the line drawPanel adds after it.
func TestPanelEdges(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	const w, h = 1236, 1648
	s := &settings{FontSize: 50, Margin: 60, Direction: map[string]string{}}
	p := progress{chapter: "A chapter", at: 3, of: 12, page: 40, total: 300, share: 0.25, back: 7}
	v := newContentsView([]entry{{title: "One"}, {title: "Two"}}, 0, false, fonts, w, h)
	defer v.close()
	for name, m := range map[string]*menu{"menu": drawMenu(s, "k", 1.16, true, false, fonts, w),
		"remove": drawRemove(fonts, w), "nav": drawNav(p, fonts, w, h), "contents": v.head} {
		for id, r := range m.buttons {
			// dark at the bottom of its box, or just under it (a bar's parts stand on the bar's own border)
			x, dark := (r.Min.X+r.Max.X)/2, false
			for _, y := range []int{r.Max.Y - 2, r.Max.Y + 1} {
				dark = dark || y < m.img.Rect.Dy() && m.img.GrayAt(x, y).Y < 128
			}
			if !dark {
				t.Errorf("%s: %s's bottom edge not drawn (%v, panel %d px)", name, id, r, m.img.Rect.Dy())
			}
		}
	}
}
