package opds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shamsainkreader/files"
)

// A Page is a catalogue's page as it came, kept beside the reader (catalogues/FOLDER.json) so the catalogue's books
// show at once, offline too, and are then read again.
type Page struct {
	URL string    `json:"url"` // the address it was finally read from: its links are resolved against it
	At  time.Time `json:"at"`  // when it was read
	XML []byte    `json:"xml"`
}

// Feed reads the page.
func (p *Page) Feed() (*Feed, error) {
	base, err := url.Parse(p.URL)
	if err != nil {
		return nil, err
	}
	return ParseOPDS1(bytes.NewReader(p.XML), base)
}

func keptPath(dir string, c Catalogue) string {
	return filepath.Join(dir, "catalogues", c.Folder+".json")
}

// Keep keeps p as catalogue c's first page.
func Keep(dir string, c Catalogue, p *Page) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return files.Write(keptPath(dir, c), b)
}

// Kept is catalogue c's first page as last read, nil when there is none (or it cannot be read).
func Kept(dir string, c Catalogue) *Page {
	b, err := os.ReadFile(keptPath(dir, c))
	if err != nil {
		return nil
	}
	var p Page
	if json.Unmarshal(b, &p) != nil || len(p.XML) == 0 {
		return nil
	}
	return &p
}

// Catalogues are the catalogues the reader offers: FarsiEPUB's first, then those of catalogues.json beside the reader
// (copied over USB until there is a keyboard): [{"name": "Standard Ebooks", "url": "https://..."}], each one's folder
// under books/ its own ("folder", else its name made a folder's), one with no name or no http(s) address left out,
// as is one whose address is already offered. A file that cannot be read is said in the log and left out whole.
func Catalogues(dir string) []Catalogue {
	all := []Catalogue{FarsiEPUB}
	b, err := os.ReadFile(filepath.Join(dir, "catalogues.json"))
	if err != nil {
		return all
	}
	var more []Catalogue
	if err := json.Unmarshal(b, &more); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: catalogues.json:", err)
		return all
	}
	seen := map[string]bool{FarsiEPUB.URL: true}
	folders := map[string]bool{FarsiEPUB.Folder: true}
	for _, c := range more {
		u, err := url.Parse(c.URL)
		if c.Name == "" || err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || seen[c.URL] {
			continue
		}
		if c.Folder == "" {
			c.Folder = c.Name
		}
		c.Folder = folderName(c.Folder)
		for base, i := c.Folder, 2; folders[c.Folder]; i++ {
			c.Folder = fmt.Sprintf("%s-%d", base, i)
		}
		seen[c.URL], folders[c.Folder] = true, true
		all = append(all, c)
	}
	return all
}

// folderName is a catalogue's folder under books/: lowercase, a dash for each run of what is not a letter or a digit.
func folderName(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 0x7f && !strings.ContainsRune("‌‏‎", r):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		default:
			dash = true
		}
	}
	if b.Len() == 0 {
		return "catalogue"
	}
	return b.String()
}
