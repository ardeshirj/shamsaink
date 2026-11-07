//go:build blitz

package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	blitz "shamsainkengine"
)

// The pictures of the getting-started book (reader/welcome) that are not a design render: the library with real books
// (titles and authors only). WELCOME: the folder to write them into (reader/welcome/pictures); FONTS as for the renders.
func TestWelcomePictures(t *testing.T) {
	dir, fontDir := os.Getenv("WELCOME"), os.Getenv("FONTS")
	if dir == "" || fontDir == "" {
		t.Skip("WELCOME and FONTS not set")
	}
	f := blitz.LoadFonts(fontDir)
	defer SetLanguage("en")
	SetLanguage("en")
	w, h := 1264, 1680 // the Kobo Libra Colour, as the design renders
	books := []Book{
		{Title: "Moby-Dick", Author: "Herman Melville", Read: 35},
		{Title: "زنده به گور", Author: "صادق هدایت", Read: 62, Narrated: true, RTL: true},
	}
	// All chosen, so the rows and the counts agree
	head := LibraryHead(libraryTopBar, LibraryView{Counts: [4]int{2, 0, 0, 2}, Filter: 3}, f, w)
	lib := NewLibrary(books, "shamsaink/books", head, f, w, h)
	defer lib.Close()
	img := screenOf(w, h, lib.Draw).SubImage(image.Rect(0, 0, w, 740)) // the rows' part: below them the page is empty
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(filepath.Join(dir, "library-en.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
}
