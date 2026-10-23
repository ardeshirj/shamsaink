// webwords: where webrender puts every word of a book made by experiment 03's spanwords, for its tools/linecmp. Each
// spine document is laid out on its own (webrender has no book), with the shared page settings and the reader's
// default font named (serif), and nothing is drawn: webrender hands every element id's position to the backend
// (CreateAnchors), which is all the comparison needs. Writes boxes.json (each word's page and top-left), pages.json
// (each page's spine item and lines of text: the distinct word tops on it) and timings.json. FONTS_DIR holds the fonts
// (no system fonts). EXTRA_CSS is added to the page settings.
//
//	webwords SPANNED_DIR OUT_DIR FONTS_DIR
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/benoitkugler/webrender/backend"
	"github.com/benoitkugler/webrender/css/parser"
	"github.com/benoitkugler/webrender/html/document"
	"github.com/benoitkugler/webrender/html/tree"
	"github.com/benoitkugler/webrender/logger"
	"github.com/benoitkugler/webrender/matrix"
	"github.com/benoitkugler/webrender/text"
	"github.com/benoitkugler/webrender/utils"
	"github.com/go-text/typesetting/fontscan"
)

type fl = backend.Fl

const pageCSS = `@page { size: 1072px 1448px; margin: 60px; } html { font-size: 34px; font-family: serif; }`

// --- a backend that draws nothing and keeps the anchors ---

type doc struct {
	heights []fl
	anchors [][]backend.Anchor
}

func (d *doc) AddPage(left, top, right, bottom fl) backend.Page {
	d.heights = append(d.heights, bottom)
	return nop{}
}
func (d *doc) CreateAnchors(a [][]backend.Anchor)         { d.anchors = a }
func (d *doc) SetAttachments([]backend.Attachment)        {}
func (d *doc) EmbedFile(string, backend.Attachment)       {}
func (d *doc) SetMetadata(string, utils.DocumentMetadata) {}
func (d *doc) SetBookmarks([]backend.BookmarkNode)        {}

type nop struct{}

func (nop) AddInternalLink(_, _, _, _ fl, _ string)                        {}
func (nop) AddExternalLink(_, _, _, _ fl, _ string)                        {}
func (nop) AddFileAnnotation(_, _, _, _ fl, _ string)                      {}
func (nop) SetMediaBox(_, _, _, _ fl)                                      {}
func (nop) SetTrimBox(_, _, _, _ fl)                                       {}
func (nop) SetBleedBox(_, _, _, _ fl)                                      {}
func (nop) GetBoundingBox() (fl, fl, fl, fl)                               { return 0, 0, 0, 0 }
func (nop) SetBoundingBox(_, _, _, _ fl)                                   {}
func (n nop) State() backend.GraphicState                                  { return n }
func (nop) SetAlphaMask(backend.Canvas)                                    {}
func (nop) SetColorPattern(backend.Canvas, fl, fl, matrix.Transform, bool) {}
func (nop) SetBlendingMode(string)                                         {}
func (nop) SetLineWidth(fl)                                                {}
func (nop) SetDash([]fl, fl)                                               {}
func (nop) SetStrokeOptions(backend.StrokeOptions)                         {}
func (nop) SetTextPaint(backend.PaintOp)                                   {}
func (nop) DrawRasterImage(backend.RasterImage, fl, fl)                    {}
func (nop) DrawGradient(backend.GradientLayout, fl, fl)                    {}
func (nop) GetTransform() matrix.Transform                                 { return matrix.Identity() }
func (nop) DrawWithOpacity(fl, backend.Canvas)                             {}
func (n nop) NewGroup(_, _, _, _ fl) backend.Canvas                        { return n }
func (nop) OnNewStack(f func())                                            { f() }
func (nop) Transform(matrix.Transform)                                     {}
func (nop) MoveTo(_, _ fl)                                                 {}
func (nop) LineTo(_, _ fl)                                                 {}
func (nop) CubicTo(_, _, _, _, _, _ fl)                                    {}
func (nop) ClosePath()                                                     {}
func (nop) Rectangle(_, _, _, _ fl)                                        {}
func (nop) Clip(bool)                                                      {}
func (nop) SetAlpha(fl, bool)                                              {}
func (nop) SetColorRgba(parser.RGBA, bool)                                 {}
func (nop) Paint(backend.PaintOp)                                          {}
func (nop) DrawText([]backend.TextDrawing)                                 {}
func (nop) AddFont(backend.Font, []byte) *backend.FontChars {
	return &backend.FontChars{Cmap: map[backend.GID][]rune{}, Extents: map[backend.GID]backend.GlyphExtents{}}
}

// fetch is webrender's default fetcher with the MIME parameters dropped (as in cmd/webrender).
func fetch(u string) (utils.RemoteRessource, error) {
	r, err := utils.DefaultUrlFetcher(u)
	if i := strings.IndexByte(r.MimeType, ';'); i > 0 {
		r.MimeType = strings.TrimSpace(r.MimeType[:i])
	}
	return r, err
}

type word struct {
	Spine int    `json:"spine"`
	ID    string `json:"id"`
}

type box struct {
	Spine int               `json:"spine"`
	ID    string            `json:"id"`
	Page  int               `json:"page"`
	Rects []image.Rectangle `json:"rects"`
}

type pageInfo struct {
	Spine int `json:"spine"`
	Lines int `json:"lines"`
}

func main() {
	src, out, fonts := os.Args[1], os.Args[2], os.Args[3]
	logger.ProgressLogger.SetOutput(io.Discard)
	logger.WarningLogger.SetOutput(io.Discard)

	fm := fontscan.NewFontMap(nil)
	files, _ := filepath.Glob(filepath.Join(fonts, "*.ttf"))
	for _, f := range files {
		r, err := os.Open(f)
		check(err)
		check(fm.AddFont(r, f, ""))
	}
	fc := text.NewFontConfigurationGotext(fm)
	user, err := tree.NewCSSDefault(utils.InputString(pageCSS + os.Getenv("EXTRA_CSS")))
	check(err)

	spineTxt, err := os.ReadFile(filepath.Join(src, "spine.txt"))
	check(err)
	spine := strings.Fields(string(spineTxt))

	at := map[word]box{}
	var pages []pageInfo
	var layout time.Duration
	for si, file := range spine {
		raw, err := os.ReadFile(filepath.Join(src, file))
		check(err)
		// webrender's HTML5 parser takes the XML declaration for the root element: drop it
		if i := bytes.Index(raw, []byte("?>")); bytes.HasPrefix(raw, []byte("<?xml")) && i > 0 {
			raw = raw[i+2:]
		}
		abs, _ := filepath.Abs(filepath.Join(src, file))
		t := time.Now()
		h, err := tree.NewHTML(utils.InputString(string(raw)), "file://"+abs, fetch, "")
		check(err)
		rendered := document.Render(h, []tree.CSS{user}, true, fc)
		layout += time.Since(t)
		d := &doc{}
		rendered.Write(d, 4.0/3, nil) // one PDF point per CSS px; anchors come back with y up from the page's bottom
		for pi, as := range d.anchors {
			n := len(pages) + pi
			var tops []int
			for _, a := range as {
				if !strings.HasPrefix(a.Name, "w") {
					continue
				}
				x, y := int(a.X+0.5), int(d.heights[pi]-a.Y+0.5)
				at[word{si, a.Name}] = box{si, a.Name, n, []image.Rectangle{image.Rect(x, y, x+1, y+1)}}
				tops = append(tops, y)
			}
			pages = append(pages, pageInfo{si, lines(tops)})
		}
		fmt.Fprintf(os.Stderr, "%s: %d pages\n", file, len(d.anchors))
	}

	var words []word
	raw, err := os.ReadFile(filepath.Join(src, "words.json"))
	check(err)
	check(json.Unmarshal(raw, &words))
	boxes := make([]box, 0, len(words))
	for _, w := range words {
		b, ok := at[w]
		if !ok {
			b = box{w.Spine, w.ID, -1, nil}
		}
		boxes = append(boxes, b)
	}
	check(os.MkdirAll(out, 0o755))
	write(filepath.Join(out, "boxes.json"), boxes)
	write(filepath.Join(out, "pages.json"), pages)
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	t := map[string]any{"engine": "webrender", "pages": len(pages), "layout_ms": layout.Milliseconds(), "max_rss_mb": float64(ru.Maxrss) / 1024}
	write(filepath.Join(out, "timings.json"), t)
	j, _ := json.Marshal(t)
	fmt.Println(string(j))
}

// lines counts the distinct tops of a page's words (a line's words share a top, within a few px).
func lines(tops []int) int {
	sort.Ints(tops)
	n := 0
	for i, y := range tops {
		if i == 0 || y-tops[i-1] > 4 {
			n++
		}
	}
	return n
}

func write(p string, v any) {
	j, err := json.Marshal(v)
	check(err)
	check(os.WriteFile(p, j, 0o644))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
