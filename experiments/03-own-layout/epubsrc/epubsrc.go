// Package epubsrc opens an EPUB with readium/go-toolkit and gives it to the layout engine as a layout.Source.
package epubsrc

import (
	"context"
	"path/filepath"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type Book struct {
	Pub *pub.Publication
	ctx context.Context
}

// Open parses the EPUB with go-toolkit's EPUB parser directly (its streamer also brings the PDF and audiobook ones).
func Open(ctx context.Context, path string) (*Book, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u, err := url.FromFilepath(abs)
	if err != nil {
		return nil, err
	}
	a := asset.File(u)
	f, err := a.CreateFetcher(ctx, asset.Dependencies{ArchiveFactory: archive.NewArchiveFactory()}, "")
	if err != nil {
		return nil, err
	}
	pb, err := epub.NewParser(nil).Parse(ctx, a, f)
	if err != nil {
		return nil, err
	}
	return &Book{Pub: pb.Build(), ctx: ctx}, nil
}

func (b *Book) Spine() []string {
	var s []string
	for _, l := range b.Pub.Manifest.ReadingOrder {
		s = append(s, l.Href.String())
	}
	return s
}

func (b *Book) Read(path string) ([]byte, error) {
	u, err := url.URLFromDecodedPath(path)
	if err != nil {
		return nil, err
	}
	data, rerr := b.Pub.Get(b.ctx, manifest.Link{Href: manifest.NewHREF(u)}).Read(b.ctx, 0, 0)
	if rerr != nil {
		return nil, rerr
	}
	return data, nil
}
