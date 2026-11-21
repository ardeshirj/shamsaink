package library

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// How a file found is matched to a book of the library, so that what is kept for a book (by its ID) follows it
// (docs/DESIGN.md, "The library's books: an ID each"). The path and the hash are only evidence: a book's ID is its
// hash when first seen and never changes after.

// A file found by a scan: its path relative to DIR, its hash, its modification time (Unix seconds).
type file struct {
	rel, hash string
	mod       int64
}

// match makes the library's books from the books kept (library.json's, old) and the files found (by path), the rules
// in this order:
//  1. a path a book has, the same hash: unchanged;
//  2. a path a book has, a new hash: a new edition, the same book (unless another of its files is unchanged: then the
//     changed one is a copy no more, and goes on to 3);
//  3. a new path, the hash of a book: its file moved or renamed if it has none left, else an identical copy, one book;
//     the hash of a place file no book holds (places/ID.place, its second line): the book it was kept for, back
//     (moved while library.json was lost, or library.json made again);
//  4. else a new book, its ID its hash (FreeID).
//
// fresh are the IDs whose metadata is to be read (new, or a new edition). The books are in the order of their paths.
func match(dir string, old []Book, found []file) (books []Book, fresh map[string]bool) {
	fresh = map[string]bool{}
	byID := map[string]*Book{}
	byPath := map[string]*Book{}
	var all []*Book
	for _, o := range old {
		if o.ID == "" { // library.json before IDs: the key was both
			o.ID, o.Hash = o.Key, o.Key
		}
		o.Key = ""
		if o.ID == "" {
			continue
		}
		b := byID[o.ID]
		if b == nil { // two entries with one key (identical copies, before IDs) are one book
			c := o // its Path and Copies made again at the end
			b = &c
			byID[o.ID] = b
			all = append(all, b)
		}
		for _, p := range append([]string{o.Path}, o.Copies...) {
			byPath[p] = b
		}
	}

	paths := map[*Book][]string{} // each book's files found
	var unplaced, changed []file
	for _, f := range found { // 1
		switch b := byPath[f.rel]; {
		case b == nil:
			unplaced = append(unplaced, f)
		case f.hash == b.Hash:
			paths[b] = append(paths[b], f.rel)
		default:
			changed = append(changed, f)
		}
	}
	for _, f := range changed { // 2
		if b := byPath[f.rel]; len(paths[b]) == 0 {
			fmt.Printf("library: %s: a new edition of its book\n", f.rel)
			b.Hash = f.hash
			paths[b] = append(paths[b], f.rel)
			fresh[b.ID] = true
		} else {
			unplaced = append(unplaced, f)
		}
	}
	slices.SortFunc(unplaced, func(a, b file) int { return strings.Compare(a.rel, b.rel) })

	byHash := map[string]*Book{}
	for _, b := range all {
		if _, ok := byHash[b.Hash]; !ok {
			byHash[b.Hash] = b
		}
	}
	var left map[string]string // places no book holds: hash → ID, read once, only if needed
	for _, f := range unplaced {
		if b := byHash[f.hash]; b != nil { // 3
			if len(paths[b]) == 0 {
				fmt.Printf("library: %s: moved (was %s)\n", f.rel, b.Path)
			} else {
				fmt.Printf("library: %s: a copy of %s\n", f.rel, paths[b][0])
			}
			paths[b] = append(paths[b], f.rel)
			continue
		}
		if left == nil {
			left = leftPlaces(dir, byID)
		}
		id := left[f.hash]
		if id != "" {
			fmt.Printf("library: %s: the book %s, by its place\n", f.rel, id)
		} else { // 4
			id = FreeID(dir, f.hash, func(id string) bool { return byID[id] != nil })
		}
		delete(left, f.hash)
		b := &Book{ID: id, Hash: f.hash, Added: f.mod}
		byID[id], byHash[f.hash] = b, b
		all = append(all, b)
		paths[b] = []string{f.rel}
		fresh[id] = true
	}

	for _, b := range all {
		p := paths[b]
		if len(p) == 0 {
			continue // its files gone: dropped (what is kept for it stays, for rule 3)
		}
		slices.Sort(p)
		b.Path, b.Copies, b.Source = p[0], p[1:], ""
		if len(b.Copies) == 0 {
			b.Copies = nil
		}
		if parts := strings.Split(b.Path, "/"); len(parts) == 3 {
			b.Source = parts[1]
		}
		books = append(books, *b)
	}
	slices.SortFunc(books, func(a, b Book) int { return strings.Compare(a.Path, b.Path) })
	return books, fresh
}

// FreeID is the ID for a new book of hash: the hash itself, unless a book has it (taken) or a place file kept under
// it is another file's; then the hash with "-2", "-3", ...
func FreeID(dir, hash string, taken func(id string) bool) string {
	for n := 1; ; n++ {
		id := hash
		if n > 1 {
			id = fmt.Sprintf("%s-%d", hash, n)
		}
		if taken(id) {
			continue
		}
		if h := PlaceHash(dir, id); h == "" || h == hash {
			return id
		}
	}
}

// leftPlaces are the place files no book of byID holds and that say their file's hash: hash → ID (the latest kept
// when two say one hash).
func leftPlaces(dir string, byID map[string]*Book) map[string]string {
	out := map[string]string{}
	names, _ := filepath.Glob(filepath.Join(dir, "places", "*.place"))
	newest := map[string]int64{}
	for _, n := range names {
		id := strings.TrimSuffix(filepath.Base(n), ".place")
		if byID[id] != nil {
			continue
		}
		h := PlaceHash(dir, id)
		st, err := os.Stat(n)
		if h == "" || err != nil {
			continue
		}
		if t := st.ModTime().UnixNano(); out[h] == "" || t > newest[h] {
			out[h], newest[h] = id, t
		}
	}
	return out
}

// PlaceHash is the hash of the file a book's place was kept for (its place file's second line), "" if none says one
// (no place, or kept before 2026-11-21: then by the hash that is its ID).
func PlaceHash(dir, id string) string {
	f, err := os.Open(filepath.Join(dir, "places", id+".place"))
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	if s.Scan() && s.Scan() {
		return strings.TrimSpace(s.Text())
	}
	return ""
}

// Find is the ID and hash of the book in the file at path: the library's book with that file, as library.json has it
// or once scanned (a file new, changed or moved since); a file outside DIR/books is no book of the library, its ID
// its hash. rel is its path relative to DIR ("" outside it).
func Find(dir, path string) (id, hash, rel string, err error) {
	if hash, err = Hash(path); err != nil {
		return "", "", "", err
	}
	adir, err1 := filepath.Abs(dir)
	apath, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return hash, hash, "", nil
	}
	r, err := filepath.Rel(adir, apath)
	if err != nil || !strings.HasPrefix(filepath.ToSlash(r), "books/") {
		return hash, hash, "", nil
	}
	rel = filepath.ToSlash(r)
	in := func(books []Book) string {
		for _, b := range books {
			if b.Hash == hash && (b.Path == rel || slices.Contains(b.Copies, rel)) {
				return b.ID
			}
		}
		return ""
	}
	var kept []Book
	if b, err := os.ReadFile(filepath.Join(dir, "library.json")); err == nil {
		json.Unmarshal(b, &kept)
	}
	if id := in(kept); id != "" {
		return id, hash, rel, nil
	}
	if id := in(Scan(dir)); id != "" {
		return id, hash, rel, nil
	}
	return hash, hash, rel, nil
}
