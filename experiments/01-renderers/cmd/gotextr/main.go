// Bare go-text route: go-text/typesetting alone (HarfBuzz port, bidi segmentation, its greedy line wrapper),
// our own justification and pagination, glyphs drawn through the raster package's per-glyph mask cache
// (what an e-reader would keep between page turns).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"einkrender/raster"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html"
)

const (
	pageW, pageH = 1072, 1448
	margin       = 60.0
	bodyMargin   = 17.0
	fontPx       = 34.0
	lineHeight   = 1.9
)

type oneFace struct{ f *font.Face }

func (o oneFace) ResolveFace(rune) *font.Face { return o.f }

type block struct {
	kind         string
	text         []rune
	bold         bool
	size         float64
	lines        []shaping.Line
	above, below float64
}

var ws = regexp.MustCompile(`[ \t\r\n]+`)

func textOf(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		textOf(c, b)
	}
}

func blocks(n *html.Node, out *[]*block) {
	if n.Type == html.ElementNode && (n.Data == "h1" || n.Data == "h2" || n.Data == "p") {
		var sb strings.Builder
		textOf(n, &sb)
		s := strings.TrimSpace(ws.ReplaceAllString(sb.String(), " "))
		*out = append(*out, &block{kind: n.Data, text: []rune(s), bold: n.Data != "p"})
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		blocks(c, out)
	}
}

func load(path string) *font.Face {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	f, err := font.ParseTTF(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return f
}

func fx(v float64) fixed.Int26_6 { return fixed.Int26_6(v * 64) }
func fl(v fixed.Int26_6) float64 { return float64(v) / 64 }

type frag struct {
	b    *block
	a, z int
	y    float64
}

func maxRSSMB() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Maxrss) / 1024
}

func main() {
	in, fontDir, outDir := os.Args[1], os.Args[2], os.Args[3]
	t0 := time.Now()
	f, _ := os.Open(in)
	doc, err := html.Parse(f)
	if err != nil {
		panic(err)
	}
	var bs []*block
	blocks(doc, &bs)
	reg, bold := load(fontDir+"/Parastoo.ttf"), load(fontDir+"/Parastoo-Bold.ttf")
	parsed := time.Since(t0)

	bodyW := pageW - 2*margin - 2*bodyMargin
	bodyH := pageH - 2*margin

	t1 := time.Now()
	var shaper shaping.HarfbuzzShaper
	var seg shaping.Segmenter
	var wrapper shaping.LineWrapper
	for _, b := range bs {
		b.size = fontPx
		switch b.kind {
		case "h1":
			b.size = fontPx * 1.5
		case "h2":
			b.size = fontPx * 1.25
		}
		if b.kind == "p" {
			b.below = 0.5 * fontPx
		} else {
			b.above, b.below = 1.5*b.size, 1.0*b.size
		}
		face := reg
		if b.bold {
			face = bold
		}
		input := shaping.Input{Text: b.text, RunStart: 0, RunEnd: len(b.text), Direction: di.DirectionRTL,
			Face: face, Size: fx(b.size), Script: language.Arabic, Language: language.NewLanguage("fa")}
		var runs []shaping.Output
		for _, part := range seg.Split(input, oneFace{face}) {
			runs = append(runs, shaper.Shape(part))
		}
		lines, _ := wrapper.WrapParagraph(shaping.WrapConfig{Direction: di.DirectionRTL}, int(bodyW), b.text, shaping.NewSliceIterator(runs))
		// the wrapper reuses its buffers on the next call: keep copies
		for _, l := range lines {
			c := make(shaping.Line, len(l))
			for k, r := range l {
				r.Glyphs = slices.Clone(r.Glyphs)
				c[k] = r
			}
			b.lines = append(b.lines, c)
		}
	}
	// pagination, as in the canvas route
	lh := func(b *block) float64 { return lineHeight * b.size }
	var pages [][]frag
	var cur []frag
	y, prevBelow := 0.0, 0.0
	for _, b := range bs {
		if len(cur) > 0 {
			y += max(b.above, prevBelow)
		}
		a := 0
		for a < len(b.lines) {
			z := a
			for z < len(b.lines) && y+float64(z+1-a)*lh(b) <= bodyH {
				z++
			}
			if z == a {
				if len(cur) == 0 {
					z = a + 1
				} else {
					pages = append(pages, cur)
					cur, y = nil, 0
					continue
				}
			}
			cur = append(cur, frag{b, a, z, y})
			y += float64(z-a) * lh(b)
			a = z
			if a < len(b.lines) {
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

	cache := raster.NewGlyphCache()
	os.MkdirAll(outDir, 0o755)
	var times []time.Duration
	left := margin + bodyMargin
	right := left + bodyW
	for i, frags := range pages {
		ts := time.Now()
		page := raster.NewPage(pageW, pageH)
		clip := page.Img.Bounds()
		for _, fr := range frags {
			b := fr.b
			ext, _ := b.lines[0][0].Face.FontHExtents()
			scale := b.size / float64(b.lines[0][0].Face.Upem())
			asc, desc := float64(ext.Ascender)*scale, -float64(ext.Descender)*scale
			for li := fr.a; li < fr.z; li++ {
				line := slices.Clone(b.lines[li])
				slices.SortFunc(line, func(p, q shaping.Output) int { return int(p.VisualIndex - q.VisualIndex) })
				width, spaces := 0.0, 0
				for _, r := range line {
					width += fl(r.Advance)
					for _, g := range r.Glyphs {
						if g.ClusterIndex < len(b.text) && b.text[g.ClusterIndex] == ' ' && g.RuneCount == 1 {
							spaces++
						}
					}
				}
				last := li == len(b.lines)-1
				x, extra := right-width, 0.0 // right-aligned (RTL start)
				switch {
				case b.kind != "p":
					x = left + (bodyW-width)/2
				case !last && spaces > 0:
					x, extra = left, (bodyW-width)/float64(spaces)
				}
				top := margin + fr.y + float64(li-fr.a)*lh(b)
				base := top + (lh(b)-(asc+desc))/2 + asc
				for _, r := range line {
					for _, g := range r.Glyphs {
						gx := x + fl(g.XOffset)
						gy := base - fl(g.YOffset)
						cache.Draw(page, r.Face, g.GlyphID, float32(b.size), float32(gx), float32(gy), 0, clip)
						x += fl(g.XAdvance)
						if extra > 0 && g.ClusterIndex < len(b.text) && b.text[g.ClusterIndex] == ' ' && g.RuneCount == 1 {
							x += extra
						}
					}
				}
			}
		}
		times = append(times, time.Since(ts))
		page.Save(fmt.Sprintf("%s/page-%02d.png", outDir, i+1))
	}
	var rest time.Duration
	for _, t := range times[1:] {
		rest += t
	}
	res := map[string]any{
		"renderer":         "go-text bare (greedy wrap, own justify)",
		"pages":            len(pages),
		"parse_ms":         parsed.Milliseconds(),
		"layout_ms":        layout.Milliseconds(),
		"draw_first_ms":    float64(times[0].Microseconds()) / 1000,
		"draw_avg_rest_ms": float64((rest / time.Duration(max(1, len(times)-1))).Microseconds()) / 1000,
		"glyph_cache":      fmt.Sprintf("%d hits / %d misses", cache.Hits, cache.Misses),
		"max_rss_mb":       maxRSSMB(),
	}
	j, _ := json.MarshalIndent(res, "", "  ")
	os.WriteFile(outDir+"/timings.json", j, 0o644)
	fmt.Println(string(j))
}
