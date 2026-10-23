package layout

import (
	"bytes"
	"image"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"

	"einklayout/raster"
)

const eps = 0.01

// Paginate puts the lines on pages: a new page for each spine item and forced break; at least Orphans lines of a
// split paragraph at the bottom and, when that allows it, Widows at the top; a paragraph that avoids breaks inside moves whole; a heading
// (break-after: avoid) moves to the next page if the start of what follows would not fit after it. Margins at the top
// of a page after an unforced break are dropped, as browsers do.
func (b *Book) Paginate() {
	b.Pages = nil
	H := float64(b.opts.Height) - 2*b.opts.Margin
	var cur []*Line
	y := 0.0
	page := 0
	newPage := func() {
		b.Pages = append(b.Pages, cur)
		cur, y = nil, 0
		page++
	}
	started := false // anything placed yet
	for i, p := range b.Paras {
		if p.Forced && started && (len(cur) > 0 || y > 0) {
			newPage()
		}
		gap := p.Gap
		if y == 0 && !p.Forced && started {
			gap = 0 // the top of a page after an unforced break
		}
		n := len(p.Lines)
		if n == 0 {
			y += gap
			p.page = page
			continue
		}
		lh := p.Lines[0].Height
		fits := func(from float64) int { return int(math.Floor((H-from)/lh + eps)) }
		// keep a heading with the start of the next paragraph
		if p.AvoidAfter && y > 0 && i+1 < len(b.Paras) {
			q := b.Paras[i+1]
			need := y + gap + float64(n)*lh
			if len(q.Lines) > 0 && !q.Forced {
				need += q.Gap + float64(min(len(q.Lines), b.opts.Orphans))*q.Lines[0].Height
			}
			if need > H+eps {
				newPage()
				gap = 0
			}
		}
		y += gap
		started = true
		p.page = page
		k := 0 // lines placed
		for k < n {
			room := fits(y)
			rest := n - k
			switch {
			case room >= rest:
				room = rest
			case y == 0: // a page of its own is not enough: split anyway, keeping widows if possible
				if rest-room < b.opts.Widows && rest > b.opts.Widows {
					room = rest - b.opts.Widows
				}
			case p.AvoidInside && k == 0:
				room = 0
			default:
				// widows give way to orphans: when keeping Widows lines for the next page would leave fewer than
				// Orphans here, the widows rule is dropped (Chromium does this; a 3-line paragraph splits 2+1)
				if rest-room < b.opts.Widows && (k > 0 || rest-b.opts.Widows >= b.opts.Orphans) {
					room = rest - b.opts.Widows
				}
				if k == 0 && room < b.opts.Orphans {
					room = 0
				}
			}
			if room <= 0 {
				if y == 0 {
					room = 1
				} else {
					newPage()
					continue
				}
			}
			for _, l := range p.Lines[k : k+room] {
				l.Page, l.Top = page, b.opts.Margin+y
				cur = append(cur, l)
				y += l.Height
			}
			k += room
			if k < n {
				newPage()
			}
		}
	}
	if len(cur) > 0 || len(b.Pages) == 0 {
		b.Pages = append(b.Pages, cur)
	}
}

// IDPage is the page (from 0) where the element with this id in spine item spine starts, -1 if not found.
func (b *Book) IDPage(spine int, id string) int {
	if spine < 0 || spine >= len(b.ids) {
		return -1
	}
	pos, ok := b.ids[spine][id]
	if !ok {
		return -1
	}
	for _, l := range pos.p.Lines {
		if pos.a < l.B || (pos.a == l.B && l == pos.p.Lines[len(pos.p.Lines)-1]) {
			return l.Page
		}
	}
	return pos.p.page
}

// IDRects are the boxes of the element's text on its first page, one per line (glyph extents, line box height).
func (b *Book) IDRects(spine int, id string) []image.Rectangle {
	pos, ok := b.ids[spine][id]
	if !ok || pos.a >= pos.b {
		return nil
	}
	page := b.IDPage(spine, id)
	var rs []image.Rectangle
	for _, l := range pos.p.Lines {
		if l.B <= pos.a || l.A >= pos.b || l.Page != page {
			continue
		}
		x0, x1 := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, g := range l.Glyphs {
			// a cluster can hold several runes (a mark joins the rune before it, even a space): count the glyph if
			// any rune of the range in it is not a space
			in := false
			for c := max(int(g.Cluster), pos.a); c < min(int(g.Cluster+g.Runes), pos.b); c++ {
				in = in || !isSpace(pos.p.Text[c])
			}
			if in {
				x0, x1 = min(x0, g.X), max(x1, g.X+g.Adv)
			}
		}
		if x0 < x1 {
			rs = append(rs, image.Rect(int(math.Floor(float64(x0))), int(math.Floor(l.Top)),
				int(math.Ceil(float64(x1))), int(math.Ceil(l.Top+l.Height))))
		}
	}
	return rs
}

// IDs lists the ids of a spine item (for tools).
func (b *Book) IDs(spine int) []string {
	var ids []string
	for id := range b.ids[spine] {
		ids = append(ids, id)
	}
	return ids
}

// Drawer draws pages, keeping each glyph's image between pages (what an e-reader keeps between page turns), and the
// last picture it scaled.
type Drawer struct {
	cache  *raster.GlyphCache
	imgKey Image
	img    *image.Gray
}

func NewDrawer() *Drawer { return &Drawer{cache: raster.NewGlyphCache()} }

// picture decodes and scales an image to its size on the page, in greys.
func (b *Book) picture(d *Drawer, im *Image) *image.Gray {
	if d.img != nil && d.imgKey == *im {
		return d.img
	}
	raw, err := b.fonts.src.Read(im.Path)
	if err != nil {
		return nil
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	g := image.NewGray(image.Rect(0, 0, int(math.Round(im.W)), int(math.Round(im.H))))
	xdraw.ApproxBiLinear.Scale(g, g.Bounds(), src, src.Bounds(), draw.Src, nil)
	d.imgKey, d.img = *im, g
	return g
}

func (d *Drawer) Stats() (hits, misses int) { return d.cache.Hits, d.cache.Misses }

// DrawPage draws page i (from 0) into img, black on white.
func (b *Book) DrawPage(i int, img *image.Gray, d *Drawer) {
	for k := range img.Pix {
		img.Pix[k] = 255
	}
	pg := &raster.Page{Img: img}
	clip := img.Bounds()
	for _, l := range b.Pages[i] {
		if l.Image != nil {
			if g := b.picture(d, l.Image); g != nil {
				at := image.Pt(int(math.Round(l.Image.X)), int(math.Round(l.Top)))
				draw.Draw(img, g.Bounds().Add(at), g, image.Point{}, draw.Src)
			}
			continue
		}
		base := float32(l.Top + l.Baseline)
		for _, g := range l.Glyphs {
			d.cache.Draw(pg, g.Face, g.GID, g.Size, g.X, base+g.DY, 0, clip)
		}
	}
}
