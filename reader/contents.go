package main

import (
	"fmt"
	"image"
	"net/url"
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"

	blitz "indiumengine"
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

// The table of contents as shown: a header with a way back, and the list under it, laid out and paginated by the
// engine (right to left for a right-to-left book), each entry's page and box on it.
type contentsView struct {
	head  *menu
	list  *blitz.Book
	pages int
	page  int
	img   *image.Gray // the list's page shown
	at    []image.Rectangle
	on    []int // each entry's page
}

// The list's entries as the template draws them: Pad is the indent in em.
type contentsEntry struct {
	ID, Pad, Title string
	Cur            bool
}

type contentsData struct {
	Dir     string
	Entries []contentsEntry
}

func newContentsView(es []entry, cur int, rtl bool, fonts *blitz.Fonts, w, h int) *contentsView {
	dir := "ltr"
	if rtl {
		dir = "rtl"
	}
	d := contentsData{Dir: dir}
	for i, e := range es {
		title := e.title
		if title == "" {
			title = "…"
		}
		d.Entries = append(d.Entries, contentsEntry{ID: fmt.Sprint("e", i), Cur: i == cur,
			Pad: fmt.Sprintf("%.1f", 0.3+1.5*float64(e.depth)), Title: title})
	}
	html, _ := render("contents.html", d)
	headHTML, headIDs := render("contents-head.html", nil)
	head := drawPanel(headHTML, headIDs, fonts, w, 300)
	hh := head.img.Rect.Dy()
	o := blitz.Options{Width: w, Height: h - hh, Margin: 40, FontSize: 44}
	v := &contentsView{
		head: head,
		list: blitz.New([]string{"contents.html"}, func(string) ([]byte, error) { return []byte(html), nil }, o, fonts),
		img:  image.NewGray(image.Rect(0, 0, w, h-hh)),
	}
	v.list.Layout(0)
	v.pages = v.list.Pages()
	for i := range es {
		p, rs := v.list.Find(0, fmt.Sprint("e", i))
		v.on = append(v.on, p)
		v.at = append(v.at, union(rs))
	}
	if cur >= 0 {
		v.page = max(v.on[cur], 0)
	}
	return v
}

// draw draws the view's page into the screen's page img.
func (v *contentsView) draw(img *image.Gray) {
	v.list.Draw(v.page, v.img)
	copy(img.Pix, v.head.img.Pix)
	copy(img.Pix[v.head.img.Rect.Dy()*img.Stride:], v.img.Pix)
}

// entryAt is the entry tapped at (x, y) on the screen, -1 if none.
func (v *contentsView) entryAt(x, y int) int {
	for i, r := range v.at {
		if v.on[i] == v.page && image.Pt(x, y-v.head.img.Rect.Dy()).In(r.Inset(-8)) {
			return i
		}
	}
	return -1
}

func (v *contentsView) close() { v.list.Close() }
