// Package layout lays out and draws reflowable EPUB pages with go-text (shaping, bidi, line breaking): our own
// engine, for books with simple, known markup like rtlbook's (paragraphs, headings, a stylesheet, page-number
// markers), right to left first. It reads the book through a Source, so it does not depend on go-toolkit.
package layout

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/go-text/typesetting/font"
	"golang.org/x/net/html"
)

// Source gives the book's reading order and its files, by path from the EPUB's root.
type Source interface {
	Spine() []string
	Read(path string) ([]byte, error)
}

type Options struct {
	Width, Height int     // page, px
	Margin        float64 // page margin on all sides, px
	FontSize      float64 // the root font size, px
	Fallback      string  // font file for characters the book's fonts lack, and for generic families
	Orphans       int     // fewest lines of a paragraph left at the bottom of a page (CSS default 2)
	Widows        int     // fewest at the top of the next
}

// Para is one block of inline text, with what pagination needs.
type Para struct {
	Spine                   int
	Style                   *Style
	Text                    []rune
	spans                   []span
	X0, X1                  float64 // content box, page px
	Gap                     float64 // collapsed margins (and paddings) above
	Forced                  bool    // a page break before it: a new spine item, or break-before
	AvoidAfter, AvoidInside bool
	Lines                   []*Line
	page                    int      // where it is, for ids in a paragraph without lines
	sheets                  []*Sheet // its document's stylesheets, for @font-face
}

type span struct {
	a, b int
	st   *Style
}

type idPos struct {
	p    *Para
	a, b int
}

type Book struct {
	Paras []*Para
	Pages [][]*Line
	opts  Options
	ids   []map[string]idPos // per spine item
	fonts *fonts
}

type fonts struct {
	src      Source
	byPath   map[string]*font.Face
	fallback *font.Face
}

func (fs *fonts) load(p string) *font.Face {
	if f, ok := fs.byPath[p]; ok {
		return f
	}
	b, err := fs.src.Read(p)
	var f *font.Face
	if err == nil {
		f, err = font.ParseTTF(bytes.NewReader(b))
	}
	if err != nil {
		f = nil
	}
	fs.byPath[p] = f
	return f
}

// face picks the font for a style from the @font-face rules in scope, else the fallback.
func (fs *fonts) face(st *Style, sheets []*Sheet) *font.Face {
	for _, fam := range st.Family {
		var best *font.Face
		bestScore := -1
		for _, sh := range sheets {
			for _, ff := range sh.faces {
				if !strings.EqualFold(ff.family, fam) || ff.src == "" {
					continue
				}
				score := 0
				if ff.bold == st.Bold {
					score += 2
				}
				if ff.italic == st.Italic {
					score++
				}
				if score > bestScore {
					if f := fs.load(resolve(sh.base, ff.src)); f != nil {
						best, bestScore = f, score
					}
				}
			}
		}
		if best != nil {
			return best
		}
	}
	return fs.fallback
}

// resolve a reference made in the file at base to a path from the EPUB's root.
func resolve(base, ref string) string {
	if i := strings.IndexAny(ref, "#?"); i >= 0 {
		ref = ref[:i]
	}
	if u, err := url.PathUnescape(ref); err == nil {
		ref = u
	}
	if strings.HasPrefix(ref, "/") {
		return strings.TrimPrefix(path.Clean(ref), "/")
	}
	return path.Clean(path.Join(path.Dir(base), ref))
}

// builder walks one document's tree into paragraphs.
type builder struct {
	b        *Book
	spine    int
	sheets   []*Sheet
	posM     float64 // the pending collapsed margin: largest positive and most negative so far
	negM     float64
	pad      float64 // padding met since the last paragraph (does not collapse)
	forced   bool
	cur      *Para
	curBlock *Style
	curX     [2]float64
	space    bool     // the last rune added was a collapsible space
	pending  []string // ids met before any text of the next paragraph
	href     string   // the document's path
	open     []openID // inline elements with an id, while inside them
}

type openID struct {
	id string
	p  *Para
	a  int
}

func (bd *builder) margin(m float64) {
	if m > bd.posM {
		bd.posM = m
	}
	if m < bd.negM {
		bd.negM = m
	}
}

// sameText: styles that shape text the same way (an inline element like a span usually changes nothing).
func sameText(a, b *Style) bool {
	if a == b {
		return true
	}
	if a.Bold != b.Bold || a.Italic != b.Italic || a.Size != b.Size || a.Lang != b.Lang || len(a.Family) != len(b.Family) {
		return false
	}
	for i := range a.Family {
		if a.Family[i] != b.Family[i] {
			return false
		}
	}
	return true
}

func (bd *builder) styleFor(p *Para, st *Style) {
	if n := len(p.spans); n > 0 && sameText(p.spans[n-1].st, st) {
		return
	}
	if n := len(p.spans); n > 0 && p.spans[n-1].a == len(p.Text) {
		p.spans[n-1].st = st
		return
	}
	p.spans = append(p.spans, span{a: len(p.Text), b: len(p.Text), st: st})
}

// para returns the paragraph being filled, starting one if needed.
func (bd *builder) para() *Para {
	if bd.cur == nil {
		bd.cur = &Para{Spine: bd.spine, Style: bd.curBlock, X0: bd.curX[0], X1: bd.curX[1], sheets: bd.sheets}
		for _, id := range bd.pending {
			bd.b.ids[bd.spine][id] = idPos{bd.cur, 0, 0}
		}
		bd.pending = bd.pending[:0]
		for i := range bd.open { // inline elements that started before (in an empty paragraph)
			if bd.open[i].p == nil {
				bd.open[i].p = bd.cur
			}
		}
	}
	return bd.cur
}

func (bd *builder) text(s string, st *Style) {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			if bd.cur == nil || bd.space || len(bd.cur.Text) == 0 {
				continue
			}
			r = ' '
			bd.space = true
		} else {
			bd.space = false
		}
		p := bd.para()
		bd.styleFor(p, st)
		if noBreakBefore(r) && len(p.Text) > 0 && p.Text[len(p.Text)-1] == ' ' {
			// Unicode's line breaking (go-text) never breaks before closing punctuation, even after a space;
			// Chromium does, after any space. An invisible break opportunity gives the same lines.
			p.Text = append(p.Text, '\u200b')
		}
		p.Text = append(p.Text, r)
		p.spans[len(p.spans)-1].b = len(p.Text)
	}
}

// noBreakBefore: punctuation of the line-break classes CL, CP, EX, IS and SY (UAX #14 rule LB13), Latin and Arabic.
func noBreakBefore(r rune) bool {
	switch r {
	case ')', ']', '}', '!', '?', ',', '.', ':', ';', '/', '\u060c', '\u061b', '\u061f', '\u066b', '\u066c', '\u06d4':
		return true
	}
	return false
}

// close ends the paragraph being filled, if any.
func (bd *builder) close() {
	p := bd.cur
	if p == nil {
		return
	}
	bd.cur, bd.space = nil, false
	for len(p.Text) > 0 && p.Text[len(p.Text)-1] == ' ' {
		p.Text = p.Text[:len(p.Text)-1]
	}
	for i := range p.spans {
		p.spans[i].b = min(p.spans[i].b, len(p.Text))
	}
	for id, pos := range bd.b.ids[bd.spine] {
		if pos.p == p && pos.b > len(p.Text) {
			pos.b = len(p.Text)
			bd.b.ids[bd.spine][id] = pos
		}
	}
	p.Gap = bd.posM + bd.negM + bd.pad
	p.Forced = bd.forced
	bd.posM, bd.negM, bd.pad, bd.forced = 0, 0, 0, false
	bd.b.Paras = append(bd.b.Paras, p)
}

func isBlock(st *Style) bool { return st.Display != "inline" && st.Display != "inline-block" }

func (bd *builder) walk(n *html.Node, parent *Style, x0, x1 float64) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			bd.text(c.Data, parent)
		case html.ElementNode:
			bd.element(c, parent, x0, x1)
		}
	}
}

func (bd *builder) element(n *html.Node, parent *Style, x0, x1 float64) {
	st := computeStyle(n, parent, bd.sheets, bd.b.opts.FontSize)
	if st.Display == "none" || n.Data == "svg" || n.Data == "image" {
		return
	}
	if n.Data == "img" {
		bd.image(n, x0, x1)
		return
	}
	id, hasID := attr(n, "id")
	if !isBlock(st) {
		if n.Data == "br" {
			bd.text(" ", parent)
			bd.space = true
			return
		}
		if hasID {
			if bd.cur == nil {
				bd.open = append(bd.open, openID{id: id})
			} else {
				bd.open = append(bd.open, openID{id: id, p: bd.cur, a: len(bd.cur.Text)})
			}
		}
		bd.walk(n, st, x0, x1)
		if hasID {
			o := bd.open[len(bd.open)-1]
			bd.open = bd.open[:len(bd.open)-1]
			switch {
			case o.p == nil: // nothing yet: it marks the start of the next paragraph
				bd.pending = append(bd.pending, id)
			case o.p == bd.cur:
				bd.b.ids[bd.spine][id] = idPos{o.p, o.a, len(bd.cur.Text)}
			default:
				bd.b.ids[bd.spine][id] = idPos{o.p, o.a, len(o.p.Text)}
			}
		}
		return
	}
	bd.close()
	cbw := x1 - x0
	if st.BreakBefore == "always" {
		bd.forced = true
	}
	bd.margin(st.Margin[0].px(cbw))
	if pt := st.Padding[0].px(cbw); pt > 0 {
		bd.pad += bd.posM + bd.negM + pt
		bd.posM, bd.negM = 0, 0
	}
	cx0 := x0 + st.Margin[3].px(cbw) + st.Padding[3].px(cbw)
	cx1 := x1 - st.Margin[1].px(cbw) - st.Padding[1].px(cbw)
	first := len(bd.b.Paras)
	if hasID {
		bd.pending = append(bd.pending, id) // the block's id: the start of its first paragraph
	}
	prevBlock, prevX := bd.curBlock, bd.curX
	bd.curBlock, bd.curX = st, [2]float64{cx0, cx1}
	bd.walk(n, st, cx0, cx1)
	bd.close()
	bd.curBlock, bd.curX = prevBlock, prevX
	if hasID && len(bd.b.Paras) > first { // the whole first paragraph
		if pos, ok := bd.b.ids[bd.spine][id]; ok && pos.p == bd.b.Paras[first] {
			pos.b = len(pos.p.Text)
			bd.b.ids[bd.spine][id] = pos
		}
	}
	if pb := st.Padding[2].px(cbw); pb > 0 {
		bd.pad += pb // simplification: bottom padding is added to the next gap
	}
	bd.margin(st.Margin[2].px(cbw))
	if len(bd.b.Paras) > first {
		last := bd.b.Paras[len(bd.b.Paras)-1]
		if st.BreakAfter == "avoid" {
			last.AvoidAfter = true
		}
		if st.BreakInside == "avoid" {
			for _, p := range bd.b.Paras[first:] {
				p.AvoidInside = true
			}
		}
	}
	if st.BreakAfter == "always" {
		bd.forced = true
	}
}

// image places an <img> as a block of its own (covers, figures): its natural size, scaled down to fit the text area
// (max-width and max-height 100%, as rtlbook's stylesheet asks), aligned by text-align.
func (bd *builder) image(n *html.Node, x0, x1 float64) {
	src, ok := attr(n, "src")
	if !ok {
		return
	}
	p := resolve(bd.href, src)
	raw, err := bd.b.fonts.src.Read(p)
	if err != nil {
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return
	}
	bd.close()
	w, h := float64(cfg.Width), float64(cfg.Height)
	maxH := float64(bd.b.opts.Height) - 2*bd.b.opts.Margin
	scale := min(1, (x1-x0)/w, maxH/h)
	w, h = w*scale, h*scale
	st := bd.curBlock
	x := x0 + (x1-x0-w)/2
	switch st.Align {
	case "left":
		x = x0
	case "right":
		x = x1 - w
	case "start", "justify":
		if st.RTL {
			x = x1 - w
		} else {
			x = x0
		}
	}
	para := &Para{Spine: bd.spine, Style: st, X0: x0, X1: x1, sheets: bd.sheets}
	para.Lines = []*Line{{Para: para, Height: h, Image: &Image{Path: p, X: x, W: w, H: h}}}
	bd.cur = para
	bd.close()
}

// Build reads every document of the spine into paragraphs (no shaping yet).
func Build(src Source, opts Options) (*Book, error) {
	if opts.Orphans == 0 {
		opts.Orphans = 2
	}
	if opts.Widows == 0 {
		opts.Widows = 2
	}
	b := &Book{opts: opts, fonts: &fonts{src: src, byPath: map[string]*font.Face{}}}
	if opts.Fallback != "" { // a file on disk, not in the book
		if raw, err := os.ReadFile(opts.Fallback); err == nil {
			b.fonts.fallback, _ = font.ParseTTF(bytes.NewReader(raw))
		}
	}
	ua := ParseSheet(uaCSS, "", 0)
	cache := map[string]*Sheet{}
	order := 100000
	for si, href := range src.Spine() {
		b.ids = append(b.ids, map[string]idPos{})
		raw, err := src.Read(href)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", href, err)
		}
		doc, err := html.Parse(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", href, err)
		}
		sheets := []*Sheet{ua}
		var htmlEl *html.Node
		var find func(*html.Node)
		find = func(n *html.Node) {
			if n.Type == html.ElementNode {
				switch n.Data {
				case "html":
					htmlEl = n
				case "link":
					if rel, _ := attr(n, "rel"); strings.Contains(strings.ToLower(rel), "stylesheet") {
						if h, ok := attr(n, "href"); ok {
							p := resolve(href, h)
							sh, ok := cache[p]
							if !ok {
								if css, err := src.Read(p); err == nil {
									sh = ParseSheet(string(css), p, order)
									order += len(sh.rules)
								}
								cache[p] = sh
							}
							if sh != nil {
								sheets = append(sheets, sh)
							}
						}
					}
				case "style":
					if n.FirstChild != nil {
						sh := ParseSheet(n.FirstChild.Data, href, order)
						order += len(sh.rules)
						sheets = append(sheets, sh)
					}
				case "body":
					return
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				find(c)
			}
		}
		find(doc)
		if htmlEl == nil {
			continue
		}
		bd := &builder{b: b, spine: si, sheets: sheets, forced: si > 0, href: href}
		W := float64(opts.Width) - 2*opts.Margin
		bd.element(htmlEl, rootStyle(opts.FontSize), opts.Margin, opts.Margin+W)
		bd.close()
		for _, id := range bd.pending { // ids after the last text: the end of the document
			if n := len(b.Paras); n > 0 {
				p := b.Paras[n-1]
				b.ids[si][id] = idPos{p, len(p.Text), len(p.Text)}
			}
		}
	}
	return b, nil
}
