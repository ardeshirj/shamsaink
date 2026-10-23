// pages: lays out a whole EPUB with our engine (go-toolkit reads it, go-text shapes it) and draws every page. Writes
// page-NNN.png, timings.json, boxes.json (for every element id: its page and boxes, to compare line and page
// breaks with crengine and Chromium) and pages.json (each page's spine item and lines of text) into OUT_DIR.
// Settings are experiment 01's: 1072×1448, 60 px margins, 34 px.
//
//	pages BOOK.epub OUT_DIR
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"
	"syscall"
	"time"

	"einklayout/epubsrc"
	"einklayout/layout"
)

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func maxRSSMB() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Maxrss) / 1024
}

type box struct {
	Spine int               `json:"spine"`
	ID    string            `json:"id"`
	Page  int               `json:"page"`
	Rects []image.Rectangle `json:"rects"`
}

func main() {
	in, outDir := os.Args[1], os.Args[2]
	check(os.MkdirAll(outDir, 0o755))
	t := map[string]any{}

	t0 := time.Now()
	src, err := epubsrc.Open(context.Background(), in)
	check(err)
	t["readium_open_ms"] = ms(time.Since(t0))

	t1 := time.Now()
	b, err := layout.Build(src, layout.Options{Width: 1072, Height: 1448, Margin: 60, FontSize: 34,
		Fallback: "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"})
	check(err)
	t["read_and_style_ms"] = ms(time.Since(t1))
	t2 := time.Now()
	b.Shape()
	t["shape_and_wrap_ms"] = ms(time.Since(t2))
	t3 := time.Now()
	b.Paginate()
	t["paginate_ms"] = ms(time.Since(t3))
	t["layout_total_ms"] = ms(time.Since(t1))
	t["pages"] = len(b.Pages)
	t["paragraphs"] = len(b.Paras)
	t["rss_after_layout_mb"] = maxRSSMB()

	d := layout.NewDrawer()
	img := image.NewGray(image.Rect(0, 0, 1072, 1448))
	var draws []time.Duration
	for i := range b.Pages {
		s := time.Now()
		b.DrawPage(i, img, d)
		draws = append(draws, time.Since(s))
		f, err := os.Create(fmt.Sprintf("%s/page-%03d.png", outDir, i+1))
		check(err)
		check(png.Encode(f, img))
		f.Close()
	}
	t["draw_first_ms"] = ms(draws[0])
	sort.Slice(draws, func(a, c int) bool { return draws[a] < draws[c] })
	t["draw_page_median_ms"] = ms(draws[len(draws)/2])
	hits, misses := d.Stats()
	t["glyph_cache"] = fmt.Sprintf("%d hits / %d misses", hits, misses)
	t["rss_end_mb"] = maxRSSMB()

	var boxes []box
	for si := range src.Spine() {
		ids := b.IDs(si)
		sort.Strings(ids)
		for _, id := range ids {
			boxes = append(boxes, box{si, id, b.IDPage(si, id), b.IDRects(si, id)})
		}
	}
	type pageInfo struct {
		Spine int `json:"spine"`
		Lines int `json:"lines"`
	}
	var infos []pageInfo
	for _, ls := range b.Pages {
		pi := pageInfo{Spine: -1}
		for _, l := range ls {
			if pi.Spine < 0 {
				pi.Spine = l.Para.Spine
			}
			if l.Image == nil {
				pi.Lines++
			}
		}
		infos = append(infos, pi)
	}
	j, _ := json.Marshal(infos)
	check(os.WriteFile(outDir+"/pages.json", j, 0o644))
	j, _ = json.Marshal(boxes)
	check(os.WriteFile(outDir+"/boxes.json", j, 0o644))
	j, _ = json.MarshalIndent(t, "", "  ")
	check(os.WriteFile(outDir+"/timings.json", j, 0o644))
	fmt.Println(string(j))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
