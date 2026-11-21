package library

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"shamsainkreader/files"
)

// What a book's menu in the library changes (libraryview.go): whether it is finished, its place, the book itself.
//
// A book marked finished or not finished by hand has DIR/places/ID.mark: "finished" or "reading", and the share read
// when it was marked ("none" if it had no place). The mark holds while the place says the same; once the book is read
// on (or opened for the first time), its place decides again, as a book never marked.

// markOf is a book's mark: 1 marked finished, -1 marked not finished, 0 none (or no longer holding), and when it was
// marked. opened and share are its place's (Place).
func markOf(dir, id string, opened bool, share float64) (int, time.Time) {
	p := filepath.Join(dir, "places", id+".mark")
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, time.Time{}
	}
	m, at := mark(b, opened, share), time.Time{}
	if st, err := os.Stat(p); err == nil && m != 0 {
		at = st.ModTime()
	}
	return m, at
}

// mark is what a mark file b says, as markOf.
func mark(b []byte, opened bool, share float64) int {
	f := strings.Fields(string(b))
	if len(f) != 2 || f[0] != "finished" && f[0] != "reading" {
		return 0
	}
	if f[1] == "none" {
		if opened {
			return 0
		}
	} else if s, err := strconv.ParseFloat(f[1], 64); err != nil || !opened || s != share {
		return 0
	}
	if f[0] == "finished" {
		return 1
	}
	return -1
}

// SetMark marks a book finished, or not finished, as its place is now.
func SetMark(dir, id string, finished bool) error {
	state := map[bool]string{true: "finished", false: "reading"}[finished]
	at := "none"
	if _, share, ok := Place(dir, id); ok {
		at = strconv.FormatFloat(share, 'g', -1, 64)
	}
	return files.Write(filepath.Join(dir, "places", id+".mark"), []byte(state+" "+at+"\n"))
}

// StartAgain forgets where a book was and its mark: it opens at its start, a new book in the library. Its highlights
// stay.
func StartAgain(dir, id string) error {
	return removeAll(filepath.Join(dir, "places", id+".place"), filepath.Join(dir, "places", id+".mark"))
}

// Remove deletes a book's files (every copy) and everything kept for it: its place, mark, highlights (and a copy of
// them that did not parse, and those a new edition no longer had) and cache; the reader's settings for it are the
// caller's. library.json drops it at the next scan.
func Remove(dir string, b Book) error {
	for _, p := range append([]string{b.Path}, b.Copies...) {
		if err := removeAll(filepath.Join(dir, p)); err != nil {
			return err
		}
	}
	return removeAll(
		filepath.Join(dir, "places", b.ID+".place"),
		filepath.Join(dir, "places", b.ID+".mark"),
		filepath.Join(dir, "highlights", b.ID+".json"),
		filepath.Join(dir, "highlights", b.ID+".json.bad"),
		filepath.Join(dir, "highlights", b.ID+".unplaced.json"),
		filepath.Join(dir, "cache", b.Hash+".cache"))
}

// removeAll removes the files, those already gone ignored; the first other error is returned.
func removeAll(paths ...string) error {
	var first error
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) && first == nil {
			first = fmt.Errorf("%s: %w", filepath.Base(p), err)
		}
	}
	return first
}
