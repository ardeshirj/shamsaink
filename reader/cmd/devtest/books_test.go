package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"shamsainkreader/library"
)

// Every book of e2e/books builds to the same bytes each time (its key, and a fixture's place for it, depend on them),
// as an EPUB: mimetype first and stored, every file there, no README.
func TestBooks(t *testing.T) {
	root(t)
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		a, err := buildBook(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		b, _ := buildBook(e.Name())
		if !bytes.Equal(a, b) {
			t.Errorf("%s: two builds differ", e.Name())
		}
		z, err := zip.NewReader(bytes.NewReader(a), int64(len(a)))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if f := z.File[0]; f.Name != "mimetype" || f.Method != zip.Store {
			t.Errorf("%s: the first file is %s, method %d: not mimetype, stored", e.Name(), f.Name, f.Method)
		}
		for _, f := range z.File {
			if f.Name == "README.md" || f.Name == "tone.sh" {
				t.Errorf("%s: its %s is in the EPUB", e.Name(), f.Name)
			}
		}
		// a fixture named after the book keeps its place under the book's key: an edit to the book changes the key,
		// and the place would be ignored without a word (e2e/fixtures/README.md)
		if _, err := os.Stat(filepath.Join("e2e", "fixtures", e.Name())); err == nil {
			tmp := filepath.Join(t.TempDir(), e.Name()+".epub")
			os.WriteFile(tmp, a, 0o644)
			key, err := library.Key(tmp)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join("e2e", "fixtures", e.Name(), "places", key+".place")); err != nil {
				t.Errorf("%s: its fixture has no places/%s.place, the book's key now: rename the place file", e.Name(), key)
			}
		}
	}
}
