package library

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
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

	"shamsainkreader/files"
)

// A book in the library: its file, what its metadata says, and how far it has been read.
type Book struct {
	Path     string `json:"path"` // relative to DIR
	Key      string `json:"key"`  // as openBook's: see Key
	Title    string `json:"title"`
	Author   string `json:"author"`
	Lang     string `json:"lang"`
	Source   string `json:"source"`             // its folder under books/ ("farsiepub"), "" when directly in books/
	Added    int64  `json:"added"`              // the file's modification time, Unix seconds
	Narrated bool   `json:"narrated,omitempty"` // it has media overlays (SMIL in its manifest)
	V        int    `json:"v,omitempty"`        // metaVersion when read; older entries are read again
}

// metaVersion is what an entry of library.json was read with: 1 since a book's narration is read too.
const metaVersion = 1

// keySample is how much of a book's start and end its key reads.
const keySample = 64 << 10

// Key is a book's key, 12 hex digits of a SHA-256 over its size, its first 64 KB and its last 64 KB (the whole file
// when it is under 128 KB), as openBook makes it. Not the name or the modification time: the Kobo's storage is FAT,
// whose times the kernel turns into UTC with a timezone it may not have been given (every book's time was 8 hours
// earlier after a reboot on 2026-11-05, and every book was new), and a renamed or moved book keeps its place. An EPUB
// is a ZIP file whose central directory, at the end, lists the CRC of every file in it, so a changed book changes its
// last 64 KB. To make it in a shell (docs/devices/kobo/README.md): { echo SIZE; head -c 65536 F; tail -c 65536 F; }
// | sha256sum | cut -c1-12 (a small file: cat F instead of head and tail).
func Key(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%d\n", st.Size())
	if st.Size() <= 2*keySample {
		_, err = io.Copy(h, f)
	} else if _, err = io.CopyN(h, f, keySample); err == nil {
		if _, err = f.Seek(-keySample, io.SeekEnd); err == nil {
			_, err = io.CopyN(h, f, keySample)
		}
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

// readMeta reads an EPUB's metadata; a variable so a test can count the books read.
var readMeta = epubMeta

// Scan lists the EPUBs in DIR/books/*.epub and DIR/books/*/*.epub (hidden ones left out: see hidden) with their metadata: from DIR/library.json
// when the book's key is there, else read from the EPUB. Books whose file is gone are dropped, and library.json is
// written again only if anything changed.
func Scan(dir string) []Book {
	t := time.Now()
	var old []Book
	if b, err := os.ReadFile(filepath.Join(dir, "library.json")); err == nil {
		if err := json.Unmarshal(b, &old); err != nil {
			fmt.Printf("library: library.json does not parse (%v); read again\n", err)
			old = nil
		}
	}
	known := map[string]Book{}
	for _, b := range old {
		known[b.Path+"\x00"+b.Key] = b
	}
	epubs, _ := filepath.Glob(filepath.Join(dir, "books", "*.epub"))
	more, _ := filepath.Glob(filepath.Join(dir, "books", "*", "*.epub"))
	epubs = append(epubs, more...)
	slices.Sort(epubs)

	var books []Book
	read := 0
	for _, f := range epubs {
		st, err := os.Stat(f)
		if err != nil || st.IsDir() {
			continue
		}
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if hidden(rel) {
			continue
		}
		key, err := Key(f)
		if err != nil {
			fmt.Printf("library: %s: %v\n", rel, err)
			continue
		}
		if b, ok := known[rel+"\x00"+key]; ok && b.V == metaVersion {
			books = append(books, b)
			continue
		}
		b := Book{Path: rel, Key: key, Added: st.ModTime().Unix(), V: metaVersion}
		if parts := strings.Split(rel, "/"); len(parts) == 3 {
			b.Source = parts[1]
		}
		read++
		m, err := readMeta(f)
		if err != nil {
			fmt.Printf("library: %s: %v\n", rel, err)
		}
		b.Title, b.Author, b.Lang = strings.TrimSpace(m.Title), strings.TrimSpace(m.Author), strings.TrimSpace(m.Lang)
		b.Narrated = m.Narrated
		if b.Title == "" {
			b.Title = tidyName(filepath.Base(f))
		}
		books = append(books, b)
	}
	if read > 0 || len(books) != len(old) {
		if b, err := json.MarshalIndent(books, "", "  "); err == nil {
			if err := files.Write(filepath.Join(dir, "library.json"), append(b, '\n')); err != nil {
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

// meta is what an EPUB's package document says of the book: Narrated when its manifest has a media overlay (SMIL).
type meta struct {
	Title, Author, Lang string
	Narrated            bool
}

const dcNS = "http://purl.org/dc/elements/1.1/"

// epubMeta reads the first title, creator and language of the EPUB's package document, and whether its manifest has a
// media overlay, with the standard library
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
		if ee, ok := tok.(xml.EndElement); ok && ee.Name.Local == "manifest" {
			return m, nil // the metadata comes before it
		}
		se, ok := tok.(xml.StartElement)
		if ok && se.Name.Local == "item" {
			for _, a := range se.Attr {
				if a.Name.Local == "media-type" && a.Value == "application/smil+xml" {
					m.Narrated = true
				}
			}
			continue
		}
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
	}
}

// Place reads a book's place file: when it was last kept, and the share of the book read (0 to 1, -1 if the old
// form without one); ok false when the book was never opened.
func Place(dir, key string) (when time.Time, share float64, ok bool) {
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
func progress(dir string, b Book) float64 {
	if _, share, ok := Place(dir, b.Key); ok {
		return share
	}
	return 0
}

// hidden says whether a book's path (relative to DIR, with slashes) has a part starting with a dot: macOS's `._name`
// beside each file a Mac copies onto the devices' FAT drives (Finder metadata, not an EPUB), `.Trashes` and the like.
func hidden(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
