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

	"shamsainkreader/library"
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
	for _, tab := range []string{"text", "page"} {
		for _, dir := range []string{"", "rtl", "ltr"} {
			for _, lh := range []float32{0, 1.35} {
				if tab == "page" && lh > 0 || tab == "text" && dir != "" {
					continue
				}
				m := DrawText(Text{Tab: tab, FontSize: 50, LineHeight: lh, Lines: 1.16, Margin: 60, Dir: dir}, fonts, w, h)
				save(fmt.Sprintf("text-%s-dir%s-lh%g", tab, dir, lh), m.img)
			}
		}
	}
	save("remove", DrawHighlightRemove(fonts, w).img)
	for _, code := range []string{"en", "fa"} { // the Light panel under the bar
		SetLanguage(code)
		save("light-"+code, DrawLight(40, 7, 10, 160, fonts, w).img)
	}
	SetLanguage("en")
	// the battery's level under the battery, in both languages: its edge under the battery's
	for _, code := range []string{"en", "fa"} {
		SetLanguage(code)
		b := DrawTopBar(TopBar{Start: "arrow-left", Title: "Title", Time: "8:48", Battery: 82, Menu: true}, fonts, w)
		batt := b.Box("topbar-battery")
		lvl := DrawBattery(82, b.Rect().Dy(), batt, fonts, w)
		img := image.NewGray(image.Rect(0, 0, w, 400))
		for i := range img.Pix {
			img.Pix[i] = 255
		}
		b.Over(img)
		lvl.Over(img)
		save("battery-"+code, img)
		if r := lvl.Rect(); r.Min.Y != b.Rect().Dy() || code == "en" && abs(r.Max.X-batt.Max.X) > 4 || code == "fa" && abs(r.Min.X-batt.Min.X) > 4 {
			t.Errorf("%s: level at %v, battery at %v", code, r, batt)
		}
	}
	SetLanguage("en")
	books := []Book{
		{Title: "Buried Alive", Author: "Arnold Bennett", Read: 62, Narrated: true},
		{Title: "رود کم‌آب", Author: "مارا الیس", Read: 100, Finished: true, RTL: true},
		{Title: "The Adventures of Sherlock Holmes", Author: "A. Conan Doyle", Read: -1},
		{Title: "Haji agha ocr", Read: 4},
		{Title: "سفر به ماه", Read: 30, RTL: true},
	}
	for i := range 12 {
		books = append(books, Book{Title: fmt.Sprintf("A new book %d", i), Author: "Someone", New: true, Read: -1})
	}
	books = append(books, Book{Title: "بوف کور", Author: "صادق هدایت", New: true, Read: -1, RTL: true}) // a date in a Farsi row
	topBar := TopBar{Time: "20:48", Battery: 82, Menu: true}
	view := LibraryView{Counts: [4]int{4, 12, 1, 17}}
	for f, name := range library.Filters {
		v := view
		v.Filter = f
		var shown []Book // the books under the filter
		now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
		for _, b := range books {
			if f == 3 || f == 1 && b.New || f == 2 && b.Finished || f == 0 && !b.New && !b.Finished {
				if f == 1 || f == 2 { // New and Finished: when each was added or finished, over a year
					b.When = AddedWhen(now.AddDate(0, 0, -[]int{0, 1, 3, 20, 400}[len(shown)%5]), now)
				}
				shown = append(shown, b)
			}
		}
		lib := NewLibrary(shown, LibraryHead(topBar, v, fonts, w), fonts, w, h)
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
		lib := NewLibrary(books, LibraryHead(topBar, v, fonts, w), fonts, w, h)
		img := image.NewGray(image.Rect(0, 0, w, h))
		lib.Draw(img)
		p, under := DrawLibraryMenu(lib.TopBarHeight(), true, fonts, w), lib.TopBarHeight()
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
	// a book's menu, details and the question before removing it, by a row held: under it, or over the last row
	for _, row := range []int{2, -1} {
		lib := NewLibrary(books, LibraryHead(topBar, view, fonts, w), fonts, w, h)
		if row < 0 { // the last row of the first page
			for i := range books {
				if lib.on[i] == 0 {
					row = i
				}
			}
		}
		for _, name := range []string{"menu", "menu-finished", "menu-new", "details", "remove"} {
			var p *Panel
			switch name {
			case "menu", "menu-finished", "menu-new":
				p = DrawBookMenu(name == "menu-finished", name != "menu-new", fonts, w)
			case "details":
				p = DrawBookDetails(BookDetails{Title: books[row].Title, Author: books[row].Author, RTL: books[row].RTL, Lang: "fa",
					Source: "farsiepub", File: "a-long-file-name_of-a-book-ocr.epub", Size: 3 << 20, Narrated: true, Opened: true, Read: 42,
					Added: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), LastRead: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)}, fonts, w)
			case "remove":
				p = DrawBookRemove(books[row].Title, books[row].RTL, fonts, w)
			}
			p.Hang(lib.EntryBox(row), lib.HeadHeight(), lib.RowsBottom())
			img := image.NewGray(image.Rect(0, 0, w, h))
			lib.Draw(img)
			p.Over(img)
			save(fmt.Sprintf("library-book-%s-row%d", name, row), img)
			if r := p.Rect(); r.Max.Y > lib.RowsBottom() || r.Min.Y < lib.HeadHeight() || r.Overlaps(lib.EntryBox(row)) {
				t.Errorf("book %s by row %d: at %v, the row at %v", name, row, r, lib.EntryBox(row))
			}
		}
		lib.Close()
	}
	// the language panel under the library's bar (its menu's entry Settings opens it): the switch at the row's far end, on
	// and off. The library is always English, and a book's menu has no Settings; the book's bar, menu, panels and
	// contents in Farsi are drawn mirrored.
	defer SetLanguage("en")
	for _, code := range []string{"en", "fa"} {
		SetLanguage(code)
		if code == "en" { // the library, and its Settings, are English only
			v := view
			v.MenuOn = true
			topBar := TopBar{Time: "20:48", Battery: 82, Menu: true}
			lib := NewLibrary(books, LibraryHead(topBar, v, fonts, w), fonts, w, h)
			for _, follow := range []bool{true, false} {
				img := image.NewGray(image.Rect(0, 0, w, h))
				lib.Draw(img)
				p := DrawLibraryLanguage(follow, lib.TopBarHeight(), fonts, w)
				p.Over(img)
				save(fmt.Sprintf("library-language-%s-follow%v", code, follow), img)
				save(fmt.Sprintf("library-language-%s-follow%v-panel", code, follow), p.img)
				r, ok := p.buttons["lang-follow"]
				if !ok || r.Empty() || !r.In(p.img.Bounds()) {
					t.Errorf("%s: switch %v (found %v)", code, r, ok)
				} else if r.Max.X <= w-60 || r.Min.X <= w/2 {
					t.Errorf("%s: switch at %v, not at the row's far end", code, r)
				}
				if p.y0 != lib.TopBarHeight() {
					t.Errorf("%s: panel at %d, want %d", code, p.y0, lib.TopBarHeight())
				}
			}
			img := image.NewGray(image.Rect(0, 0, w, h))
			lib.Draw(img)
			m := DrawLibraryMenu(lib.TopBarHeight(), true, fonts, w)
			m.Over(img)
			save("library-menu-"+code, img)
			save("library-menu-"+code+"-panel", m.img)
			for _, id := range []string{"language", "leave"} {
				if r, ok := m.buttons[id]; !ok || r.Empty() || !r.In(m.img.Bounds()) {
					t.Errorf("%s: menu entry %s: %v (found %v)", code, id, r, ok)
				}
			}
			if m.buttons["language"].Max.Y > m.buttons["leave"].Min.Y {
				t.Errorf("%s: Settings is not above Leave", code)
			}
			lib.Close()
		}
		// a book's menu, in the book's language: Leave alone, no Settings
		bm := DrawLibraryMenu(160, false, fonts, w)
		if _, ok := bm.buttons["language"]; ok {
			t.Errorf("%s: a book's menu has Settings", code)
		}
		if _, ok := bm.buttons["leave"]; !ok {
			t.Errorf("%s: a book's menu has no Leave", code)
		}
		save("lang-"+code+"-book-menu", bm.img)
		for _, n := range []int{5, 100} {
			save(fmt.Sprintf("lang-%s-topbar-library%d", code, n), DrawTopBar(TopBar{Start: "arrow-left", Title: "Title", Time: "20:48", Phones: true, Battery: n, Menu: true}, fonts, w).img)
		}
		for _, tab := range []string{"text", "page"} {
			save("lang-"+code+"-"+tab, DrawText(Text{Tab: tab, FontSize: 50, LineHeight: 0, Lines: 1.16, Margin: 60}, fonts, w, h).img)
		}
		save("lang-"+code+"-remove", DrawHighlightRemove(fonts, w).img)
		cv := NewContents([]Entry{{Title: "Chapter one"}, {Title: "باران از صبح", Depth: 1}, {Title: "Mixed: کتاب Book"}}, 1, RTL(), fonts, w, h)
		img := image.NewGray(image.Rect(0, 0, w, h))
		cv.Draw(img)
		save("lang-"+code+"-contents", img)
		cv.Close()
		// a Farsi book's contents, in either language: its entries right to left, in Parastoo
		cv = NewContents([]Entry{{Title: "دیباچه"}, {Title: "بخش یکم: مرده‌خورها"}, {Title: "باران از صبح", Depth: 1}, {Title: "کوچهٔ خالی", Depth: 1},
			{Title: "بخش دوم: سفر به ماه"}}, 2, true, fonts, w, h)
		img = image.NewGray(image.Rect(0, 0, w, h))
		cv.Draw(img)
		save("lang-"+code+"-contents-farsi-book", img)
		cv.Close()
		save("lang-"+code+"-dock", DrawDock(Progress{Chapter: "Chapter", At: 3, Of: 12, Page: 40, Total: 300, Share: 0.25, Back: 7, RTL: RTL()}, fonts, w, h).img)
	}
	SetLanguage("en")
	// no books: an empty list
	lib := NewLibrary(nil, LibraryHead(topBar, view, fonts, w), fonts, w, h)
	img := image.NewGray(image.Rect(0, 0, w, h))
	lib.Draw(img)
	save("library-empty", img)
	lib.Close()
	save("splash", Splash(t.TempDir(), fonts, w, h))
	save("splash-kobo", Splash(t.TempDir(), fonts, 1264, 1680))
	save("loading-kobo", Loading(t.TempDir(), fonts, 1264, 1680))
	ticks := []float64{0, 0.09, 0.18, 0.31, 0.45, 0.58, 0.7, 0.84}
	for _, rtl := range []bool{true, false} {
		for _, back := range []int{0, 7} {
			for _, share := range []float64{0, 0.25, 1} {
				p := Progress{Chapter: "Tom & Jerry <the first> chapter", At: 3, Of: 12, Page: 40, Total: 300, Share: share, Back: back,
					Ticks: ticks, RTL: rtl, Narrated: back == 0}
				save(fmt.Sprintf("dock-rtl%v-back%d-share%g", rtl, back, share), DrawDock(p, fonts, w, h).img)
			}
		}
	}
	long := "The Adventures of Sherlock Holmes and a much longer title to cut"
	for _, at := range []int{11, 12} { // a long chapter title cut; one page left, and the last page
		p := Progress{Chapter: long + " and longer still, a chapter's title", At: at, Of: 12, Page: 40, Share: 0.4, Ticks: ticks}
		save(fmt.Sprintf("dock-long-at%d", at), DrawDock(p, fonts, w, h).img)
	}
	book := TopBar{Start: "arrow-left", Title: long, Time: "20:48", Phones: true, LightOn: true, Battery: 82}
	save("topbar-library", DrawTopBar(TopBar{Start: "magnifying-glass", Title: "Library", Time: "20:48", Battery: 82, Menu: true}, fonts, w).img)
	save("topbar-book-ltr", DrawTopBar(book, fonts, w).img)
	rtl := book
	rtl.RTL, rtl.Title = true, "رود کم‌آب"
	save("topbar-book-rtl", DrawTopBar(rtl, fonts, w).img)
	rtl.Title = long
	save("topbar-book-rtl-long", DrawTopBar(rtl, fonts, w).img)
	for _, n := range []int{5, 100} {
		b := book
		b.Battery, b.PhonesOn = n, true
		save(fmt.Sprintf("topbar-battery%d", n), DrawTopBar(b, fonts, w).img)
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
	p := Progress{Chapter: "A chapter", At: 3, Of: 12, Page: 40, Total: 300, Share: 0.25, Back: 7, Ticks: []float64{0.3}, Narrated: true}
	v := NewContents([]Entry{{Title: "One"}, {Title: "Two"}}, 0, false, fonts, w, h)
	defer v.Close()
	topBar := DrawTopBar(TopBar{Start: "arrow-left", Title: "Title", Time: "20:48", Phones: true, Battery: 50, Menu: true}, fonts, w)
	for name, m := range map[string]*Panel{"text": DrawText(Text{Tab: "text", FontSize: 50, Lines: 1.16, Margin: 60}, fonts, w, h),
		"text-page": DrawText(Text{Tab: "page", FontSize: 50, Lines: 1.16, Margin: 60}, fonts, w, h),
		"light":     DrawLight(40, 3, 10, 160, fonts, w), "light-fa": func() *Panel { SetLanguage("fa"); defer SetLanguage("en"); return DrawLight(40, 3, 10, 160, fonts, w) }(),
		"remove": DrawHighlightRemove(fonts, w), "dock": DrawDock(p, fonts, w, h), "contents": v.head, "topbar": topBar,
		"library-head": LibraryHead(TopBar{Time: "20:48", Battery: 50, Menu: true}, LibraryView{Counts: [4]int{1, 2, 3, 6}, SortOn: true}, fonts, w),
		"library-menu": DrawLibraryMenu(160, true, fonts, w), "book-menu": DrawLibraryMenu(160, false, fonts, w),
		"library-sort": DrawLibrarySort(0, 320, fonts, w), "library-book-menu": DrawBookMenu(false, true, fonts, w),
		"library-book-remove": DrawBookRemove("A title", false, fonts, w)} {
		for id, r := range m.buttons {
			if last, ok := listRows[name]; ok && id != last { // a list's rows: grey lines between them, its border under the last
				continue
			}
			if strings.HasPrefix(id, "topbar-") || iconButtons[id] {
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

// The panels that are a list whose rows are its buttons, by their last row's id.
var listRows = map[string]string{"library-book-menu": "book-remove"}

// The buttons that are an icon alone (no border), besides the bar's.
var iconButtons = map[string]bool{"list-sort": true, "contents-close": true, "chapter-prev": true, "chapter-next": true, "contents": true, "text": true,
	"read-aloud": true}

func abs(n int) int { return max(n, -n) }
