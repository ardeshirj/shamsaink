package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"shamsainkreader/files"
)

// A mark holds while the place is as it was when marked, and gives way once the book is read on or first opened.
func TestMarks(t *testing.T) {
	dir := t.TempDir()
	place := func(s string) { files.Write(filepath.Join(dir, "places", "k.place"), []byte(s)) }
	book := func() Item { return Shelf([]Book{{Path: "books/k.epub", ID: "k"}}, dir)[0] }

	// never opened, marked finished: Finished, not New; opened later, New's mark gone: Reading
	if err := SetMark(dir, "k", true); err != nil {
		t.Fatal(err)
	}
	if b := book(); !b.Finished() || b.In(1) || !b.In(2) {
		t.Errorf("never opened, marked finished: %+v", b)
	}
	place("0 10 0.4\n")
	if b := book(); b.Finished() || !b.In(0) {
		t.Errorf("opened after the mark: %+v", b)
	}
	// half read, marked finished; read on: Reading again
	SetMark(dir, "k", true)
	if b := book(); !b.Finished() || b.In(0) || time.Since(b.FinishedAt()) > time.Minute {
		t.Errorf("marked finished at 0.4: %+v", b)
	}
	place("1 0 0.6\n")
	if b := book(); b.Finished() {
		t.Errorf("read on after the mark: %+v", b)
	}
	// read to the end, marked not finished: Reading
	place("3 0 0.99\n")
	if b := book(); !b.Finished() {
		t.Errorf("at 0.99: %+v", b)
	}
	SetMark(dir, "k", false)
	if b := book(); b.Finished() || !b.In(0) {
		t.Errorf("marked not finished at 0.99: %+v", b)
	}
	// started again: New, the mark gone, the highlights kept
	files.Write(filepath.Join(dir, "highlights", "k.json"), []byte("[]\n"))
	if err := StartAgain(dir, "k"); err != nil {
		t.Fatal(err)
	}
	if b := book(); b.Opened || b.Mark != 0 || !b.In(1) {
		t.Errorf("started again: %+v", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "highlights", "k.json")); err != nil {
		t.Errorf("highlights gone: %v", err)
	}
}

// Removing a book deletes its files (its copies too) and all kept for it, and only its; the library drops it.
func TestRemoveBook(t *testing.T) {
	dir := libDir(t)
	copyFile(t, dir, "books/farsiepub/c.epub", "books/c-copy.epub")
	books := byPath(Scan(dir))
	a, c := books["books/a.epub"], books["books/c-copy.epub"]
	if len(c.Copies) != 1 {
		t.Fatalf("c: %+v", c)
	}
	var kept []string
	for _, k := range []string{a.ID, c.ID} {
		for _, f := range []string{"places/" + k + ".place", "places/" + k + ".mark", "highlights/" + k + ".json",
			"highlights/" + k + ".json.bad", "highlights/" + k + ".unplaced.json", "cache/" + k + ".cache"} {
			files.Write(filepath.Join(dir, f), []byte("x"))
			kept = append(kept, f)
		}
	}
	if err := Remove(dir, c); err != nil {
		t.Fatal(err)
	}
	for i, f := range kept {
		_, err := os.Stat(filepath.Join(dir, f))
		if gone := err != nil; gone != (i >= 6) {
			t.Errorf("%s: gone %v", f, gone)
		}
	}
	for _, p := range append([]string{c.Path}, c.Copies...) {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("%s is still there", p)
		}
	}
	if after := byPath(Scan(dir)); len(after) != len(books)-1 || after[c.Path].ID != "" {
		t.Errorf("library after: %v", after)
	}
	// a second time: the file is gone, nothing else to do
	if err := Remove(dir, c); err != nil {
		t.Errorf("again: %v", err)
	}
}
