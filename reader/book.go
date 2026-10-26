package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	book     *blitz.Book
	rtl      bool
	doc, at  int
	cache    string // DIR/cache/: by the book's name, size and date; safe to delete (laid out again)
	place    string // DIR/places/: the same name; deleting it starts the book again
	restored bool
	saved    bool
}

func openBook(ctx context.Context, path, dir string, o blitz.Options, fonts *blitz.Fonts) (*reading, error) {
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
	key := fmt.Sprintf("%s-%d-%d", filepath.Base(abs), st.Size(), st.ModTime().Unix())
	r := &reading{
		pub:  p,
		book: blitz.New(spine, read, o, fonts),
		// as go-toolkit says (its stated direction, else its language); a book that says neither turns left to
		// right, as in Readium (a setting to reverse it: stage 3)
		rtl:   p.Manifest.Metadata.EffectiveReadingProgression() == manifest.RTL,
		cache: filepath.Join(dir, "cache", key+".cache"),
		place: filepath.Join(dir, "places", key+".place"),
	}
	if data, err := os.ReadFile(r.cache); err == nil {
		r.restored = r.book.Restore(data)
		r.saved = r.restored
	}
	if b, err := os.ReadFile(r.place); err == nil {
		fmt.Sscan(string(b), &r.doc, &r.at)
	}
	r.doc = min(max(r.doc, 0), len(spine)-1)
	if !r.seek(r.doc, r.at) && !r.next() {
		return nil, fmt.Errorf("%s: no pages", path)
	}
	return r, nil
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

// keep writes the place, and the cache once every document is paginated (once).
func (r *reading) keep() {
	os.MkdirAll(filepath.Dir(r.place), 0o755)
	os.MkdirAll(filepath.Dir(r.cache), 0o755)
	os.WriteFile(r.place, []byte(fmt.Sprintf("%d %d\n", r.doc, r.at)), 0o644)
	if !r.saved {
		if data := r.book.Save(); data != nil {
			os.WriteFile(r.cache, data, 0o644)
			r.saved = true
		}
	}
}
