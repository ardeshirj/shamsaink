package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"path"
	"strings"
)

// A catalogue on the device (e2e/scenarios/catalogue.txt): devtest serves one from this computer, as FarsiEPUB's feed
// would, so the Catalogues side is tried with no site in the way.
//
//	catalogue-serve BOOK...  an OPDS 1.2 catalogue named E2E Catalogue (as wide as FarsiEPUB's name, so the kept
//	                         render's box is tapped) holding the test books named (e2e/books, built as the step book
//	                         builds them), an entry each (urn:e2e:BOOK, its EPUB open access, its language its name's
//	                         start), added to the device's catalogues.json (its downloads into books/e2e/, removed when
//	                         the scenario ends; the reader's catalogues.json and downloads.json put back with its other
//	                         files) and made the last catalogue shown (library-view.json), so no other catalogue is read;
//	                         on the simulated device its Wi-Fi on, as a device's is while devtest reaches it (sim/wifi).
//	                         Its main page two sections (Browse and filter's, catalogue-filters:test-front): "Test books"
//	                         (kind=acquisition) with every book, and "By language", its link of no kind (as
//	                         Ebooksgratuits'), a section of sections, English and Farsi (test-language). Every page of
//	                         books has its total and two facet groups (test-books): Language (All, English, Farsi, their
//	                         counts) and Narrated (Any, and Narrated, which no book is: none match)
func (p *player) catalogueServe(books []string) (string, error) {
	d := p.d
	files := map[string][]byte{}
	entries := map[string]*strings.Builder{"": {}, "en": {}, "fa": {}} // every book, and each language's
	n := map[string]int{}
	for _, b := range books {
		data, err := buildBook(b)
		if err != nil {
			return "", err
		}
		files[b+".epub"] = data
		lang := strings.SplitN(b, "-", 2)[0]
		e := fmt.Sprintf(`<entry><id>urn:e2e:%[1]s</id><title>%[1]s</title><author><name>ShamsaInk</name></author>
<updated>2026-11-22T00:00:00Z</updated><dc:language>%[2]s</dc:language>
<link rel="http://opds-spec.org/acquisition/open-access" href="books/%[1]s.epub" type="application/epub+zip"/></entry>
`, html.EscapeString(b), lang)
		entries[""].WriteString(e)
		if w, ok := entries[lang]; ok {
			w.WriteString(e)
		}
		n[""]++
		n[lang]++
	}
	const head = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/" xmlns:thr="http://purl.org/syndication/thread/1.0"
 xmlns:opds="http://opds-spec.org/2010/catalog" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
`
	section := func(title, href, typ string, count int) string {
		c := ""
		if count > 0 {
			c = fmt.Sprintf(` thr:count="%d"`, count)
		}
		return fmt.Sprintf(`<entry><id>urn:e2e:%s</id><title>%s</title><updated>2026-11-22T00:00:00Z</updated>
<link rel="subsection" href="%s" type="%s"%s/></entry>
`, href, title, href, typ, c)
	}
	const acq = "application/atom+xml;profile=opds-catalog;kind=acquisition"
	files["catalog.xml"] = []byte(head + `<id>urn:e2e</id><title>E2E</title><updated>2026-11-22T00:00:00Z</updated>
` + section("Test books", "books.xml", acq, n[""]) + section("By language", "langs.xml", "application/atom+xml", 0) + `</feed>`)
	files["langs.xml"] = []byte(head + `<id>urn:e2e:langs</id><title>By language</title><updated>2026-11-22T00:00:00Z</updated>
` + section("English", "books-en.xml", acq, n["en"]) + section("Farsi", "books-fa.xml", acq, n["fa"]) + `</feed>`)
	// a page of books: its total, then the facets with the page's own choices active
	page := func(name, lang string, narrated bool) []byte {
		facet := func(group, title, href string, count int, on bool) string {
			s := fmt.Sprintf(`<link rel="http://opds-spec.org/facet" href="%s" type="%s" title="%s" opds:facetGroup="%s"`, href, acq, title, group)
			if count > 0 {
				s += fmt.Sprintf(` thr:count="%d"`, count)
			}
			if on {
				s += ` opds:activeFacet="true"`
			}
			return s + "/>\n"
		}
		body, total := entries[lang].String(), n[lang]
		if narrated { // none of our books is narrated
			body, total = "", 0
		}
		return []byte(head + fmt.Sprintf(`<id>urn:e2e:%s</id><title>Test books</title><updated>2026-11-22T00:00:00Z</updated>
<opensearch:totalResults>%d</opensearch:totalResults>
`, name, total) +
			facet("Language", "All", "books.xml", 0, lang == "" && !narrated) + facet("Language", "English", "books-en.xml", n["en"], lang == "en") +
			facet("Language", "Farsi", "books-fa.xml", n["fa"], lang == "fa") +
			facet("Narrated", "Any", "books.xml", 0, !narrated) + facet("Narrated", "Narrated", "none.xml", 0, narrated) + body + `</feed>`)
	}
	files["books.xml"] = page("books", "", false)
	files["books-en.xml"] = page("books-en", "en", false)
	files["books-fa.xml"] = page("books-fa", "fa", false)
	files["none.xml"] = page("none", "", true)
	l, err := d.listenHere()
	if err != nil {
		return "", fmt.Errorf("serving the catalogue: %v", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[path.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".xml") {
			w.Header().Set("Content-Type", "application/atom+xml;profile=opds-catalog;kind=acquisition")
		} else {
			w.Header().Set("Content-Type", "application/epub+zip")
		}
		w.Write(b)
	})}
	go srv.Serve(l)
	addr := "http://" + l.Addr().String() + "/catalog.xml"
	list := fmt.Sprintf(`[{"name": "E2E Catalogue", "url": %q, "folder": "e2e"}]`, addr)
	if _, err := d.sh("cat > "+d.a.Dir+"/catalogues.json", []byte(list)); err != nil {
		srv.Close()
		return "", fmt.Errorf("catalogues.json on the device: %v", err)
	}
	// the last catalogue shown: the served one, so FarsiEPUB's (the first) is not read (the reader's view put back after)
	view := map[string]any{}
	if b, _ := d.sh("cat "+d.a.Dir+"/library-view.json 2>/dev/null", nil); len(b) > 0 {
		json.Unmarshal(b, &view)
	}
	view["catalogue"] = addr
	b, _ := json.Marshal(view)
	if _, err := d.sh("cat > "+d.a.Dir+"/library-view.json", b); err != nil {
		srv.Close()
		return "", fmt.Errorf("library-view.json on the device: %v", err)
	}
	if d.a.Local != "" { // the simulated device's Wi-Fi on (device/wifi.go, simWiFi)
		d.sh("mkdir -p "+d.a.Local+"/sim && echo on > "+d.a.Local+"/sim/wifi", nil)
	}
	p.cleanups = append(p.cleanups, func() {
		srv.Close()
		d.sh("rm -rf "+d.a.Dir+"/books/e2e", nil)
	})
	return fmt.Sprintf("%d books at %s", len(books), addr), nil
}
