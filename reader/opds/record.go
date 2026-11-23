package opds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"shamsainkreader/files"
)

// The books downloaded from catalogues: which catalogue entry each file came from (downloads.json beside the reader).
// A later download of the same entry goes over the same file, so the library's scan sees a new edition of the same
// book and keeps its place and highlights (docs/DESIGN.md, "The library's books"), whatever the server calls the new
// file. A file since moved or removed is forgotten, and the entry downloaded as a new book. An entry is known by its
// catalogue (the address of the catalogue's start) and its Atom id; a catalogue's ids are its own, so the same id in
// two catalogues is two entries.
type Record struct {
	mu        sync.Mutex // downloads run beside the screen that reads it
	path      string
	Downloads []Saved `json:"downloads"`
}

// Saved is one entry's file.
type Saved struct {
	Catalogue string    `json:"catalogue"`        // the catalogue's start, as configured
	Entry     string    `json:"entry"`            // the entry's Atom id
	Path      string    `json:"path"`             // the file, relative to the reader's folder (books/farsiepub/x.epub)
	Updated   time.Time `json:"updated,omitzero"` // the entry's <updated> when downloaded: a newer one is a new edition
	At        time.Time `json:"at"`               // when it was downloaded
}

// LoadRecord reads dir's downloads.json; none, or one that cannot be read, is an empty record (its downloads then
// come again as new books: nothing is lost but the link to their entries).
func LoadRecord(dir string) *Record {
	r := &Record{path: filepath.Join(dir, "downloads.json")}
	if b, err := os.ReadFile(r.path); err == nil {
		json.Unmarshal(b, r)
	}
	return r
}

// Find is the file (relative to the reader's folder) of the entry's last download, "" if it has none or its file is
// gone from dir.
func (r *Record) Find(dir, catalogue, entry string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.Downloads {
		if d.Catalogue == catalogue && d.Entry == entry {
			if _, err := os.Stat(filepath.Join(dir, d.Path)); err == nil {
				return d.Path
			}
			return ""
		}
	}
	return ""
}

// Get is the entry's download, false if it has none.
func (r *Record) Get(catalogue, entry string) (Saved, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.Downloads {
		if d.Catalogue == catalogue && d.Entry == entry {
			return d, true
		}
	}
	return Saved{}, false
}

// Put records d, in place of the entry's earlier download, and saves the record.
func (r *Record) Put(d Saved) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.Downloads {
		if r.Downloads[i].Catalogue == d.Catalogue && r.Downloads[i].Entry == d.Entry {
			r.Downloads[i] = d
			return r.save()
		}
	}
	r.Downloads = append(r.Downloads, d)
	return r.save()
}

func (r *Record) save() error {
	b, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return err
	}
	return files.Write(r.path, b)
}
