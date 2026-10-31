package main

import (
	"archive/zip"
	"cmp"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// A book in the library: its file, what its metadata says, and how far it has been read.
type libBook struct {
	Path   string `json:"path"` // relative to DIR
	Key    string `json:"key"`  // as openBook's: base name, size, modification time
	Title  string `json:"title"`
	Author string `json:"author"`
	Lang   string `json:"lang"`
	Source string `json:"source"` // its folder under books/ ("farsiepub"), "" when directly in books/
	Added  int64  `json:"added"`  // the file's modification time, Unix seconds
}

// bookKey is a book's key from its file's name, size and modification time, as openBook makes it.
func bookKey(name string, size int64, mod time.Time) string {
	return fmt.Sprintf("%s-%d-%d", name, size, mod.Unix())
}

// readMeta reads an EPUB's metadata; a variable so a test can count the books read.
var readMeta = epubMeta

// scanLibrary lists the EPUBs in DIR/books/*.epub and DIR/books/*/*.epub with their metadata: from DIR/library.json
// when the book's key is there, else read from the EPUB. Books whose file is gone are dropped, and library.json is
// written again only if anything changed.
func scanLibrary(dir string) []libBook {
	t := time.Now()
	var old []libBook
	if b, err := os.ReadFile(filepath.Join(dir, "library.json")); err == nil {
		if err := json.Unmarshal(b, &old); err != nil {
			fmt.Printf("library: library.json does not parse (%v); read again\n", err)
			old = nil
		}
	}
	known := map[string]libBook{}
	for _, b := range old {
		known[b.Path+"\x00"+b.Key] = b
	}
	files, _ := filepath.Glob(filepath.Join(dir, "books", "*.epub"))
	more, _ := filepath.Glob(filepath.Join(dir, "books", "*", "*.epub"))
	files = append(files, more...)
	slices.Sort(files)

	var books []libBook
	read := 0
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil || st.IsDir() {
			continue
		}
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		key := bookKey(filepath.Base(f), st.Size(), st.ModTime())
		if b, ok := known[rel+"\x00"+key]; ok {
			books = append(books, b)
			continue
		}
		b := libBook{Path: rel, Key: key, Added: st.ModTime().Unix()}
		if parts := strings.Split(rel, "/"); len(parts) == 3 {
			b.Source = parts[1]
		}
		read++
		m, err := readMeta(f)
		if err != nil {
			fmt.Printf("library: %s: %v\n", rel, err)
		}
		b.Title, b.Author, b.Lang = strings.TrimSpace(m.Title), strings.TrimSpace(m.Author), strings.TrimSpace(m.Lang)
		if b.Title == "" {
			b.Title = tidyName(filepath.Base(f))
		}
		books = append(books, b)
	}
	if read > 0 || len(books) != len(old) {
		if b, err := json.MarshalIndent(books, "", "  "); err == nil {
			if err := writeFile(filepath.Join(dir, "library.json"), append(b, '\n')); err != nil {
				fmt.Printf("library: library.json not written: %v\n", err)
			}
		}
	}
	fmt.Printf("library: %d books, %d read from their files, %.0f ms\n", len(books), read, float64(time.Since(t).Microseconds())/1000)
	return books
}

// tidyName makes a title from a file's name: no extension, "-" and "_" as spaces, the first letter upper case.
func tidyName(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	c, n := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(c)) + name[n:]
}

// meta is what an EPUB's package document says of the book.
type meta struct{ Title, Author, Lang string }

const dcNS = "http://purl.org/dc/elements/1.1/"

// epubMeta reads the first title, creator and language of the EPUB's package document, with the standard library
// only (go-toolkit is heavier). On an error it returns what it found, if anything, with the error.
func epubMeta(file string) (m meta, err error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return m, err
	}
	defer z.Close()
	open := func(name string) ([]byte, error) {
		for _, f := range z.File {
			if f.Name == name {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(io.LimitReader(rc, 8<<20))
			}
		}
		return nil, fmt.Errorf("%s not in the EPUB", name)
	}
	c, err := open("META-INF/container.xml")
	if err != nil {
		return m, err
	}
	var root string
	d := xml.NewDecoder(strings.NewReader(string(c)))
	d.Strict = false
	for root == "" {
		tok, err := d.Token()
		if err != nil {
			return m, fmt.Errorf("container.xml: no rootfile")
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "rootfile" {
			for _, a := range se.Attr {
				if a.Name.Local == "full-path" {
					root = a.Value
				}
			}
		}
	}
	pkg, err := open(path.Clean(root))
	if err != nil {
		return m, err
	}
	d = xml.NewDecoder(strings.NewReader(string(pkg)))
	d.Strict = false
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return m, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Space != dcNS {
			continue
		}
		var dst *string
		switch se.Name.Local {
		case "title":
			dst = &m.Title
		case "creator":
			dst = &m.Author
		case "language":
			dst = &m.Lang
		default:
			continue
		}
		var s string
		if err := d.DecodeElement(&s, &se); err != nil {
			return m, err
		}
		if *dst == "" {
			*dst = strings.TrimSpace(s)
		}
		if m.Title != "" && m.Author != "" && m.Lang != "" {
			return m, nil
		}
	}
}

// placeOf reads a book's place file: when it was last kept, and the share of the book read (0 to 1, -1 if the old
// form without one); ok false when the book was never opened.
func placeOf(dir, key string) (when time.Time, share float64, ok bool) {
	p := filepath.Join(dir, "places", key+".place")
	st, err := os.Stat(p)
	if err != nil {
		return time.Time{}, 0, false
	}
	share = -1
	if b, err := os.ReadFile(p); err == nil {
		var doc, at int
		var s float64
		if n, _ := fmt.Sscan(string(b), &doc, &at, &s); n == 3 {
			share = min(max(s, 0), 1)
		}
	}
	return st.ModTime(), share, true
}

// progress is how much of the book is read, 0 to 1: 0 if never opened, -1 if opened but not known (a place kept
// before the share was).
func progress(dir string, b libBook) float64 {
	if _, share, ok := placeOf(dir, b.Key); ok {
		return share
	}
	return 0
}

// libraryOrder is the books in the order to show: the one read most recently, the others read (most recent first),
// then those never opened, newest added first. A book is read when its place file exists, and when is the file's
// time.
func libraryOrder(books []libBook, dir string) []libBook {
	type entry struct {
		b    libBook
		when time.Time
		read bool
	}
	es := make([]entry, len(books))
	for i, b := range books {
		w, _, ok := placeOf(dir, b.Key)
		es[i] = entry{b, w, ok}
	}
	slices.SortStableFunc(es, func(a, b entry) int {
		if a.read != b.read {
			if a.read {
				return -1
			}
			return 1
		}
		if a.read {
			return b.when.Compare(a.when)
		}
		return cmp.Compare(b.b.Added, a.b.Added)
	})
	out := make([]libBook, len(es))
	for i, e := range es {
		out[i] = e.b
	}
	return out
}
