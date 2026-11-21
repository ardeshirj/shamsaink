package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"shamsainkreader/files"
)

// keep writes a place for book id as the reader does: "DOC AT SHARE", the file's hash, its path.
func keep(t *testing.T, dir, id, hash, rel string) {
	t.Helper()
	if err := files.Write(filepath.Join(dir, "places", id+".place"), []byte("2 3 0.4000\n"+hash+"\n"+rel+"\n")); err != nil {
		t.Fatal(err)
	}
}

func move(t *testing.T, dir, from, to string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(filepath.Join(dir, to)), 0o755)
	if err := os.Rename(filepath.Join(dir, from), filepath.Join(dir, to)); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, dir, from, to string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, from))
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(filepath.Join(dir, to)), 0o755)
	os.WriteFile(filepath.Join(dir, to), b, 0o644)
}

// Each rule of match, one scan after another: a book's ID stays through a move, a new edition, a copy; a new book's
// ID is its hash.
func TestBookIDs(t *testing.T) {
	dir := t.TempDir()
	makeEPUB(t, filepath.Join(dir, "books", "a.epub"), `<dc:title>A</dc:title>`)
	makeEPUB(t, filepath.Join(dir, "books", "b.epub"), `<dc:title>B</dc:title>`)
	m := byPath(Scan(dir))
	a := m["books/a.epub"]
	if a.ID == "" || a.ID != a.Hash || m["books/b.epub"].ID == a.ID {
		t.Fatalf("new books: %+v", m)
	}

	// moved into a folder: the same book, its source the folder's
	move(t, dir, "books/a.epub", "books/farsiepub/a2.epub")
	m = byPath(Scan(dir))
	if b := m["books/farsiepub/a2.epub"]; b.ID != a.ID || b.Source != "farsiepub" || len(m) != 2 {
		t.Fatalf("moved: %+v", m)
	}

	// an identical copy: one book, two files, its first by path opened
	copyFile(t, dir, "books/farsiepub/a2.epub", "books/a-copy.epub")
	books := Scan(dir)
	m = byPath(books)
	if b := m["books/a-copy.epub"]; len(books) != 2 || b.ID != a.ID || len(b.Copies) != 1 || b.Copies[0] != "books/farsiepub/a2.epub" {
		t.Fatalf("copy: %+v", books)
	}

	// a new edition of b, at its path: the same book, read again
	b := m["books/b.epub"]
	makeEPUB(t, filepath.Join(dir, "books", "b.epub"), `<dc:title>B, second edition</dc:title>`)
	m = byPath(Scan(dir))
	if nb := m["books/b.epub"]; nb.ID != b.ID || nb.Hash == b.Hash || nb.Title != "B, second edition" {
		t.Fatalf("new edition: %+v, was %+v", nb, b)
	}

	// one copy changed: it is a copy no more, a new book; the other keeps the book
	makeEPUB(t, filepath.Join(dir, "books", "a-copy.epub"), `<dc:title>A, changed</dc:title>`)
	m = byPath(Scan(dir))
	if len(m) != 3 || m["books/farsiepub/a2.epub"].ID != a.ID || m["books/a-copy.epub"].ID == a.ID || len(m["books/farsiepub/a2.epub"].Copies) != 0 {
		t.Fatalf("a copy changed: %+v", m)
	}

	// every file gone: dropped
	os.Remove(filepath.Join(dir, "books", "a-copy.epub"))
	os.Remove(filepath.Join(dir, "books", "b.epub"))
	if m = byPath(Scan(dir)); len(m) != 1 {
		t.Fatalf("removed: %+v", m)
	}
}

// Two identical files found in one scan are one book.
func TestCopiesFoundTogether(t *testing.T) {
	dir := t.TempDir()
	makeEPUB(t, filepath.Join(dir, "books", "x.epub"), `<dc:title>X</dc:title>`)
	copyFile(t, dir, "books/x.epub", "books/more/x.epub")
	books := Scan(dir)
	if len(books) != 1 || books[0].Path != "books/more/x.epub" || len(books[0].Copies) != 1 {
		t.Fatalf("%+v", books)
	}
}

// library.json lost: each book opened finds its ID again through its place file, even moved or changed since it was
// first seen; a book never opened gets its hash as ID again.
func TestLibraryMadeAgain(t *testing.T) {
	dir := t.TempDir()
	makeEPUB(t, filepath.Join(dir, "books", "a.epub"), `<dc:title>A</dc:title>`)
	makeEPUB(t, filepath.Join(dir, "books", "n.epub"), `<dc:title>Never opened</dc:title>`)
	m := byPath(Scan(dir))
	a, n := m["books/a.epub"], m["books/n.epub"]
	// a new edition, read, then moved: its ID is no longer its hash
	makeEPUB(t, filepath.Join(dir, "books", "a.epub"), `<dc:title>A, second edition</dc:title>`)
	a2 := byPath(Scan(dir))["books/a.epub"]
	keep(t, dir, a2.ID, a2.Hash, a2.Path)
	move(t, dir, "books/a.epub", "books/moved.epub")

	os.Remove(filepath.Join(dir, "library.json"))
	m = byPath(Scan(dir))
	if m["books/moved.epub"].ID != a.ID || m["books/n.epub"].ID != n.ID {
		t.Fatalf("made again: %+v; want %s and %s", m, a.ID, n.ID)
	}
}

// library.json before IDs: each key becomes the ID and hash, nothing read again; two entries with one key (identical
// copies) are one book.
func TestLibraryBeforeIDs(t *testing.T) {
	dir := t.TempDir()
	makeEPUB(t, filepath.Join(dir, "books", "a.epub"), `<dc:title>A</dc:title>`)
	copyFile(t, dir, "books/a.epub", "books/f/a.epub")
	h, _ := Hash(filepath.Join(dir, "books", "a.epub"))
	old := []map[string]any{
		{"path": "books/a.epub", "key": h, "title": "A", "added": 5, "v": 1},
		{"path": "books/f/a.epub", "key": h, "title": "A", "added": 6, "v": 1},
	}
	b, _ := json.Marshal(old)
	os.WriteFile(filepath.Join(dir, "library.json"), b, 0o644)
	reads := 0
	readMeta = func(f string) (meta, error) { reads++; return epubMeta(f) }
	defer func() { readMeta = epubMeta }()
	books := Scan(dir)
	if reads != 0 || len(books) != 1 || books[0].ID != h || books[0].Hash != h || books[0].Key != "" || len(books[0].Copies) != 1 {
		t.Fatalf("%d read, %+v", reads, books)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "library.json")); json.Valid(raw) && string(raw) != "" {
		var kept []map[string]any
		json.Unmarshal(raw, &kept)
		if _, ok := kept[0]["key"]; ok {
			t.Errorf("key written again: %s", raw)
		}
	}
}

// A new book whose hash is the ID of another (a book first seen with these bytes, changed since, and its first
// edition added again) gets an ID of its own.
func TestFreeID(t *testing.T) {
	dir := t.TempDir()
	makeEPUB(t, filepath.Join(dir, "books", "a.epub"), `<dc:title>A</dc:title>`)
	first, _ := os.ReadFile(filepath.Join(dir, "books", "a.epub"))
	a := Scan(dir)[0]
	makeEPUB(t, filepath.Join(dir, "books", "a.epub"), `<dc:title>A, second edition</dc:title>`)
	Scan(dir)
	os.WriteFile(filepath.Join(dir, "books", "first.epub"), first, 0o644)
	m := byPath(Scan(dir))
	if m["books/a.epub"].ID != a.ID || m["books/first.epub"].ID != a.ID+"-2" {
		t.Fatalf("%+v", m)
	}
	// a place file kept under the hash for another file is not taken either
	if id := FreeID(dir, "abc", func(string) bool { return false }); id != "abc" {
		t.Errorf("free: %s", id)
	}
	keep(t, dir, "abc", "other", "books/o.epub")
	if id := FreeID(dir, "abc", func(string) bool { return false }); id != "abc-2" {
		t.Errorf("a place for another file: %s", id)
	}
}

// Find gives the ID of the book a file is, scanning when library.json does not have it yet; outside the books, the
// hash.
func TestFind(t *testing.T) {
	dir := t.TempDir()
	makeEPUB(t, filepath.Join(dir, "books", "a.epub"), `<dc:title>A</dc:title>`)
	a := Scan(dir)[0]
	move(t, dir, "books/a.epub", "books/b.epub")
	id, hash, rel, err := Find(dir, filepath.Join(dir, "books", "b.epub"))
	if err != nil || id != a.ID || hash != a.Hash || rel != "books/b.epub" {
		t.Fatalf("moved, not scanned: %s %s %s %v", id, hash, rel, err)
	}
	out := filepath.Join(t.TempDir(), "o.epub")
	makeEPUB(t, out, `<dc:title>O</dc:title>`)
	id, hash, rel, err = Find(dir, out)
	if err != nil || id != hash || rel != "" {
		t.Fatalf("outside: %s %s %q %v", id, hash, rel, err)
	}
}
