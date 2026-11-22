package opds

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// epub is the smallest file the download takes for a book: a ZIP with META-INF/container.xml; text tells editions
// apart.
func epub(t *testing.T, text string) []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, body := range map[string]string{"mimetype": "application/epub+zip", "META-INF/container.xml": "<container/>", "book.opf": text} {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	z.Close()
	return b.Bytes()
}

const catalogue = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <id>urn:test</id><title>Test</title>
  <entry>
    <id>urn:test:one</id><title>One</title><updated>2026-10-20T00:00:00Z</updated>
    <link rel="http://opds-spec.org/acquisition/buy" href="/buy/one" type="application/epub+zip"/>
    <link rel="http://opds-spec.org/acquisition/open-access" href="/books/one.epub" type="application/epub+zip"/>
  </entry>
</feed>`

// server is a catalogue at /opds; its book at /books/one.epub is served as books[0] with the name name[0] (headers as
// the test sets them).
type server struct {
	*httptest.Server
	book       []byte
	name       string
	status     int
	contentLen string
}

func newServer(t *testing.T) *server {
	s := &server{status: 200}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/opds":
			w.Header().Set("Content-Type", "application/atom+xml;profile=opds-catalog")
			w.Write([]byte(catalogue))
		case "/books/one.epub":
			if s.name != "" {
				w.Header().Set("Content-Disposition", `attachment; filename="`+s.name+`"`)
			}
			if s.contentLen != "" {
				w.Header().Set("Content-Length", s.contentLen)
			}
			w.WriteHeader(s.status)
			w.Write(s.book)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func TestFetch(t *testing.T) {
	s := newServer(t)
	dir := t.TempDir()
	c := Catalogue{Name: "Test", URL: s.URL + "/opds", Folder: "test"}
	f, err := Get(context.Background(), s.Client(), c.URL)
	if err != nil {
		t.Fatal(err)
	}
	book := &f.Publications[0]
	if l := EPUB(book); l == nil || l.Href.String() != s.URL+"/books/one.epub" {
		t.Fatalf("the open-access EPUB, not the one to buy: %+v", l)
	}
	rec := LoadRecord(dir)

	// the first download: the server's name, in the catalogue's folder, recorded
	s.book, s.name = epub(t, "first"), "Buried Alive.epub"
	got, err := Fetch(context.Background(), s.Client(), dir, c, book, rec, nil)
	if err != nil || got != "books/test/Buried Alive.epub" {
		t.Fatalf("%q %v", got, err)
	}
	if again := LoadRecord(dir); again.Find(dir, c.URL, "urn:test:one") != got {
		t.Errorf("not recorded: %+v", again.Downloads)
	}
	if rec.Newer(c, book) {
		t.Error("the same <updated> is not newer")
	}

	// the entry again, the server naming it otherwise: over the same file (a new edition of the same book)
	s.book, s.name = epub(t, "second"), "buried-alive-2nd.epub"
	if got2, err := Fetch(context.Background(), s.Client(), dir, c, book, rec, nil); err != nil || got2 != got {
		t.Fatalf("%q %v, not over %q", got2, err, got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, got)); !bytes.Equal(b, s.book) {
		t.Error("the file is not the new edition")
	}
	if names := list(t, filepath.Join(dir, "books", "test")); names != "Buried Alive.epub" {
		t.Errorf("the folder: %s", names)
	}

	// its file gone (the reader removed it): a new book, under the server's name
	os.Remove(filepath.Join(dir, got))
	if got3, err := Fetch(context.Background(), s.Client(), dir, c, book, rec, nil); err != nil || got3 != "books/test/buried-alive-2nd.epub" {
		t.Fatalf("%q %v", got3, err)
	}
}

func TestDownloadRefused(t *testing.T) {
	s := newServer(t)
	for _, c := range []struct {
		name       string
		book       []byte
		status     int
		contentLen string
		want       error
	}{
		{"a 206 for the whole file", epub(t, "x"), 206, "", ErrUnreachable},
		{"not found", nil, 404, "", ErrUnreachable},
		{"not an EPUB", []byte("<html>Down for maintenance</html>"), 200, "", ErrNotEPUB},
		{"a ZIP that is not an EPUB", func() []byte {
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			z.Create("a.txt")
			z.Close()
			return b.Bytes()
		}(), 200, "", ErrNotEPUB},
	} {
		dir := t.TempDir()
		s.book, s.status, s.contentLen = c.book, c.status, c.contentLen
		_, err := Download(context.Background(), s.Client(), s.URL+"/books/one.epub", dir, "", nil)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, not %v", c.name, err, c.want)
		}
		if names := list(t, dir); names != "" {
			t.Errorf("%s: left %s", c.name, names)
		}
	}
}

func TestDownloadCut(t *testing.T) {
	book := epub(t, "x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000") // more than is sent: the connection ends early
		w.Write(book)
	}))
	defer srv.Close()
	dir := t.TempDir()
	_, err := Download(context.Background(), srv.Client(), srv.URL+"/b.epub", dir, "", nil)
	if !errors.Is(err, ErrCut) {
		t.Errorf("%v, not cut", err)
	}
	if names := list(t, dir); names != "" {
		t.Errorf("left %s", names)
	}
}

func TestGetRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/opds+json")
			w.Write([]byte(`{"metadata":{}}`))
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html><body>Temporarily Down for Maintenance</body></html>"))
		default:
			w.WriteHeader(503)
		}
	}))
	defer srv.Close()
	for path, want := range map[string]error{"/json": ErrNotFeed, "/html": ErrNotFeed, "/down": ErrUnreachable} {
		if _, err := Get(context.Background(), srv.Client(), srv.URL+path); !errors.Is(err, want) {
			t.Errorf("%s: %v, not %v", path, err, want)
		}
	}
}

func TestSafeName(t *testing.T) {
	for in, want := range map[string]string{
		"Buried Alive.epub":                "Buried Alive.epub",
		"ibsen_peer_gynt.epub":             "ibsen_peer_gynt.epub",
		"../../etc/passwd":                 "passwd.epub",
		`a:b*c?.epub`:                      "a_b_c_.epub",
		".hidden.epub":                     "hidden.epub",
		"holmes.kepub.epub":                "holmes.epub",
		"":                                 "book.epub",
		"newsendbook.php":                  "newsendbook.epub",
		"زنده بگور.epub":                   "زنده بگور.epub",
		strings.Repeat("ب", 100) + ".epub": strings.Repeat("ب", 60) + ".epub",
	} {
		if got := safeName(in); got != want {
			t.Errorf("%q: %q, not %q", in, got, want)
		}
	}
}

func list(t *testing.T, dir string) string {
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return strings.Join(names, " ")
}
