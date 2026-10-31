package main

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"shamsainkreader/ui"
)

// The library's views of the books: a filter (which books), a sort (in what order), by the names in ui.FilterNames and ui.SortNames. The choices are kept in DIR/library-view.json.

// A book with what its place file says: whether it was opened, when it was last read and how much of it.
type shelfBook struct {
	libBook
	opened bool
	when   time.Time
	share  float64 // 0 to 1; -1 when opened but not known
}

// shelfOf reads every book's place file.
func shelfOf(books []libBook, dir string) []shelfBook {
	out := make([]shelfBook, len(books))
	for i, b := range books {
		w, s, ok := placeOf(dir, b.Key)
		out[i] = shelfBook{b, ok, w, s}
	}
	return out
}

// The share from which a book counts as finished.
const finishedShare = 0.98

func (b shelfBook) finished() bool { return b.opened && b.share >= finishedShare }

// in says whether the book is under filter f (an index of ui.FilterNames): Reading is opened and not finished (a
// share not known is reading), New is never opened, Finished is opened and read to the end, All is every book.
func (b shelfBook) in(f int) bool {
	switch f {
	case 0:
		return b.opened && !b.finished()
	case 1:
		return !b.opened
	case 2:
		return b.finished()
	}
	return true
}

// langCode is the primary subtag of a book's language, lower case ("fa-IR" is "fa"); "" if it has none.
func langCode(lang string) string {
	c := strings.ToLower(strings.TrimSpace(strings.SplitN(lang, "-", 2)[0]))
	switch c {
	case "per", "fas":
		return "fa"
	}
	return c
}

// arabicScript says whether s has a letter of the Arabic script (the blocks Persian is written in).
func arabicScript(s string) bool {
	for _, r := range s {
		if r >= 0x600 && r <= 0x6ff || r >= 0xfb50 && r <= 0xfdff || r >= 0xfe70 && r <= 0xfeff {
			return true
		}
	}
	return false
}

// bookLang is the language of a book: its primary subtag, else Farsi when its title has Arabic-script letters, else
// "" (no group).
func bookLang(b libBook) string {
	if c := langCode(b.Lang); c != "" {
		return c
	}
	if arabicScript(b.Title) {
		return "fa"
	}
	return ""
}

// countsOf is how many books each filter has.
func countsOf(books []shelfBook) (n [4]int) {
	for _, b := range books {
		for f := range n {
			if b.in(f) {
				n[f]++
			}
		}
	}
	return n
}

// The sort each filter starts with.
var defaultSorts = [4]string{"recent", "added", "recent", "title"}

// sortBooks puts the books in the order sort (a name of ui.SortNames). Recent is by when the place file was kept,
// newest first, then the books never opened, newest added first; Title and Author ignore case, a book with no author
// last in Author; Added is newest first.
func sortBooks(books []shelfBook, sort string) {
	title := func(a, b shelfBook) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)), strings.Compare(a.Path, b.Path))
	}
	slices.SortStableFunc(books, func(a, b shelfBook) int {
		switch sort {
		case "recent":
			if a.opened != b.opened {
				if a.opened {
					return -1
				}
				return 1
			}
			if a.opened {
				return cmp.Or(b.when.Compare(a.when), title(a, b))
			}
			return cmp.Or(cmp.Compare(b.Added, a.Added), title(a, b))
		case "author":
			if (a.Author == "") != (b.Author == "") {
				if a.Author == "" {
					return 1
				}
				return -1
			}
			return cmp.Or(strings.Compare(strings.ToLower(a.Author), strings.ToLower(b.Author)), title(a, b))
		case "added":
			return cmp.Or(cmp.Compare(b.Added, a.Added), title(a, b))
		}
		return title(a, b)
	})
}

// lastRead is the key of the book read most recently, "" if none was opened.
func lastRead(books []shelfBook) (key string) {
	var when time.Time
	for _, b := range books {
		if b.opened && b.when.After(when) {
			key, when = b.Key, b.when
		}
	}
	return key
}

// The choices as kept: the filter, the sort of each filter.
type libraryView struct {
	Filter string            `json:"filter,omitempty"`
	Sorts  map[string]string `json:"sorts,omitempty"`
}

// loadView reads DIR/library-view.json; ok is false when there is none (or it does not parse).
func loadView(dir string) (v libraryView, ok bool) {
	b, err := os.ReadFile(filepath.Join(dir, "library-view.json"))
	if err != nil || json.Unmarshal(b, &v) != nil {
		return libraryView{}, false
	}
	return v, true
}

func (v libraryView) save(dir string) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "library-view.json"), append(b, '\n'))
}

// sortOf is the sort kept for filter f (an index of ui.FilterNames), its default if none.
func (v libraryView) sortOf(f int) string {
	if s := v.Sorts[ui.FilterNames[f]]; slices.Contains(ui.SortNames, s) {
		return s
	}
	return defaultSorts[f]
}

// setSort keeps sort for filter f.
func (v *libraryView) setSort(f int, sort string) {
	if v.Sorts == nil {
		v.Sorts = map[string]string{}
	}
	v.Sorts[ui.FilterNames[f]] = sort
}

// startFilter is the filter to open the library with: the one kept, but New when that is Reading and no book is being
// read; with none kept Reading if any book is being read, else New. counts are the books'.
func startFilter(v libraryView, counts [4]int) int {
	f := slices.Index(ui.FilterNames, v.Filter)
	if f < 0 {
		f = 0
	}
	if f == 0 && counts[0] == 0 {
		return 1
	}
	return f
}
