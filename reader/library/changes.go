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
// A book marked finished or not finished by hand has DIR/places/KEY.Mark: "finished" or "reading", and the share read
// when it was marked ("none" if it had no place). The mark holds while the place says the same; once the book is read
// on (or opened for the first time), its place decides again, as a book never marked.

// markOf is a book's mark: 1 marked finished, -1 marked not finished, 0 none (or no longer holding), and when it was
// marked. opened and share are its place's (Place).
func markOf(dir, key string, opened bool, share float64) (int, time.Time) {
	p := filepath.Join(dir, "places", key+".mark")
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
func SetMark(dir, key string, finished bool) error {
	state := map[bool]string{true: "finished", false: "reading"}[finished]
	at := "none"
	if _, share, ok := Place(dir, key); ok {
		at = strconv.FormatFloat(share, 'g', -1, 64)
	}
	return files.Write(filepath.Join(dir, "places", key+".mark"), []byte(state+" "+at+"\n"))
}

// StartAgain forgets where a book was and its mark: it opens at its start, a new book in the library. Its highlights
// stay.
func StartAgain(dir, key string) error {
	return removeAll(filepath.Join(dir, "places", key+".place"), filepath.Join(dir, "places", key+".mark"))
}

// Remove deletes a book's file and everything kept for it: its place, mark, highlights (and a copy of them that
// did not parse) and cache; the reader's settings for it are the caller's. library.json drops it at the next scan.
func Remove(dir string, b Book) error {
	if err := removeAll(filepath.Join(dir, b.Path)); err != nil {
		return err
	}
	return removeAll(
		filepath.Join(dir, "places", b.Key+".place"),
		filepath.Join(dir, "places", b.Key+".mark"),
		filepath.Join(dir, "highlights", b.Key+".json"),
		filepath.Join(dir, "highlights", b.Key+".json.bad"),
		filepath.Join(dir, "cache", b.Key+".cache"))
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
