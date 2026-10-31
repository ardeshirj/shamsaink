package ui

import (
	"fmt"

	blitz "shamsainkengine"
)

// An entry of the table of contents as shown: its title and how deep it is.
type Entry struct {
	Title string
	Depth int
}

// The table of contents as shown: a header with a way back, and the list under it (a List: laid out and paginated by
// the engine, right to left for a right-to-left book, each entry's page and box on it), without a pager.
type Contents struct{ *List }

// The list's entries as the template draws them: Pad is the indent in em.
type contentsEntry struct {
	ID, Pad, Title string
	Cur            bool
}

type contentsData struct {
	Dir     string
	Entries []contentsEntry
}

// NewContents lays out the table of contents es, entry cur (-1: none) marked and its page shown first.
func NewContents(es []Entry, cur int, rtl bool, fonts *blitz.Fonts, w, h int) *Contents {
	dir := "ltr"
	if rtl {
		dir = "rtl"
	}
	d := contentsData{Dir: dir}
	for i, e := range es {
		title := e.Title
		if title == "" {
			title = "…"
		}
		d.Entries = append(d.Entries, contentsEntry{ID: fmt.Sprint("e", i), Cur: i == cur,
			Pad: fmt.Sprintf("%.1f", 0.3+1.5*float64(e.Depth)), Title: title})
	}
	html, _ := render("contents.html", d)
	headHTML, headIDs := render("contents-head.html", nil)
	head := drawPanel(headHTML, headIDs, fonts, w, 300)
	return &Contents{NewList(head, html, len(es), cur, false, rtl, fonts, w, h)}
}
