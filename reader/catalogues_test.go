package main

import (
	"net/url"
	"strings"
	"testing"

	"shamsainkreader/opds"
	"shamsainkreader/ui"
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
// its facets as groups, the funnel's count, Clear, a menu as its own list
// with the way back, and back to the front page.
func TestCatalogueFilters(t *testing.T) {
	c := &catalogues{list: []opds.Catalogue{{Name: "T", URL: "https://example.org/opds/root.xml"}}}
	c.set(feed(t, section("a", "All books", "all.xml", "application/atom+xml;profile=opds-catalog;kind=acquisition")+
		section("b", "Authors", "authors.xml", "application/atom+xml")+
		section("s", "Subjects", "subjects.xml", "application/atom+xml;profile=opds-catalog;kind=navigation")), false)
	head := c.headView(false)
	if !head.Browse || head.Funnel || head.Chosen || head.InForce != 0 {
		t.Errorf("front page: list icon %v funnel %v chosen %v in force %d, want the list icon alone (sections, no facets), nothing in force", head.Browse, head.Funnel, head.Chosen, head.InForce)
	}
	v, acts := c.filtersView(-1, true)
	if v.Browse == nil || len(v.Browse.Choices) != 3 || len(acts) != 3 || v.Clear || len(v.Groups) != 0 {
		t.Fatalf("front page: %+v", v)
	}

	// Authors: read, a menu (sections and no books): its own list
	c.openTop(acts[1].top)
	c.set(feed(t, section("x", "Austen, Jane", "austen.xml", "application/atom+xml")+section("y", "Balzac", "balzac.xml", "application/atom+xml")), false)
	v, acts = c.filtersView(-1, true)
	if v.Open == nil || v.Open.Name != "Authors" || len(acts) != 2 || acts[0].sec != 0 || !v.Clear {
		t.Fatalf("a menu: %+v", v)
	}
	if !c.headView(false).Chosen || c.inForce() != 0 {
		t.Errorf("in a section: the funnel filled, no number")
	}

	// an author: books, facets; Browse: Authors ticked, the author's name a chip, no back arrow
	c.open(0, 0)
	c.set(feed(t, `<opensearch:totalResults>3278</opensearch:totalResults>
<link rel="http://opds-spec.org/facet" href="austen.xml" title="Any" opds:facetGroup="Language" opds:activeFacet="true"/>
<link rel="http://opds-spec.org/facet" href="austen-fr.xml" title="French" opds:facetGroup="Language" thr:count="12"/>`+book("emma")), false)
	v, _ = c.filtersView(-1, true)
	if v.Open != nil || v.Back || v.Title != "" || v.Browse == nil || !v.Browse.Choices[1].On || v.Books != 3278 || len(v.Groups) != 0 ||
		len(v.Picked) != 1 || v.Picked[0] != (ui.Picked{Label: "Austen, Jane", Fixed: true}) {
		t.Fatalf("an author, Browse: %+v", v)
	}
	// Filter's page: its facets alone, nothing in force yet (Any, the first), nothing to clear
	v, acts = c.filtersView(-1, false)
	if v.Browse != nil || len(v.Picked) != 0 || v.Clear {
		t.Fatalf("an author, Filter: %+v", v)
	}
	if len(v.Groups) != 1 || !v.Groups[0].Choices[0].On || v.Groups[0].Choices[1].Count != 12 {
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
	// Filter's Clear all: French off, the author's own page again, still under Authors; then nothing more to clear
	if v, _ := c.filtersView(-1, false); !v.Clear || len(v.Picked) != 1 || v.Picked[0].Label != "French" {
		t.Errorf("French, Filter: clear %v, row %+v", v.Clear, v.Picked)
	}
	up := len(c.up)
	if !c.clearFacets() || c.url != c.sec || len(c.up) != up || c.clearFacets() {
		t.Errorf("Filter's clear: %q (its section %q), %d levels up", c.url, c.sec, len(c.up))
	}
	c.narrow(fr.href) // as it was, for what follows
	c.set(feed(t, `<link rel="http://opds-spec.org/facet" href="austen.xml" title="Any" opds:facetGroup="Language"/>
<link rel="http://opds-spec.org/facet" href="austen-fr.xml" title="French" opds:facetGroup="Language" opds:activeFacet="true"/>`), false)

	// up a level, then cleared: the front page as it was, nothing in force
	c.back()
	if c.title != "Authors" || !c.menu() {
		t.Errorf("back: %q, a menu %v", c.title, c.menu())
	}
	if !c.clear() || c.addr() != "https://example.org/opds/root.xml" || len(c.up) != 0 || c.clear() {
		t.Errorf("clear: %q, %d levels up", c.addr(), len(c.up))
	}
}

// Several facet groups in force: each group's choice as the catalogue marks it, the funnel counting them; a catalogue
// that marks none, the choice followed last alone (the page shown is its page), no earlier one.
func TestCatalogueFacetsChosen(t *testing.T) {
	c := &catalogues{list: []opds.Catalogue{{Name: "T", URL: "https://example.org/opds/root.xml"}}}
	groups := func(lang, sort string) string {
		mark := func(on bool) string {
			if on {
				return ` opds:activeFacet="true"`
			}
			return ""
		}
		return `<link rel="http://opds-spec.org/facet" href="all.xml" title="Any" opds:facetGroup="Language"` + mark(lang == "") + `/>
<link rel="http://opds-spec.org/facet" href="fr.xml" title="French" opds:facetGroup="Language"` + mark(lang == "fr") + `/>
<link rel="http://opds-spec.org/facet" href="popular.xml" title="Popular" opds:facetGroup="Sort by"` + mark(sort == "popular") + `/>
<link rel="http://opds-spec.org/facet" href="fr-new.xml" title="Newest" opds:facetGroup="Sort by"` + mark(sort == "new") + `/>` + book("emma")
	}
	chosen := func() (lang, sort string) {
		v, _ := c.filtersView(-1, false)
		if len(v.Groups) != 2 {
			t.Fatalf("groups: %+v", v.Groups)
		}
		on := func(g ui.FilterGroup) string {
			for _, ch := range g.Choices {
				if ch.On {
					return ch.Label
				}
			}
			return ""
		}
		return on(v.Groups[0]), on(v.Groups[1])
	}

	// marked: French and Newest, both shown, two in force
	c.set(feed(t, groups("", "popular")), false)
	c.narrow("https://example.org/opds/fr.xml")
	c.set(feed(t, groups("fr", "popular")), false)
	c.narrow("https://example.org/opds/fr-new.xml")
	c.set(feed(t, groups("fr", "new")), false)
	if l, s := chosen(); l != "French" || s != "Newest" || c.inForce() != 2 {
		t.Errorf("marked: %q and %q, %d in force; want French and Newest, 2", l, s, c.inForce())
	}
	// the row under the title: each, its x its group's first choice
	if v, _ := c.filtersView(-1, false); len(v.Picked) != 2 || v.Picked[0].Label != "French" || v.Picked[1].Label != "Newest" || v.Picked[0].Fixed ||
		len(c.unpick) != 2 || c.unpick[0] != "https://example.org/opds/all.xml" || c.unpick[1] != "https://example.org/opds/popular.xml" {
		t.Errorf("the row: %+v, removed by %q", v.Picked, c.unpick)
	}

	// marks none: Newest, followed last, alone; French, earlier, not shown
	c.set(feed(t, groups("x", "x")), false)
	if l, s := chosen(); l != "" || s != "Newest" || c.inForce() != 1 {
		t.Errorf("unmarked: %q and %q, %d in force; want only Newest, 1", l, s, c.inForce())
	}

	// marks none and nothing followed (the front page as it is): none chosen
	c.clear()
	c.set(feed(t, groups("x", "x")), false)
	if l, s := chosen(); l != "" || s != "" || c.inForce() != 0 {
		t.Errorf("front page unmarked: %q and %q, %d in force; want none", l, s, c.inForce())
	}
}

// A choice's page being read: the panel as it was, Loading… in its title, no choice taken; the new one once read.
func TestCatalogueFiltersKeptWhileLoading(t *testing.T) {
	c := &catalogues{list: []opds.Catalogue{{Name: "T", URL: "https://example.org/opds/root.xml"}}}
	c.set(feed(t, section("a", "All books", "all.xml", "")+section("b", "Authors", "authors.xml", "")), false)
	before, _ := c.filtersView(-1, true)
	_, acts := c.filtersView(-1, true)
	c.openTop(acts[1].top)
	c.loading = true
	v, acts := c.filtersView(-1, true)
	if !v.Loading || acts != nil || v.Browse == nil || len(v.Browse.Choices) != len(before.Browse.Choices) || len(v.Picked) != len(before.Picked) || c.unpick != nil {
		t.Fatalf("loading: %+v, %d choices taken; want the panel as before, none taken", v, len(acts))
	}
	c.loading = false
	c.set(feed(t, section("x", "Austen, Jane", "austen.xml", "")), false)
	if v, acts := c.filtersView(-1, true); v.Loading || v.Open == nil || v.Open.Name != "Authors" || len(acts) != 1 {
		t.Errorf("read: %+v; want Authors' list", v)
	}
}

// relSection is a section whose link has a rel (OPDS 1.2's sort/new, featured...).
func relSection(id, title, href, rel string) string {
	return `<entry><id>` + id + `</id><title>` + title + `</title><updated>2026-11-23T00:00:00Z</updated>
<link rel="` + rel + `" href="` + href + `" type="application/atom+xml;profile=opds-catalog"/></entry>`
}

// Landing: a first page of sections only tried for books in the catalogue's own order of what is newest, featured,
// holding books, then the rest, three at most; a section of sections left for the next; none: the first page, said.
func TestCatalogueLanding(t *testing.T) {
	front := feed(t, section("a", "Authors", "authors.xml", "")+
		relSection("p", "Popular", "popular.xml", "http://opds-spec.org/sort/popular")+
		section("b", "All books", "all.xml", "application/atom+xml;profile=opds-catalog;kind=acquisition")+
		relSection("n", "New", "new.xml", "http://opds-spec.org/sort/new"))
	if got := landOrder(front.Navigation); len(got) != 3 || got[0] != 3 || got[1] != 1 || got[2] != 2 {
		t.Errorf("order: %v, want New, Popular, All books", got)
	}

	c := &catalogues{list: []opds.Catalogue{{Name: "T", URL: "https://example.org/opds/root.xml"}}}
	c.set(front, false)
	j, ok := c.landNext()
	if !ok || c.sections[j].Title != "New" {
		t.Fatalf("first: %d %v", j, ok)
	}
	c.openTop(j)
	c.set(feed(t, section("x", "Austen, Jane", "austen.xml", "")), false) // sections only: the next
	if j, ok = c.landNext(); !ok || len(c.up) != 0 || c.sections[j].Title != "Popular" {
		t.Fatalf("after a section of sections: %d %v, %d up", j, ok, len(c.up))
	}
	c.openTop(j)
	c.set(feed(t, book("emma")), false) // books: landed
	if _, ok = c.landNext(); ok || len(c.up) != 1 || c.title != "Popular" || c.land != nil {
		t.Fatalf("landed: %v, %q", ok, c.title)
	}
	if h := c.headView(false); h.Section != "Popular" || !h.Browse || h.Chosen {
		t.Errorf("landed head: %+v", h)
	}
	if len(c.books) != 1 || len(c.sections) != 0 {
		t.Errorf("landed: %d books, %d sections", len(c.books), len(c.sections))
	}
	if _, ok = c.landNext(); ok { // a later page read: no landing again
		t.Errorf("landed twice")
	}

	// none holding books: the first page, Choose what to browse
	c = &catalogues{list: []opds.Catalogue{{Name: "T", URL: "https://example.org/opds/root.xml"}}}
	c.set(feed(t, section("a", "Authors", "authors.xml", "")), false)
	j, _ = c.landNext()
	c.openTop(j)
	c.set(feed(t, section("x", "Austen, Jane", "austen.xml", "")), false)
	if _, ok = c.landNext(); ok || len(c.up) != 0 {
		t.Fatalf("none: %v, %d up", ok, len(c.up))
	}
	if v := (&reader{}).catalogueView(c); !v.Choose || len(v.Books) != 0 {
		t.Errorf("none, view: %+v", v)
	}

	// gone elsewhere while landing (Browse): landing stops
	c = &catalogues{list: []opds.Catalogue{{Name: "T", URL: "https://example.org/opds/root.xml"}}}
	c.set(front, false)
	c.landNext()
	c.openTop(0) // Authors, chosen in Browse
	c.set(feed(t, section("x", "Austen, Jane", "austen.xml", "")), false)
	if _, ok = c.landNext(); ok || c.land != nil || len(c.up) != 1 {
		t.Errorf("elsewhere: %v, %d up", ok, len(c.up))
	}
}
