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
	book := func() Item { return Shelf([]Book{{Path: "books/k.epub", Key: "k"}}, dir)[0] }

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

// Removing a book deletes its file and all kept for it, and only its; the library drops it.
func TestRemoveBook(t *testing.T) {
	dir := libDir(t)
	books := byPath(Scan(dir))
	a, c := books["books/a.epub"], books["books/farsiepub/c.epub"]
	var kept []string
	for _, k := range []string{a.Key, c.Key} {
		for _, f := range []string{"places/" + k + ".place", "places/" + k + ".mark", "highlights/" + k + ".json",
			"highlights/" + k + ".json.bad", "cache/" + k + ".cache"} {
			files.Write(filepath.Join(dir, f), []byte("x"))
			kept = append(kept, f)
		}
	}
	if err := Remove(dir, c); err != nil {
		t.Fatal(err)
	}
	for i, f := range kept {
		_, err := os.Stat(filepath.Join(dir, f))
		if gone := err != nil; gone != (i >= 5) {
			t.Errorf("%s: gone %v", f, gone)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, c.Path)); err == nil {
		t.Error("the book's file is still there")
	}
	if after := byPath(Scan(dir)); len(after) != len(books)-1 || after[c.Path].Key != "" {
		t.Errorf("library after: %v", after)
	}
	// a second time: the file is gone, nothing else to do
	if err := Remove(dir, c); err != nil {
		t.Errorf("again: %v", err)
	}
}
