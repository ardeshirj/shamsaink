package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
)

// An entry of the book's table of contents: its title, how deep it is, its spine document and the id it points to
// in it ("" for the document's start).
type entry struct {
	title      string
	depth, doc int
	id         string
}

// contents is the book's table of contents, flattened in order, entries outside the spine left out; without one,
// one entry a spine document.
func contents(toc manifest.LinkList, spine []string) []entry {
	index := map[string]int{}
	for i, s := range spine {
		index[s] = i
		if u, err := url.PathUnescape(s); err == nil {
			index[u] = i
		}
	}
	var out []entry
	var walk func(ls manifest.LinkList, depth int)
	walk = func(ls manifest.LinkList, depth int) {
		for _, l := range ls {
			path, id, _ := strings.Cut(l.Href.String(), "#")
			doc, ok := index[path]
			if !ok {
				if u, err := url.PathUnescape(path); err == nil {
					doc, ok = index[u]
				}
			}
			if ok {
				out = append(out, entry{title: strings.TrimSpace(l.Title), depth: depth, doc: doc, id: id})
			}
			walk(l.Children, depth+1)
		}
	}
	walk(toc, 0)
	if len(out) == 0 {
		for i := range spine {
			out = append(out, entry{title: fmt.Sprintf("Part %d", i+1), doc: i})
		}
	}
	return out
}

// at is entry e's page in its document (its first if its id is not found); lays the document out.
func (r *reading) entryAt(e entry) int {
	first, _ := r.book.Layout(e.doc)
	if e.id != "" {
		if p, _ := r.book.Find(e.doc, e.id); p >= first {
			return p - first
		}
	}
	return 0
}

// chapter is the entry the place is in: the last one starting at or before it; -1 if it is before them all. Only the
// current document's entries are looked for in it (laid out already).
func (r *reading) chapter(es []entry) int {
	cur := -1
	for i, e := range es {
		if e.doc > r.doc || e.doc == r.doc && r.entryAt(e) > r.at {
			break
		}
		cur = i
	}
	return cur
}

// nextChapter and prevChapter are the entries to go to for "Next chapter" and "Previous chapter": the first that
// starts after the page shown; the start of the chapter shown if past its first page, else the one before. -1 if none.
func (r *reading) nextChapter(es []entry) int {
	for i := r.chapter(es) + 1; i < len(es); i++ {
		if es[i].doc > r.doc || r.entryAt(es[i]) > r.at {
			return i
		}
	}
	return -1
}

func (r *reading) prevChapter(es []entry) int {
	for i := r.chapter(es); i >= 0; i-- {
		if es[i].doc < r.doc || r.entryAt(es[i]) < r.at {
			return i
		}
	}
	return -1
}
