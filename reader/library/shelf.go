package library

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"shamsainkreader/files"
)

// The library's views of the books: a filter (which books), a sort (in what order), by the names in Filters and Sorts
// (in the ui's buttons' ids too). The choices are kept in DIR/library-view.json.
var (
	Filters = []string{"reading", "new", "finished", "all"}
	Sorts   = []string{"recent", "title", "author", "added"}
)

// A book with what its place file says: whether it was opened, when it was last read and how much of it.
type Item struct {
	Book
	Opened bool
	When   time.Time // when its place was last kept
	Share  float64   // 0 to 1; -1 when opened but not known
	Mark   int       // marked by hand (changes.go): 1 finished, -1 not finished, 0 neither
	Marked time.Time // when it was marked
}

// Shelf reads every book's place file.
func Shelf(books []Book, dir string) []Item {
	out := make([]Item, len(books))
	for i, b := range books {
		w, s, ok := Place(dir, b.Key)
		m, at := markOf(dir, b.Key, ok, s)
		out[i] = Item{b, ok, w, s, m, at}
	}
	return out
}

// The share from which a book counts as finished.
const finishedShare = 0.98

// finished: as marked, else read to the end.
func (b Item) Finished() bool {
	if b.Mark != 0 {
		return b.Mark > 0
	}
	return b.Opened && b.Share >= finishedShare
}

// finishedAt is when a finished book was finished: when it was marked so, else when it was last read.
func (b Item) FinishedAt() time.Time {
	if b.Mark > 0 {
		return b.Marked
	}
	return b.When
}

// in says whether the book is under filter f (an index of Filters): Reading is opened and not finished (a
// share not known is reading), New is never opened nor marked finished, Finished is read to the end or marked so, All
// is every book.
func (b Item) In(f int) bool {
	switch f {
	case 0:
		return b.Opened && !b.Finished()
	case 1:
		return !b.Opened && !b.Finished()
	case 2:
		return b.Finished()
	}
	return true
}

// LangCode is the primary subtag of a book's language, lower case ("fa-IR" is "fa"); "" if it has none.
func LangCode(lang string) string {
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

// Lang is the language of a book: its primary subtag, else Farsi when its title has Arabic-script letters, else
// "" (no group).
func Lang(b Book) string {
	if c := LangCode(b.Lang); c != "" {
		return c
	}
	if arabicScript(b.Title) {
		return "fa"
	}
	return ""
}

// Counts is how many books each filter has.
func Counts(books []Item) (n [4]int) {
	for _, b := range books {
		for f := range n {
			if b.In(f) {
				n[f]++
			}
		}
	}
	return n
}

// The sort each filter starts with.
var defaultSorts = [4]string{"recent", "added", "recent", "title"}

// Sort puts the books in the order sort (a name of Sorts). Recent is by when the place file was kept,
// newest first, then the books never opened, newest added first; Title and Author ignore case, a book with no author
// last in Author; Added is newest first.
func Sort(books []Item, sort string) {
	title := func(a, b Item) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)), strings.Compare(a.Path, b.Path))
	}
	slices.SortStableFunc(books, func(a, b Item) int {
		switch sort {
		case "recent":
			if a.Opened != b.Opened {
				if a.Opened {
					return -1
				}
				return 1
			}
			if a.Opened {
				return cmp.Or(b.When.Compare(a.When), title(a, b))
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

// The choices as kept: the filter, the sort of each filter.
type View struct {
	Filter string            `json:"filter,omitempty"`
	Sorts  map[string]string `json:"sorts,omitempty"`
}

// LoadView reads DIR/library-view.json; ok is false when there is none (or it does not parse).
func LoadView(dir string) (v View, ok bool) {
	b, err := os.ReadFile(filepath.Join(dir, "library-view.json"))
	if err != nil || json.Unmarshal(b, &v) != nil {
		return View{}, false
	}
	return v, true
}

func (v View) Save(dir string) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return files.Write(filepath.Join(dir, "library-view.json"), append(b, '\n'))
}

// SortOf is the sort kept for filter f (an index of Filters), its default if none.
func (v View) SortOf(f int) string {
	if s := v.Sorts[Filters[f]]; slices.Contains(Sorts, s) {
		return s
	}
	return defaultSorts[f]
}

// SetSort keeps sort for filter f.
func (v *View) SetSort(f int, sort string) {
	if v.Sorts == nil {
		v.Sorts = map[string]string{}
	}
	v.Sorts[Filters[f]] = sort
}

// StartFilter is the filter to open the library with: the one kept, but New when that is Reading and no book is being
// read; with none kept Reading if any book is being read, else New. counts are the books'.
func StartFilter(v View, counts [4]int) int {
	f := slices.Index(Filters, v.Filter)
	if f < 0 {
		f = 0
	}
	if f == 0 && counts[0] == 0 {
		return 1
	}
	return f
}
