package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Our own test books (e2e/books/NAME/, the EPUB's files as they go in the zip; e2e/books/README.md): built here into
// EPUBs, so the scenarios need no download for them. The same folder gives the same bytes every time (each file stored,
// not compressed, in name order after mimetype, all dated 2026-01-01): the reader names a book by its bytes
// (library.Key), and a fixture's place for it is kept under that name.
const booksDir = "e2e/books"

// testBooks is where the step `book NAME` puts a book, under the device's folder.
const testBooks = "books/test"

// buildBook zips e2e/books/NAME into an EPUB: mimetype first, then META-INF/ and OEBPS/, by name; what is beside them (the
// README, a script that made a file) stays out.
func buildBook(name string) ([]byte, error) {
	root := filepath.Join(booksDir, name)
	if _, err := os.Stat(filepath.Join(root, "mimetype")); err != nil {
		return nil, fmt.Errorf("no book %s (no %s/mimetype)", name, root)
	}
	var files []string
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if (strings.HasPrefix(rel, "META-INF/") || strings.HasPrefix(rel, "OEBPS/")) && !strings.HasPrefix(e.Name(), ".") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, f := range append([]string{"mimetype"}, files...) {
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return nil, err
		}
		w, err := z.CreateRaw(&zip.FileHeader{Name: f, Method: zip.Store, ModifiedDate: 0x5c21, // 2026-01-01, DOS date
			CRC32: crc32.ChecksumIEEE(data), CompressedSize64: uint64(len(data)), UncompressedSize64: uint64(len(data))})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// testBook puts e2e/books/NAME, built, on the device as books/test/NAME.epub, unless the same file is there.
func (d *dev) testBook(name string) (string, error) {
	data, err := buildBook(name)
	if err != nil {
		return "", err
	}
	path := testBooks + "/" + name + ".epub"
	full := d.a.Dir + "/" + path
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	if out, _ := d.sh("test -f "+full+" && sha256sum "+full+" | cut -c1-64", nil); strings.TrimSpace(string(out)) == sum {
		return path + " is there, as built", nil
	}
	if _, err := d.sh("mkdir -p $(dirname "+full+") && cat > "+full, data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s built and copied (%d bytes)", path, len(data)), nil
}

// buildBooks writes every book of e2e/books into dir as NAME.epub (devtest books DIR): to open them by hand.
func buildBooks(dir string) bool {
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	os.MkdirAll(dir, 0o755)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := buildBook(e.Name())
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, e.Name()+".epub"), data, 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return false
		}
		fmt.Printf("%s/%s.epub  %d bytes  %x\n", dir, e.Name(), len(data), sha256.Sum256(data))
	}
	return true
}
