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
	v := newContentsView(toc, r.chapter(toc), r.rtl, fonts, w, h)
	defer v.close()
	img := image.NewGray(image.Rect(0, 0, w, h))
	v.draw(img)
	save("contents.png", img)
	nav := drawNav(progress{chapter: "a chapter's title", at: 3, of: 12, page: 40, total: 160, share: 0.25, back: 7,
		rtl: r.rtl}, fonts, w, h)
	page := image.NewGray(image.Rect(0, 0, w, h))
	r.book.Draw(r.page(), page)
	nav.over(page)
	for _, id := range navIDs {
		if b, ok := nav.buttons[id]; !ok || b.Empty() {
			t.Errorf("button %s not on the panel", id)
		}
	}
	save("nav.png", page)
}
