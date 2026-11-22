package opds

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
)

// A Catalogue is an OPDS catalogue the reader offers: its name, the address of its start, and the folder under books/
// its downloads go into (so the library shows them by their source).
type Catalogue struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Folder string `json:"folder"`
}

// FarsiEPUB is the catalogue built in: FarsiEPUB's library (ROADMAP, stage 7: its address ships with the reader).
var FarsiEPUB = Catalogue{Name: "FarsiEPUB", URL: "https://farsiepub.org/opds/catalog.xml", Folder: "farsiepub"}

var ErrNoEPUB = errors.New("the book has no EPUB to download")

// EntryID is a book's id in its catalogue: its Atom id (the parser gives it as the identifier, or beside a
// dc:identifier as an alternate of scheme atom:id).
func EntryID(m *manifest.Manifest) string {
	for _, a := range m.Metadata.AltIdentifiers {
		if a.Scheme == "atom:id" {
			return a.Value
		}
	}
	return m.Metadata.Identifier
}

// Fetch downloads a catalogue's book into the reader's folder dir: into books/FOLDER/, over the file of the entry's
// last download when it is still there (a new edition of the same book), and records it. It gives the file, relative
// to dir. progress as Download's.
func Fetch(ctx context.Context, client *http.Client, dir string, c Catalogue, book *manifest.Manifest, rec *Record, progress func(done, total int64)) (string, error) {
	link := EPUB(book)
	if link == nil {
		return "", ErrNoEPUB
	}
	id := EntryID(book)
	over := ""
	if id != "" {
		if p := rec.Find(dir, c.URL, id); p != "" {
			over = filepath.Join(dir, p)
		}
	}
	dst, err := Download(ctx, client, link.Href.String(), filepath.Join(dir, "books", c.Folder), over, progress)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(dir, dst)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if id != "" {
		d := Saved{Catalogue: c.URL, Entry: id, Path: rel, At: time.Now().UTC()}
		if book.Metadata.Modified != nil {
			d.Updated = book.Metadata.Modified.UTC()
		}
		if err := rec.Put(d); err != nil {
			return rel, fmt.Errorf("downloaded, but not recorded: %w", err)
		}
	}
	return rel, nil
}

// Newer says whether the catalogue has a newer edition of a book already downloaded: its <updated> later than when it
// was downloaded. False for a book never downloaded, or one whose catalogue gives no dates (Ebooksgratuits').
func (r *Record) Newer(c Catalogue, book *manifest.Manifest) bool {
	d, ok := r.Get(c.URL, EntryID(book))
	return ok && book.Metadata.Modified != nil && !d.Updated.IsZero() && book.Metadata.Modified.After(d.Updated)
}
