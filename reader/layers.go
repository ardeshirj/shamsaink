package main

import (
	"image"

	blitz "shamsainkengine"
	"shamsainkreader/ui"
)

// The screen as layers: the page buffers hold the engine's pixels only, and what goes over the page shown (its
// highlights, a selection's bars, the narration's underline, the panels) is drawn over a copy of it in
// rd.over each time part of the screen is shown, only that part put together. So a highlight made or removed never draws
// the page again, and a narrated word puts together its own box only.

// What goes over the page shown, worked out once for it: its highlights' bands.
type pageLayers struct {
	p       *drawn // the page buffer it is for, at doc and at (nil: to work out again)
	doc, at int
	bands   []image.Rectangle
}

// layers is what goes over the page shown, worked out again if the page has changed since.
func (s *session) layers() *pageLayers {
	l, p, r := &s.lay, s.shown, s.r
	if l.p == p && l.doc == p.doc && l.at == p.at {
		return l
	}
	*l = pageLayers{p: p, doc: p.doc, at: p.at}
	if r.marks.in(p.doc) {
		first, _ := r.book.Layout(p.doc)
		l.bands = bands(r.book.Words(first+p.at), func(w blitz.Word) bool { return r.marks.at(p.doc, w) >= 0 })
	}
	return l
}

// relayer has what goes over the page worked out again, for a highlight made or removed, or the pages drawn again.
func (s *session) relayer() { s.lay.p = nil }

// compose puts the screen together in rd.over within r: the page shown, then each layer over it, the lowest first.
func (s *session) compose(r image.Rectangle) *image.Gray {
	over, page := s.rd.over, s.shown.img
	r = r.Intersect(over.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i := y * over.Stride
		copy(over.Pix[i+r.Min.X:i+r.Max.X], page.Pix[i+r.Min.X:i+r.Max.X])
	}
	darken(over, clip(s.layers().bands, r))
	if s.sel != nil {
		fill(over, clip(s.sel.bars, r))
	}
	if s.nar != nil {
		fill(over, clip(s.nar.bars, r))
	}
	panels := []*ui.Panel{s.mn, s.nav, s.sheet}
	if s.nav != nil || s.sheet != nil { // the bar over the dock or the sheet, and the bar's list
		panels = append(panels, s.bar, s.list)
	}
	for _, m := range panels {
		if m != nil && m.Rect().Overlaps(r) {
			m.Over(over)
		}
	}
	return over
}

// refresh shows the screen within r, put together, with waveform.
func (s *session) refresh(r image.Rectangle, waveform uint32) {
	r = r.Intersect(s.rd.over.Rect)
	if !r.Empty() {
		s.scr.ShowPart(s.compose(r), r, waveform, false)
	}
}

// showPage shows the whole screen, put together.
func (s *session) showPage(waveform uint32, flash bool) {
	s.scr.Show(s.compose(s.rd.over.Rect), waveform, flash)
}

// clip is the boxes of rs within r, the empty ones left out.
func clip(rs []image.Rectangle, r image.Rectangle) []image.Rectangle {
	var out []image.Rectangle
	for _, b := range rs {
		if b = b.Intersect(r); !b.Empty() {
			out = append(out, b)
		}
	}
	return out
}

// fill blackens the boxes.
func fill(img *image.Gray, rs []image.Rectangle) {
	for _, r := range rs {
		r = r.Intersect(img.Rect)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			row := img.Pix[y*img.Stride:]
			for x := r.Min.X; x < r.Max.X; x++ {
				row[x] = 0
			}
		}
	}
}
