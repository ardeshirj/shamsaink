//go:build blitz

package main

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	blitz "indiumengine"
	"indiumreader/ui"
)

// On a real book (BOOK, an .epub; FONTS a fallback fonts folder; OUT a folder for the pictures): "Next chapter"
// from the start goes through every entry of the table of contents that starts on a page of its own, in order,
// "Previous chapter" back again; the contents view and the bottom panel are drawn into OUT to look at.
func TestContents(t *testing.T) {
	book := os.Getenv("BOOK")
	if book == "" {
		t.Skip("BOOK not set")
	}
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	const w, h = 1236, 1648
	o := blitz.Options{Width: w, Height: h, Margin: 60, FontSize: 50}
	r, err := openBook(context.Background(), book, t.TempDir(), o, fonts, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	toc := contents(r.pub.Manifest.TableOfContents, r.spine)
	t.Logf("%d entries, %d documents, right to left %v", len(toc), len(r.spine), r.rtl)
	r.seek(0, 0)
	var visited []int
	for c := r.nextChapter(toc); c >= 0 && len(visited) <= len(toc); c = r.nextChapter(toc) {
		at := r.entryAt(toc[c])
		before := [2]int{r.doc, r.at}
		if !r.seek(toc[c].doc, at) {
			t.Fatalf("entry %d: document %d has no pages", c, toc[c].doc)
		}
		if !(r.doc > before[0] || r.doc == before[0] && r.at > before[1]) {
			t.Fatalf("entry %d did not move forward: %v to %d %d", c, before, r.doc, r.at)
		}
		if got := r.chapter(toc); got < c {
			t.Errorf("at entry %d, chapter says %d", c, got)
		}
		visited = append(visited, c)
	}
	if len(visited) == 0 {
		t.Fatal("no chapter after the start")
	}
	for back := 0; r.prevChapter(toc) >= 0; back++ {
		c := r.prevChapter(toc)
		r.seek(toc[c].doc, r.entryAt(toc[c]))
		if back > len(toc) {
			t.Fatal("Previous chapter does not reach the start")
		}
	}
	t.Logf("next chapter: %d stops; back to document %d page %d", len(visited), r.doc, r.at)

	out := os.Getenv("OUT")
	if out == "" {
		return
	}
	save := func(name string, img *image.Gray) {
		f, _ := os.Create(filepath.Join(out, name))
		defer f.Close()
		png.Encode(f, img)
	}
	r.seek(toc[visited[len(visited)/2]].doc, 0)
	v := ui.NewContents(uiEntries(toc), r.chapter(toc), r.rtl, fonts, w, h)
	defer v.Close()
	img := image.NewGray(image.Rect(0, 0, w, h))
	v.Draw(img)
	save("contents.png", img)
	nav := ui.DrawNav(ui.Progress{Chapter: "a chapter's title", At: 3, Of: 12, Page: 40, Total: 160, Share: 0.25, Back: 7,
		RTL: r.rtl}, fonts, w, h)
	page := image.NewGray(image.Rect(0, 0, w, h))
	r.book.Draw(r.page(), page)
	nav.Over(page)
	save("nav.png", page)
}
