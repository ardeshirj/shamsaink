package opds

import (
	"net/url"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/mediatype"
)

// A feed with what the live ones surveyed do not show: relative and templated links, facets, groups, prices, indirect
// acquisition, both Dublin Core namespaces, a menu entry, OpenSearch counts.
const feed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:dc="http://purl.org/dc/terms/" xmlns:dce="http://purl.org/dc/elements/1.1/"
      xmlns:opds="http://opds-spec.org/2010/catalog" xmlns:os="http://a9.com/-/spec/opensearch/1.1/"
      xmlns:thr="http://purl.org/syndication/thread/1.0">
  <id>urn:test</id>
  <title>Test &amp; more</title>
  <updated>2026-11-01T10:00:00Z</updated>
  <os:totalResults>120</os:totalResults>
  <os:itemsPerPage>2</os:itemsPerPage>
  <link rel="next" href="page2.xml" type="application/atom+xml;profile=opds-catalog;kind=acquisition"/>
  <link rel="search" href="search?q={searchTerms}" type="application/atom+xml"/>
  <link rel="http://opds-spec.org/facet" href="?lang=fa" title="Farsi" opds:facetGroup="Language" opds:activeFacet="true" thr:count="7"/>
  <link rel="http://opds-spec.org/facet" href="?lang=en" title="English" opds:facetGroup="Language"/>
  <entry>
    <title>Book one</title>
    <id>urn:test:one</id>
    <updated>2026-10-20</updated>
    <dce:identifier>urn:uuid:1234</dce:identifier>
    <dc:language>fa</dc:language>
    <dc:issued>1963</dc:issued>
    <author><name>An Author</name><uri>/authors/1</uri></author>
    <category term="fiction" label="Fiction" scheme="http://example.org/s"/>
    <summary type="html">&lt;p&gt;Short.&lt;/p&gt;</summary>
    <link rel="collection" href="/new" title="New"/>
    <link rel="http://opds-spec.org/acquisition/buy" href="/buy/1" type="text/html">
      <opds:price currencycode="EUR">4.99</opds:price>
      <opds:indirectAcquisition type="application/vnd.adobe.adept+xml">
        <opds:indirectAcquisition type="application/epub+zip"/>
      </opds:indirectAcquisition>
    </link>
    <link rel="http://opds-spec.org/acquisition/open-access" href="../books/one.epub" type="application/epub+zip"/>
    <link rel="http://opds-spec.org/image/thumbnail" href="/covers/1.jpg" type="image/jpeg"/>
  </entry>
  <entry>
    <title>Poetry</title>
    <id>urn:test:poetry</id>
    <updated>2026-10-20T00:00:00Z</updated>
    <link rel="subsection" href="poetry.xml" type="application/atom+xml;profile=opds-catalog;kind=acquisition" thr:count="12"/>
  </entry>
</feed>`

func TestParseOPDS1(t *testing.T) {
	base, _ := url.Parse("https://example.org/opds/root.xml")
	f, err := ParseOPDS1(strings.NewReader(feed), base)
	if err != nil {
		t.Fatal(err)
	}
	if f.Metadata.Title != "Test & more" || f.Metadata.Identifier != "urn:test" || f.Metadata.Modified == nil {
		t.Errorf("metadata: %+v", f.Metadata)
	}
	if f.Metadata.NumberOfItems == nil || *f.Metadata.NumberOfItems != 120 || *f.Metadata.ItemsPerPage != 2 {
		t.Errorf("counts: %+v", f.Metadata)
	}
	if l := f.Links.FirstWithRel("next"); l == nil || l.Href.String() != "https://example.org/opds/page2.xml" {
		t.Errorf("next: %+v", l)
	}
	if l := f.Links.FirstWithRel("search"); l == nil || !l.Href.IsTemplated() ||
		l.Href.String() != "https://example.org/opds/search?q={searchTerms}" {
		t.Errorf("search: %+v %q", l, l.Href.String())
	}
	if len(f.Facets) != 1 || f.Facets[0].Title != "Language" || len(f.Facets[0].Links) != 2 {
		t.Fatalf("facets: %+v", f.Facets)
	}
	if fa := f.Facets[0].Links[0]; fa.Href.String() != "https://example.org/opds/root.xml?lang=fa" ||
		fa.Properties.Get("numberOfItems") != 7 || fa.Properties.Get("activeFacet") != true {
		t.Errorf("facet: %q %+v", fa.Href.String(), fa.Properties)
	}

	// the book went into its group, the menu entry into the feed's navigation
	if len(f.Publications) != 0 || len(f.Groups) != 1 || len(f.Groups[0].Publications) != 1 {
		t.Fatalf("groups: %+v", f.Groups)
	}
	if g := f.Groups[0]; g.Title != "New" || g.Links[0].Href.String() != "https://example.org/new" {
		t.Errorf("group: %+v", g)
	}
	if len(f.Navigation) != 1 || f.Navigation[0].Title != "Poetry" ||
		f.Navigation[0].Href.String() != "https://example.org/opds/poetry.xml" ||
		f.Navigation[0].Properties.Get("numberOfItems") != 12 {
		t.Errorf("navigation: %+v", f.Navigation)
	}

	p := f.Groups[0].Publications[0]
	md := p.Metadata
	if md.Identifier != "urn:uuid:1234" || len(md.AltIdentifiers) != 1 || md.AltIdentifiers[0].Value != "urn:test:one" {
		t.Errorf("ids: %q %+v", md.Identifier, md.AltIdentifiers)
	}
	if md.LocalizedTitle.String() != "Book one" || md.Modified == nil || md.Published == nil ||
		md.Published.Year() != 1963 || len(md.Languages) != 1 || md.Languages[0] != "fa" {
		t.Errorf("metadata: %+v", md)
	}
	if len(md.Authors) != 1 || md.Authors[0].Links[0].Href.String() != "https://example.org/authors/1" {
		t.Errorf("authors: %+v", md.Authors)
	}
	if len(md.Subjects) != 1 || md.Subjects[0].Code != "fiction" || md.Subjects[0].LocalizedName.String() != "Fiction" {
		t.Errorf("subjects: %+v", md.Subjects)
	}
	if md.Description != "<p>Short.</p>" {
		t.Errorf("description: %q", md.Description)
	}
	if len(p.Links) != 3 {
		t.Fatalf("links: %+v", p.Links)
	}
	buy := p.Links[0]
	if pr, ok := buy.Properties.Get("price").(Price); !ok || pr.Currency != "EUR" || pr.Value != 4.99 {
		t.Errorf("price: %+v", buy.Properties)
	}
	ia, ok := buy.Properties.Get("indirectAcquisition").([]IndirectAcquisition)
	if !ok || len(ia) != 1 || ia[0].Type != "application/vnd.adobe.adept+xml" || ia[0].Children[0].Type != "application/epub+zip" {
		t.Errorf("indirect: %+v", buy.Properties)
	}
	if open := p.Links[1]; open.Href.String() != "https://example.org/books/one.epub" ||
		open.MediaType == nil || !open.MediaType.Equal(&mediatype.EPUB) {
		t.Errorf("open access: %q %v", open.Href.String(), open.MediaType)
	}
}

func TestNotAFeed(t *testing.T) {
	for _, s := range []string{"", "<html><body>Down for maintenance</body></html>", `{"metadata":{}}`} {
		if _, err := ParseOPDS1(strings.NewReader(s), nil); err == nil {
			t.Errorf("%q: no error", s)
		}
	}
}

func TestLatin1(t *testing.T) {
	doc := "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><feed xmlns=\"http://www.w3.org/2005/Atom\"><title>Caf\xe9</title></feed>"
	f, err := ParseOPDS1(strings.NewReader(doc), nil)
	if err != nil || f.Metadata.Title != "Café" {
		t.Errorf("%v %+v", err, f)
	}
}

// A plain Atom feed's file (Standard Ebooks' open feed of new releases): an enclosure is taken when there is no
// acquisition link, an EPUB's only (not the Kobo edition).
func TestEnclosure(t *testing.T) {
	base, _ := url.Parse("https://example.org/feed")
	f, err := ParseOPDS1(strings.NewReader(`<feed xmlns="http://www.w3.org/2005/Atom"><id>x</id><title>New</title>
<entry><id>urn:se:1</id><title>Zeno</title>
<link href="/a.kepub.epub" rel="enclosure" type="application/kepub+zip"/>
<link href="/a.epub" rel="enclosure" type="application/epub+zip"/>
<link href="/a_advanced.epub" rel="enclosure" type="application/epub+zip"/></entry></feed>`), base)
	if err != nil {
		t.Fatal(err)
	}
	if l := EPUB(&f.Publications[0]); l == nil || l.Href.String() != "https://example.org/a.epub" {
		t.Errorf("%+v", l)
	}
}
