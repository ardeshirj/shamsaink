// Package opds reads OPDS catalogues into go-toolkit's model: every book is a manifest.Manifest (as in Readium's
// Kotlin and Swift toolkits, whose OPDS1Parser is the reference: read, not copied), and the feed around the books is
// the small Feed below.
package opds

import (
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
)

// Rels used by OPDS 1.2 and read here.
const (
	RelAcquisition = "http://opds-spec.org/acquisition" // and every rel that starts with it
	RelOpenAccess  = "http://opds-spec.org/acquisition/open-access"
	RelImage       = "http://opds-spec.org/image"
	RelThumbnail   = "http://opds-spec.org/image/thumbnail"
	RelFacet       = "http://opds-spec.org/facet"
	RelCollection  = "collection"
)

// Feed is one page of a catalogue: its links (self, start, next, search), the books on it as Readium manifests, the
// navigation entries (the menus of a catalogue's tree), facets and groups.
type Feed struct {
	Metadata     FeedMetadata
	Links        manifest.LinkList
	Facets       []Facet
	Groups       []Group
	Publications []manifest.Manifest
	Navigation   manifest.LinkList
}

// FeedMetadata is what the feed says about itself; the counts come from OpenSearch's elements, when present.
type FeedMetadata struct {
	Identifier    string
	Title         string
	Modified      *time.Time
	NumberOfItems *int
	ItemsPerPage  *int
	CurrentIndex  *int
}

// Facet is one of a feed's ways to narrow it (by language, by subject...), its choices as links.
type Facet struct {
	Title string
	Links manifest.LinkList
}

// Group gathers the entries that link to the same collection (OPDS 1's grouped feeds).
type Group struct {
	Title        string
	Links        manifest.LinkList
	Publications []manifest.Manifest
	Navigation   manifest.LinkList
}

// IndirectAcquisition is the chain of types behind an acquisition link that does not lead to the book itself (a page,
// a licence, an archive): each type, then what it holds.
type IndirectAcquisition struct {
	Type     string                `json:"type"`
	Children []IndirectAcquisition `json:"child,omitempty"`
}

// Price of an acquisition link.
type Price struct {
	Currency string  `json:"currency"`
	Value    float64 `json:"value"`
}
