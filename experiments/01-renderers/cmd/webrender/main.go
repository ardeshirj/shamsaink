// webrender route: the chapter's XHTML + CSS laid out by github.com/benoitkugler/webrender (a Go port of
// WeasyPrint, paged media), drawn by our own greyscale backend into 1072×1448 PNGs.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"einkrender/raster"

	"github.com/benoitkugler/webrender/backend"
	"github.com/benoitkugler/webrender/css/parser"
	"github.com/benoitkugler/webrender/html/document"
	"github.com/benoitkugler/webrender/html/tree"
	"github.com/benoitkugler/webrender/logger"
	"github.com/benoitkugler/webrender/matrix"
	"github.com/benoitkugler/webrender/text"
	"github.com/benoitkugler/webrender/utils"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
)

type fl = backend.Fl

const pageCSS = `@page { size: 1072px 1448px; margin: 60px; } html { font-size: 34px; }`

// --- the drawing target ---

type doc struct {
	pages     []*canvas
	fonts     map[text.FontOrigin]*fontEntry
	glyphs    *raster.GlyphCache
	pageTimes []time.Duration
	pageStart time.Time
	strokes   int
}

type fontEntry struct {
	chars *backend.FontChars
	face  *font.Face
}

type state struct {
	ctm       matrix.Transform
	fill      uint8
	fillAlpha fl
	clip      image.Rectangle
	bbox      [4]fl
}

type canvas struct {
	d    *doc
	page *raster.Page
	st   state
	path *raster.Path
}

func (d *doc) AddPage(left, top, right, bottom fl) backend.Page {
	d.endPage()
	w, h := int(math.Round(float64(right))), int(math.Round(float64(bottom)))
	p := raster.NewPage(w, h)
	c := &canvas{d: d, page: p, path: raster.NewPath()}
	// PDF-style device space (y up) to pixels (y down).
	c.st = state{ctm: matrix.New(1, 0, 0, -1, 0, fl(h)), fill: 0, fillAlpha: 1, clip: p.Img.Bounds()}
	d.pages = append(d.pages, c)
	d.pageStart = time.Now()
	return c
}

func (d *doc) endPage() {
	if !d.pageStart.IsZero() {
		d.pageTimes = append(d.pageTimes, time.Since(d.pageStart))
		d.pageStart = time.Time{}
	}
}

func (d *doc) CreateAnchors([][]backend.Anchor)                                  {}
func (d *doc) SetAttachments([]backend.Attachment)                               {}
func (d *doc) EmbedFile(string, backend.Attachment)                              {}
func (d *doc) SetMetadata(string, utils.DocumentMetadata)                        {}
func (d *doc) SetBookmarks([]backend.BookmarkNode)                               {}
func (c *canvas) AddInternalLink(_, _, _, _ fl, _ string)                        {}
func (c *canvas) AddExternalLink(_, _, _, _ fl, _ string)                        {}
func (c *canvas) AddFileAnnotation(_, _, _, _ fl, _ string)                      {}
func (c *canvas) SetMediaBox(_, _, _, _ fl)                                      {}
func (c *canvas) SetTrimBox(_, _, _, _ fl)                                       {}
func (c *canvas) SetBleedBox(_, _, _, _ fl)                                      {}
func (c *canvas) GetBoundingBox() (fl, fl, fl, fl)                               { b := c.st.bbox; return b[0], b[1], b[2], b[3] }
func (c *canvas) SetBoundingBox(l, t, r, b fl)                                   { c.st.bbox = [4]fl{l, t, r, b} }
func (c *canvas) State() backend.GraphicState                                    { return c }
func (c *canvas) SetAlphaMask(backend.Canvas)                                    {}
func (c *canvas) SetColorPattern(backend.Canvas, fl, fl, matrix.Transform, bool) {}
func (c *canvas) SetBlendingMode(string)                                         {}
func (c *canvas) SetLineWidth(fl)                                                {}
func (c *canvas) SetDash([]fl, fl)                                               {}
func (c *canvas) SetStrokeOptions(backend.StrokeOptions)                         {}
func (c *canvas) SetTextPaint(backend.PaintOp)                                   {}
func (c *canvas) DrawRasterImage(backend.RasterImage, fl, fl)                    {}
func (c *canvas) DrawGradient(backend.GradientLayout, fl, fl)                    {}
func (c *canvas) GetTransform() matrix.Transform                                 { return c.st.ctm }
func (c *canvas) DrawWithOpacity(fl, backend.Canvas)                             {}

// NewGroup draws straight onto the page (opacity groups are not needed for book text).
func (c *canvas) NewGroup(x, y, w, h fl) backend.Canvas {
	return &canvas{d: c.d, page: c.page, st: c.st, path: raster.NewPath()}
}

func (c *canvas) OnNewStack(f func()) {
	saved := c.st
	f()
	c.st = saved
}

// Transform applies mt before the current CTM (PDF "cm").
func (c *canvas) Transform(mt matrix.Transform) {
	o := c.st.ctm
	c.st.ctm = matrix.New(
		o.A*mt.A+o.C*mt.B, o.B*mt.A+o.D*mt.B,
		o.A*mt.C+o.C*mt.D, o.B*mt.C+o.D*mt.D,
		o.A*mt.E+o.C*mt.F+o.E, o.B*mt.E+o.D*mt.F+o.F)
}

func apply(m matrix.Transform, x, y fl) (float32, float32) {
	return float32(m.A*x + m.C*y + m.E), float32(m.B*x + m.D*y + m.F)
}

func (c *canvas) MoveTo(x, y fl) { c.path.MoveTo(apply(c.st.ctm, x, y)) }
func (c *canvas) LineTo(x, y fl) { c.path.LineTo(apply(c.st.ctm, x, y)) }
func (c *canvas) ClosePath()     { c.path.Close() }
func (c *canvas) CubicTo(x1, y1, x2, y2, x3, y3 fl) {
	a, b := apply(c.st.ctm, x1, y1)
	e, f := apply(c.st.ctm, x2, y2)
	g, h := apply(c.st.ctm, x3, y3)
	c.path.CubeTo(a, b, e, f, g, h)
}
func (c *canvas) Rectangle(x, y, w, h fl) {
	c.MoveTo(x, y)
	c.LineTo(x+w, y)
	c.LineTo(x+w, y+h)
	c.LineTo(x, y+h)
	c.ClosePath()
}

func (c *canvas) Clip(evenOdd bool) {
	if !c.path.Empty() {
		c.st.clip = c.st.clip.Intersect(c.path.Bounds())
	}
	c.path = raster.NewPath()
}

func (c *canvas) SetAlpha(a fl, stroke bool) {
	if !stroke {
		c.st.fillAlpha = a
	}
}

func lum(col parser.RGBA) uint8 {
	l := 0.299*col.R + 0.587*col.G + 0.114*col.B
	// over white with the colour's alpha
	v := 1 - col.A*(1-l)
	return uint8(math.Round(float64(v) * 255))
}

func (c *canvas) SetColorRgba(col parser.RGBA, stroke bool) {
	if !stroke {
		c.st.fill = lum(col)
	}
}

func (c *canvas) Paint(op backend.PaintOp) {
	if op&(backend.FillNonZero|backend.FillEvenOdd) != 0 && c.st.fill < 250 {
		c.page.Fill(c.path, c.st.fill, c.st.clip)
	}
	if op&backend.Stroke != 0 {
		c.d.strokes++
	}
	c.path = raster.NewPath()
}

func (c *canvas) AddFont(f backend.Font, content []byte) *backend.FontChars {
	if e, ok := c.d.fonts[f.Origin()]; ok {
		return e.chars
	}
	face, err := font.ParseTTF(bytes.NewReader(content))
	if err != nil {
		fmt.Fprintln(os.Stderr, "font:", err)
	}
	fmt.Fprintf(os.Stderr, "font used: %q %+v\n", f.Origin().File, f.Description().Family)
	e := &fontEntry{chars: &backend.FontChars{Cmap: map[backend.GID][]rune{}, Extents: map[backend.GID]backend.GlyphExtents{}}, face: face}
	c.d.fonts[f.Origin()] = e
	return e.chars
}

// DrawText follows the PDF TJ semantics webrender's own PDF backend uses: before each glyph move by Offset,
// after it by the font width W minus Kerning (so the net pen move is the shaped advance).
func (c *canvas) DrawText(texts []backend.TextDrawing) {
	for _, t := range texts {
		tm := t.Matrix()
		full := c.st.ctm
		// glyph space -> text space -> user -> device
		comb := mul(full, tm)
		scale := float32(math.Sqrt(math.Abs(float64(comb.A*comb.D - comb.B*comb.C))))
		pen := fl(0)
		for _, run := range t.Runs {
			e := c.d.fonts[run.Font.Origin()]
			for _, g := range run.Glyphs {
				pen += g.Offset
				if g.Glyph != 0xFFFFFFFF && e != nil && e.face != nil {
					x, y := apply(comb, pen*t.FontSize/1000, g.Rise*t.FontSize/1000)
					c.d.glyphs.Draw(c.page, e.face, font.GID(g.Glyph), float32(t.FontSize)*scale, x, y, c.st.fill, c.st.clip)
				}
				w := fl(0)
				if e != nil {
					w = fl(e.chars.Extents[g.Glyph].Width)
				}
				pen += w - fl(g.Kerning)
			}
		}
	}
}

// mul returns the transform applying b then a.
func mul(a, b matrix.Transform) matrix.Transform {
	return matrix.New(
		a.A*b.A+a.C*b.B, a.B*b.A+a.D*b.B,
		a.A*b.C+a.C*b.D, a.B*b.C+a.D*b.D,
		a.A*b.E+a.C*b.F+a.E, a.B*b.E+a.D*b.F+a.F)
}

// fetch is webrender's default fetcher with the MIME parameters dropped ("text/css; charset=utf-8" is
// otherwise refused as a stylesheet type).
func fetch(u string) (utils.RemoteRessource, error) {
	r, err := utils.DefaultUrlFetcher(u)
	if i := strings.IndexByte(r.MimeType, ';'); i > 0 {
		r.MimeType = strings.TrimSpace(r.MimeType[:i])
	}
	return r, err
}

func maxRSSMB() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Maxrss) / 1024
}

func main() {
	in, outDir := os.Args[1], os.Args[2]
	logger.ProgressLogger.SetOutput(io.Discard)
	logger.WarningLogger.SetOutput(os.Stderr)

	fm := fontscan.NewFontMap(nil)
	if err := fm.UseSystemFonts(os.TempDir()); err != nil {
		fmt.Fprintln(os.Stderr, "system fonts:", err)
	}
	fc := text.NewFontConfigurationGotext(fm)

	t0 := time.Now()
	raw, err := os.ReadFile(in)
	if err != nil {
		panic(err)
	}
	// webrender's HTML5 parser takes the XML declaration for the root element: drop it.
	if i := bytes.Index(raw, []byte("?>")); bytes.HasPrefix(raw, []byte("<?xml")) && i > 0 {
		raw = raw[i+2:]
	}
	abs, _ := filepath.Abs(in)
	h, err := tree.NewHTML(utils.InputString(string(raw)), "file://"+abs, fetch, "")
	if err != nil {
		panic(err)
	}
	user, err := tree.NewCSSDefault(utils.InputString(pageCSS + os.Getenv("EXTRA_CSS")))
	if err != nil {
		panic(err)
	}
	parsed := time.Since(t0)
	t1 := time.Now()
	rendered := document.Render(h, []tree.CSS{user}, true, fc)
	layout := time.Since(t1)

	d := &doc{fonts: map[text.FontOrigin]*fontEntry{}, glyphs: raster.NewGlyphCache()}
	t2 := time.Now()
	rendered.Write(d, 4.0/3, nil) // zoom 4/3: one PDF point per CSS px, so 1072×1448 px pages
	d.endPage()
	drawAll := time.Since(t2)

	os.MkdirAll(outDir, 0o755)
	for i, p := range d.pages {
		p.page.Save(fmt.Sprintf("%s/page-%02d.png", outDir, i+1))
	}
	var first, rest time.Duration
	for i, t := range d.pageTimes {
		if i == 0 {
			first = t
		} else {
			rest += t
		}
	}
	avgRest := time.Duration(0)
	if len(d.pageTimes) > 1 {
		avgRest = rest / time.Duration(len(d.pageTimes)-1)
	}
	res := map[string]any{
		"renderer":         "webrender",
		"pages":            len(d.pages),
		"parse_ms":         parsed.Milliseconds(),
		"layout_ms":        layout.Milliseconds(),
		"draw_all_ms":      drawAll.Milliseconds(),
		"draw_first_ms":    float64(first.Microseconds()) / 1000,
		"draw_avg_rest_ms": float64(avgRest.Microseconds()) / 1000,
		"glyph_cache":      fmt.Sprintf("%d hits / %d misses", d.glyphs.Hits, d.glyphs.Misses),
		"strokes_skipped":  d.strokes,
		"max_rss_mb":       maxRSSMB(),
	}
	j, _ := json.MarshalIndent(res, "", "  ")
	os.WriteFile(outDir+"/timings.json", j, 0o644)
	fmt.Println(string(j))
}
