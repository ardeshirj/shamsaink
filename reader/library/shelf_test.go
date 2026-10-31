package library

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// books for the tests: a opened and half read, b finished, c opened (share not known), d, e and f never opened.
func testShelf() []Item {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	mk := func(path, title, author, lang string, added int64, opened bool, ago time.Duration, share float64) Item {
		return Item{Book{Path: path, Key: path, Title: title, Author: author, Lang: lang, Added: added}, opened, t0.Add(-ago), share, 0, time.Time{}}
	}
	return []Item{
		mk("a", "banana", "Zed", "fa-IR", 10, true, time.Hour, 0.5),
		mk("b", "Cherry", "alpha", "en", 20, true, time.Minute, 1),
		mk("c", "apple", "", "fa", 30, true, 2*time.Hour, -1),
		mk("d", "Date", "Mid", "", 40, false, 0, 0),
		mk("e", "elder", "", "EN-us", 50, false, 0, 0),
		mk("f", "Fig", "Beta", "per", 5, false, 0, 0),
	}
}

func paths(bs []Item) (p []string) {
	for _, b := range bs {
		p = append(p, b.Path)
	}
	return p
}

func TestFilters(t *testing.T) {
	bs := testShelf()
	if n := Counts(bs); n != [4]int{2, 3, 1, 6} {
		t.Errorf("counts %v", n)
	}
	bs[0].Share = 0.98
	if !bs[0].Finished() || bs[0].In(0) {
		t.Errorf("0.98 is finished")
	}
	bs[0].Share = 0.979
	if bs[0].Finished() || !bs[0].In(0) {
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
		Sort(bs, sort)
		if got := paths(bs); !slices.Equal(got, want) {
			t.Errorf("%s: %v; want %v", sort, got, want)
		}
	}
}

func TestLibraryView(t *testing.T) {
	dir := t.TempDir()
	if _, ok := LoadView(dir); ok {
		t.Fatal("a view from nothing")
	}
	v := View{Filter: "finished"}
	v.SetSort(2, "author")
	if err := v.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadView(dir)
	if !ok || got.Filter != "finished" || got.SortOf(2) != "author" {
		t.Errorf("view %+v", got)
	}
	for f, want := range []string{"recent", "added", "recent", "title"} {
		if s := got.SortOf(f); f != 2 && s != want {
			t.Errorf("default sort of %d: %q", f, s)
		}
	}
	// the start: Reading if any book is being read, else New; a kept filter, but Reading with none is New
	for _, c := range []struct {
		view   View
		counts [4]int
		want   int
	}{
		{View{}, [4]int{2, 3, 1, 6}, 0},
		{View{}, [4]int{0, 3, 1, 4}, 1},
		{View{Filter: "reading"}, [4]int{0, 3, 1, 4}, 1},
		{View{Filter: "all"}, [4]int{0, 3, 1, 4}, 3},
		{View{Filter: "finished"}, [4]int{2, 3, 0, 5}, 2},
		{View{Filter: "nonsense"}, [4]int{2, 3, 0, 5}, 0},
	} {
		if f := StartFilter(c.view, c.counts); f != c.want {
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
		if got := Lang(Book{Title: c.title, Lang: c.lang}); got != c.want {
			t.Errorf("%q %q: %q; want %q", c.title, c.lang, got, c.want)
		}
	}
}

// A view kept by an earlier build, with a language, still loads.
func TestLibraryViewOldLang(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "library-view.json"), []byte(`{"filter": "all", "lang": "fa", "sorts": {"all": "added"}}`), 0o644)
	v, ok := LoadView(dir)
	if !ok || v.Filter != "all" || v.SortOf(3) != "added" {
		t.Errorf("view %+v %v", v, ok)
	}
}
