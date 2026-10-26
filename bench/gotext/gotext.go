// Package gotext is experiment 03's layout engine (pure Go, on go-text) as an engine.Document.
package gotext

import (
	"image"

	engine "einkengine"
	"einkengine/epub"
	"einklayout/layout"
)

func init() { engine.Register("gotext", Open) }

type doc struct {
	b *layout.Book
	d *layout.Drawer
}

func Open(b *epub.Book, o engine.Options) (engine.Document, error) {
	lb, err := layout.Build(b, layout.Options{Width: o.Width, Height: o.Height, Margin: float64(o.Margin),
		FontSize: float64(o.FontSize), Fallback: o.Fallback})
	if err != nil {
		return nil, err
	}
	lb.Shape()
	lb.Paginate()
	return &doc{lb, layout.NewDrawer()}, nil
}

func (d *doc) Pages() int                  { return len(d.b.Pages) }
func (d *doc) Draw(n int, img *image.Gray) { d.b.DrawPage(n, img, d.d) }
func (d *doc) Close()                      {}

func (d *doc) Find(spine int, id string) (int, []image.Rectangle) {
	p := d.b.IDPage(spine, id)
	if p < 0 {
		return -1, nil
	}
	return p, d.b.IDRects(spine, id)
}

func (d *doc) PageLines() []engine.PageInfo {
	var ps []engine.PageInfo
	for _, ls := range d.b.Pages {
		pi := engine.PageInfo{Spine: -1}
		for _, l := range ls {
			if pi.Spine < 0 {
				pi.Spine = l.Para.Spine
			}
			if l.Image == nil {
				pi.Lines++
			}
		}
		ps = append(ps, pi)
	}
	return ps
}
