package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"shamsainkreader/files"
	"shamsainkreader/library"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"

	blitz "shamsainkengine"
)

// openPub parses the EPUB with go-toolkit's EPUB parser (as bench/epub does).
func openPub(ctx context.Context, path string) (*pub.Publication, error) {
	u, err := url.FromFilepath(path)
	if err != nil {
		return nil, err
	}
	a := asset.File(u)
	f, err := a.CreateFetcher(ctx, asset.Dependencies{ArchiveFactory: archive.NewArchiveFactory()}, "")
	if err != nil {
		return nil, err
	}
	pb, err := epub.NewParser(nil).Parse(ctx, a, f)
	if err != nil {
		return nil, err
	}
	return pb.Build(), nil
}

// A book being read: the publication, the engine's book, and the place, as (spine document, page in it): page
// numbers across the book shift until every document is paginated.
type reading struct {
	pub      *pub.Publication
	spine    []string
	read     blitz.ReadFunc
	fonts    *blitz.Fonts
	book     *blitz.Book
	key      string // the book's ID in the library (library.Book): what is kept for it is kept by it
	hash     string // its file's (library.Hash)
	rel      string // its file's path relative to DIR ("" outside it)
	lang     string // the book's language (bookLanguage): which fallback fonts come first
	bookRTL  bool   // readsRTL: its stated direction, else its language (neither: left to right, as Readium)
	rtl      bool   // as the pages turn: the book's, or as set for it
	doc, at  int
	cache    string // DIR/cache/HASH.cache: the file's; safe to delete (laid out again)
	place    string // DIR/places/ID.place: the book's; deleting it starts the book again
	restored bool
	saved    bool
	marks    *highlights
	// anchor is the word the place stands for across relayouts, {block, byte}, while the place is still where the last
	// relayout put it (anchorAt): kept, the page holding it is found again each time, and a setting changed and changed
	// back comes back to the page it left; a page turned moves the place, and the next relayout takes that page's first word
	anchor   [2]int
	anchorAt [2]int // {doc, at} the last relayout left the place at; {-1, -1} for none
}

// openBook opens a book at its place; direction is the way set for it by its ID ("rtl", "ltr", else the book's). A
// place kept for another file of the book (a new edition) gives only how far it was read, and its highlights are
// shown only where their text still is.
func openBook(ctx context.Context, path, dir string, o blitz.Options, fonts *blitz.Fonts, direction func(id string) string) (*reading, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	p, err := openPub(ctx, abs)
	if err != nil {
		return nil, err
	}
	var spine []string
	for _, l := range p.Manifest.ReadingOrder {
		spine = append(spine, l.Href.String())
	}
	read := func(path string) ([]byte, error) {
		u, err := url.URLFromDecodedPath(path)
		if err != nil {
			return nil, err
		}
		data, rerr := p.Get(ctx, manifest.Link{Href: manifest.NewHREF(u)}).Read(ctx, 0, 0)
		if rerr != nil {
			return nil, rerr
		}
		return data, nil
	}
	id, hash, rel, err := library.Find(dir, abs)
	if err != nil {
		return nil, err
	}
	r := &reading{
		pub:      p,
		spine:    spine,
		read:     read,
		fonts:    fonts,
		key:      id,
		hash:     hash,
		rel:      rel,
		anchorAt: [2]int{-1, -1},
		lang:     bookLanguage(p.Manifest.Metadata.Languages, p.Manifest.Metadata.Title()),
		bookRTL:  readsRTL(p.Manifest.Metadata),
		cache:    filepath.Join(dir, "cache", hash+".cache"),
		place:    filepath.Join(dir, "places", id+".place"),
		marks:    loadHighlights(dir, id),
	}
	r.direct(direction(id))
	r.make(o)
	// the place file: "DOC AT SHARE", then the hash of the file it was kept for and its path (none before 2026-11-21:
	// kept by the hash, so for this file)
	b, err := os.ReadFile(r.place)
	placed := err == nil
	other := false
	if placed {
		lines := strings.SplitN(string(b), "\n", 3)
		var share float64 // the old form has none
		n, _ := fmt.Sscan(lines[0], &r.doc, &r.at, &share)
		if len(lines) > 1 {
			h := strings.TrimSpace(lines[1])
			other = h != "" && h != hash
		}
		switch {
		case n < 2 || other && n < 3:
			r.doc, r.at = 0, 0
		case other:
			r.doc, r.at = r.atShare(share)
			fmt.Printf("book: a new edition: opened at %.0f%%, as far as it was read\n", share*100)
		}
	}
	r.doc = min(max(r.doc, 0), len(spine)-1)
	if !r.seek(r.doc, r.at) && !r.next() {
		return nil, fmt.Errorf("%s: no pages", path)
	}
	if other || !placed && len(r.marks.list) > 0 { // highlights kept for another file (no place to say: checked)
		if n := r.marks.unplace(r.text); n > 0 {
			fmt.Printf("book: %d highlights no longer where they were: set aside\n", n)
		}
	}
	if r.marks.tidy(r.text) {
		r.marks.save()
	}
	return r, nil
}

// atShare is the page at share s of the book (0 to 1), by documents: the document s falls in, as far into it.
func (r *reading) atShare(s float64) (doc, at int) {
	n := float64(len(r.spine))
	doc = min(max(int(s*n), 0), len(r.spine)-1)
	first, last := r.book.Layout(doc)
	return doc, int((s*n - float64(doc)) * float64(last-first))
}

// direct sets the way pages turn: "rtl" or "ltr", else the book's.
func (r *reading) direct(d string) {
	r.rtl = d == "rtl" || d != "ltr" && r.bookRTL
}

// make makes the engine's book for page settings o, its pages from the cache if it was saved for them.
func (r *reading) make(o blitz.Options) {
	o.Lang = r.lang
	r.book = blitz.New(r.spine, r.read, o, r.fonts)
	r.restored, r.saved = false, false
	if data, err := os.ReadFile(r.cache); err == nil {
		r.restored = r.book.Restore(data)
		r.saved = r.restored
	}
}

// relayout makes the book again for new page settings. The place stays on the page that holds the word it stands for
// (a word's place in its document's text stays when the book is laid out again): the page's first word, or the word
// kept by the relayout before when the place has not moved since (anchor). So a setting changed and changed back comes
// back to the same page (2026-11-06: kept as the same share of the document, rounded down, each change moved it a page
// back; the walk found it). A page with no words (a picture) keeps the same share of its document.
func (r *reading) relayout(o blitz.Options) {
	first, last := r.book.Layout(r.doc)
	share := float64(r.at) / float64(max(last-first, 1))
	word, hasWord := r.anchor, r.anchorAt == [2]int{r.doc, r.at}
	if !hasWord {
		word, hasWord = firstWord(r.book.Words(first + r.at))
	}
	r.book.Close()
	r.make(o)
	first, last = r.book.Layout(r.doc)
	at := int(share * float64(last-first))
	if hasWord {
		for p := 0; p < last-first; p++ { // the last page whose first word is not past the place's word
			if w, ok := firstWord(r.book.Words(first + p)); ok {
				if before(word, w) { // bookhighlights.go's: {block, byte} in document order
					break
				}
				at = p
			}
		}
	}
	r.seek(r.doc, at)
	r.anchorAt = [2]int{-1, -1}
	if hasWord {
		r.anchor, r.anchorAt = word, [2]int{r.doc, r.at}
	}
}

// firstWord is where the first of a page's words is in its document's text, {block, byte} (Words gives them line by
// line, in visual order).
func firstWord(ws []blitz.Word) ([2]int, bool) {
	if len(ws) == 0 {
		return [2]int{}, false
	}
	f := [2]int{ws[0].Block, ws[0].Start}
	for _, w := range ws[1:] {
		if p := [2]int{w.Block, w.Start}; before(p, f) {
			f = p
		}
	}
	return f, true
}

// seek goes to page at of document doc (its last page if at is past it); false if the document has no pages.
func (r *reading) seek(doc, at int) bool {
	first, last := r.book.Layout(doc)
	if last == first {
		return false
	}
	r.doc, r.at = doc, min(max(at, 0), last-first-1)
	return true
}

// page is the current page's number across the book now.
func (r *reading) page() int {
	first, _ := r.book.Layout(r.doc)
	return first + r.at
}

// next and prev turn a page, across documents; false at the end or the start of the book.
func (r *reading) next() bool {
	first, last := r.book.Layout(r.doc)
	if r.at+1 < last-first {
		r.at++
		return true
	}
	for d := r.doc + 1; d < len(r.pub.Manifest.ReadingOrder); d++ {
		if r.seek(d, 0) {
			return true
		}
	}
	r.book.Layout(r.doc) // the current document laid out again, if the others pushed it out
	return false
}

func (r *reading) prev() bool {
	if r.at > 0 {
		r.at--
		return true
	}
	for d := r.doc - 1; d >= 0; d-- {
		if r.seek(d, 1<<30) {
			return true
		}
	}
	r.book.Layout(r.doc)
	return false
}

// peek is the next page: its document, its page in it and its number across the book now; false at the end. The
// place stays where it is.
func (r *reading) peek() (doc, at, n int, ok bool) {
	d, a := r.doc, r.at
	if ok = r.next(); ok {
		doc, at, n = r.doc, r.at, r.page()
	}
	r.doc, r.at = d, a
	r.book.Layout(d) // the current document first again among those kept laid out
	return doc, at, n, ok
}

// close closes the engine's book (its memory given back) and the EPUB.
func (r *reading) close() {
	if r.book != nil {
		r.book.Close()
		r.pub.Close()
		r.book = nil
	}
}

// share is how much of the book is read, 0 to 1: by pages once every document is paginated (rest false), else by
// documents, the page's share of its own added.
func (r *reading) share(rest bool) float64 {
	if !rest {
		return float64(r.page()) / float64(max(r.book.Pages()-1, 1))
	}
	first, last := r.book.Layout(r.doc)
	return (float64(r.doc) + float64(r.at)/float64(max(last-first, 1))) / float64(len(r.spine))
}

// keep writes the place with the share read, and the cache once every document is paginated (once). The caller says
// whether the rest of the book is still being paginated: the session owns that, and the share depends on it.
func (r *reading) keep(rest bool) {
	files.Write(r.place, []byte(fmt.Sprintf("%d %d %.4f\n%s\n%s\n", r.doc, r.at, r.share(rest), r.hash, r.rel)))
	if !r.saved {
		if data := r.book.Save(); data != nil {
			files.Write(r.cache, data)
			r.saved = true
		}
	}
}

// persian says whether a book with these languages (BCP 47, as its metadata gives them) is in Farsi: "fa", "fa-IR",
// or the ISO 639-2 codes "per" and "fas"; with none given, rtl decides.
func persian(langs []string, rtl bool) bool {
	if len(langs) == 0 {
		return rtl
	}
	for _, l := range langs {
		switch strings.ToLower(strings.SplitN(l, "-", 2)[0]) {
		case "fa", "per", "fas":
			return true
		}
	}
	return false
}
