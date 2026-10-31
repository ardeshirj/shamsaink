//go:build blitz

// Package blitz is the Rust engine (engine/go, module shamsainkengine) as an engine.Document: the bench's adapter,
// turning an epub.Book into the spine and read function the binding takes. Registers "blitz".
package blitz

import (
	engine "einkengine"
	"einkengine/epub"
	blitz "shamsainkengine"
)

func init() { engine.Register("blitz", open) }

type doc struct{ *blitz.Book }

func open(b *epub.Book, o engine.Options) (engine.Document, error) {
	return doc{blitz.Open(b.Spine(), b.Read,
		blitz.Options{Width: o.Width, Height: o.Height, Margin: o.Margin, FontSize: float32(o.FontSize)})}, nil
}

func (d doc) PageLines() []engine.PageInfo {
	ps := d.Book.PageLines()
	out := make([]engine.PageInfo, len(ps))
	for i, p := range ps {
		out[i] = engine.PageInfo{Spine: p.Spine, Lines: p.Lines}
	}
	return out
}
