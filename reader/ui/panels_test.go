//go:build blitz

package ui

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
					m := DrawMenu(Menu{FontSize: 50, LineHeight: lh, Lines: 1.16, Margin: 60, Dir: dir, BookRTL: true, Narrated: narrated, HideClock: hide}, fonts, w)
					save(fmt.Sprintf("menu-narrated%v-hide%v-dir%s-lh%g", narrated, hide, dir, lh), m.img)
				}
			}
		}
	}
	save("remove", DrawRemove(fonts, w).img)
	save("splash", Splash(t.TempDir(), fonts, w, h))
	save("splash-kobo", Splash(t.TempDir(), fonts, 1264, 1680))
	for _, rtl := range []bool{true, false} {
		for _, back := range []int{0, 7} {
			for _, share := range []float64{0, 0.25, 1} {
				p := Progress{Chapter: "Tom & Jerry <the first> chapter", At: 3, Of: 12, Page: 40, Total: 300, Share: share, Back: back, RTL: rtl}
				save(fmt.Sprintf("nav-rtl%v-back%d-share%g", rtl, back, share), DrawNav(p, fonts, w, h).img)
			}
		}
	}
	var es []Entry
	for i := range 40 {
		es = append(es, Entry{Title: fmt.Sprintf("Chapter %d of the part", i), Depth: i % 3})
	}
	es[3].Title = "Salt & pepper"
	es[4].Title = "Less < more"
	es[5].Title = ""
	es[6].Title = "a > b"
	for _, rtl := range []bool{true, false} {
		v := NewContents(es, 7, rtl, fonts, w, h)
		if v.Pages() < 2 {
			t.Errorf("contents: %d pages, want at least 2", v.Pages())
		}
		for page := range 2 {
			v.SetPage(page)
			img := image.NewGray(image.Rect(0, 0, w, h))
			v.Draw(img)
			save(fmt.Sprintf("contents-rtl%v-page%d", rtl, page), img)
		}
		v.Close()
	}
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	NewClock(fonts).Draw(img, time.Date(2026, 10, 29, 20, 48, 0, 0, time.UTC), 60, w/2, false, false)
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
	p := Progress{Chapter: "A chapter", At: 3, Of: 12, Page: 40, Total: 300, Share: 0.25, Back: 7}
	v := NewContents([]Entry{{Title: "One"}, {Title: "Two"}}, 0, false, fonts, w, h)
	defer v.Close()
	for name, m := range map[string]*Panel{"menu": DrawMenu(Menu{FontSize: 50, Lines: 1.16, Margin: 60, Narrated: true}, fonts, w),
		"remove": DrawRemove(fonts, w), "nav": DrawNav(p, fonts, w, h), "contents": v.head} {
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
