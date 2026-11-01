//go:build blitz

package ui

import (
	"bytes"
	"fmt"
	"image"
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
	draw  func(f *blitz.Fonts, w, h int) *image.Gray
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

func screenOf(w, h int, draw func(img *image.Gray)) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	draw(img)
	return img
}

// designRenders has, for each screen of the spec that is drawn, the states to draw it in. The page (reading) is the
// engine's and is not here.
var designRenders = map[string][]renderCase{
	"topbar": {{"book", enFa, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawTopBar(TopBar{Start: "arrow-left", Title: "Chapter one", Time: "20:48", Phones: true, Battery: 82, RTL: RTL()}, f, w).img
	}}},
	"dock": {{"narrated", enFa, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawDock(Progress{Chapter: "Chapter one", At: 3, Of: 12, Page: 40, Total: 300, Share: 0.25, Back: 7,
			Ticks: []float64{0.2, 0.5}, Narrated: true, RTL: RTL()}, f, w, h).img
	}}, {"plain", enFa, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawDock(Progress{Chapter: "Chapter one", At: 3, Of: 12, Page: 40, Share: 0.6, RTL: RTL()}, f, w, h).img
	}}},
	"light": {{"panel", enFa, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawLight(40, 7, 10, 160, f, w).img }}},
	"text": {{"text", enFa, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawText(Text{Tab: "text", FontSize: 50, Lines: 1.16, Margin: 60}, f, w, h).img
	}}, {"page", enFa, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawText(Text{Tab: "page", FontSize: 50, Lines: 1.16, Margin: 60}, f, w, h).img
	}}},
	"contents": {{"list", enFa, func(f *blitz.Fonts, w, h int) *image.Gray {
		cv := NewContents([]Entry{{Title: "Chapter one"}, {Title: "باران از صبح", Depth: 1}, {Title: "Mixed: کتاب Book"}, {Title: "Chapter four"}}, 1, RTL(), f, w, h)
		defer cv.Close()
		return screenOf(w, h, cv.Draw)
	}}},
	"battery": {{"level", enFa, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawBattery(82, 160, image.Rect(w-300, 20, w-200, 120), f, w).img
	}}},
	"menu": {{"book", enFa, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawLibraryMenu(160, false, f, w).img }},
		{"library", en, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawLibraryMenu(160, true, f, w).img }}},
	"library": {{"reading", en, func(f *blitz.Fonts, w, h int) *image.Gray {
		topBar := TopBar{Start: "magnifying-glass", Time: "20:48", Battery: 82, Menu: true}
		lib := NewLibrary(designBooks(), LibraryHead(topBar, LibraryView{Counts: [4]int{4, 6, 1, 10}}, f, w), f, w, h)
		defer lib.Close()
		return screenOf(w, h, lib.Draw)
	}}},
	"library-language": {{"follow", en, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawLibraryLanguage(true, 160, f, w).img }},
		{"english", en, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawLibraryLanguage(false, 160, f, w).img }}},
	"library-sort": {{"title", en, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawLibrarySort(slicesIndex(library.Sorts, "title"), 300, f, w).img
	}}},
	"library-book-menu": {{"new", enFa, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawBookMenu(false, false, f, w).img }},
		{"opened", enFa, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawBookMenu(false, true, f, w).img }},
		{"finished", enFa, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawBookMenu(true, true, f, w).img }}},
	"library-book-details": {{"opened", en, func(f *blitz.Fonts, w, h int) *image.Gray {
		return DrawBookDetails(BookDetails{Title: "A Long Voyage", Author: "An Author", Lang: "en", Source: "a folder", File: "a-long-voyage.epub",
			Size: 3 << 20, Narrated: true, Opened: true, Read: 42, Added: designDay(2), LastRead: designDay(30)}, f, w).img
	}}},
	"library-book-remove": {{"ask", en, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawBookRemove("A Long Voyage", false, f, w).img }}},
	"highlight-menu": {{"new", enFa, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawHighlightMenu(false, true, f, w).img }},
		{"made", enFa, func(f *blitz.Fonts, w, h int) *image.Gray { return DrawHighlightMenu(true, false, f, w).img }}},
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
					img := c.draw(fonts, d.Screen.Width, d.Screen.Height)
					path := filepath.Join("..", "design", "golden", d.Name, name+".png")
					compareGolden(t, path, name, img, update)
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
