package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// books for the tests: a opened and half read, b finished, c opened (share not known), d, e and f never opened.
func testShelf() []shelfBook {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	mk := func(path, title, author, lang string, added int64, opened bool, ago time.Duration, share float64) shelfBook {
		return shelfBook{libBook{Path: path, Key: path, Title: title, Author: author, Lang: lang, Added: added}, opened, t0.Add(-ago), share, 0, time.Time{}}
	}
	return []shelfBook{
		mk("a", "banana", "Zed", "fa-IR", 10, true, time.Hour, 0.5),
		mk("b", "Cherry", "alpha", "en", 20, true, time.Minute, 1),
		mk("c", "apple", "", "fa", 30, true, 2*time.Hour, -1),
		mk("d", "Date", "Mid", "", 40, false, 0, 0),
		mk("e", "elder", "", "EN-us", 50, false, 0, 0),
		mk("f", "Fig", "Beta", "per", 5, false, 0, 0),
	}
}

func paths(bs []shelfBook) (p []string) {
	for _, b := range bs {
		p = append(p, b.Path)
	}
	return p
}

func TestFilters(t *testing.T) {
	bs := testShelf()
	if n := countsOf(bs); n != [4]int{2, 3, 1, 6} {
		t.Errorf("counts %v", n)
	}
	bs[0].share = 0.98
	if !bs[0].finished() || bs[0].in(0) {
		t.Errorf("0.98 is finished")
	}
	bs[0].share = 0.979
	if bs[0].finished() || !bs[0].in(0) {
		t.Errorf("0.979 is reading")
	}
}

func TestSorts(t *testing.T) {
	for sort, want := range map[string][]string{
		"recent": {"b", "a", "c", "e", "d", "f"}, // read newest first, then added newest first
		"title":  {"c", "a", "b", "d", "e", "f"},
		"author": {"b", "f", "d", "a", "c", "e"}, // alpha, Beta, Mid, Zed; no author last, by title
		"added":  {"e", "d", "c", "b", "a", "f"},
	} {
		bs := testShelf()
		sortBooks(bs, sort)
		if got := paths(bs); !slices.Equal(got, want) {
			t.Errorf("%s: %v; want %v", sort, got, want)
		}
	}
}

func TestLibraryView(t *testing.T) {
	dir := t.TempDir()
	if _, ok := loadView(dir); ok {
		t.Fatal("a view from nothing")
	}
	v := libraryView{Filter: "finished"}
	v.setSort(2, "author")
	if err := v.save(dir); err != nil {
		t.Fatal(err)
	}
	got, ok := loadView(dir)
	if !ok || got.Filter != "finished" || got.sortOf(2) != "author" {
		t.Errorf("view %+v", got)
	}
	for f, want := range []string{"recent", "added", "recent", "title"} {
		if s := got.sortOf(f); f != 2 && s != want {
			t.Errorf("default sort of %d: %q", f, s)
		}
	}
	// the start: Reading if any book is being read, else New; a kept filter, but Reading with none is New
	for _, c := range []struct {
		view   libraryView
		counts [4]int
		want   int
	}{
		{libraryView{}, [4]int{2, 3, 1, 6}, 0},
		{libraryView{}, [4]int{0, 3, 1, 4}, 1},
		{libraryView{Filter: "reading"}, [4]int{0, 3, 1, 4}, 1},
		{libraryView{Filter: "all"}, [4]int{0, 3, 1, 4}, 3},
		{libraryView{Filter: "finished"}, [4]int{2, 3, 0, 5}, 2},
		{libraryView{Filter: "nonsense"}, [4]int{2, 3, 0, 5}, 0},
	} {
		if f := startFilter(c.view, c.counts); f != c.want {
			t.Errorf("start %+v %v: %d; want %d", c.view, c.counts, f, c.want)
		}
	}
}

func TestLanguageFallback(t *testing.T) {
	for _, c := range []struct{ title, lang, want string }{
		{"رود کم‌آب", "", "fa"},
		{"Title ﻣﺜﺎﻝ", "", "fa"}, // presentation forms
		{"Buried Alive", "", ""},
		{"رود", "en", "en"}, // the EPUB's own language wins
		{"x", "fa-IR", "fa"},
	} {
		if got := bookLang(libBook{Title: c.title, Lang: c.lang}); got != c.want {
			t.Errorf("%q %q: %q; want %q", c.title, c.lang, got, c.want)
		}
	}
}

// A view kept by an earlier build, with a language, still loads.
func TestLibraryViewOldLang(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "library-view.json"), []byte(`{"filter": "all", "lang": "fa", "sorts": {"all": "added"}}`), 0o644)
	v, ok := loadView(dir)
	if !ok || v.Filter != "all" || v.sortOf(3) != "added" {
		t.Errorf("view %+v %v", v, ok)
	}
}
