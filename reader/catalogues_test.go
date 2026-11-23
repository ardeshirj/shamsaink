package main

import (
	"net/url"
	"strings"
	"testing"

	"shamsainkreader/opds"
)

// feed parses an OPDS 1 page given as its entries and links, as a catalogue at example.org would send it.
func feed(t *testing.T, body string) *opds.Feed {
	t.Helper()
	base, _ := url.Parse("https://example.org/opds/")
	f, err := opds.ParseOPDS1(strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opds="http://opds-spec.org/2010/catalog" xmlns:thr="http://purl.org/syndication/thread/1.0"
 xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
<id>urn:t</id><title>T</title><updated>2026-11-23T00:00:00Z</updated>`+body+`</feed>`), base)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func section(id, title, href, typ string) string {
	return `<entry><id>` + id + `</id><title>` + title + `</title><updated>2026-11-23T00:00:00Z</updated>
<link rel="subsection" href="` + href + `" type="` + typ + `"/></entry>`
}

func book(id string) string {
	return `<entry><id>` + id + `</id><title>` + id + `</title><updated>2026-11-23T00:00:00Z</updated>
<link rel="http://opds-spec.org/acquisition/open-access" href="` + id + `.epub" type="application/epub+zip"/></entry>`
}

// Browse and filter follows the catalogue (Only what the catalogue can answer): its front page's sections as Browse,
// › only on a section known to hold sections, its facets as groups, the funnel's count, Clear, a menu as its own list
// with the way back, and back to the front page.
func TestCatalogueFilters(t *testing.T) {
	c := &catalogues{list: []opds.Catalogue{{Name: "T", URL: "https://example.org/opds/root.xml"}}, navs: map[string]bool{}}
	c.set(feed(t, section("a", "All books", "all.xml", "application/atom+xml;profile=opds-catalog;kind=acquisition")+
		section("b", "Authors", "authors.xml", "application/atom+xml")+
		section("s", "Subjects", "subjects.xml", "application/atom+xml;profile=opds-catalog;kind=navigation")), false)
	head := c.headView(false)
	if !head.Funnel || head.Chosen || head.InForce != 0 {
		t.Errorf("front page: funnel %v chosen %v in force %d, want a funnel, nothing in force", head.Funnel, head.Chosen, head.InForce)
	}
	v, acts := c.filtersView(-1)
	if v.Browse == nil || len(v.Browse.Choices) != 3 || len(acts) != 3 || v.Clear || len(v.Groups) != 0 {
		t.Fatalf("front page: %+v", v)
	}
	if b := v.Browse.Choices; b[0].Deeper || b[1].Deeper || !b[2].Deeper {
		t.Errorf("›: kind=acquisition %v, no kind %v, kind=navigation %v; want only the last", b[0].Deeper, b[1].Deeper, b[2].Deeper)
	}

	// Authors: read, a menu (sections and no books): its own list, and its kind now known
	c.openTop(acts[1].top)
	c.set(feed(t, section("x", "Austen, Jane", "austen.xml", "application/atom+xml")+section("y", "Balzac", "balzac.xml", "application/atom+xml")), false)
	v, acts = c.filtersView(-1)
	if v.Open == nil || v.Open.Name != "Authors" || len(acts) != 2 || acts[0].sec != 0 || !v.Clear {
		t.Fatalf("a menu: %+v", v)
	}
	if !c.headView(false).Chosen || c.inForce() != 0 {
		t.Errorf("in a section: the funnel filled, no number")
	}

	// an author: books, facets; the section chosen in Browse, the level's title and the way back
	c.open(0, 0)
	c.set(feed(t, `<opensearch:totalResults>3278</opensearch:totalResults>
<link rel="http://opds-spec.org/facet" href="austen.xml" title="Any" opds:facetGroup="Language" opds:activeFacet="true"/>
<link rel="http://opds-spec.org/facet" href="austen-fr.xml" title="French" opds:facetGroup="Language" thr:count="12"/>`+book("emma")), false)
	v, acts = c.filtersView(-1)
	if v.Open != nil || !v.Back || v.Title != "Austen, Jane" || v.Browse == nil || v.Browse.Chosen != "Authors" || v.Books != 3278 {
		t.Fatalf("an author: %+v", v)
	}
	if !v.Browse.Choices[1].Deeper {
		t.Errorf("Authors read and found a menu: its › now shown")
	}
	if len(v.Groups) != 1 || v.Groups[0].Chosen != "Any" || v.Groups[0].Choices[1].Count != 12 {
		t.Fatalf("its facets: %+v", v.Groups)
	}
	fr := acts[len(acts)-1]
	if fr.href != "https://example.org/opds/austen-fr.xml" {
		t.Fatalf("French's link: %q", fr.href)
	}

	// French: narrowed in place, nothing matches: one facet in force, the list says none match
	c.narrow(fr.href)
	c.set(feed(t, `<link rel="http://opds-spec.org/facet" href="austen.xml" title="Any" opds:facetGroup="Language"/>
<link rel="http://opds-spec.org/facet" href="austen-fr.xml" title="French" opds:facetGroup="Language" opds:activeFacet="true"/>`), false)
	if h := c.headView(false); h.InForce != 1 || !h.Chosen {
		t.Errorf("French: in force %d chosen %v, want 1 and true", h.InForce, h.Chosen)
	}
	rd := &reader{}
	if !rd.catalogueView(c).NoMatch {
		t.Errorf("French with no books: no match not said")
	}
	if c.under() != 1 {
		t.Errorf("narrowed: still under Authors, got %d", c.under())
	}

	// up a level, then cleared: the front page as it was, nothing in force
	c.back()
	if c.title != "Authors" || !c.menu() {
		t.Errorf("back: %q, a menu %v", c.title, c.menu())
	}
	if !c.clear() || c.addr() != "https://example.org/opds/root.xml" || len(c.up) != 0 || c.clear() {
		t.Errorf("clear: %q, %d levels up", c.addr(), len(c.up))
	}
}
