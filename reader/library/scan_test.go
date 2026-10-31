package library

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeEPUB writes a small EPUB whose package document has the given metadata lines.
func makeEPUB(t *testing.T, file, metadata string) {
	t.Helper()
	makeEPUBWith(t, file, metadata, "")
}

// makeEPUBWith writes a small EPUB whose package document has the given metadata lines and manifest items.
func makeEPUBWith(t *testing.T, file, metadata, manifest string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	add := func(name, body string) {
		w, _ := z.Create(name)
		w.Write([]byte(body))
	}
	add("mimetype", "application/epub+zip")
	add("META-INF/container.xml", `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)
	add("OEBPS/content.opf", `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/">`+metadata+`</metadata><manifest>`+manifest+`</manifest></package>`)
	z.Close()
	f.Close()
}

func libDir(t *testing.T) string {
	dir := t.TempDir()
	books := filepath.Join(dir, "books")
	makeEPUB(t, filepath.Join(books, "a.epub"), `<dc:title> The Title </dc:title><dc:creator>An Author</dc:creator><dc:creator>Second</dc:creator><dc:language>fa</dc:language>`)
	makeEPUB(t, filepath.Join(books, "haji-agha_ocr.epub"), `<dc:creator>Someone</dc:creator>`)
	os.WriteFile(filepath.Join(books, "broken.epub"), []byte("not a zip"), 0o644)
	makeEPUB(t, filepath.Join(books, "farsiepub", "c.epub"), `<dc:title>In a folder</dc:title>`)
	return dir
}

func byPath(books []Book) map[string]Book {
	m := map[string]Book{}
	for _, b := range books {
		m[b.Path] = b
	}
	return m
}

func TestScanLibrary(t *testing.T) {
	dir := libDir(t)
	reads := 0
	readMeta = func(f string) (meta, error) { reads++; return epubMeta(f) }
	defer func() { readMeta = epubMeta }()

	books := Scan(dir)
	if len(books) != 4 || reads != 4 {
		t.Fatalf("%d books, %d read; want 4, 4", len(books), reads)
	}
	m := byPath(books)
	if b := m["books/a.epub"]; b.Title != "The Title" || b.Author != "An Author" || b.Lang != "fa" || b.Source != "" || b.Added == 0 {
		t.Errorf("a: %+v", b)
	}
	if b := m["books/haji-agha_ocr.epub"]; b.Title != "Haji agha ocr" || b.Author != "Someone" {
		t.Errorf("no title: %+v", b)
	}
	if b := m["books/broken.epub"]; b.Title != "Broken" || b.Author != "" {
		t.Errorf("broken: %+v", b)
	}
	if b := m["books/farsiepub/c.epub"]; b.Title != "In a folder" || b.Source != "farsiepub" {
		t.Errorf("source: %+v", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "library.json")); err != nil {
		t.Fatal(err)
	}

	// the second scan reads no file, and does not write library.json again
	before, _ := os.Stat(filepath.Join(dir, "library.json"))
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "library.json"), old, old)
	reads = 0
	books = Scan(dir)
	if len(books) != 4 || reads != 0 {
		t.Fatalf("second scan: %d books, %d read; want 4, 0", len(books), reads)
	}
	after, _ := os.Stat(filepath.Join(dir, "library.json"))
	if after.ModTime().Unix() != old.Unix() || before.ModTime().Unix() == old.Unix() {
		t.Errorf("library.json written again")
	}

	// a removed book is dropped; a new one is read
	os.Remove(filepath.Join(dir, "books", "a.epub"))
	makeEPUB(t, filepath.Join(dir, "books", "d.epub"), `<dc:title>New</dc:title>`)
	reads = 0
	books = Scan(dir)
	m = byPath(books)
	if _, ok := m["books/a.epub"]; ok || len(books) != 4 || reads != 1 || m["books/d.epub"].Title != "New" {
		t.Errorf("after change: %d books, %d read, %+v", len(books), reads, books)
	}
}

func TestLibraryOrder(t *testing.T) {
	dir := libDir(t)
	now := time.Now()
	// the two never opened: c is newer than haji
	for p, ago := range map[string]time.Duration{"books/haji-agha_ocr.epub": 10 * time.Hour, "books/farsiepub/c.epub": 5 * time.Hour} {
		os.Chtimes(filepath.Join(dir, p), now.Add(-ago), now.Add(-ago))
	}
	books := Scan(dir)
	m := byPath(books)
	place := func(path, body string, ago time.Duration) {
		p := filepath.Join(dir, "places", m[path].Key+".place")
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
		os.Chtimes(p, now.Add(-ago), now.Add(-ago))
	}
	place("books/broken.epub", "3 4 0.5000\n", time.Hour)
	place("books/a.epub", "1 2\n", time.Minute) // the old form
	order := Shelf(books, dir)
	Sort(order, "recent")
	var got []string
	for _, b := range order {
		got = append(got, b.Path)
	}
	want := []string{"books/a.epub", "books/broken.epub", "books/farsiepub/c.epub", "books/haji-agha_ocr.epub"}
	if len(got) != 4 {
		t.Fatalf("order %v", got)
	}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Errorf("order %v; want %v", got, want)
	}
	if p := progress(dir, m["books/broken.epub"]); p != 0.5 {
		t.Errorf("progress %v; want 0.5", p)
	}
	if p := progress(dir, m["books/a.epub"]); p != -1 {
		t.Errorf("old form: %v; want -1", p)
	}
	if p := progress(dir, m["books/farsiepub/c.epub"]); p != 0 {
		t.Errorf("never opened: %v; want 0", p)
	}
}

// A book whose manifest has a media overlay is narrated, and an entry of library.json read before narration was looked
// for is read again once.
func TestNarrated(t *testing.T) {
	dir := t.TempDir()
	makeEPUBWith(t, filepath.Join(dir, "books", "n.epub"), `<dc:title>Told</dc:title>`,
		`<item id="t" href="t.xhtml" media-type="application/xhtml+xml" media-overlay="s"/><item id="s" href="t.smil" media-type="application/smil+xml"/>`)
	makeEPUB(t, filepath.Join(dir, "books", "p.epub"), `<dc:title>Plain</dc:title>`)
	m := byPath(Scan(dir))
	if !m["books/n.epub"].Narrated || m["books/p.epub"].Narrated {
		t.Fatalf("narrated: %+v", m)
	}

	old := m["books/n.epub"]
	old.V, old.Narrated = 0, false
	b, _ := json.Marshal([]Book{old, m["books/p.epub"]})
	os.WriteFile(filepath.Join(dir, "library.json"), b, 0o644)
	reads := 0
	readMeta = func(f string) (meta, error) { reads++; return epubMeta(f) }
	defer func() { readMeta = epubMeta }()
	m = byPath(Scan(dir))
	if reads != 1 || !m["books/n.epub"].Narrated {
		t.Errorf("%d read, %+v; want the old entry read again", reads, m["books/n.epub"])
	}
}
