package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/library"
	"shamsainkreader/opds"
	"shamsainkreader/ui"
)

// A book's dialog in the library (libraryview.go), opened by a finger held on its tile (library/changes.go): its
// details, with Mark as finished or new, its cover, and Remove (asked first); a catalogue's book's, by a tap or a hold
// on its cover, with its download or Read.

// showBook shows a book's dialog (kind: "details", "remove", "catalogue"; variant: which of its cases, the kept
// renders' names) in the middle of the screen, in the place of what was open.
func (s *libraryScreen) showBook(p *ui.Panel, kind, variant string) {
	s.showDialog(p, fmt.Sprintf("book %s (%s)", kind, variant))
	s.setKind(kind)
}

// bookDialog opens the details of the book in row i, held.
func (s *libraryScreen) bookDialog(i int) {
	s.held = i
	b := s.items[i]
	s.showBook(ui.DrawBookDetails(bookDetails(s.rd.dir, b, s.set), s.rd.fonts, s.scr.W), "details", bookCase(b.Finished(), b.Opened))
	fmt.Printf("library: %s held: its details\n", b.Path)
}

// catBookDialog opens the dialog of the catalogue's book i, its cover held or tapped (how).
func (s *libraryScreen) catBookDialog(i int, how string) {
	s.held = i
	s.showBook(ui.DrawCatalogueBook(s.rd.catalogueBook(s.cat, i), s.rd.fonts, s.scr.W), "catalogue", "book")
	fmt.Printf("library: %s %s: its details\n", opds.EntryID(&s.cat.books[i]), how)
}

// catBookAct does the catalogue's book's dialog's action (Get or Read), the dialog closed: the book's file to open
// when it is here, else "".
func (s *libraryScreen) catBookAct() string {
	i := s.held
	r := s.panel.Rect()
	s.panel, s.held = nil, -1
	s.setKind("")
	if p := s.catBook(i); p != "" {
		return p
	}
	s.v.Draw(s.rd.over)
	s.show(r, device.WfREAGL, false)
	return ""
}

// bookFinish marks the book held as finished.
func (s *libraryScreen) bookFinish() {
	b := s.items[s.held]
	if err := library.MarkFinished(s.rd.dir, b.ID); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: mark:", err)
	}
	fmt.Printf("library: %s marked finished\n", b.Path)
	s.changed()
}

// bookNew marks the book held as new.
func (s *libraryScreen) bookNew() {
	b := s.items[s.held]
	if err := library.MarkNew(s.rd.dir, b.ID); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: mark new:", err)
	}
	fmt.Printf("library: %s marked new\n", b.Path)
	s.changed()
}

// bookCover shows the book held's tile with its cover's words, or its picture, kept by its ID; its dialog again.
func (s *libraryScreen) bookCover(words bool) {
	b := s.items[s.held]
	if s.set.Cover == nil {
		s.set.Cover = map[string]string{}
	}
	if words {
		s.set.Cover[b.ID] = "words"
	} else {
		delete(s.set.Cover, b.ID)
	}
	s.set.save(s.rd.dir)
	page := s.v.Page()
	s.build() // the tile behind, as it will be when the dialog closes
	s.v.SetPage(min(page, max(s.v.Pages()-1, 0)))
	s.showBook(ui.DrawBookDetails(bookDetails(s.rd.dir, b, s.set), s.rd.fonts, s.scr.W), "details", bookCase(b.Finished(), b.Opened))
	fmt.Printf("library: %s cover: %s\n", b.Path, map[bool]string{true: "words", false: "picture"}[words])
}

// bookRemoveAsk asks before the book held is removed, in its details' place.
func (s *libraryScreen) bookRemoveAsk() {
	b := s.items[s.held]
	s.showBook(ui.DrawBookRemove(b.Title, writtenRTL(library.Lang(b.Book)), 1+len(b.Copies), s.rd.fonts, s.scr.W), "remove", "ask")
	fmt.Printf("library: %s: remove?\n", b.Path)
}

// bookRemove removes the book held, and what was chosen for it (its page direction, its cover).
func (s *libraryScreen) bookRemove() {
	b := s.items[s.held]
	if err := library.Remove(s.rd.dir, b.Book); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: remove:", err)
	} else {
		fmt.Printf("library: %s removed\n", b.Path)
		_, dir := s.set.Direction[b.ID]
		_, cover := s.set.Cover[b.ID]
		if dir || cover { // the page direction and the cover chosen for it
			delete(s.set.Direction, b.ID)
			delete(s.set.Cover, b.ID)
			s.set.save(s.rd.dir)
		}
	}
	s.changed()
}

// changed follows a book changed: the library scanned again, on the same page if it still has one, the dialog closed.
func (s *libraryScreen) changed() {
	page := s.v.Page()
	s.all = library.Shelf(library.Scan(s.rd.dir), s.rd.dir)
	s.panel, s.held = nil, -1
	s.setKind("")
	s.build()
	s.v.SetPage(min(page, max(s.v.Pages()-1, 0)))
	s.v.Draw(s.rd.over)
	s.showAll(device.WfREAGL, false)
}

// bookDetails is what a book's dialog shows of it (ui.DrawBookDetails).
func bookDetails(dir string, b library.Item, set *settings) ui.BookDetails {
	d := ui.BookDetails{Title: b.Title, Author: b.Author, Lang: library.Lang(b.Book), Source: b.Source,
		File: filepath.Base(b.Path), RTL: writtenRTL(library.Lang(b.Book)), Narrated: b.Narrated,
		Picture: b.Cover != "", Words: set.Cover[b.ID] == "words",
		Opened: b.Opened, Finished: b.Finished(), Read: -1, Added: time.Unix(b.Added, 0), Copies: 1 + len(b.Copies),
		Year: b.Year}
	if st, err := os.Stat(filepath.Join(dir, b.Path)); err == nil {
		d.Size = st.Size()
	}
	if b.Opened {
		d.LastRead = b.When
		if b.Share >= 0 {
			d.Read = int(b.Share*100 + 0.5)
		}
	}
	return d
}

// bookCase is which of its cases a book's menu or details is drawn in, as the kept renders name them: finished, opened
// or new.
func bookCase(finished, opened bool) string {
	switch {
	case finished:
		return "finished"
	case opened:
		return "opened"
	}
	return "new"
}
