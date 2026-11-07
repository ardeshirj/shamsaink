package library

import (
	"os"
	"path/filepath"
	"testing"
)

// The guide beside the reader is put in the books on the first start only: once removed, it stays removed, and the
// copy beside the reader is never listed.
func TestWelcome(t *testing.T) {
	dir := t.TempDir()
	makeEPUB(t, filepath.Join(dir, guide), `<dc:title>Getting started</dc:title>`)
	inBooks := filepath.Join(dir, "books", guide)

	Welcome(dir)
	books := Scan(dir) // writes library.json
	if len(books) != 1 || books[0].Path != "books/getting-started.epub" || books[0].Title != "Getting started" {
		t.Fatalf("first start: %+v", books)
	}

	os.Remove(inBooks) // removed in the library
	Scan(dir)
	Welcome(dir) // the next start, or one after an upgrade
	if _, err := os.Stat(inBooks); err == nil {
		t.Error("the guide came back after it was removed")
	}
}

// With no guide beside the reader (a development install) the first start does nothing.
func TestWelcomeNoGuide(t *testing.T) {
	dir := t.TempDir()
	Welcome(dir)
	if _, err := os.Stat(filepath.Join(dir, "books")); err == nil {
		t.Error("books/ made with no guide to put in it")
	}
}
