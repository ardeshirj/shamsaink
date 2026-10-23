// pages: lays out a whole EPUB with an engine and draws every page. Writes page-NNN.png and timings.json into OUT_DIR;
// with WORDS.json (from experiment 03's spanwords), boxes.json: each listed element's page and boxes, for
// tools/linecmp; and pages.json (each page's spine item and lines of text) when the engine can say.
// Settings are experiment 01's: 1072×1448, 60 px margins, 34 px text.
//
//	pages [-engine gotext|cre] [-fallback FONT] BOOK.epub OUT_DIR [WORDS.json]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"
	"syscall"
	"time"

	engine "einkengine"
	_ "einkengine/engines"
	"einkengine/epub"
)

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func maxRSSMB() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Maxrss) / 1024
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

func main() {
	name := flag.String("engine", "gotext", fmt.Sprint("layout engine: ", engine.Names()))
	fallback := flag.String("fallback", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "font for characters the book's fonts lack")
	flag.Parse()
	in, outDir := flag.Arg(0), flag.Arg(1)
	check(os.MkdirAll(outDir, 0o755))
	t := map[string]any{"engine": *name}

	t0 := time.Now()
	src, err := epub.Open(context.Background(), in)
	check(err)
	t["readium_open_ms"] = ms(time.Since(t0))
	t1 := time.Now()
	d, err := engine.Open(*name, src, engine.Options{Width: 1072, Height: 1448, Margin: 60, FontSize: 34,
		Fallback: *fallback})
	check(err)
	defer d.Close()
	t["layout_ms"] = ms(time.Since(t1))
	t["pages"] = d.Pages()
	t["rss_after_layout_mb"] = maxRSSMB()

	img := image.NewGray(image.Rect(0, 0, 1072, 1448))
	var draws []time.Duration
	for i := 0; i < d.Pages(); i++ {
		s := time.Now()
		d.Draw(i, img)
		draws = append(draws, time.Since(s))
		f, err := os.Create(fmt.Sprintf("%s/page-%03d.png", outDir, i+1))
		check(err)
		check(png.Encode(f, img))
		f.Close()
	}
	t["draw_first_ms"] = ms(draws[0])
	sort.Slice(draws, func(a, c int) bool { return draws[a] < draws[c] })
	t["draw_page_median_ms"] = ms(draws[len(draws)/2])
	t["rss_end_mb"] = maxRSSMB()

	if flag.NArg() > 2 {
		raw, err := os.ReadFile(flag.Arg(2))
		check(err)
		var words []word
		check(json.Unmarshal(raw, &words))
		var boxes []box
		for _, w := range words {
			p, rs := d.Find(w.Spine, w.ID)
			boxes = append(boxes, box{w.Spine, w.ID, p, rs})
		}
		j, _ := json.Marshal(boxes)
		check(os.WriteFile(outDir+"/boxes.json", j, 0o644))
	}
	if pl, ok := d.(engine.PageLines); ok {
		j, _ := json.Marshal(pl.PageLines())
		check(os.WriteFile(outDir+"/pages.json", j, 0o644))
	}
	j, _ := json.MarshalIndent(t, "", "  ")
	check(os.WriteFile(outDir+"/timings.json", j, 0o644))
	fmt.Println(string(j))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
