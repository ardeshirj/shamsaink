package main

import (
	"os"
	"strings"
	"testing"

	blitz "shamsainkengine"
)

// A narrated paragraph that runs onto the next page: FindLines gives its lines on its first page only, FindAllLines
// every line with its page, the first page's the same, then the next page's, so the narration can turn the page when
// the voice reaches them (underline).
func TestParagraphOverPages(t *testing.T) {
	dir := os.Getenv("FONTS")
	if dir == "" {
		t.Skip("FONTS not set")
	}
	fonts := blitz.LoadFonts(dir)
	html := `<html><body><p>Before.</p><p id="p">` + strings.Repeat("A sentence of a paragraph that is long. ", 60) + `</p></body></html>`
	b := blitz.New([]string{"a.html"}, func(string) ([]byte, error) { return []byte(html), nil },
		blitz.Options{Width: 600, Height: 400, Margin: 20, FontSize: 20}, fonts)
	defer b.Close()
	page, first := b.FindLines(0, "p")
	all := b.FindAllLines(0, "p")
	if page < 0 || len(first) == 0 || len(all) <= len(first) {
		t.Fatalf("page %d, %d lines on it, %d in all: the paragraph should run onto another page", page, len(first), len(all))
	}
	for i, l := range all {
		switch {
		case i < len(first) && (l.Page != page || l.Box != first[i]):
			t.Errorf("line %d: page %d %v, want FindLines' page %d %v", i, l.Page, l.Box, page, first[i])
		case i == len(first) && l.Page != page+1:
			t.Errorf("line %d, the first after the first page's: page %d, want %d", i, l.Page, page+1)
		case i > 0 && l.Page < all[i-1].Page:
			t.Errorf("line %d: page %d after page %d", i, l.Page, all[i-1].Page)
		}
	}
}
