// Package epub opens an EPUB with readium/go-toolkit: the publication (reading order, media overlays) for the
// reader, and its files by path for engines that read them through Go (a layout.Source).
package epub

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
	Path string // the EPUB file, for engines that open it themselves
	Pub  *pub.Publication
	ctx  context.Context
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
	return &Book{Path: abs, Pub: pb.Build(), ctx: ctx}, nil
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
