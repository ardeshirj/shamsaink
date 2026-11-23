package opds

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
)

var (
	ErrUnreachable = errors.New("the catalogue was not reached")          // no network, a refusal, a status not 200
	ErrNotFeed     = errors.New("the catalogue's page could not be read") // not an OPDS 1 feed (OPDS 2 not yet read)
	ErrCut         = errors.New("the download stopped before its end")
	ErrNotEPUB     = errors.New("the file is not an EPUB book")
	ErrCancelled   = errors.New("the download was cancelled")
)

// maxFeed is the most a catalogue's page may be: Wikisource's single page of 711 books is 1.1 MB (experiment 18).
const maxFeed = 8 << 20

// Get reads one page of a catalogue at addr with client (the updater's: its timeouts and CA roots). Its links come
// resolved against the address it was finally read from, after redirects.
func Get(ctx context.Context, client *http.Client, addr string) (*Feed, error) {
	f, _, err := GetPage(ctx, client, addr)
	return f, err
}

// GetPage is Get, with the page as it came, to keep for offline (Page).
func GetPage(ctx context.Context, client *http.Client, addr string) (*Feed, *Page, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	req.Header.Set("Accept", "application/atom+xml;profile=opds-catalog, application/atom+xml;q=0.9, */*;q=0.5")
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, nil, fmt.Errorf("%w: %s: %s", ErrUnreachable, addr, res.Status)
	}
	if ct := res.Header.Get("Content-Type"); strings.Contains(ct, "json") {
		return nil, nil, fmt.Errorf("%w: %s is %s (OPDS 2 is not read yet)", ErrNotFeed, addr, ct)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxFeed))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s: %v", ErrUnreachable, addr, err)
	}
	p := &Page{URL: res.Request.URL.String(), At: time.Now().UTC(), XML: b}
	f, err := p.Feed()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s: %v", ErrNotFeed, addr, err)
	}
	return f, p, nil
}

// EPUB is the link to download a book by, nil when it has none ShamsaInk can take: an EPUB given freely (open access
// first, then a plain acquisition link, then an Atom enclosure: a plain Atom feed's file, as Standard Ebooks' open feed
// of new releases gives its books), not one to buy, borrow or sample, nor one behind another step (an indirect
// acquisition: a licence, a page).
func EPUB(m *manifest.Manifest) *manifest.Link {
	var plain, enclosure *manifest.Link
	for i := range m.Links {
		l := &m.Links[i]
		if l.MediaType == nil || l.MediaType.String() != "application/epub+zip" || l.Properties.Get("indirectAcquisition") != nil {
			continue
		}
		for _, r := range l.Rels {
			switch r {
			case RelOpenAccess:
				return l
			case RelAcquisition:
				if plain == nil {
					plain = l
				}
			case "enclosure":
				if enclosure == nil {
					enclosure = l
				}
			}
		}
	}
	if plain == nil {
		return enclosure
	}
	return plain
}

// Download fetches the book at addr into dir, as the updater fetches a package: streamed to a part file, its length
// checked against the one announced, then checked to be an EPUB (a ZIP with an OPF the container names) whatever the
// server called it, and only then moved into place. Its file is over (replacing it whole: a new edition of the same
// book, docs/DESIGN.md, "The library's books") when over is given, else a name of its own in dir: the server's
// (Content-Disposition, else the last part of the final address), with -2, -3... when taken. progress (may be nil)
// gets the bytes so far and the whole (0 when the server does not say). On any failure nothing is left in dir.
func Download(ctx context.Context, client *http.Client, addr, dir, over string, progress func(done, total int64)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	res, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ErrCancelled
		}
		return "", fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 { // a 206 for a whole file is a server's mistake (FarsiEPUB's until 2026-11-22): refused
		return "", fmt.Errorf("%w: %s: %s", ErrUnreachable, addr, res.Status)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	part, err := os.CreateTemp(dir, ".download-*.part")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		part.Close()
		if !ok {
			os.Remove(part.Name())
		}
	}()
	total := res.ContentLength // -1 when not said
	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, rerr := res.Body.Read(buf)
		if n > 0 {
			if _, err := part.Write(buf[:n]); err != nil {
				return "", err // the storage full: not the network's
			}
			done += int64(n)
			if progress != nil {
				progress(done, max(total, 0))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return "", ErrCancelled
			}
			return "", fmt.Errorf("%w: %v after %d bytes", ErrCut, rerr, done)
		}
	}
	if total >= 0 && done != total {
		return "", fmt.Errorf("%w: %d of %d bytes", ErrCut, done, total)
	}
	if err := part.Sync(); err != nil {
		return "", err
	}
	if err := part.Close(); err != nil {
		return "", err
	}
	if err := isEPUB(part.Name()); err != nil {
		return "", err
	}
	dst := over
	if dst == "" {
		dst = freeName(dir, fileName(res))
	}
	if err := os.Rename(part.Name(), dst); err != nil {
		return "", err
	}
	ok = true
	return dst, nil
}

// isEPUB says whether the file is an EPUB: a ZIP whose META-INF/container.xml is there (every EPUB 2 and 3 has one;
// the mimetype entry is often wrong in the wild, so it is not asked for).
func isEPUB(name string) error {
	z, err := zip.OpenReader(name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotEPUB, err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name == "META-INF/container.xml" {
			return nil
		}
	}
	return fmt.Errorf("%w: no META-INF/container.xml", ErrNotEPUB)
}

// fileName is the name the server gives the file (Content-Disposition), else the last part of the address it came
// from (after redirects: Ebooksgratuits' script sends to ./epub/NAME.epub), made safe for the device's storage (FAT on
// the Kobo) and ending in .epub.
func fileName(res *http.Response) string {
	name := ""
	if _, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition")); err == nil {
		name = params["filename"]
	}
	if name == "" && res.Request != nil && res.Request.URL != nil {
		if p, err := url.PathUnescape(res.Request.URL.Path); err == nil {
			name = path.Base(p)
		}
	}
	return safeName(name)
}

// safeName keeps a file name to what FAT and the reader's scan take: no folders, none of FAT's reserved characters or
// control characters, no leading dot (hidden), at most 120 bytes before .epub, which it ends in.
func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimLeft(strings.TrimSpace(name), ".")
	stem := strings.TrimSuffix(name, path.Ext(name))
	if strings.EqualFold(path.Ext(name), ".epub") && strings.HasSuffix(strings.ToLower(stem), ".kepub") {
		stem = stem[:len(stem)-len(".kepub")] // a Kobo edition is still an EPUB to us
	}
	for len(stem) > 120 {
		_, size := lastRune(stem)
		stem = stem[:len(stem)-size]
	}
	if stem == "" || stem == "/" || stem == "." {
		stem = "book"
	}
	return stem + ".epub"
}

func lastRune(s string) (rune, int) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i]&0xC0 != 0x80 {
			return []rune(s[i:])[0], len(s) - i
		}
	}
	return 0, 1
}

// freeName is dir/name, or dir/STEM-2.epub, -3... when that is taken.
func freeName(dir, name string) string {
	p := filepath.Join(dir, name)
	stem := strings.TrimSuffix(name, ".epub")
	for i := 2; ; i++ {
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s-%d.epub", stem, i))
	}
}
