// readalong: readium/go-toolkit opens the EPUB and reads its media overlay (SMIL, as guided navigation), crengine lays
// out and draws the pages, and the narration is played back without sound: at each word's clip, the page holding the
// word is shown (turning when needed) and the word's boxes are greyed. Writes the pages, a few highlighted frames,
// words.json (every word's time, page and boxes), timings.json and index.html (the read-along in a browser) into
// OUT_DIR.
//
//	readalong BOOK.epub OUT_DIR
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"einkcre/cre"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

const W, H = 1072, 1448

// One timed piece of text: a word, in the spine item at Spine.
type cue struct {
	ID    string            `json:"id"`
	Spine int               `json:"-"`
	Begin float64           `json:"begin"`
	End   float64           `json:"end"`
	Page  int               `json:"page"`
	Rects []image.Rectangle `json:"rects"`
}

// words flattens a guided navigation tree into the objects that have both text and audio (SMIL pars).
func words(objs []guidednavigation.GuidedNavigationObject, spine int, out []cue) []cue {
	for _, o := range objs {
		if c := o.AudioClip(); c != nil && c.Begin != nil && c.End != nil && o.TextRef != nil {
			out = append(out, cue{ID: o.TextRef.Fragment(), Spine: spine, Begin: c.Begin.Seconds(), End: c.End.Seconds()})
		}
		out = words(o.Children, spine, out)
	}
	return out
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func maxRSSMB() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Maxrss) / 1024
}

func savePNG(path string, img image.Image) {
	f, err := os.Create(path)
	check(err)
	check(png.Encode(f, img))
	f.Close()
}

// highlight greys the background under the boxes, keeping the text black (a multiply, as a marker pen would).
func highlight(dst, page *image.Gray, rects []image.Rectangle) {
	for _, r := range rects {
		r = r.Intersect(dst.Rect)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				i := dst.PixOffset(x, y)
				dst.Pix[i] = uint8(int(page.Pix[i]) * 170 / 255)
			}
		}
	}
}

// restore copies the boxes back from the clean page (un-highlighting the previous word).
func restore(dst, page *image.Gray, rects []image.Rectangle) {
	for _, r := range rects {
		r = r.Intersect(dst.Rect)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			i := dst.PixOffset(r.Min.X, y)
			copy(dst.Pix[i:i+r.Dx()], page.Pix[i:i+r.Dx()])
		}
	}
}

func union(rs ...[]image.Rectangle) image.Rectangle {
	var u image.Rectangle
	for _, r := range rs {
		for _, b := range r {
			u = u.Union(b)
		}
	}
	return u
}

func main() {
	book, outDir := os.Args[1], os.Args[2]
	check(os.MkdirAll(outDir, 0o755))
	abs, _ := filepath.Abs(book)
	t := map[string]any{}

	// 1. The book through Readium: reading order and the media overlay of each item.
	t0 := time.Now()
	u, err := url.FromFilepath(abs)
	check(err)
	ctx := context.Background()
	// go-toolkit's EPUB parser directly: its streamer would also bring in the PDF and audiobook parsers.
	a := asset.File(u)
	f, err := a.CreateFetcher(ctx, asset.Dependencies{ArchiveFactory: archive.NewArchiveFactory()}, "")
	check(err)
	pb, err := epub.NewParser(nil).Parse(ctx, a, f)
	check(err)
	p := pb.Build()
	var cues []cue
	if s, ok := p.FindService(pub.MediaOverlayService_Name).(pub.GuidedNavigationService); ok {
		for i, l := range p.Manifest.ReadingOrder {
			href := l.Href.String()
			if !s.HasGuideForResource(href) {
				continue
			}
			doc, err := s.GuideForResource(ctx, href)
			check(err)
			cues = words(doc.Guided, i, cues)
		}
	}
	sort.SliceStable(cues, func(a, b int) bool { return cues[a].Begin < cues[b].Begin })
	t["readium_open_ms"] = ms(time.Since(t0))
	t["words_timed"] = len(cues)
	t["rss_after_readium_mb"] = maxRSSMB()

	// 2. crengine, with the settings of experiment 01's KOReader run (KOReader's defaults for a new book, CSS
	// word spacing so it can be checked against Chromium and KOReader's own pages).
	t1 := time.Now()
	cre.Init("", 0, "/opt/crengine/data/hyph/")
	cre.RegisterFont("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	d := cre.New(W, H)
	d.SetString("crengine.font.fallback.faces", "DejaVu Sans")
	d.SetInt("crengine.render.dpi", 96)
	d.SetInt("crengine.render.block.rendering.flags", 0x7FFFFFFF)
	d.SetStyleSheet("/opt/crengine/data/epub.css")
	d.SetInt("crengine.doc.embedded.styles.enabled", 1)
	d.SetInt("crengine.doc.embedded.fonts.enabled", 1)
	d.SetInt("font.kerning.mode", 3)
	d.SetInt("font.hinting.mode", 2)
	d.SetInt("crengine.style.space.width.scale.percent", 100)
	d.SetInt("crengine.style.space.condensing.percent", 100)
	d.SetInterlineSpace(100)
	d.SetString("window.status.line", "1")
	d.SetString("crengine.textlang.main.lang", "fa")
	d.SetString("crengine.textlang.embedded.langs.enabled", "1")
	d.SetString("crengine.textlang.hyphenation.enabled", "1")
	d.SetFontSize(34)
	for _, side := range []string{"left", "top", "right", "bottom"} {
		d.SetInt("crengine.page.margin."+side, 60)
	}
	t["cre_init_ms"] = ms(time.Since(t1))
	t2 := time.Now()
	if !d.Load(abs) {
		panic("crengine could not load " + abs)
	}
	t["cre_load_ms"] = ms(time.Since(t2))
	t3 := time.Now()
	d.Render()
	t["cre_layout_ms"] = ms(time.Since(t3))
	pages := d.PageCount()
	t["pages"] = pages
	t["rss_after_layout_mb"] = maxRSSMB()

	// 3. Every page, once (the reference images).
	page := image.NewGray(image.Rect(0, 0, W, H))
	var draws []time.Duration
	for i := 0; i < pages; i++ {
		s := time.Now()
		d.DrawPage(i, page)
		draws = append(draws, time.Since(s))
		savePNG(fmt.Sprintf("%s/page-%03d.png", outDir, i+1), page)
	}
	sort.Slice(draws, func(a, b int) bool { return draws[a] < draws[b] })
	t["draw_page_median_ms"] = ms(draws[len(draws)/2])
	t["rss_after_saving_pages_mb"] = maxRSSMB()

	// 4. Play the narration: per word, find its page and boxes; turn the page if needed; un-highlight the previous
	// word and highlight this one on the screen image. The changed area is what an e-ink partial refresh would update.
	screen := image.NewGray(page.Rect)
	shown := -1
	var prev []image.Rectangle
	var lookups, updates []time.Duration
	var turns, missing int
	var areaSum, areaMax int
	for i := range cues {
		c := &cues[i]
		s := time.Now()
		c.Page = d.IDPage(c.Spine, c.ID)
		if c.Page < 0 {
			missing++
			continue
		}
		if c.Page != shown {
			d.DrawPage(c.Page, page) // also makes it the page shown, for IDRects
			copy(screen.Pix, page.Pix)
			shown, prev = c.Page, nil
			turns++
		}
		c.Rects = d.IDRects(c.Spine, c.ID)
		lookups = append(lookups, time.Since(s))
		s = time.Now()
		restore(screen, page, prev)
		highlight(screen, page, c.Rects)
		updates = append(updates, time.Since(s))
		dirty := union(prev, c.Rects)
		a := dirty.Dx() * dirty.Dy()
		areaSum += a
		if a > areaMax {
			areaMax = a
		}
		prev = c.Rects
		if i < 3 || i == len(cues)-1 || (i > 0 && cues[i-1].Page != c.Page) {
			savePNG(fmt.Sprintf("%s/frame-%s.png", outDir, c.ID), screen)
		}
	}
	med := func(ds []time.Duration) float64 {
		if len(ds) == 0 {
			return 0
		}
		sort.Slice(ds, func(a, b int) bool { return ds[a] < ds[b] })
		return ms(ds[len(ds)/2])
	}
	worst := func(ds []time.Duration) float64 {
		if len(ds) == 0 {
			return 0
		}
		return ms(ds[len(ds)-1])
	}
	t["word_lookup_median_ms"], t["word_lookup_max_ms"] = med(lookups), worst(lookups)
	t["word_highlight_median_ms"], t["word_highlight_max_ms"] = med(updates), worst(updates)
	t["page_turns"], t["words_not_found"] = turns, missing
	if n := len(cues) - missing; n > 0 {
		t["refresh_area_avg_px"], t["refresh_area_max_px"] = areaSum/n, areaMax
		t["refresh_area_avg_pct_of_screen"] = float64(areaSum/n) * 100 / (W * H)
	}
	t["rss_end_mb"] = maxRSSMB() // peak resident memory so far, at each point

	j, _ := json.MarshalIndent(cues, "", " ")
	check(os.WriteFile(outDir+"/words.json", j, 0o644))
	check(writeViewer(outDir, cues))
	j, _ = json.MarshalIndent(t, "", "  ")
	check(os.WriteFile(outDir+"/timings.json", j, 0o644))
	fmt.Println(string(j))
	d.Close()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
