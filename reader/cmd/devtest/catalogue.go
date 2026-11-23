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
//	                         render's box is tapped) whose main page is one section, "Test books", holding the test
//	                         books named (e2e/books, built as the step book builds them), an entry each (urn:e2e:BOOK,
//	                         its EPUB open access), added to the device's catalogues.json (its downloads into
//	                         books/e2e/, removed when the scenario ends; the reader's catalogues.json and
//	                         downloads.json put back with its other files) and made the last catalogue shown
//	                         (library-view.json), so no other catalogue is read; on the simulated device its Wi-Fi
//	                         on, as a device's is while devtest reaches it (sim/wifi)
func (p *player) catalogueServe(books []string) (string, error) {
	d := p.d
	files := map[string][]byte{}
	var entries strings.Builder
	for _, b := range books {
		data, err := buildBook(b)
		if err != nil {
			return "", err
		}
		files[b+".epub"] = data
		fmt.Fprintf(&entries, `<entry><id>urn:e2e:%[1]s</id><title>%[1]s</title><author><name>ShamsaInk</name></author>
<updated>2026-11-22T00:00:00Z</updated><dc:language>%[2]s</dc:language>
<link rel="http://opds-spec.org/acquisition/open-access" href="books/%[1]s.epub" type="application/epub+zip"/></entry>
`, html.EscapeString(b), strings.SplitN(b, "-", 2)[0])
	}
	files["catalog.xml"] = []byte(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:thr="http://purl.org/syndication/thread/1.0">
<id>urn:e2e</id><title>E2E</title><updated>2026-11-22T00:00:00Z</updated>
<entry><id>urn:e2e:books</id><title>Test books</title><updated>2026-11-22T00:00:00Z</updated>
<link rel="subsection" href="books.xml" type="application/atom+xml;profile=opds-catalog;kind=acquisition" thr:count="` +
		fmt.Sprint(len(books)) + `"/></entry></feed>`)
	files["books.xml"] = []byte(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/">
<id>urn:e2e:books</id><title>Test books</title><updated>2026-11-22T00:00:00Z</updated>
` + entries.String() + `</feed>`)
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
