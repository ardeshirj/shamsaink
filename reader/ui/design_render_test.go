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
	"sort"
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

var en = []string{"en"}
var enFa = []string{"en", "fa"}

// designBooks are invented books (never a real book's text) for the library's renders.
func designBooks() []Book {
	books := []Book{
		{Title: "A Long Voyage", Author: "An Author", Read: 62, Narrated: true},
		{Title: "رود کم‌آب", Author: "مارا الیس", Read: 100, Finished: true, RTL: true},
		{Title: "Tales of a Small Town", Author: "Another Author", Read: -1},
		{Title: "سفر به ماه", Read: 30, RTL: true},
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

// libraryTopBar is the bar of the library's renders.
var libraryTopBar = TopBar{Start: "magnifying-glass", Time: "20:48", Phones: true, Battery: 82, Menu: true}

func libraryHead(f *blitz.Fonts, w int) *Panel {
	return LibraryHead(libraryTopBar, LibraryView{Counts: [4]int{4, 6, 1, 10}}, f, w)
}

// barHeight is the library's bar: where the panels that hang from it start.
func barHeight(f *blitz.Fonts, w int) int { return DrawTopBar(libraryTopBar, f, w).Rect().Dy() }

// batteryBox is the battery's place on that bar.
func batteryBox(f *blitz.Fonts, w int) image.Rectangle {
	return DrawTopBar(libraryTopBar, f, w).Box("topbar-battery")
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
	if RTL() {
		return "فصل یکم"
	}
	return "Chapter one"
}

// designRenders has, for each screen of the spec that is drawn, the states to draw it in. The page (reading) is the
// engine's and is not here.
var designRenders = map[string][]renderCase{
	"splash": {{"grey", enFa, func(f *blitz.Fonts, w, h int) rendered { return opened(Splash(tmp(), f, w, h, false)) }},
		{"colour", enFa, func(f *blitz.Fonts, w, h int) rendered { return opened(Splash(tmp(), f, w, h, true)) }}},
	"loading": {{"grey", enFa, func(f *blitz.Fonts, w, h int) rendered { return opened(Loading(tmp(), f, w, h, false)) }},
		{"colour", enFa, func(f *blitz.Fonts, w, h int) rendered { return opened(Loading(tmp(), f, w, h, true)) }}},
	"topbar": {{"book", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: true, Battery: 82, Menu: true, RTL: RTL()}, f, w))
	}}, {"battery-open", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawTopBar(TopBar{Start: "arrow-left", Title: chapter(), Time: "20:48", Phones: true, Battery: 82, Menu: true, BatteryOn: true, RTL: RTL()}, f, w))
	}}},
	"dock": {{"narrated", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawDock(Progress{Chapter: chapter(), At: 3, Of: 12, Page: 40, Total: 300, Share: 0.25, Back: 7,
			Ticks: []float64{0.2, 0.5}, Narrated: true, RTL: RTL()}, f, w, h))
	}}, {"plain", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawDock(Progress{Chapter: chapter(), At: 3, Of: 12, Page: 40, Share: 0.6, RTL: RTL()}, f, w, h))
	}}},
	"light": {{"panel", enFa, func(f *blitz.Fonts, w, h int) rendered { return pr(DrawLight(40, 7, 10, false, barHeight(f, w), f, w)) }},
		{"dark", en, func(f *blitz.Fonts, w, h int) rendered { return pr(DrawLight(40, 7, 10, true, barHeight(f, w), f, w)) }}},
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
	}}, {"mixed", en, func(f *blitz.Fonts, w, h int) rendered { // English and Farsi entries in one list: each in its own direction
		cv := NewContents([]Entry{{Title: "Chapter one"}, {Title: "باران از صبح", Depth: 1}, {Title: "Mixed: کتاب Book"}, {Title: "Chapter four"}}, 1, false, f, w, h)
		defer cv.Close()
		return rendered{img: screenOf(w, h, cv.Draw), boxes: cv.Boxes(), placement: "screen"}
	}}},
	"battery": {{"level", enFa, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawBattery(82, barHeight(f, w), batteryBox(f, w), f, w))
	}}},
	"menu": {{"book", enFa, func(f *blitz.Fonts, w, h int) rendered { return pr(DrawLibraryMenu(barHeight(f, w), false, f, w)) }},
		{"library", en, func(f *blitz.Fonts, w, h int) rendered { return pr(DrawLibraryMenu(barHeight(f, w), true, f, w)) }}},
	"library": {{"reading", en, func(f *blitz.Fonts, w, h int) rendered {
		lib := NewLibrary(designBooks(), libraryHead(f, w), f, w, h)
		defer lib.Close()
		return rendered{img: screenOf(w, h, lib.Draw), boxes: lib.Boxes(), placement: "screen"}
	}}},
	"library-language": {{"follow", en, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawLibraryLanguage(true, 10, barHeight(f, w), f, w))
	}},
		{"english", en, func(f *blitz.Fonts, w, h int) rendered {
			return pr(DrawLibraryLanguage(false, 0, barHeight(f, w), f, w))
		}}},
	"library-sort": {{"title", en, func(f *blitz.Fonts, w, h int) rendered {
		return pr(DrawLibrarySort(slicesIndex(library.Sorts, "title"), libraryHead(f, w).img.Rect.Dy(), f, w))
	}}},
	"library-book-menu": {{"new", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawBookMenu(false, false, f, w)) }},
		{"opened", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawBookMenu(false, true, f, w)) }},
		{"finished", enFa, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawBookMenu(true, true, f, w)) }}},
	"library-book-details": {{"opened", en, func(f *blitz.Fonts, w, h int) rendered {
		return rel(DrawBookDetails(BookDetails{Title: "A Long Voyage", Author: "An Author", Lang: "en", Source: "a folder", File: "a-long-voyage.epub",
			Size: 3 << 20, Narrated: true, Opened: true, Read: 42, Added: designDay(2), LastRead: designDay(30)}, f, w))
	}}},
	"library-book-remove": {{"ask", en, func(f *blitz.Fonts, w, h int) rendered { return rel(DrawBookRemove("A Long Voyage", false, f, w)) }}},
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
// compared only with FONTS set (the fonts of reader/out/fonts: other fonts draw other letters); GOLDEN=update keeps
// the drawings as the new ones; DESIGN_OUT: a folder for the pictures that differ.
func TestScreenRenders(t *testing.T) {
	dir := os.Getenv("FONTS")
	if dir == "" {
		t.Skip("FONTS not set: renders are compared with the fonts of reader/out/fonts")
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
		for _, screen := range names {
			for _, c := range designRenders[screen] {
				for _, lang := range c.langs {
					SetLanguage(lang)
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
	d := s.Devices[0]
	for screen, cases := range designRenders {
		var found []string
		for _, c := range cases {
			for _, lang := range c.langs {
				SetLanguage(lang)
				found = append(found, c.draw(fonts, d.Screen.Width, d.Screen.Height).ids()...)
			}
		}
		for _, p := range design.CheckControls(s, d.Name, screen, found) {
			t.Error(p)
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
