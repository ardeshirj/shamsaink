package opds

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestKept(t *testing.T) {
	s := newServer(t)
	dir := t.TempDir()
	c := Catalogue{Name: "Test", URL: s.URL + "/opds", Folder: "test"}
	if Kept(dir, c) != nil {
		t.Fatal("a page kept before any was read")
	}
	_, p, err := GetPage(context.Background(), s.Client(), c.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := Keep(dir, c, p); err != nil {
		t.Fatal(err)
	}
	f, err := Kept(dir, c).Feed()
	if err != nil {
		t.Fatal(err)
	}
	if l := EPUB(&f.Publications[0]); l == nil || l.Href.String() != s.URL+"/books/one.epub" {
		t.Errorf("the kept page's links not resolved against where it came from: %+v", l)
	}
}

func TestCatalogues(t *testing.T) {
	dir := t.TempDir()
	if cs := Catalogues(dir); len(cs) != 1 || cs[0] != FarsiEPUB {
		t.Fatalf("with no file: %+v", cs)
	}
	os.WriteFile(filepath.Join(dir, "catalogues.json"), []byte(`[
		{"name": "Standard Ebooks", "url": "https://standardebooks.org/feeds/opds"},
		{"name": "Again", "url": "https://farsiepub.org/opds/catalog.xml"},
		{"name": "No address"},
		{"name": "A file", "url": "file:///etc/passwd"},
		{"name": "FarsiEPUB", "url": "https://example.org/opds"},
		{"name": "Mine", "url": "http://192.168.1.2/opds", "folder": "../home"}
	]`), 0o644)
	got := Catalogues(dir)
	want := []Catalogue{FarsiEPUB,
		{Name: "Standard Ebooks", URL: "https://standardebooks.org/feeds/opds", Folder: "standard-ebooks"},
		{Name: "FarsiEPUB", URL: "https://example.org/opds", Folder: "farsiepub-2"},
		{Name: "Mine", URL: "http://192.168.1.2/opds", Folder: "home"}}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %+v, not %+v", i, got[i], want[i])
		}
	}
	os.WriteFile(filepath.Join(dir, "catalogues.json"), []byte(`{not json`), 0o644)
	if cs := Catalogues(dir); len(cs) != 1 {
		t.Errorf("a file that cannot be read: %+v", cs)
	}
}
