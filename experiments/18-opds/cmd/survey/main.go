// survey reads OPDS 1 feeds and says what each uses: entries, books and menus, ids and dates, the acquisition links'
// rels and types, pagination and search, and the EPUB a reader would pick; with the parse's time and memory.
//
//	survey [-n repeats] [-pages N] [-base URL] FEED...   (a FEED is a file or an http(s) address)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
	"opdsexp/opds"
)

func main() {
	repeats := flag.Int("n", 5, "parses timed (the best is shown)")
	pages := flag.Int("pages", 1, "pages followed through next links (addresses only)")
	baseFlag := flag.String("base", "", "base address for a feed read from a file")
	flag.Parse()
	for _, arg := range flag.Args() {
		src := arg
		for p := 0; p < *pages && src != ""; p++ {
			next, err := survey(src, *baseFlag, *repeats)
			if err != nil {
				fmt.Printf("%s: %v\n\n", src, err)
				break
			}
			src = next
		}
	}
}

func survey(src, baseStr string, repeats int) (next string, err error) {
	var data []byte
	var base *url.URL
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		t0 := time.Now()
		res, err := http.Get(src)
		if err != nil {
			return "", err
		}
		ttfb := time.Since(t0)
		data, err = io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return "", err
		}
		fmt.Printf("%s\n  fetch: %s, first byte %d ms, all %d ms, %s\n", src, res.Status, ttfb.Milliseconds(),
			time.Since(t0).Milliseconds(), res.Header.Get("Content-Type"))
		if res.StatusCode != 200 {
			return "", fmt.Errorf("status %s", res.Status)
		}
		base = res.Request.URL
	} else {
		data, err = os.ReadFile(src)
		if err != nil {
			return "", err
		}
		fmt.Printf("%s\n", src)
		if baseStr != "" {
			base, _ = url.Parse(baseStr)
		}
	}

	// time: the best of n; memory: allocated during one parse, and what the result keeps once collected
	var best time.Duration
	var f *opds.Feed
	for i := 0; i < repeats; i++ {
		runtime.GC()
		t0 := time.Now()
		f, err = opds.ParseOPDS1(bytes.NewReader(data), base)
		d := time.Since(t0)
		if err != nil {
			return "", err
		}
		if i == 0 || d < best {
			best = d
		}
	}
	f = nil
	var m0, m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)
	f, _ = opds.ParseOPDS1(bytes.NewReader(data), base)
	runtime.ReadMemStats(&m1)
	runtime.GC()
	runtime.ReadMemStats(&m2)
	kept := int64(m2.HeapAlloc) - int64(m0.HeapAlloc)
	fmt.Printf("  size %d KB; parse %.1f ms (best of %d), allocated %.1f MB, result kept %.2f MB\n",
		len(data)/1024, float64(best.Microseconds())/1000, repeats,
		float64(m1.TotalAlloc-m0.TotalAlloc)/1e6, float64(kept)/1e6)
	runtime.KeepAlive(f)

	report(f)
	if l := f.Links.FirstWithRel("next"); l != nil {
		next = l.Href.String()
	}
	fmt.Println()
	return next, nil
}

func report(f *opds.Feed) {
	pubs := append([]manifest.Manifest{}, f.Publications...)
	nav := append(manifest.LinkList{}, f.Navigation...)
	for _, g := range f.Groups {
		pubs = append(pubs, g.Publications...)
		nav = append(nav, g.Navigation...)
	}
	fmt.Printf("  feed: %q id %q", f.Metadata.Title, f.Metadata.Identifier)
	if f.Metadata.NumberOfItems != nil {
		fmt.Printf(", %d items in all", *f.Metadata.NumberOfItems)
	}
	if f.Metadata.ItemsPerPage != nil {
		fmt.Printf(", %d a page", *f.Metadata.ItemsPerPage)
	}
	fmt.Printf("\n  books %d, menus %d, groups %d, facets %d\n", len(pubs), len(nav), len(f.Groups), len(f.Facets))
	for _, l := range f.Links {
		mt := ""
		if l.MediaType != nil {
			mt = l.MediaType.String()
		}
		fmt.Printf("  link %v %s %s\n", l.Rels, mt, short(l.Href.String()))
	}
	for i, l := range nav {
		if i == 5 {
			fmt.Printf("  menu ... (%d more)\n", len(nav)-5)
			break
		}
		fmt.Printf("  menu %q → %s\n", l.Title, short(l.Href.String()))
	}
	if len(pubs) == 0 {
		return
	}

	withID, withAlt, withUpdated, withLang, withDesc, picked, several := 0, 0, 0, 0, 0, 0, 0
	rels := map[string]int{}
	for _, p := range pubs {
		if p.Metadata.Identifier != "" {
			withID++
		}
		if len(p.Metadata.AltIdentifiers) > 0 {
			withAlt++
		}
		if p.Metadata.Modified != nil {
			withUpdated++
		}
		if len(p.Metadata.Languages) > 0 {
			withLang++
		}
		if p.Metadata.Description != "" {
			withDesc++
		}
		epubs := 0
		for _, l := range p.Links {
			mt := ""
			if l.MediaType != nil {
				mt = l.MediaType.String()
			}
			for _, r := range l.Rels {
				if strings.HasPrefix(r, opds.RelAcquisition) {
					k := strings.TrimPrefix(r, opds.RelAcquisition) + " " + mt
					if l.Properties.Get("indirectAcquisition") != nil {
						k += " (indirect)"
					}
					rels[k]++
					if strings.HasPrefix(mt, "application/epub+zip") {
						epubs++
					}
				}
			}
		}
		if epubs > 0 {
			picked++
		}
		if epubs > 1 {
			several++
		}
	}
	n := len(pubs)
	fmt.Printf("  books with: id %d/%d (an Atom id besides dc:identifier %d), updated %d, language %d, description %d\n",
		withID, n, withAlt, withUpdated, withLang, withDesc)
	fmt.Printf("  an EPUB to download: %d/%d (several EPUBs: %d)\n", picked, n, several)
	keys := make([]string, 0, len(rels))
	for k := range rels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  acquisition%s ×%d\n", k, rels[k])
	}
	p := pubs[0]
	fmt.Printf("  first book: %q id %q", p.Metadata.LocalizedTitle.String(), p.Metadata.Identifier)
	if p.Metadata.Modified != nil {
		fmt.Printf(" updated %s", p.Metadata.Modified.Format("2006-01-02"))
	}
	fmt.Println()
	for _, l := range p.Links {
		mt := ""
		if l.MediaType != nil {
			mt = l.MediaType.String()
		}
		fmt.Printf("    %v %s %q %s\n", l.Rels, mt, l.Title, short(l.Href.String()))
	}
}

func short(s string) string {
	if len(s) > 110 {
		return s[:107] + "..."
	}
	return s
}
