package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"time"

	blitz "shamsainkengine"
	"shamsainkreader/device"
	"shamsainkreader/files"
	"shamsainkreader/ui"
)

// A highlight: a span of one spine document's text, from the start of one word to the end of another, each as the
// engine gives it (its inline block in document order, its byte in that block's text), so it stays when the book is
// laid out again for other settings; its text, and when it was made.
type highlight struct {
	Doc  int       `json:"doc"`
	From [2]int    `json:"from"` // block, byte
	To   [2]int    `json:"to"`   // block, byte (the end of the last word)
	Text string    `json:"text"`
	Made time.Time `json:"made"`
}

// highlights are a book's, kept in DIR/highlights/KEY.json in the order made.
type highlights struct {
	path string
	list []highlight
}

func loadHighlights(dir, key string) *highlights {
	h := &highlights{path: filepath.Join(dir, "highlights", key+".json")}
	if b, err := os.ReadFile(h.path); err == nil {
		if err := json.Unmarshal(b, &h.list); err != nil {
			// kept for a person to look at; the next save would write over it
			h.list = nil
			os.Rename(h.path, h.path+".bad")
			fmt.Printf("highlights: %s does not parse (%v); kept as %s.bad\n", h.path, err, filepath.Base(h.path))
		}
	}
	return h
}

func (h *highlights) save() {
	b, _ := json.MarshalIndent(h.list, "", "  ")
	files.Write(h.path, append(b, '\n'))
}

// add keeps hl, taking in the highlights it overlaps (one span over them all), so no text is under two; its text is
// read by text when it has none or took any in. It returns hl as kept.
func (h *highlights) add(hl highlight, text func(doc int, from, to [2]int) string) highlight {
	took := false
	// again until none is left: a span made longer can reach one passed over
	for more := true; more; {
		more = false
		h.list = slices.DeleteFunc(h.list, func(o highlight) bool {
			if o.Doc != hl.Doc || !before(o.From, hl.To) || !before(hl.From, o.To) {
				return false
			}
			if before(o.From, hl.From) {
				hl.From = o.From
			}
			if before(hl.To, o.To) {
				hl.To = o.To
			}
			more, took = true, true
			return true
		})
	}
	if took || hl.Text == "" {
		hl.Text = text(hl.Doc, hl.From, hl.To)
	}
	h.list = append(h.list, hl)
	return hl
}

// tidy takes in the highlights kept overlapping before add did; true if any were.
func (h *highlights) tidy(text func(doc int, from, to [2]int) string) bool {
	old := h.list
	h.list = nil
	for _, hl := range old {
		h.add(hl, text)
	}
	return len(h.list) < len(old)
}

// before says whether (block, byte) a comes before b in the text.
func before(a, b [2]int) bool { return a[0] < b[0] || a[0] == b[0] && a[1] < b[1] }

// covers says whether word w of document doc is in the highlight.
func (hl highlight) covers(doc int, w blitz.Word) bool {
	return hl.Doc == doc && !before([2]int{w.Block, w.End}, hl.From) && before([2]int{w.Block, w.Start}, hl.To)
}

// at is the highlight word w of document doc is in, -1 if none.
func (h *highlights) at(doc int, w blitz.Word) int {
	return slices.IndexFunc(h.list, func(hl highlight) bool { return hl.covers(doc, w) })
}

func (h *highlights) in(doc int) bool {
	return slices.ContainsFunc(h.list, func(hl highlight) bool { return hl.Doc == doc })
}

// bands are the boxes to mark for the words in, one per line (the words' boxes joined, the spaces between them
// with them).
func bands(words []blitz.Word, in func(blitz.Word) bool) []image.Rectangle {
	var out []image.Rectangle
	for _, w := range words {
		if !in(w) {
			continue
		}
		// words of one line share its top and bottom
		if i := slices.IndexFunc(out, func(r image.Rectangle) bool { return r.Min.Y == w.Box.Min.Y && r.Max.Y == w.Box.Max.Y }); i >= 0 {
			out[i] = out[i].Union(w.Box)
		} else {
			out = append(out, w.Box)
		}
	}
	return out
}

// darken greys the boxes on the page as a highlighter would: white to light grey, black stays black.
func darken(img *image.Gray, rs []image.Rectangle) {
	for _, r := range rs {
		r = r.Intersect(img.Rect)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			row := img.Pix[y*img.Stride:]
			for x := r.Min.X; x < r.Max.X; x++ {
				row[x] = uint8(uint32(row[x]) * 13 / 16)
			}
		}
	}
}

// text is document doc's text from one (block, byte) to another.
func (r *reading) text(doc int, from, to [2]int) string {
	return r.book.Text(doc, from[0], from[1], to[0], to[1])
}

// wordAt is the word at (x, y) among words: the one under it, else the nearest on its line within reach of a
// finger; -1 if none.
func wordAt(words []blitz.Word, x, y int) int {
	const reach = 40 // px
	best, dist := -1, reach+1
	for i, w := range words {
		if y < w.Box.Min.Y || y >= w.Box.Max.Y {
			continue
		}
		d := max(w.Box.Min.X-x, x-w.Box.Max.X+1, 0)
		if d < dist {
			best, dist = i, d
		}
	}
	return best
}

// A selection being made: the page's words, the one held first and the one the finger is on, and the bars under the
// words between them as shown.
type selection struct {
	words    []blitz.Word
	from, to int
	bars     []image.Rectangle
}

// span is the selection's first and last word in the text.
func (s *selection) span() (a, b blitz.Word) {
	a, b = s.words[s.from], s.words[s.to]
	if before([2]int{b.Block, b.Start}, [2]int{a.Block, a.Start}) {
		a, b = b, a
	}
	return a, b
}

func (s *selection) in(w blitz.Word) bool {
	a, b := s.span()
	p := [2]int{w.Block, w.Start}
	return !before(p, [2]int{a.Block, a.Start}) && !before([2]int{b.Block, b.Start}, p)
}

func union(rs []image.Rectangle) image.Rectangle {
	var u image.Rectangle
	for _, r := range rs {
		u = u.Union(r)
	}
	return u
}

// held handles a finger held on a word: on a highlight, its menu (Remove highlight); else the words from that one to
// the one under the finger are chosen, underlined, and once the finger is lifted the menu by them (Highlight).
func (s *session) held(g device.Gesture) {
	r := s.r
	switch {
	case s.mn != nil || s.nav != nil || s.sheet != nil:
	case g.Hold == device.Held:
		words := r.book.Words(r.page())
		i := wordAt(words, g.X, g.Y)
		if i < 0 {
			break
		}
		if s.unmark = r.marks.at(r.doc, words[i]); s.unmark >= 0 {
			hl := r.marks.list[s.unmark]
			s.from = words[i]
			s.openMark(union(bands(words, func(w blitz.Word) bool { return hl.covers(r.doc, w) })), true)
			break
		}
		s.sel = &selection{words: words, from: i, to: i}
		s.showSel()
	case s.sel != nil:
		sel := s.sel
		j := wordAt(sel.words, g.X, g.Y)
		if g.Hold == device.Dragged {
			if j >= 0 && j != sel.to {
				sel.to = j
				s.showSel()
			}
			break
		}
		if j >= 0 && j != sel.to {
			sel.to = j
			s.showSel()
		}
		s.from, _ = sel.span()
		s.openMark(union(bands(sel.words, sel.in)), false)
	}
}

// openMark opens the highlight's menu by the words in box (made: a highlight's, else words chosen).
func (s *session) openMark(box image.Rectangle, made bool) {
	s.mn = ui.DrawHighlightMenu(made, s.narrated, s.rd.fonts, s.scr.W)
	s.mn.Hang(box, 0, s.scr.H)
	s.mn.Centre(box, s.scr.W)
	s.refresh(s.mn.Rect(), device.WfREAGL)
	r := s.mn.Rect() // where it is, for a test that taps its entries by name (devtest scenario)
	fmt.Printf("highlight menu: %s at %d,%d\n", map[bool]string{true: "a highlight", false: "words chosen"}[made], r.Min.X, r.Min.Y)
}

// closeMark closes the highlight's menu, and the words chosen if any (their bars rubbed out).
func (s *session) closeMark() {
	rect := s.mn.Rect()
	if s.sel != nil {
		rect = rect.Union(union(s.sel.bars))
	}
	s.mn, s.sel, s.unmark = nil, nil, -1
	s.refresh(rect, device.WfREAGL)
}

// mark makes a highlight of the words chosen.
func (s *session) mark() {
	r, sel := s.r, s.sel
	t := time.Now()
	a, b := sel.span()
	hl := r.marks.add(highlight{Doc: r.doc, From: [2]int{a.Block, a.Start}, To: [2]int{b.Block, b.End}, Made: time.Now()}, r.text)
	r.marks.save()
	// its bands (and any it took in) over the page, the bars and the menu gone
	rect := s.mn.Rect().Union(union(sel.bars))
	s.mn, s.sel = nil, nil
	s.relayer()
	bs := bands(sel.words, func(w blitz.Word) bool { return hl.covers(r.doc, w) })
	s.refresh(rect.Union(union(bs)), device.WfREAGL)
	fmt.Printf("highlight %d made: %d lines, %d bytes, %.0f ms\n", len(r.marks.list), len(bs), len(hl.Text), ms(t))
}

// showSel underlines the selection's words (with A2, as the narration's), the bars it had erased.
func (s *session) showSel() {
	sel := s.sel
	old := union(sel.bars)
	sel.bars = sel.bars[:0]
	for _, b := range bands(sel.words, sel.in) {
		sel.bars = append(sel.bars, image.Rect(b.Min.X, b.Max.Y-8, b.Max.X, b.Max.Y-2))
	}
	s.refresh(old.Union(union(sel.bars)), device.WfA2)
}
