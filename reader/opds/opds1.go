package opds

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

const (
	nsAtom       = "http://www.w3.org/2005/Atom"
	nsOPDS       = "http://opds-spec.org/2010/catalog"
	nsOpenSearch = "http://a9.com/-/spec/opensearch/1.1/"
	nsThread     = "http://purl.org/syndication/thread/1.0"
)

// The XML as read. Atom's elements are matched with their namespace; Dublin Core's by their local name only, since
// feeds use both of its namespaces (dc/terms and dc/elements/1.1) and neither clashes with an Atom name read here.
type (
	xText struct {
		Type  string `xml:"type,attr"`
		Text  string `xml:",chardata"`
		Inner string `xml:",innerxml"`
	}
	xPerson struct {
		Name string `xml:"http://www.w3.org/2005/Atom name"`
		URI  string `xml:"http://www.w3.org/2005/Atom uri"`
	}
	xCategory struct {
		Term   string `xml:"term,attr"`
		Label  string `xml:"label,attr"`
		Scheme string `xml:"scheme,attr"`
	}
	xPrice struct {
		Currency string `xml:"currencycode,attr"`
		Value    string `xml:",chardata"`
	}
	xIndirect struct {
		Type     string      `xml:"type,attr"`
		Children []xIndirect `xml:"http://opds-spec.org/2010/catalog indirectAcquisition"`
	}
	xLink struct {
		Href       string      `xml:"href,attr"`
		Rel        string      `xml:"rel,attr"`
		Type       string      `xml:"type,attr"`
		Title      string      `xml:"title,attr"`
		Count      string      `xml:"http://purl.org/syndication/thread/1.0 count,attr"`
		FacetGroup string      `xml:"http://opds-spec.org/2010/catalog facetGroup,attr"`
		Active     string      `xml:"http://opds-spec.org/2010/catalog activeFacet,attr"`
		Prices     []xPrice    `xml:"http://opds-spec.org/2010/catalog price"`
		Indirect   []xIndirect `xml:"http://opds-spec.org/2010/catalog indirectAcquisition"`
	}
	xEntry struct {
		ID         string      `xml:"http://www.w3.org/2005/Atom id"`
		Title      string      `xml:"http://www.w3.org/2005/Atom title"`
		Updated    string      `xml:"http://www.w3.org/2005/Atom updated"`
		Published  string      `xml:"http://www.w3.org/2005/Atom published"`
		Summary    *xText      `xml:"http://www.w3.org/2005/Atom summary"`
		Content    *xText      `xml:"http://www.w3.org/2005/Atom content"`
		Authors    []xPerson   `xml:"http://www.w3.org/2005/Atom author"`
		Links      []xLink     `xml:"http://www.w3.org/2005/Atom link"`
		Categories []xCategory `xml:"http://www.w3.org/2005/Atom category"`
		Identifier []string    `xml:"identifier"`
		Language   []string    `xml:"language"`
		Issued     string      `xml:"issued"`
		Publisher  []string    `xml:"publisher"`
	}
)

// ParseOPDS1 reads an OPDS 1.x feed from r, its links resolved against base (the address the feed was read from,
// after redirects). The feed is read as a stream, one entry at a time, so a large feed costs its result, not its text.
func ParseOPDS1(r io.Reader, base *url.URL) (*Feed, error) {
	d := xml.NewDecoder(r)
	d.Strict = false // feeds in the wild: stray ampersands, HTML's named entities
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charsetReader

	var f Feed
	depth := 0
	seenFeed := false
	groups := map[string]int{} // a collection's href → its index in f.Groups
	facets := map[string]int{}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("opds: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				if t.Name.Local != "feed" || t.Name.Space != nsAtom {
					return nil, fmt.Errorf("opds: not an Atom feed (<%s>)", t.Name.Local)
				}
				seenFeed = true
				continue
			}
			if depth != 2 {
				continue
			}
			depth-- // DecodeElement reads up to and including the end element
			switch {
			case t.Name.Space == nsAtom && t.Name.Local == "entry":
				var e xEntry
				if err := d.DecodeElement(&e, &t); err != nil {
					return nil, fmt.Errorf("opds: entry: %w", err)
				}
				addEntry(&f, &e, base, groups)
			case t.Name.Space == nsAtom && t.Name.Local == "link":
				var l xLink
				if err := d.DecodeElement(&l, &t); err != nil {
					return nil, fmt.Errorf("opds: link: %w", err)
				}
				link, ok := toLink(&l, base)
				if !ok {
					continue
				}
				if l.Rel == RelFacet {
					title := l.FacetGroup
					i, seen := facets[title]
					if !seen {
						i = len(f.Facets)
						facets[title] = i
						f.Facets = append(f.Facets, Facet{Title: title})
					}
					f.Facets[i].Links = append(f.Facets[i].Links, link)
				} else {
					f.Links = append(f.Links, link)
				}
			case t.Name.Space == nsAtom && (t.Name.Local == "id" || t.Name.Local == "title" || t.Name.Local == "updated"):
				var s string
				if err := d.DecodeElement(&s, &t); err != nil {
					return nil, fmt.Errorf("opds: %s: %w", t.Name.Local, err)
				}
				s = strings.TrimSpace(s)
				switch t.Name.Local {
				case "id":
					f.Metadata.Identifier = s
				case "title":
					f.Metadata.Title = s
				case "updated":
					f.Metadata.Modified = parseDate(s)
				}
			case t.Name.Space == nsOpenSearch:
				var s string
				if err := d.DecodeElement(&s, &t); err != nil {
					return nil, fmt.Errorf("opds: %s: %w", t.Name.Local, err)
				}
				n, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil {
					continue
				}
				switch t.Name.Local {
				case "totalResults":
					f.Metadata.NumberOfItems = &n
				case "itemsPerPage":
					f.Metadata.ItemsPerPage = &n
				case "startIndex":
					f.Metadata.CurrentIndex = &n
				}
			default:
				if err := d.Skip(); err != nil {
					return nil, fmt.Errorf("opds: %w", err)
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	if !seenFeed {
		return nil, fmt.Errorf("opds: empty document")
	}
	return &f, nil
}

// addEntry puts an entry in the feed: a book when it has an acquisition link, otherwise a navigation link (a menu of
// the catalogue's tree); into its group when it links to a collection.
func addEntry(f *Feed, e *xEntry, base *url.URL, groups map[string]int) {
	var group *Group
	for i := range e.Links {
		if e.Links[i].Rel != RelCollection {
			continue
		}
		if l, ok := toLink(&e.Links[i], base); ok {
			key := l.Href.String()
			gi, seen := groups[key]
			if !seen {
				gi = len(f.Groups)
				groups[key] = gi
				f.Groups = append(f.Groups, Group{Title: e.Links[i].Title, Links: manifest.LinkList{l}})
			}
			group = &f.Groups[gi]
			break
		}
	}

	if isPublication(e) {
		m := toManifest(e, base)
		if group != nil {
			group.Publications = append(group.Publications, m)
		} else {
			f.Publications = append(f.Publications, m)
		}
		return
	}
	nav, ok := navigationLink(e, base)
	if !ok {
		return
	}
	if group != nil {
		group.Navigation = append(group.Navigation, nav)
	} else {
		f.Navigation = append(f.Navigation, nav)
	}
}

func isPublication(e *xEntry) bool {
	for _, l := range e.Links {
		if strings.HasPrefix(l.Rel, RelAcquisition) {
			return true
		}
	}
	return false
}

// navigationLink is the link a menu entry leads to: its first link to an OPDS feed, else its first link; titled with
// the entry's title.
func navigationLink(e *xEntry, base *url.URL) (manifest.Link, bool) {
	pick := -1
	for i, l := range e.Links {
		if l.Rel == RelCollection {
			continue
		}
		if strings.Contains(l.Type, "profile=opds-catalog") || strings.HasPrefix(l.Type, "application/atom+xml") {
			pick = i
			break
		}
		if pick < 0 {
			pick = i
		}
	}
	if pick < 0 {
		return manifest.Link{}, false
	}
	link, ok := toLink(&e.Links[pick], base)
	if ok {
		link.Title = strings.TrimSpace(e.Title)
	}
	return link, ok
}

func toManifest(e *xEntry, base *url.URL) manifest.Manifest {
	var m manifest.Manifest
	md := &m.Metadata
	md.LocalizedTitle = manifest.NewLocalizedStringFromString(strings.TrimSpace(e.Title))
	id := strings.TrimSpace(e.ID)
	for _, s := range e.Identifier {
		if s = strings.TrimSpace(s); s != "" {
			md.Identifier = s
			break
		}
	}
	if md.Identifier == "" {
		md.Identifier = id
	} else if id != "" && id != md.Identifier {
		md.AltIdentifiers = append(md.AltIdentifiers, manifest.AltIdentifier{Value: id, Scheme: "atom:id"})
	}
	md.Modified = parseDate(e.Updated)
	if md.Published = parseDate(e.Issued); md.Published == nil {
		md.Published = parseDate(e.Published)
	}
	for _, s := range e.Language {
		if s = strings.TrimSpace(s); s != "" {
			md.Languages = append(md.Languages, s)
		}
	}
	for _, s := range e.Publisher {
		if s = strings.TrimSpace(s); s != "" {
			md.Publishers = append(md.Publishers, manifest.Contributor{LocalizedName: manifest.NewLocalizedStringFromString(s)})
		}
	}
	for _, a := range e.Authors {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		c := manifest.Contributor{LocalizedName: manifest.NewLocalizedStringFromString(name)}
		if a.URI != "" {
			if l, ok := toLink(&xLink{Href: a.URI}, base); ok {
				c.Links = manifest.LinkList{l}
			}
		}
		md.Authors = append(md.Authors, c)
	}
	for _, c := range e.Categories {
		name := c.Label
		if name == "" {
			name = c.Term
		}
		if name == "" {
			continue
		}
		md.Subjects = append(md.Subjects, manifest.Subject{
			LocalizedName: manifest.NewLocalizedStringFromString(name), Scheme: c.Scheme, Code: c.Term})
	}
	if e.Content != nil && strings.TrimSpace(e.Content.Text+e.Content.Inner) != "" {
		md.Description = text(e.Content)
	} else if e.Summary != nil {
		md.Description = text(e.Summary)
	}
	for i := range e.Links {
		if e.Links[i].Rel == RelCollection {
			continue
		}
		if l, ok := toLink(&e.Links[i], base); ok {
			m.Links = append(m.Links, l)
		}
	}
	return m
}

// text is an Atom text construct as it is: plain text, or HTML (escaped, so its character data) or XHTML (its inner
// markup), for the reader to clean.
func text(t *xText) string {
	if t.Type == "xhtml" {
		return strings.TrimSpace(t.Inner)
	}
	return strings.TrimSpace(t.Text)
}

// toLink is a link with its href resolved against base, its OPDS properties (count, price, indirect acquisition) in
// Properties as Readium's model has them. A link without a usable href is left out.
func toLink(l *xLink, base *url.URL) (manifest.Link, bool) {
	raw := strings.TrimSpace(l.Href)
	if raw == "" {
		return manifest.Link{}, false
	}
	templated := strings.Contains(raw, "{")
	abs := raw
	if !templated {
		u, err := url.Parse(raw)
		if err != nil {
			return manifest.Link{}, false
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		abs = u.String()
	} else if base != nil && !strings.Contains(raw, "://") {
		// a relative template: resolve the part before its first brace, keep the rest as it is
		cut := strings.Index(raw, "{")
		if u, err := url.Parse(raw[:cut]); err == nil {
			abs = base.ResolveReference(u).String() + raw[cut:]
		}
	}
	href, err := manifest.NewHREFFromString(abs, templated)
	if err != nil {
		return manifest.Link{}, false
	}
	link := manifest.Link{Href: href, Title: strings.TrimSpace(l.Title)}
	if l.Rel != "" {
		link.Rels = manifest.Strings{l.Rel}
	}
	if l.Type != "" {
		if mt, err := mediatype.NewOfString(l.Type); err == nil {
			link.MediaType = &mt
		}
	}
	props := manifest.Properties{}
	if n, err := strconv.Atoi(strings.TrimSpace(l.Count)); err == nil {
		props["numberOfItems"] = n
	}
	if len(l.Prices) > 0 {
		if v, err := strconv.ParseFloat(strings.TrimSpace(l.Prices[0].Value), 64); err == nil {
			props["price"] = Price{Currency: l.Prices[0].Currency, Value: v}
		}
	}
	if len(l.Indirect) > 0 {
		props["indirectAcquisition"] = indirect(l.Indirect)
	}
	if l.Active == "true" {
		props["activeFacet"] = true
	}
	if len(props) > 0 {
		link.Properties = props
	}
	return link, true
}

func indirect(xs []xIndirect) []IndirectAcquisition {
	out := make([]IndirectAcquisition, 0, len(xs))
	for _, x := range xs {
		out = append(out, IndirectAcquisition{Type: x.Type, Children: indirect(x.Children)})
	}
	return out
}

// parseDate reads the dates feeds use: RFC 3339 with or without a zone or seconds' fraction, a day, a month, a year.
func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02", "2006-01", "2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

// charsetReader accepts what Go's decoder does not know but feeds declare: Latin-1 and its Windows superset read as
// Latin-1 (close enough for metadata); anything else is an error, as before.
func charsetReader(label string, r io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return r, nil
	case "iso-8859-1", "latin1", "latin-1", "windows-1252", "cp1252":
		return &latin1{r: r}, nil
	}
	return nil, fmt.Errorf("charset %q not supported", label)
}

type latin1 struct {
	r   io.Reader
	buf []byte
}

func (l *latin1) Read(p []byte) (int, error) {
	if len(l.buf) == 0 {
		in := make([]byte, len(p)/2+1)
		n, err := l.r.Read(in)
		for _, b := range in[:n] {
			l.buf = append(l.buf, string(rune(b))...)
		}
		if n == 0 {
			return 0, err
		}
	}
	n := copy(p, l.buf)
	l.buf = l.buf[n:]
	return n, nil
}
