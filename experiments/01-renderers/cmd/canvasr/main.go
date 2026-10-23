// canvas route: our own minimal EPUB layout (headings and paragraphs from the chapter's XHTML, the book CSS's
// values hard-coded) on tdewolff/canvas: its pure-Go HarfBuzz + FriBidi, Knuth line breaking, justification;
// pages cut at line boundaries, drawn by canvas's rasterizer.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"einkrender/raster"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"golang.org/x/net/html"
)

const (
	pageW, pageH = 1072, 1448
	margin       = 60.0
	bodyMargin   = 17.0 // body { margin: 0 0.5em } at 34 px
	fontPx       = 34.0
	lineHeight   = 1.9
	dpi          = 300.0
)

func mm(px float64) float64 { return px * 25.4 / dpi }
func pt(px float64) float64 { return px * 72 / dpi }

type block struct {
	kind  string // h1, h2, p
	runs  []run
	text  *canvas.Text
	bnd   []float64 // line tops in px from the block's top; last = block height
	above float64   // margin before, px
	below float64   // margin after, px
}

type run struct {
	s    string
	bold bool
}

var ws = regexp.MustCompile(`[ \t\r\n]+`)

func collect(n *html.Node, bold bool, out *[]run) {
	switch n.Type {
	case html.TextNode:
		*out = append(*out, run{ws.ReplaceAllString(n.Data, " "), bold})
	case html.ElementNode:
		if n.Data == "b" || n.Data == "strong" {
			bold = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c, bold, out)
		}
	}
}

func blocks(n *html.Node, out *[]*block) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "h1", "h2", "p":
			b := &block{kind: n.Data}
			collect(n, n.Data != "p", &b.runs)
			// trim the paragraph's outer spaces
			if len(b.runs) > 0 {
				b.runs[0].s = strings.TrimLeft(b.runs[0].s, " ")
				b.runs[len(b.runs)-1].s = strings.TrimRight(b.runs[len(b.runs)-1].s, " ")
			}
			*out = append(*out, b)
			return
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		blocks(c, out)
	}
}

func maxRSSMB() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Maxrss) / 1024
}

type frag struct {
	b    *block
	a, z int     // lines [a, z)
	y    float64 // top on the page body, px
}

func main() {
	in, fontDir, outDir, lb, bodyAlign := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	t0 := time.Now()
	f, err := os.Open(in)
	if err != nil {
		panic(err)
	}
	doc, err := html.Parse(f)
	if err != nil {
		panic(err)
	}
	var bs []*block
	blocks(doc, &bs)
	family := canvas.NewFontFamily("BookFont")
	family.MustLoadFontFile(fontDir+"/Parastoo.ttf", canvas.FontRegular)
	family.MustLoadFontFile(fontDir+"/Parastoo-Bold.ttf", canvas.FontBold)
	parsed := time.Since(t0)

	bodyW := pageW - 2*margin - 2*bodyMargin
	bodyH := pageH - 2*margin
	opts := &canvas.TextOptions{}
	if lb == "greedy" {
		opts.Linebreaker = canvas.GreedyLinebreaker{}
	} else {
		opts.Linebreaker = canvas.KnuthLinebreaker{}
	}

	// Layout: every block shaped and broken into lines.
	t1 := time.Now()
	for _, b := range bs {
		size, align := fontPx, canvas.Justify
		if bodyAlign == "right" {
			align = canvas.Right
		}
		switch b.kind {
		case "h1":
			size, align = fontPx*1.5, canvas.Center
		case "h2":
			size, align = fontPx*1.25, canvas.Center
		}
		if b.kind == "p" {
			b.above, b.below = 0, 0.5*fontPx
		} else {
			b.above, b.below = 1.5*size, 1.0*size
		}
		reg := family.Face(pt(size), canvas.Black, canvas.FontRegular, canvas.FontNormal)
		bold := family.Face(pt(size), canvas.Black, canvas.FontBold, canvas.FontNormal)
		o := *opts
		o.LineStretch = mm(lineHeight*size)/reg.Metrics().LineHeight - 1
		rt := canvas.NewRichText(reg)
		for _, r := range b.runs {
			if r.bold {
				rt.WriteFace(bold, r.s)
			} else {
				rt.WriteFace(reg, r.s)
			}
		}
		b.text = rt.ToText(mm(bodyW), 0, align, canvas.Top, &o)
		var base []float64
		b.text.WalkLines(func(y float64, _ []canvas.TextSpan) { base = append(base, -y*dpi/25.4) })
		for _, y := range base {
			b.bnd = append(b.bnd, y-base[0])
		}
		b.bnd = append(b.bnd, b.text.Height*dpi/25.4)
	}
	// Pagination at line boundaries (no widow/orphan rules, margins collapsed as CSS does).
	var pages [][]frag
	var cur []frag
	y, prevBelow := 0.0, 0.0
	for _, b := range bs {
		gap := max(b.above, prevBelow)
		if len(cur) == 0 {
			gap = 0
		}
		y += gap
		a := 0
		for a < len(b.bnd)-1 {
			z := a
			for z < len(b.bnd)-1 && y+b.bnd[z+1]-b.bnd[a] <= bodyH {
				z++
			}
			if z == a { // not even one line fits: new page
				if len(cur) == 0 {
					z = a + 1
				} else {
					pages = append(pages, cur)
					cur, y = nil, 0
					continue
				}
			}
			cur = append(cur, frag{b, a, z, y})
			y += b.bnd[z] - b.bnd[a]
			a = z
			if a < len(b.bnd)-1 {
				pages = append(pages, cur)
				cur, y = nil, 0
			}
		}
		prevBelow = b.below
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	layout := time.Since(t1)

	// Draw: each fragment rasterized into its own band (lines outside it are clipped), pasted on the page.
	res := canvas.DPI(dpi)
	os.MkdirAll(outDir, 0o755)
	var times []time.Duration
	t2 := time.Now()
	for i, frags := range pages {
		ts := time.Now()
		page := raster.NewPage(pageW, pageH)
		for _, fr := range frags {
			h := fr.b.bnd[fr.z] - fr.b.bnd[fr.a]
			// a bleed on both sides: glyph ink may overhang the line's ends (an initial ح does)
			const bleed = bodyMargin
			c := canvas.New(mm(bodyW+2*bleed), mm(h))
			ctx := canvas.NewContext(c)
			ctx.DrawText(mm(bleed), mm(h)+mm(fr.b.bnd[fr.a]), fr.b.text)
			img := rasterizer.Draw(c, res, canvas.DefaultColorSpace)
			at := image.Pt(int(margin+bodyMargin-bleed), int(margin+fr.y))
			r := img.Bounds().Add(at)
			draw.Draw(page.Img, r, img, img.Bounds().Min, draw.Over)
		}
		times = append(times, time.Since(ts))
		page.Save(fmt.Sprintf("%s/page-%02d.png", outDir, i+1))
		_ = i
	}
	drawAll := time.Since(t2)
	var rest time.Duration
	for _, t := range times[1:] {
		rest += t
	}
	res2 := map[string]any{
		"renderer":                  "canvas (" + lb + ", " + bodyAlign + ")",
		"pages":                     len(pages),
		"parse_ms":                  parsed.Milliseconds(),
		"layout_ms":                 layout.Milliseconds(),
		"draw_all_ms_incl_png_save": drawAll.Milliseconds(),
		"draw_first_ms":             float64(times[0].Microseconds()) / 1000,
		"draw_avg_rest_ms":          float64((rest / time.Duration(max(1, len(times)-1))).Microseconds()) / 1000,
		"max_rss_mb":                maxRSSMB(),
	}
	j, _ := json.MarshalIndent(res2, "", "  ")
	os.WriteFile(outDir+"/timings.json", j, 0o644)
	fmt.Println(string(j))
}
