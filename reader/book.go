package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"

	blitz "indiumengine"
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
	key      string // the book's name, size and date
	bookRTL  bool   // as go-toolkit says (its stated direction, else its language; neither: left to right, as Readium)
	rtl      bool   // as the pages turn: the book's, or as set for it
	farsi    bool   // in Farsi, as its metadata says; none said: if it reads right to left (the clock's digits)
	doc, at  int
	cache    string // DIR/cache/: by the book's name, size and date; safe to delete (laid out again)
	place    string // DIR/places/: the same name; deleting it starts the book again
	restored bool
	saved    bool
	marks    *highlights
}

// openBook opens a book at its place; direction is the way set for it by its key ("rtl", "ltr", else the book's).
func openBook(ctx context.Context, path, dir string, o blitz.Options, fonts *blitz.Fonts, direction func(key string) string) (*reading, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
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
	key := bookKey(filepath.Base(abs), st.Size(), st.ModTime())
	r := &reading{
		pub:     p,
		spine:   spine,
		read:    read,
		fonts:   fonts,
		key:     key,
		bookRTL: p.Manifest.Metadata.EffectiveReadingProgression() == manifest.RTL,
		cache:   filepath.Join(dir, "cache", key+".cache"),
		place:   filepath.Join(dir, "places", key+".place"),
		marks:   loadHighlights(dir, key),
	}
	r.farsi = persian(p.Manifest.Metadata.Languages, r.bookRTL)
	r.direct(direction(key))
	r.make(o)
	if b, err := os.ReadFile(r.place); err == nil {
		var share float64 // the old form has none
		if n, _ := fmt.Sscan(string(b), &r.doc, &r.at, &share); n < 2 {
			r.doc, r.at = 0, 0
		}
	}
	r.doc = min(max(r.doc, 0), len(spine)-1)
	if !r.seek(r.doc, r.at) && !r.next() {
		return nil, fmt.Errorf("%s: no pages", path)
	}
	if r.marks.tidy(r.text) {
		r.marks.save()
	}
	return r, nil
}

// direct sets the way pages turn: "rtl" or "ltr", else the book's.
func (r *reading) direct(d string) {
	r.rtl = d == "rtl" || d != "ltr" && r.bookRTL
}

// make makes the engine's book for page settings o, its pages from the cache if it was saved for them.
func (r *reading) make(o blitz.Options) {
	r.book = blitz.New(r.spine, r.read, o, r.fonts)
	r.restored, r.saved = false, false
	if data, err := os.ReadFile(r.cache); err == nil {
		r.restored = r.book.Restore(data)
		r.saved = r.restored
	}
}

// relayout makes the book again for new page settings; the place stays in its document, as the same share of it.
func (r *reading) relayout(o blitz.Options) {
	first, last := r.book.Layout(r.doc)
	share := float64(r.at) / float64(max(last-first, 1))
	r.book.Close()
	r.make(o)
	first, last = r.book.Layout(r.doc)
	r.seek(r.doc, int(share*float64(last-first)))
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
	writeFile(r.place, []byte(fmt.Sprintf("%d %d %.4f\n", r.doc, r.at, r.share(rest))))
	if !r.saved {
		if data := r.book.Save(); data != nil {
			writeFile(r.cache, data)
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
