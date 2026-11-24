//go:build blitz

package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	blitz "shamsainkengine"

	"shamsainkreader/design"
	"shamsainkreader/library"
)

// A render of a screen of the design system (reader/design/spec/screens): what to draw, in which languages.
type renderCase struct {
	name  string
	langs []string // the interface's languages to draw it in
	draw  func(f *blitz.Fonts, w, h int) rendered
}

// The languages a screen is drawn in, by where it shows: en for the library and its panels (with no book open the
// interface is English); enFa for what also shows over a book, whose interface may be Farsi (docs/design/README.md,
// "Renders"). English is never mirrored (2026-11-22): a right-to-left book in English has its own parts right to left
// (the dock's progress row, its contents: their cases "rtl-book").
var en = []string{"en"}
var enFa = []string{"en", "fa"}

// setRenderLanguage sets the interface for a render's language.
func setRenderLanguage(l string) { SetLanguage(l) }

// designBooks are invented books (never a real book's text) for the library's renders.
func designBooks() []Book {
	books := []Book{
		{Title: "A Long Voyage", Author: "An Author", Read: 62, Narrated: true},
		{Title: "رود کم‌آب", Author: "مارا الیس", Read: 100, Finished: true, RTL: true},
		{Title: "Tales of a Small Town", Author: "Another Author", Read: -1},
		{Title: "سفر به ماه", Read: 30, RTL: true},
		// titles too long for their row: cut with "…" before the share read (fitTitles)
		{Title: "The Remarkably Long and Winding Account of a Voyage Around Every Lighthouse of the Northern Coast", Author: "A Patient Author", Read: 45, Narrated: true},
		{Title: "سرگذشت دراز و پرپیچ‌وخم سفری به گرد همه‌ی فانوس‌های دریایی کرانه‌ی شمال", Author: "نویسنده‌ای شکیبا", Read: 20, RTL: true},
	}
	for i := range 6 {
		books = append(books, Book{Title: fmt.Sprintf("A new book %d", i), Author: "Someone", New: true, Read: -1})
	}
	return books
}

// rendered is what a render drew: the picture, and the ids of the buttons on it.
type rendered struct {
	img       *image.Gray
	rgba      *image.RGBA                // the colour picture, for a screen that has one (the img is its grey)
	boxes     map[string]image.Rectangle // where each button is
	placement string                     // "screen": the boxes are on the screen; "relative": on the panel, which is hung by something (a row, words)
}

// ids are the buttons the render drew.
func (r rendered) ids() []string {
	var ids []string
	for id := range r.boxes {
		ids = append(ids, id)
	}
	return ids
}

// pr is a panel as the reader places it on the screen; rel one that is hung by a row or by words, its boxes on itself.
func pr(p *Panel) rendered { return rendered{img: p.img, boxes: p.Boxes(), placement: "screen"} }

func rel(p *Panel) rendered {
	r := rendered{img: p.img, boxes: map[string]image.Rectangle{}, placement: "relative"}
	for id, b := range p.Boxes() {
		r.boxes[id] = b.Sub(p.Rect().Min)
	}
	return r
}

// libraryTopBar is the bar of the library's renders (the headphones on a device with sound).
func libraryTopBar() TopBar {
	return TopBar{Time: "20:48", Phones: renderHas("sound"), Battery: 82, Menu: true}
}

// designTiles is designBooks with the years their books would give (one without).
func designTiles() []Book {
	bs := designBooks()
	years := []string{"1851", "۱۳۹۸", "1923", "", "1888", "۱۴۰۱"}
	for i := range bs {
		if i < len(years) {
			bs[i].Year = years[i]
		} else {
			bs[i].Year = "2026"
		}
	}
	return bs
}

// tiles is the library's tiles under head, its page page.
func tiles(bs []Book, head *Panel, page int, f *blitz.Fonts, w, h int) rendered {
	l := NewLibraryTiles(bs, "", head, f, w, h)
	defer l.Close()
	l.SetPage(page)
	return rendered{img: screenOf(w, h, l.Draw), boxes: l.Boxes(), placement: "screen"}
}

// markBooks is one shelf with every state a tile marks (new, reading, finished; narrated; both directions), for the
// tiles' marks, in the All filter.
func markBooks() []Book {
	return []Book{
		{Title: "A Long Voyage", Author: "An Author", Year: "1851", Read: 62, Narrated: true},
		{Title: "رود کم‌آب", Author: "مارا الیس", Year: "۱۳۹۸", Read: 100, Finished: true, RTL: true},
		{Title: "Tales of a Small Town", Author: "Another Author", Year: "1923", New: true, Read: -1},
		{Title: "سفر به ماه", Year: "۱۴۰۱", Read: 8, RTL: true},
		{Title: "زنده به گور", Author: "صادق هدایت", New: true, Narrated: true, Read: -1, RTL: true},
		{Title: "The Lighthouse Keeper", Author: "A Patient Author", Year: "1888", Read: 91},
	}
}

// coverArt is a cover picture of our own, drawn from html (a book's cover is never committed: CLAUDE.md), 600 by 900.
func coverArt(f *blitz.Fonts, html string) *image.Gray {
	const w, h = 600, 900
	page := `<html><head><style>body { margin: 0; } .c { width: 600px; height: 900px; display: flex; flex-direction: column;
align-items: center; justify-content: center; text-align: center; font-family: "DejaVu Serif", serif; position: relative; }
.end { position: absolute; bottom: 0; left: 0; font-size: 20px; line-height: 1; }</style></head><body>` +
		strings.Replace(html, "</div></div>", `</div><span class="end">.</span></div>`, 1) + `</body></html>`
	b := blitz.New([]string{"cover.html"}, readFor(page), blitz.Options{Width: w, Height: h, FontSize: 44}, f)
	defer b.Close()
	b.Layout(0)
	img := image.NewGray(image.Rect(0, 0, w, h))
	b.Draw(0, img)
	return img
}

// coverBooks is markBooks with three of them given picture covers of our own, the rest their words.
func coverBooks(f *blitz.Fonts) []Book {
	bs := markBooks()
	bs[0].Cover = coverArt(f, `<div class="c" style="background: #1e1e1e; color: white;">
<div style="width: 300px; height: 300px; border: 10px solid white; border-radius: 50%; margin-bottom: 70px;"></div>
<div style="font-size: 76px; letter-spacing: 6px;">A LONG<br>VOYAGE</div><div style="font-size: 36px; margin-top: 40px;">AN AUTHOR</div></div>`)
	bs[3].Cover = coverArt(f, `<div class="c" style="background: #5a5a5a; color: white;">
<div style="width: 220px; height: 220px; background: #e8e8e8; border-radius: 50%; margin-bottom: 90px;"></div>
<div style="font-size: 96px; font-family: Vazirmatn, sans-serif; font-weight: bold;" lang="fa" dir="rtl">سفر به ماه</div></div>`)
	bs[5].Cover = coverArt(f, `<div class="c" style="background: #d8d8d8; color: #111; justify-content: flex-start;">
<div style="height: 330px; width: 100%; background: #9a9a9a;"></div><div style="height: 40px; width: 100%; background: #4a4a4a;"></div>
<div style="font-size: 70px; margin-top: 110px; font-style: italic;">The Lighthouse<br>Keeper</div><div style="font-size: 34px; margin-top: 50px;">A PATIENT AUTHOR</div></div>`)
	return bs
}

// darkened is img as the screen shows it in dark mode: every grey inverted (a List's pictures were painted inverted
// for it, List.SetDark).
func darkened(img *image.Gray) *image.Gray {
	for i, p := range img.Pix {
		img.Pix[i] = 255 - p
	}
	return img
}

func coverTiles(dark bool) func(f *blitz.Fonts, w, h int) rendered {
	return func(f *blitz.Fonts, w, h int) rendered {
		head := LibraryHead(libraryTopBar(), LibraryView{Counts: [4]int{4, 2, 1, 6}, Filter: 3}, f, w)
		l := NewLibraryTiles(coverBooks(f), "", head, f, w, h)
		defer l.Close()
		l.SetDark(func() bool { return dark })
		img := screenOf(w, h, l.Draw)
		if dark {
			img = darkened(img)
		}
		return rendered{img: img, boxes: l.Boxes(), placement: "screen"}
	}
}

func libraryHead(f *blitz.Fonts, w int) *Panel {
	return LibraryHead(libraryTopBar(), LibraryView{Counts: [4]int{4, 6, 1, 10}}, f, w)
}

// catalogueHead is the catalogues' head over FarsiEPUB's books (chooseOn: its choice open; loading: Loading… beside it).
func catalogueHead(f *blitz.Fonts, w int, chooseOn bool, loading ...bool) *Panel {
	return CatalogueHead(libraryTopBar(), CatalogueHeadView{Name: "FarsiEPUB", ChooseOn: chooseOn, Loading: len(loading) > 0 && loading[0]}, f, w)
}

// barHeight is the library's bar: where the panels that hang from it start.
func barHeight(f *blitz.Fonts, w int) int { return DrawTopBar(libraryTopBar(), f, w).Rect().Dy() }

// batteryBox is the battery's place on that bar.
func batteryBox(f *blitz.Fonts, w int) image.Rectangle {
	return DrawTopBar(libraryTopBar(), f, w).Box("topbar-battery")
}

// opened is an opening screen as a render: no buttons.
func opened(o *Opening) rendered {
	return rendered{img: o.Gray, rgba: o.RGBA, boxes: map[string]image.Rectangle{}, placement: "screen"}
}

// tmp is a folder to keep a drawing in that is not kept.
func tmp() string { d, _ := os.MkdirTemp("", "design"); return d }

func screenOf(w, h int, draw func(img *image.Gray)) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	draw(img)
	return img
}

// chapter is a chapter's title in the interface's language: a Farsi interface is for a Farsi book, whose titles are Farsi.
func chapter() string {
	if Language() == "fa" {
		return "فصل یکم"
	}
	return "Chapter one"
}

// designRenders has, for each screen of the spec that is drawn, the states to draw it in. The page (reading) is the
// engine's and is not here.
// renderDevice is the profile being drawn for: a render of something only some devices have (the page buttons'
// setting) follows it, as the reader does on the device.
var renderDevice design.Device

func renderHas(what string) bool { return slices.Contains(renderDevice.Has, what) }

var designRenders = map[string][]renderCase{
	"splash": {{"grey", en, func(f *blitz.Fonts, w, h int) rendered { return opened(Splash(tmp(), f, w, h, false)) }},
		{"colour", en, func(f *blitz.Fonts, w, h int) rendered { return opened(Splash(tmp(), f, w, h, true)) }}},
	"sleep": {{"couplet", en, func(f *blitz.Fonts, w, h int) rendered { // the screen's own text sets its direction, not the interface
		return opened(&Opening{Gray: DrawSleep(Sleep{Layout: "couplet", Verses: []string{"بنی آدم اعضایِ یکدیگرند", "که در آفرینش ز یک گوهرند"},
			Poet: "سعدی", Gloss: "آدمیان مانند اعضای یک پیکرند، چون همه از یک گوهر آفریده شده‌اند."}, f, w, h)})
	}}, {"title", en, func(f *blitz.Fonts, w, h int) rendered {
		return opened(&Opening{Gray: DrawSleep(Sleep{Layout: "title", Text: "Buried Alive", Credit: "Arnold Bennett"}, f, w, h)})
	}}},
	"loading": {{"grey", en, func(f *blitz.Fonts, w, h int) rendered { return opened(Loading(tmp(), f, w, h, false)) }},
		{"colour", en, func(f *blitz.Fonts, w, h int) rendered { return opened(Loading(tmp(), f, w, h, true)) }}},
	"leaving": {{"grey", en, func(f *blitz.Fonts, w, h int) rendered { return opened(Leaving(tmp(), f, w, h, false)) }},
		{"colour", en, func(f *blitz.Fonts, w, h int) rendered { return opened(Leaving(tmp(), f, w, h, true)) }}},
	"topbar": {{"battery-full", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), Battery: 100, Menu: true}, f, w))
	}}, {"book", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), Battery: 82, Menu: true}, f, w))
	}}, {"battery-medium", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), Battery: 50, Menu: true}, f, w))
	}}, {"battery-low", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), Battery: 25, Menu: true}, f, w))
	}}, {"battery-warning", enFa, func(f *blitz.Fonts, w, h int) rendered { // 15% or less: the battery with its !
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), Battery: 12, Menu: true}, f, w))
	}}, {"battery-charging", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), Battery: 45, Charging: true, Menu: true}, f, w))
	}}, {"battery-open", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), Battery: 82, BatteryOn: true, Menu: true}, f, w))
	}}, {"sound-open", en, func(f *blitz.Fonts, w, h int) rendered { // the headphones filled while the Sound panel is open
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: renderHas("sound"), PhonesOn: true, Battery: 82, Menu: true}, f, w))
	}}},
	"dock": {{"narrated", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawDock(Progress{Chapter: chapter(), At: 3, Of: 12, Page: 40, Total: 300, Share: 0.25, Back: 7,
			Ticks: []float64{0.2, 0.5}, Narrated: true, RTL: RTL()}, f, w, h))
	}}, {"plain", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawDock(Progress{Chapter: chapter(), At: 3, Of: 12, Page: 40, Share: 0.6, RTL: RTL()}, f, w, h))
	}}, {"rtl-book", en, func(f *blitz.Fonts, w, h int) rendered { // a Hebrew book, the interface English: its progress row right to left
		return pr(DrawDock(Progress{Chapter: "פרק ראשון", At: 3, Of: 12, Page: 40, Total: 300, Share: 0.25, Ticks: []float64{0.2, 0.5},
			Narrated: true, RTL: true}, f, w, h))
	}}, {"jumped", enFa, func(f *blitz.Fonts, w, h int) rendered { // a book with no narration after a jump: Back, where to tap it
		return pr(DrawDock(Progress{Chapter: chapter(), At: 3, Of: 12, Page: 40, Share: 0.6, Back: 7, RTL: RTL()}, f, w, h))
	}}},
	"light": {{"panel", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawLight(40, 7, 10, false, barHeight(f, w), f, w))
	}},
		{"dark", en, func(f *blitz.Fonts, w, h int) rendered {
			return pr(DrawLight(40, 7, 10, true, barHeight(f, w), f, w))
		}}},
	"sound": {{"panel", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawSound(Sound{Connected: true, Volume: 70}, barHeight(f, w), f, w))
	}}, {"connect", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawSound(Sound{Paired: true}, barHeight(f, w), f, w))
	}}, {"connecting", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawSound(Sound{Paired: true, State: "connecting"}, barHeight(f, w), f, w))
	}}, {"failed", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawSound(Sound{Paired: true, State: "failed"}, barHeight(f, w), f, w))
	}}, {"unpaired", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawSound(Sound{}, barHeight(f, w), f, w))
	}}},
	"reading-settings": {{"settings", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawReadingSettings(ReadingSettings{CleanLight: 0, CleanDark: 5}, barHeight(f, w), f, w))
	}}},
	"text": {{"text", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawText(Text{Tab: "text", FontSize: 50, Lines: 1.16, Margin: 60}, f, w, h))
	}}, {"page", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawText(Text{Tab: "page", FontSize: 50, Lines: 1.16, Margin: 60}, f, w, h))
	}}},
	"contents": {{"list", enFa, func(f *blitz.Fonts, w, h int) rendered {
		entries := []Entry{{Title: "Chapter one"}, {Title: "Chapter two", Depth: 1}, {Title: "Chapter three"}, {Title: "Chapter four"}}
		if RTL() { // a Farsi book's
			entries = []Entry{{Title: "دیباچه"}, {Title: "بخش یکم: مرده‌خورها"}, {Title: "باران از صبح", Depth: 1}, {Title: "کوچهٔ خالی", Depth: 1}}
		}
		cv := NewContents(entries, 1, RTL(), f, w, h)
		defer cv.Close()
		return rendered{img: screenOf(w, h, cv.Draw), boxes: cv.Boxes(), placement: "screen"}
	}}, {"rtl-book", en, func(f *blitz.Fonts, w, h int) rendered { // a Hebrew book, the interface English: its list right to left
		cv := NewContents([]Entry{{Title: "פרק ראשון"}, {Title: "פרק שני", Depth: 1}, {Title: "פרק שלישי"}, {Title: "פרק רביעי"}}, 1, true, f, w, h)
		defer cv.Close()
		return rendered{img: screenOf(w, h, cv.Draw), boxes: cv.Boxes(), placement: "screen"}
	}}, {"mixed", en, func(f *blitz.Fonts, w, h int) rendered { // English and Farsi entries in one list: each in its own direction
		cv := NewContents([]Entry{{Title: "Chapter one"}, {Title: "۱. باران از صبح", Depth: 1}, {Title: "Mixed: کتاب Book"}, {Title: "Chapter four"}}, 1, false, f, w, h)
		defer cv.Close()
		return rendered{img: screenOf(w, h, cv.Draw), boxes: cv.Boxes(), placement: "screen"}
	}}},
	"battery": {{"level", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawBattery(82, "", barHeight(f, w), batteryBox(f, w), f, w))
	}}, {"charging", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawBattery(45, BatteryState(45, true, false), barHeight(f, w), batteryBox(f, w), f, w))
	}}, {"full", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawBattery(100, BatteryState(100, false, true), barHeight(f, w), batteryBox(f, w), f, w))
	}}, {"low", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawBattery(12, BatteryState(12, false, false), barHeight(f, w), batteryBox(f, w), f, w))
	}}},
	"menu": {{"book", enFa, func(f *blitz.Fonts, w, h int) rendered { return pr(DrawMenu(barHeight(f, w), "reading", f, w)) }},
		{"library", en, func(f *blitz.Fonts, w, h int) rendered { return pr(DrawMenu(barHeight(f, w), "device", f, w)) }}},
	"library": {{"reading", en, func(f *blitz.Fonts, w, h int) rendered { // a mixed shelf: progress, finished, narrated, no author, no year, long titles
		return tiles(designTiles(), libraryHead(f, w), 0, f, w, h)
	}}, {"empty", en, func(f *blitz.Fonts, w, h int) rendered { // no books at all
		lib := NewLibrary(nil, ".adds/shamsaink/books", libraryHead(f, w), f, w, h)
		defer lib.Close()
		return rendered{img: screenOf(w, h, lib.Draw), boxes: lib.Boxes(), placement: "screen"}
	}}, {"guide", en, func(f *blitz.Fonts, w, h int) rendered {
		// the getting-started guide's picture (reader/welcome/en.md): two real books (titles and authors only), all
		// chosen so the tiles and the counts agree, cut under their row (below it the page is empty)
		books := []Book{
			{Title: "Moby-Dick", Author: "Herman Melville", Read: 35},
			{Title: "زنده به گور", Author: "صادق هدایت", Read: 62, Narrated: true, RTL: true},
		}
		head := LibraryHead(libraryTopBar(), LibraryView{Counts: [4]int{2, 0, 0, 2}, Filter: 3}, f, w)
		lib := NewLibraryTiles(books, "shamsaink/books", head, f, w, h)
		defer lib.Close()
		img := screenOf(w, h, lib.Draw).SubImage(image.Rect(0, 0, w, lib.EntryBox(0).Max.Y+24)).(*image.Gray)
		return rendered{img: img, boxes: lib.Boxes(), placement: "screen"}
	}}, {"page-2", en, func(f *blitz.Fonts, w, h int) rendered { // its second page: new books
		return tiles(designTiles(), libraryHead(f, w), 1, f, w, h)
	}}, {"new", en, func(f *blitz.Fonts, w, h int) rendered { // the New filter: when each was added, under it
		var bs []Book
		for _, b := range designTiles() {
			b.When, b.New = "12 Nov", false
			bs = append(bs, b)
		}
		return tiles(bs, LibraryHead(libraryTopBar(), LibraryView{Counts: [4]int{4, 6, 1, 10}, Filter: 1}, f, w), 0, f, w, h)
	}}, {"marks", en, func(f *blitz.Fonts, w, h int) rendered { // every mark: new, reading (8 to 91%), finished, narrated
		head := LibraryHead(libraryTopBar(), LibraryView{Counts: [4]int{4, 2, 1, 6}, Filter: 3}, f, w)
		return tiles(markBooks(), head, 0, f, w, h)
	}},
		{"covers", en, coverTiles(false)},     // planned: picture covers beside covers of words, the marks on a disc of paper
		{"covers-dark", en, coverTiles(true)}, // planned: the same in dark mode, the pictures left as they are
		{"empty-filter", en, func(f *blitz.Fonts, w, h int) rendered { // none in this filter, some in others
			lib := NewLibrary(nil, "", libraryHead(f, w), f, w, h)
			defer lib.Close()
			return rendered{img: screenOf(w, h, lib.Draw), boxes: lib.Boxes(), placement: "screen"}
		}}},
	"library-settings": {{"follow", en, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawLibrarySettings(LibrarySettings{Follow: true, SleepMin: 10, HasButtons: renderHas("buttons"), Version: "v0.0.1"}, barHeight(f, w), f, w))
	}},
		{"english", en, func(f *blitz.Fonts, w, h int) rendered {
			return pr(DrawLibrarySettings(LibrarySettings{SleepMin: 60, HasButtons: renderHas("buttons"), ReverseButtons: true, Version: "v0.0.1"}, barHeight(f, w), f, w))
		}}},
	"library-developer": {{"on", en, func(f *blitz.Fonts, w, h int) rendered { // an address from the documentation range
		return pr(DrawDeveloper(Developer{WiFiOn: true, WiFi: "on: 192.0.2.10", SSH: "SSH on: 192.0.2.10, port 2222"}, barHeight(f, w), f, w))
	}},
		{"off", en, func(f *blitz.Fonts, w, h int) rendered {
			return pr(DrawDeveloper(Developer{WiFi: "off", SSH: "SSH off"}, barHeight(f, w), f, w))
		}}},
	"library-sort": {{"title", en, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawLibrarySort(slicesIndex(library.Sorts, "title"), libraryHead(f, w).img.Rect.Dy(), f, w))
	}}},
	"catalogue": {{"books", en, func(f *blitz.Fonts, w, h int) rendered {
		// invented books, as the library's: one to download, one coming, one here, and a title too long for its row
		c := NewCatalogue(CatalogueView{Name: "FarsiEPUB", More: true, Books: []CatalogueBook{
			{Title: "رود کم‌آب", Author: "مارا الیس", RTL: true, Share: -1},
			{Title: "سفر به ماه", Author: "نویسنده‌ای شکیبا", RTL: true, Share: 42},
			{Title: "Tales of a Small Town", Author: "Another Author", Here: true, Share: -1},
			{Title: "The Remarkably Long and Winding Account of a Voyage Around Every Lighthouse of the Northern Coast", Author: "A Patient Author", Share: -1},
		}}, catalogueHead(f, w, false), f, w, h)
		defer c.Close()
		return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
	}}, {"pages", en, func(f *blitz.Fonts, w, h int) rendered { // more books than a page: the pager
		var books []CatalogueBook
		for i := range 12 {
			books = append(books, CatalogueBook{Title: fmt.Sprintf("A catalogue's book %d", i+1), Author: "Someone", Share: -1})
		}
		c := NewCatalogue(CatalogueView{Name: "FarsiEPUB", Books: books}, catalogueHead(f, w, false), f, w, h)
		defer c.Close()
		return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
	}}, {"loading", en, func(f *blitz.Fonts, w, h int) rendered {
		c := NewCatalogue(CatalogueView{Name: "FarsiEPUB", Loading: true}, catalogueHead(f, w, false, true), f, w, h)
		defer c.Close()
		return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
	}}, {"landing-heading", en, func(f *blitz.Fonts, w, h int) rendered { // a front page of sections only, entered on its newest's books, the section's name under the head
		return landingCatalogue(CatalogueHeadView{Name: "Ebooksgratuits", Browse: true, Section: "Dernières parutions"}, false, f, w, h)
	}}, {"landing-choose", en, func(f *blitz.Fonts, w, h int) rendered { // no section with books found on arriving: Browse
		return landingCatalogue(CatalogueHeadView{Name: "A Library", Browse: true}, true, f, w, h)
	}}, {"long-names", en, func(f *blitz.Fonts, w, h int) rendered { // a name cut: the head's line, its Loading… and funnel kept
		c := NewCatalogue(CatalogueView{Name: "The Very Large Public Library of Classics", Books: []CatalogueBook{{Title: "Collected Works, Volume One", Author: "A Patient Author", Share: -1}}},
			CatalogueHead(libraryTopBar(), CatalogueHeadView{Name: "The Very Large Public Library of Classics",
				Browse: true, Funnel: true, Chosen: true, InForce: 1, Loading: true, Section: "Collected Works of the Northern Coast and Its Lighthouses"}, f, w), f, w, h)
		defer c.Close()
		return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
	}}, {"unreachable", en, func(f *blitz.Fonts, w, h int) rendered {
		c := NewCatalogue(CatalogueView{Name: "FarsiEPUB", Unreachable: true}, catalogueHead(f, w, false), f, w, h)
		defer c.Close()
		return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
	}}, {"unreachable-rtl", en, func(f *blitz.Fonts, w, h int) rendered { // a right-to-left catalogue's name, isolated in the sentence
		c := NewCatalogue(CatalogueView{Name: "کتابخانه‌ی ۲", Unreachable: true}, CatalogueHead(libraryTopBar(), CatalogueHeadView{Name: "کتابخانه‌ی ۲"}, f, w), f, w, h)
		defer c.Close()
		return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
	}}, {"funnel", en, func(f *blitz.Fonts, w, h int) rendered { // sections and facets: the list icon and the funnel, nothing in force
		return funnelCatalogue(CatalogueHeadView{Name: "Example Catalogue", Browse: true, Funnel: true}, false, f, w, h)
	}}, {"section-chosen", en, func(f *blitz.Fonts, w, h int) rendered { // a section chosen, no facets (Ebooksgratuits): the list icon filled, no funnel
		return funnelCatalogue(CatalogueHeadView{Name: "Ebooksgratuits", Browse: true, Chosen: true}, false, f, w, h)
	}}, {"filtered", en, func(f *blitz.Fonts, w, h int) rendered { // a section and two facets in force: the list icon filled, the funnel's 2
		return funnelCatalogue(CatalogueHeadView{Name: "Example Catalogue", Browse: true, Funnel: true, Chosen: true, InForce: 2}, false, f, w, h)
	}}, {"no-match", en, func(f *blitz.Fonts, w, h int) rendered { // no books with them
		return funnelCatalogue(CatalogueHeadView{Name: "Example Catalogue", Browse: true, Funnel: true, Chosen: true, InForce: 2}, true, f, w, h)
	}}, {"no-funnel", en, func(f *blitz.Fonts, w, h int) rendered { // nothing to choose (one page, no sections): neither icon
		return funnelCatalogue(CatalogueHeadView{Name: "FarsiEPUB"}, false, f, w, h)
	}}},
	"catalogue-filters": {{"browse", en, func(f *blitz.Fonts, w, h int) rendered { // Browse's page (the list icon): the places, a row each, the one shown ticked
		return filtersScreen(FiltersView{Browse: &designBrowse, Clear: true}, 0, f, w, h)
	}}, {"filter", en, func(f *blitz.Fonts, w, h int) rendered { // Filter's page (the funnel): the facets as chips, those in force under the title, removable
		return filtersScreen(FiltersView{Groups: designFacets(), Books: 214, Clear: true, Picked: []Picked{{Label: "Newest"}, {Label: "French"}}}, 0, f, w, h)
	}}, {"loading", en, func(f *blitz.Fonts, w, h int) rendered { // a choice tapped: the page stays as it was until the new feed's choices come
		return filtersScreen(FiltersView{Groups: designFacets(), Loading: true, Clear: true, Picked: []Picked{{Label: "Newest"}, {Label: "French"}}}, 0, f, w, h)
	}}, {"many", en, func(f *blitz.Fonts, w, h int) rendered { // more groups than a screen: paged, its second page
		return filtersScreen(FiltersView{Groups: append(designFacets(), designMore()...), Books: 37, Clear: true, Picked: []Picked{{Label: "Newest"}, {Label: "French"}}}, 1, f, w, h)
	}}, {"more-language", en, func(f *blitz.Fonts, w, h int) rendered { // More… under Language: the group alone, a row each
		return filtersScreen(FiltersView{Open: &FilterGroup{Name: "Language", Choices: designLanguages()}, Books: 214}, 0, f, w, h)
	}}, {"more-authors", en, func(f *blitz.Fonts, w, h int) rendered { // By author: the section's own, paged
		return filtersScreen(FiltersView{Open: &FilterGroup{Name: "By author", Choices: designAuthors()}}, 0, f, w, h)
	}}, {"author-chosen", en, func(f *blitz.Fonts, w, h int) rendered { // Browse, an author picked under By author: By author ticked, the author's name a chip under the title
		b := FilterGroup{Choices: []FilterChoice{{Label: "All books"}, {Label: "Poetry"}, {Label: "Drama"},
			{Label: "New releases"}, {Label: "By author", On: true}, {Label: "By subject"}}}
		return filtersScreen(FiltersView{Browse: &b, Books: 12, Clear: true, Picked: []Picked{{Label: "Austen, Jane", Fixed: true}}}, 0, f, w, h)
	}}, {"test-front", en, func(f *blitz.Fonts, w, h int) rendered { // the test catalogue's (devtest catalogue-serve), for e2e/scenarios/catalogue.txt's taps
		return filtersScreen(FiltersView{Browse: &FilterGroup{Choices: []FilterChoice{{Label: "Test books"}, {Label: "By language"}}}}, 0, f, w, h)
	}}, {"test-books", en, func(f *blitz.Fonts, w, h int) rendered { // its Test books' Filter: Language and Narrated
		return filtersScreen(FiltersView{Books: 2,
			Groups: []FilterGroup{{Name: "Language", Choices: []FilterChoice{{Label: "All", On: true}, {Label: "English", Count: 1}, {Label: "Farsi", Count: 1}}},
				{Name: "Narrated", Choices: []FilterChoice{{Label: "Any", On: true}, {Label: "Narrated"}}}}}, 0, f, w, h)
	}}, {"test-books-fa", en, func(f *blitz.Fonts, w, h int) rendered { // its Test books' Filter with Farsi chosen: Farsi under the title
		return filtersScreen(FiltersView{Books: 1, Clear: true, Picked: []Picked{{Label: "Farsi"}},
			Groups: []FilterGroup{{Name: "Language", Choices: []FilterChoice{{Label: "All"}, {Label: "English", Count: 1}, {Label: "Farsi", Count: 1, On: true}}},
				{Name: "Narrated", Choices: []FilterChoice{{Label: "Any", On: true}, {Label: "Narrated"}}}}}, 0, f, w, h)
	}}, {"test-language", en, func(f *blitz.Fonts, w, h int) rendered { // its By language: a section of sections
		return filtersScreen(FiltersView{Clear: true, Open: &FilterGroup{Name: "By language", Choices: []FilterChoice{{Label: "English", Count: 1}, {Label: "Farsi", Count: 1}}}}, 0, f, w, h)
	}}},
	"shelf-name": {{"library", en, func(f *blitz.Fonts, w, h int) rendered {
		tb := libraryTopBar()
		tb.Shelf = "library"
		return pr(DrawShelfName("library", barHeight(f, w), DrawTopBar(tb, f, w).Box("shelf-library"), f, w))
	}}, {"catalogues", en, func(f *blitz.Fonts, w, h int) rendered {
		tb := libraryTopBar()
		tb.Shelf = "catalogues"
		return pr(DrawShelfName("catalogues", barHeight(f, w), DrawTopBar(tb, f, w).Box("shelf-catalogues"), f, w))
	}}},
	"catalogue-menu": {{"two", en, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawCatalogueMenu([]string{"FarsiEPUB", "Project Gutenberg"}, 0, catalogueHead(f, w, true).Rect().Dy(), f, w))
	}}},
	"library-book-details": {{"new", en, func(f *blitz.Fonts, w, h int) rendered {
		return rel(DrawBookDetails(BookDetails{Title: "Tales of a Small Town", Author: "Another Author", Lang: "en", File: "tales.epub",
			Size: 1 << 20, Read: -1, Added: designDay(2)}, f, w))
	}}, {"opened", en, func(f *blitz.Fonts, w, h int) rendered {
		return rel(DrawBookDetails(BookDetails{Title: "A Long Voyage", Author: "An Author", Lang: "en", Source: "a folder", File: "a-long-voyage.epub",
			Size: 3 << 20, Narrated: true, Opened: true, Read: 42, Added: designDay(2), LastRead: designDay(30)}, f, w))
	}}, {"picture", en, func(f *blitz.Fonts, w, h int) rendered { // a book with a cover picture: Cover, Picture or Words (no test book has one: not walked)
		return rel(DrawBookDetails(BookDetails{Title: "A Long Voyage", Author: "An Author", Lang: "en", Source: "a folder", File: "a-long-voyage.epub",
			Size: 3 << 20, Narrated: true, Opened: true, Read: 42, Added: designDay(2), LastRead: designDay(30), Picture: true}, f, w))
	}}, {"finished", en, func(f *blitz.Fonts, w, h int) rendered {
		return rel(DrawBookDetails(BookDetails{Title: "رود کم‌آب", Author: "مارا الیس", RTL: true, Lang: "fa", File: "rud.epub",
			Size: 2 << 20, Opened: true, Finished: true, Read: 100, Added: designDay(2), LastRead: designDay(30)}, f, w))
	}}},
	"library-book-remove": {{"ask", en, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawBookRemove("A Long Voyage", false, 1, f, w)) }}},
	"notice": {{"listen", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawNotice("listen-not-here", "", f, w)) }},
		{"wifi-no-network", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawNotice("wifi-no-network", "wifi-slash", f, w)) }},
		{"wifi-no-address", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawNotice("wifi-no-address", "wifi-slash", f, w)) }},
		{"wifi-not-here", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawNotice("wifi-not-here", "", f, w)) }},
		{"update-none", enFa, func(f *blitz.Fonts, w, h int) rendered {
			return rel(DrawNotice("update-none", "", f, w, "v0.3.6-beta"))
		}},
		{"update-not-here", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawNotice("update-not-here", "", f, w)) }},
		{"update-unreachable", enFa, func(f *blitz.Fonts, w, h int) rendered {
			return rel(DrawNotice("update-unreachable", "warning-circle", f, w))
		}},
		{"update-cut", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawNotice("update-cut", "warning-circle", f, w)) }},
		{"update-unverified", enFa, func(f *blitz.Fonts, w, h int) rendered {
			return rel(DrawNotice("update-unverified", "warning-circle", f, w))
		}},
		{"update-undone", enFa, func(f *blitz.Fonts, w, h int) rendered {
			return rel(DrawNotice("update-undone", "arrow-counter-clockwise", f, w))
		}}},
	"wifi-ask": {{"check", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawWiFiAsk("wifi-off-check", f, w)) }},
		{"update", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawWiFiAsk("wifi-off-update", f, w)) }}},
	"update-available": {{"beta", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return rel(DrawUpdateAvailable("v0.4.0-beta", f, w))
	}}},
	"update-progress": {{"downloading", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawUpdateProgress("v0.4.0-beta", 40, f, w)) }},
		{"restarting", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawUpdateProgress("v0.4.0-beta", -1, f, w)) }}},
	"highlight-menu": {{"new", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawHighlightMenu(false, true, f, w)) }},
		{"made", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawHighlightMenu(true, false, f, w)) }}},
}

func slicesIndex(a []string, s string) int {
	for i, x := range a {
		if x == s {
			return i
		}
	}
	return -1
}

// Every screen of the spec that is built has renders (and the page, which the engine draws, has none), and every render
// is of a screen of the spec.
func TestEveryScreenIsRendered(t *testing.T) {
	s, err := design.Load()
	if err != nil {
		t.Fatal(err)
	}
	for n, sc := range s.Screens {
		_, has := designRenders[n]
		switch {
		case sc.Page && has:
			t.Errorf("screen %s is the engine's page: nothing to render", n)
		case sc.Status == "built" && !sc.Page && !has:
			t.Errorf("screen %s is built and has no render in designRenders", n)
		}
	}
	for n := range designRenders {
		if s.Screens[n] == nil {
			t.Errorf("a render of %s, which is no screen of the spec", n)
		}
	}
}

// Every render, per device profile and language, drawn and compared with the one kept in reader/design/golden/. They are
// compared only with FONTS set (the fonts of reader/fonts: other fonts draw other letters); GOLDEN=update keeps
// the drawings as the new ones; DESIGN_OUT: a folder for the pictures that differ.
func TestScreenRenders(t *testing.T) {
	dir := os.Getenv("FONTS")
	if dir == "" {
		t.Skip("FONTS not set: renders are compared with the fonts of reader/fonts")
	}
	fonts := blitz.LoadFonts(dir)
	s, err := design.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer SetLanguage("en")
	update := os.Getenv("GOLDEN") == "update"
	var names []string
	for n := range designRenders {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, d := range s.Devices {
		renderDevice = d
		for _, screen := range names {
			for _, c := range designRenders[screen] {
				for _, lang := range c.langs {
					setRenderLanguage(lang)
					name := fmt.Sprintf("%s-%s-%s", screen, c.name, lang)
					r := c.draw(fonts, d.Screen.Width, d.Screen.Height)
					path := filepath.Join("..", "design", "golden", d.Name, name)
					if r.rgba != nil {
						compareGoldenRGBA(t, path+".png", name, r.rgba, update)
					} else {
						compareGolden(t, path+".png", name, r.img, update)
					}
					compareBoxes(t, path+".json", name, r, update)
				}
			}
		}
	}
}

func compareGolden(t *testing.T, path, name string, img *image.Gray, update bool) {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	if update {
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("%s: no kept render (GOLDEN=update keeps it): %v", name, err)
		return
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	g, ok := want.(*image.Gray)
	if !ok || g.Rect != img.Rect {
		t.Errorf("%s: drawn %v, kept %v", name, img.Rect, want.Bounds())
		return
	}
	diff := 0
	for y := range img.Rect.Dy() {
		a, b := img.Pix[y*img.Stride:y*img.Stride+img.Rect.Dx()], g.Pix[y*g.Stride:y*g.Stride+g.Rect.Dx()]
		for x := range a {
			if a[x] != b[x] {
				diff++
			}
		}
	}
	if diff > 0 {
		t.Errorf("%s: %d pixels differ from the kept render", name, diff)
		if out := os.Getenv("DESIGN_OUT"); out != "" {
			os.MkdirAll(out, 0o755)
			os.WriteFile(filepath.Join(out, name+".png"), buf.Bytes(), 0o644)
		}
	}
}

func designDay(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

// The buttons every screen's renders drew are the controls the spec declares for it, no more and no fewer: the check of
// the spec's controls is against what the renderer found, not against the templates' text.
func TestControlsAreWhatIsDrawn(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	s, err := design.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer SetLanguage("en")
	for _, d := range s.Devices {
		renderDevice = d
		for screen, cases := range designRenders {
			var found []string
			for _, c := range cases {
				for _, lang := range c.langs {
					setRenderLanguage(lang)
					found = append(found, c.draw(fonts, d.Screen.Width, d.Screen.Height).ids()...)
				}
			}
			for _, p := range design.CheckControls(s, d.Name, screen, found) {
				t.Errorf("%s: %s", d.Name, p)
			}
		}
	}
}

// boxesJSON is a render's buttons as the file kept beside its picture: the boxes in px, by id, and where they are
// measured from.
func boxesJSON(r rendered) []byte {
	var ids []string
	for id := range r.boxes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b bytes.Buffer
	fmt.Fprintf(&b, "{\n \"placement\": %q,\n \"boxes\": {", r.placement)
	for i, id := range ids {
		bx := r.boxes[id]
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "\n  %q: [%d, %d, %d, %d]", id, bx.Min.X, bx.Min.Y, bx.Max.X, bx.Max.Y)
	}
	b.WriteString("\n }\n}\n")
	return b.Bytes()
}

func compareBoxes(t *testing.T, path, name string, r rendered, update bool) {
	got := boxesJSON(r)
	if update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: no kept boxes (GOLDEN=update keeps them): %v", name, err)
		return
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: the buttons' boxes differ from the kept ones", name)
	}
}

// compareGoldenRGBA is compareGolden for a colour picture.
func compareGoldenRGBA(t *testing.T, path, name string, img *image.RGBA, update bool) {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	if update {
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("%s: no kept render (GOLDEN=update keeps it): %v", name, err)
		return
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	g := image.NewRGBA(want.Bounds())
	draw.Draw(g, g.Rect, want, want.Bounds().Min, draw.Src)
	if g.Rect != img.Rect || !bytes.Equal(g.Pix, img.Pix) {
		t.Errorf("%s: differs from the kept render", name)
		if out := os.Getenv("DESIGN_OUT"); out != "" {
			os.MkdirAll(out, 0o755)
			os.WriteFile(filepath.Join(out, name+".png"), buf.Bytes(), 0o644)
		}
	}
}

// funnelCatalogue is a catalogue's page under its head (the funnel's states, 2026-11-23): invented books, or none
// with the facets in force (noMatch).
func funnelCatalogue(hv CatalogueHeadView, noMatch bool, f *blitz.Fonts, w, h int) rendered {
	books := []CatalogueBook{
		{Title: "The Lighthouse Keeper", Author: "An Author", Share: -1},
		{Title: "Letters from the Coast", Author: "Another Author", Share: -1},
		{Title: "A Small Town in Winter", Author: "A Patient Author", Here: true, Share: -1},
		{Title: "Notes on the Northern Sea", Author: "Someone", Share: -1},
	}
	v := CatalogueView{Name: hv.Name, Books: books, More: true}
	if noMatch {
		v = CatalogueView{Name: hv.Name, NoMatch: true}
	}
	c := NewCatalogue(v, CatalogueHead(libraryTopBar(), hv, f, w), f, w, h)
	defer c.Close()
	return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
}

// landingCatalogue is a catalogue entered on its books (planned, 2026-11-23): invented books under the head hv, or, choose,
// none found and Browse in their place.
func landingCatalogue(hv CatalogueHeadView, choose bool, f *blitz.Fonts, w, h int) rendered {
	v := CatalogueView{Name: hv.Name, More: true, Books: []CatalogueBook{
		{Title: "Le Phare du Nord", Author: "Un Auteur", Share: -1},
		{Title: "Lettres de la côte", Author: "Une Autrice", Share: -1},
		{Title: "Une petite ville en hiver", Author: "Un Auteur Patient", Here: true, Share: -1},
		{Title: "Notes sur la mer", Author: "Quelqu'un", Share: -1},
	}}
	if choose {
		v = CatalogueView{Name: hv.Name, Choose: true}
	}
	c := NewCatalogue(v, CatalogueHead(libraryTopBar(), hv, f, w), f, w, h)
	defer c.Close()
	return rendered{img: screenOf(w, h, c.Draw), boxes: c.Boxes(), placement: "screen"}
}

// filtersScreen is Browse and filter under the bar, its page page.
func filtersScreen(v FiltersView, page int, f *blitz.Fonts, w, h int) rendered {
	l := DrawCatalogueFilters(libraryTopBar(), v, f, w, h)
	defer l.Close()
	l.SetPage(min(page, l.Pages()-1))
	boxes := l.Boxes()
	for id := range boxes { // the bar over it is the catalogue's, as under any panel
		if strings.HasPrefix(id, "topbar-") || strings.HasPrefix(id, "shelf-") {
			delete(boxes, id)
		}
	}
	return rendered{img: screenOf(w, h, l.Draw), boxes: boxes, placement: "screen"}
}

// An invented catalogue's sections and facets, as a feed would give them.
var designBrowse = FilterGroup{Choices: []FilterChoice{{Label: "All books"}, {Label: "Poetry", On: true},
	{Label: "Drama"}, {Label: "New releases"}, {Label: "By author"}, {Label: "By subject"}}}

func designFacets() []FilterGroup {
	return []FilterGroup{
		{Name: "Sort by", Choices: []FilterChoice{{Label: "Popular"}, {Label: "Newest", On: true}, {Label: "Title"}, {Label: "Author"}}},
		{Name: "Language", Choices: []FilterChoice{{Label: "All"}, {Label: "English", Count: 6412}, {Label: "French", Count: 214, On: true},
			{Label: "German", Count: 388}, {Label: "Spanish", Count: 156}, {Label: "Italian", Count: 97}, {Label: "Portuguese", Count: 61},
			{Label: "Dutch", Count: 40}}, Folded: 23},
		{Name: "Subject", Choices: []FilterChoice{{Label: "All", On: true}, {Label: "Fiction"}, {Label: "Poetry"}, {Label: "Drama"}, {Label: "History"},
			{Label: "Philosophy"}, {Label: "Children's"}, {Label: "Travel"}}, Folded: 41},
	}
}

// More groups than a screen holds, as a big catalogue gives them.
func designMore() []FilterGroup {
	return []FilterGroup{
		{Name: "Era", Choices: []FilterChoice{{Label: "All", On: true}, {Label: "Ancient"}, {Label: "Medieval"}, {Label: "1600s"}, {Label: "1700s"}, {Label: "1800s"}, {Label: "1900s"}}},
		{Name: "Length", Choices: []FilterChoice{{Label: "Any", On: true}, {Label: "Short"}, {Label: "Novella"}, {Label: "Novel"}, {Label: "Long"}}},
		{Name: "Collection", Choices: []FilterChoice{{Label: "All", On: true}, {Label: "Classics"}, {Label: "Prize winners"}, {Label: "Translations"}, {Label: "Letters and diaries"}}, Folded: 12},
	}
}

func designLanguages() []FilterChoice {
	var cs []FilterChoice
	for i, l := range []string{"All", "English", "French", "German", "Spanish", "Italian", "Portuguese", "Dutch", "Latin", "Greek", "Russian", "Swedish", "Danish", "Finnish"} {
		cs = append(cs, FilterChoice{Label: l, Count: max(0, 900-i*60) * min(i, 1), On: l == "French"})
	}
	return cs
}

func designAuthors() []FilterChoice {
	var cs []FilterChoice
	for i, n := range []string{"Austen, Jane", "Balzac, Honoré de", "Brontë, Charlotte", "Cervantes, Miguel de", "Chekhov, Anton",
		"Dickens, Charles", "Dostoevsky, Fyodor", "Dumas, Alexandre", "Eliot, George", "Flaubert, Gustave", "Gogol, Nikolai", "Hugo, Victor",
		"James, Henry", "Kafka, Franz", "Melville, Herman", "Poe, Edgar Allan"} {
		cs = append(cs, FilterChoice{Label: n, Count: 3 + i*2})
	}
	return cs
}
