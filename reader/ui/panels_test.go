//go:build blitz

package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	blitz "shamsainkengine"
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
	books := []Book{
		{Title: "Buried Alive", Author: "Arnold Bennett", Read: 62, Now: true, Narrated: true},
		{Title: "رود کم‌آب", Author: "مارا الیس", Read: 100, Finished: true, RTL: true},
		{Title: "The Adventures of Sherlock Holmes", Author: "A. Conan Doyle", Read: -1},
		{Title: "Haji agha ocr", Read: 4},
		{Title: "سفر به ماه", Read: 30, RTL: true},
	}
	for i := range 12 {
		books = append(books, Book{Title: fmt.Sprintf("A new book %d", i), Author: "Someone", New: true, Read: -1})
	}
	bar := Bar{Time: "20:48", Battery: 82, Menu: true}
	view := LibraryView{Counts: [4]int{4, 12, 1, 17}}
	for f, name := range FilterNames {
		v := view
		v.Filter = f
		var shown []Book // the books under the filter
		for _, b := range books {
			if f == 3 || f == 1 && b.New || f == 2 && b.Finished || f == 0 && !b.New && !b.Finished {
				shown = append(shown, b)
			}
		}
		lib := NewLibrary(shown, LibraryHead(bar, v, fonts, w), fonts, w, h)
		for page := range min(lib.Pages(), 2) {
			lib.SetPage(page)
			img := image.NewGray(image.Rect(0, 0, w, h))
			lib.Draw(img)
			save(fmt.Sprintf("library-%s-page%d", name, page), img)
		}
		lib.Close()
	}
	// the menu and the sort panel open: the button drawn on, the panel under the bar or under the filters
	for _, name := range []string{"library-menu-open", "library-sort-open"} {
		v := view
		v.MenuOn, v.SortOn = name == "library-menu-open", name != "library-menu-open"
		lib := NewLibrary(books, LibraryHead(bar, v, fonts, w), fonts, w, h)
		img := image.NewGray(image.Rect(0, 0, w, h))
		lib.Draw(img)
		p, under := DrawLibraryMenu(lib.BarHeight(), false, fonts, w), lib.BarHeight()
		if v.SortOn {
			under = lib.HeadHeight()
			p = DrawLibrarySort(1, under, fonts, w)
		}
		p.Over(img)
		save(name, img)
		save(name+"-panel", p.img)
		if p.y0 != under || under == 0 {
			t.Errorf("%s: panel at %d, want %d", name, p.y0, under)
		}
		lib.Close()
	}
	// the menu with a right-to-left bar (the library's is never one today): the list at the left, under the bar's end
	{
		v, rbar := view, bar
		v.MenuOn, rbar.RTL = true, true
		lib := NewLibrary(books, LibraryHead(rbar, v, fonts, w), fonts, w, h)
		img := image.NewGray(image.Rect(0, 0, w, h))
		lib.Draw(img)
		p := DrawLibraryMenu(lib.BarHeight(), true, fonts, w)
		p.Over(img)
		save("library-menu-open-rtl", img)
		if p.x0 >= w/2 || p.y0 != lib.BarHeight() {
			t.Errorf("rtl menu at (%d, %d), want the left, at %d", p.x0, p.y0, lib.BarHeight())
		}
		lib.Close()
	}
	// no books: an empty list
	lib := NewLibrary(nil, LibraryHead(bar, view, fonts, w), fonts, w, h)
	img := image.NewGray(image.Rect(0, 0, w, h))
	lib.Draw(img)
	save("library-empty", img)
	lib.Close()
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
	long := "The Adventures of Sherlock Holmes and a much longer title to cut"
	book := Bar{Start: "arrow-left", Title: long, Time: "20:48", Phones: true, LightOn: true, Battery: 82}
	save("bar-library", DrawBar(Bar{Start: "magnifying-glass", Title: "Library", Time: "20:48", Battery: 82, Menu: true}, fonts, w).img)
	save("bar-book-ltr", DrawBar(book, fonts, w).img)
	rtl := book
	rtl.RTL, rtl.Title = true, "رود کم‌آب"
	save("bar-book-rtl", DrawBar(rtl, fonts, w).img)
	rtl.Title = long
	save("bar-book-rtl-long", DrawBar(rtl, fonts, w).img)
	for _, n := range []int{5, 100} {
		b := book
		b.Battery, b.PhonesOn = n, true
		save(fmt.Sprintf("bar-battery%d", n), DrawBar(b, fonts, w).img)
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
	for _, rtl := range []bool{true, false} {
		v := testList(rtl, es, fonts, w, h)
		if v.Pages() < 2 {
			t.Errorf("list: %d pages, want at least 2", v.Pages())
		}
		for page := range 2 {
			v.SetPage(page)
			img := image.NewGray(image.Rect(0, 0, w, h))
			v.Draw(img)
			save(fmt.Sprintf("list-pager-rtl%v-page%d", rtl, page), img)
		}
		v.Close()
	}
	img = image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	NewClock(fonts).Draw(img, time.Date(2026, 10, 29, 20, 48, 0, 0, time.UTC), 60, w/2, false, false)
	save("clock", img.SubImage(image.Rect(0, 0, w, 200)))
}

// testList is a list of the entries es with a pager, the way a library would build one: rows from a template of its own.
func testList(rtl bool, es []Entry, fonts *blitz.Fonts, w, h int) *List {
	dir := "ltr"
	if rtl {
		dir = "rtl"
	}
	html := `<html dir="` + dir + `"><head><style>p { margin: 0; padding: 0.35em 0.3em; }</style></head><body>`
	for i, e := range es {
		html += fmt.Sprintf(`<p id="e%d">%s</p>`, i, e.Title)
	}
	html += `</body></html>`
	headHTML, headIDs := render("contents-head.html", nil)
	return NewList(drawPanel(headHTML, headIDs, fonts, w, 300), html, len(es), -1, true, rtl, fonts, w, h)
}

// The pager's buttons are found at their centres, and the rows of page 1 are still found under the header.
func TestPagerAt(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	const w, h = 1236, 1648
	var es []Entry
	for i := range 40 {
		es = append(es, Entry{Title: fmt.Sprintf("Row %d", i)})
	}
	for _, rtl := range []bool{true, false} {
		v := testList(rtl, es, fonts, w, h)
		v.SetPage(1)
		img := image.NewGray(image.Rect(0, 0, w, h))
		v.Draw(img)
		for _, id := range []string{"list-prev", "list-next"} {
			r := v.pager.buttons[id].Add(image.Pt(0, v.pager.y0))
			if got := v.PagerAt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2); got != id {
				t.Errorf("rtl %v: PagerAt(centre of %s) = %q", rtl, id, got)
			}
		}
		if p, n := v.pager.buttons["list-prev"], v.pager.buttons["list-next"]; (p.Min.X < n.Min.X) == rtl {
			t.Errorf("rtl %v: previous at %v, next at %v", rtl, p, n)
		}
		if got := v.PagerAt(w/2, v.head.img.Rect.Dy()+10); got != "" {
			t.Errorf("PagerAt on the list = %q", got)
		}
		found := 0
		for i := range es {
			if v.on[i] != 1 {
				continue
			}
			r := v.at[i]
			if got := v.EntryAt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2+v.head.img.Rect.Dy()); got != i {
				t.Errorf("rtl %v: EntryAt(row %d) = %d", rtl, i, got)
			}
			found++
		}
		if found == 0 {
			t.Errorf("rtl %v: no rows on page 1", rtl)
		}
		v.Close()
	}
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
	bar := DrawBar(Bar{Start: "arrow-left", Title: "Title", Time: "20:48", Phones: true, Battery: 50, Menu: true}, fonts, w)
	for name, m := range map[string]*Panel{"menu": DrawMenu(Menu{FontSize: 50, Lines: 1.16, Margin: 60, Narrated: true}, fonts, w),
		"remove": DrawRemove(fonts, w), "nav": DrawNav(p, fonts, w, h), "contents": v.head, "bar": bar,
		"library-head": LibraryHead(Bar{Time: "20:48", Battery: 50, Menu: true}, LibraryView{Counts: [4]int{1, 2, 3, 6}, SortOn: true}, fonts, w),
		"library-menu": DrawLibraryMenu(160, false, fonts, w), "library-menu-rtl": DrawLibraryMenu(160, true, fonts, w),
		"library-sort": DrawLibrarySort(0, 320, fonts, w)} {
		for id, r := range m.buttons {
			if strings.HasPrefix(id, "bar-") || id == "list-sort" {
				// an icon button has no border: some ink inside its box instead
				ink := false
				for y := r.Min.Y; y < r.Max.Y; y++ {
					for x := r.Min.X; x < r.Max.X; x++ {
						ink = ink || m.img.GrayAt(x, y).Y < 128
					}
				}
				if !ink {
					t.Errorf("%s: %s's icon not drawn (%v)", name, id, r)
				}
				continue
			}
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
