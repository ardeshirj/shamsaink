package opds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"shamsainkreader/library"
	"shamsainkreader/update"
)

// FarsiEPUB's own catalogue, over the network: only with OPDS_LIVE=1 (never on CI). Its first book downloaded twice
// into a scratch library: the second time over the same file, the library's scan then one book.
func TestLiveFarsiEPUB(t *testing.T) {
	if os.Getenv("OPDS_LIVE") == "" {
		t.Skip("OPDS_LIVE=1 to read farsiepub.org")
	}
	dir := t.TempDir()
	f, err := Get(context.Background(), update.Client, FarsiEPUB.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Publications) == 0 {
		t.Fatal("no books in the catalogue")
	}
	book := &f.Publications[0]
	rec := LoadRecord(dir)
	var last int64
	first, err := Fetch(context.Background(), update.Client, dir, FarsiEPUB, book, rec, func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	again, err := Fetch(context.Background(), update.Client, dir, FarsiEPUB, book, rec, nil)
	if err != nil || again != first {
		t.Fatalf("again: %q %v, not over %q", again, err, first)
	}
	books := library.Scan(dir)
	t.Logf("%q (%s): %s, %d bytes; the library: %d book(s), %q from %q", book.Metadata.LocalizedTitle.String(), EntryID(book), first, last, len(books), books[0].Path, books[0].Source)
	if len(books) != 1 || books[0].Path != first || books[0].Source != FarsiEPUB.Folder {
		t.Errorf("the library: %+v", books)
	}
	if _, err := os.Stat(filepath.Join(dir, "downloads.json")); err != nil {
		t.Error(err)
	}
}
