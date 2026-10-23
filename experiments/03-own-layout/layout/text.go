package layout

import (
	"slices"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// Glyph is a glyph placed on its line: X from the page's left edge, DY from the baseline (down).
type Glyph struct {
	X, DY, Adv float32
	Size       float32
	GID        font.GID
	Cluster    int32 // index of its first rune in the paragraph's text
	Runes      int32 // how many runes its cluster holds
	Face       *font.Face
}

// Line is one line of a paragraph, in visual order (left to right).
type Line struct {
	Para     *Para
	A, B     int // its runes in Para.Text
	Glyphs   []Glyph
	Height   float64 // the line box
	Baseline float64 // from the top of the line box
	Page     int     // set by Paginate
	Top      float64
	Natural  float32 // width before justification, and the width available, px
	Avail    float32
	Image    *Image // a line that is an image instead of text
}

// Image is a picture placed on a page, at X (left edge), W×H px.
type Image struct {
	Path    string
	X, W, H float64
}

// slack is kept free at the end of each line when breaking: words that fill a line to within it go to the next
// one, as in Chromium, whose text widths come out a fraction of a pixel larger (measured: 3 lines of 2134 in the
// test book were full to within 0.2 px and broke one word earlier in Chromium).
const slack = 0.25

func fx(v float64) fixed.Int26_6 { return fixed.Int26_6(v * 64) }
func fl(v fixed.Int26_6) float32 { return float32(v) / 64 }

// fontmap gives the style's face for every rune it has, the fallback for the rest.
type fontmap struct{ primary, fallback *font.Face }

func (m fontmap) ResolveFace(r rune) *font.Face {
	if m.fallback == nil || m.primary == nil {
		if m.primary != nil {
			return m.primary
		}
		return m.fallback
	}
	if _, ok := m.primary.NominalGlyph(r); ok {
		return m.primary
	}
	return m.fallback
}

func isSpace(r rune) bool { return r == ' ' || r == ' ' }

type shaper struct {
	hb      shaping.HarfbuzzShaper
	seg     shaping.Segmenter
	wrap    shaping.LineWrapper
	runs    []shaping.Output
	visual  []shaping.Output
	faceFor map[*Style]*font.Face
}

// Shape shapes, wraps, aligns and justifies every paragraph into lines.
func (b *Book) Shape() {
	s := &shaper{faceFor: map[*Style]*font.Face{}}
	for _, p := range b.Paras {
		s.para(b, p)
	}
}

func (s *shaper) face(b *Book, p *Para, st *Style) *font.Face {
	f, ok := s.faceFor[st]
	if !ok {
		f = b.fonts.face(st, p.sheets)
		s.faceFor[st] = f
	}
	return f
}

func (s *shaper) para(b *Book, p *Para) {
	if len(p.Text) == 0 || len(p.Lines) > 0 {
		return
	}
	dir := di.DirectionLTR
	if p.Style.RTL {
		dir = di.DirectionRTL
	}
	s.runs = s.runs[:0]
	for _, sp := range p.spans {
		if sp.a >= sp.b {
			continue
		}
		face := s.face(b, p, sp.st)
		in := shaping.Input{Text: p.Text, RunStart: sp.a, RunEnd: sp.b, Direction: dir, Face: face,
			Size: fx(sp.st.Size), Language: language.NewLanguage(sp.st.Lang)}
		for _, part := range s.seg.Split(in, fontmap{face, b.fonts.fallback}) {
			s.runs = append(s.runs, s.hb.Shape(part))
		}
	}
	if len(s.runs) == 0 {
		return
	}
	st := p.Style
	lh := st.LineHeightPx()
	// the baseline sits in the middle of the leading, from the paragraph's own font (CSS's strut)
	base := lh * 0.8
	if f := s.face(b, p, st); f != nil {
		if ext, ok := f.FontHExtents(); ok {
			scale := st.Size / float64(f.Upem())
			asc, desc := float64(ext.Ascender)*scale, -float64(ext.Descender)*scale
			base = (lh-(asc+desc))/2 + asc
		}
	}
	s.wrap.Prepare(shaping.WrapConfig{Direction: dir}, p.Text, shaping.NewSliceIterator(s.runs))
	first := true
	for {
		indent := 0.0
		if first {
			indent = st.Indent
		}
		width := p.X1 - p.X0 - indent
		wl, done := s.wrap.WrapNextLineF(fx(width - slack))
		if wl.Line == nil {
			break
		}
		last := done
		l := s.place(p, wl.Line, indent, &last)
		l.Height, l.Baseline = lh, base
		p.Lines = append(p.Lines, l)
		first = false
		if done {
			break
		}
	}
}

// place positions a wrapped line's glyphs: visual order, alignment, and justification on the spaces between words
// (the trailing space the wrapper zeroed does not count: counting it left every justified line one gap short).
func (s *shaper) place(p *Para, runs shaping.Line, indent float64, last *bool) *Line {
	s.visual = append(s.visual[:0], runs...)
	slices.SortFunc(s.visual, func(a, b shaping.Output) int { return int(a.VisualIndex - b.VisualIndex) })
	l := &Line{Para: p, A: len(p.Text), B: 0}
	var width float32
	spaces := 0
	n := 0
	for _, r := range s.visual {
		l.A = min(l.A, r.Runes.Offset)
		l.B = max(l.B, r.Runes.Offset+r.Runes.Count)
		width += fl(r.Advance)
		for _, g := range r.Glyphs {
			if g.ClusterIndex < len(p.Text) && isSpace(p.Text[g.ClusterIndex]) && g.RuneCount == 1 && g.Advance != 0 {
				spaces++
			}
		}
		n += len(r.Glyphs)
	}
	if l.B > 0 && p.Text[l.B-1] == ' ' {
		*last = true // a <br>: not justified
	}
	st := p.Style
	left, right := p.X0, p.X1
	if st.RTL {
		right -= indent
	} else {
		left += indent
	}
	avail := float32(right - left)
	align := st.Align
	if align == "justify" && (*last || spaces == 0 || width >= avail) {
		align = "start"
	}
	if align == "start" {
		align = map[bool]string{true: "right", false: "left"}[st.RTL]
	} else if align == "end" {
		align = map[bool]string{true: "left", false: "right"}[st.RTL]
	}
	l.Natural, l.Avail = width, avail
	x := float32(left)
	var extra float32
	switch align {
	case "right":
		x = float32(right) - width
	case "center":
		x = float32(left) + (avail-width)/2
	case "justify":
		extra = (avail - width) / float32(spaces)
	}
	l.Glyphs = make([]Glyph, 0, n)
	for _, r := range s.visual {
		size := fl(r.Size)
		for _, g := range r.Glyphs {
			adv := fl(g.Advance)
			if extra > 0 && g.Advance != 0 && g.RuneCount == 1 && g.ClusterIndex < len(p.Text) && isSpace(p.Text[g.ClusterIndex]) {
				adv += extra
			}
			l.Glyphs = append(l.Glyphs, Glyph{X: x + fl(g.XOffset), DY: -fl(g.YOffset), Adv: adv, Size: size,
				GID: g.GlyphID, Cluster: int32(g.ClusterIndex), Runes: int32(g.RuneCount), Face: r.Face})
			x += adv
		}
	}
	return l
}
