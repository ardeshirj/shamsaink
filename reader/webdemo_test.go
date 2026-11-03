//go:build blitz

package main

import (
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	blitz "shamsainkengine"
)

// The data for the browser demo of read-along (docs/demo/): on a narrated book (BOOK, an .epub; FONTS a fallback
// fonts folder; OUT the folder for the files; DOC the spine document to start from, 2 by default; SECONDS how much
// narration, 45; SIZE the text size in px, the reader's 46). It writes, as the reader would show them:
//   - page-N.png, each page the narration reaches, drawn by the engine at the Kobo's size;
//   - words.json, every narrated word with its time in the audio, its page and its boxes, from which the page's
//     bar is drawn under each box as the reader does (session.underline: from 3 to 9 px under the box);
//   - audio-N.mp3, the audio files (parts) of those words, whole, played one after the other.
//
// Nothing it writes is for the repository: the pages and the voice are the book's (OUT is under out/, ignored).
func TestExportWebDemo(t *testing.T) {
	book, out := os.Getenv("BOOK"), os.Getenv("OUT")
	if book == "" || out == "" {
		t.Skip("BOOK and OUT not set")
	}
	num := func(key string, def float64) float64 {
		if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
			return v
		}
		return def
	}
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	const w, h = 1264, 1680
	o := blitz.Options{Width: w, Height: h, Margin: 60, FontSize: float32(num("SIZE", 46)), LineHeight: 1.75}
	ctx := context.Background()
	r, err := openBook(ctx, book, t.TempDir(), o, fonts, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()

	doc, seconds := int(num("DOC", 2)), num("SECONDS", 45)
	r.book.Layout(doc)
	// the narration is many audio files (parts): from the document's first word, whole parts until SECONDS are passed
	type part struct {
		file       string
		begin, end float64 // s, in the file
		Audio      string  `json:"audio"`
	}
	var parts []*part
	var words []cue
	var total float64
	for _, c := range cues(ctx, r.pub) {
		if c.doc != doc {
			continue
		}
		if len(parts) == 0 || parts[len(parts)-1].file != c.file {
			if total > seconds {
				break
			}
			if len(parts) > 0 {
				last := parts[len(parts)-1]
				total += last.end - last.begin
			}
			parts = append(parts, &part{file: c.file, begin: c.begin})
		}
		parts[len(parts)-1].end = c.end
		words = append(words, c)
	}
	if len(words) == 0 {
		t.Fatalf("no narrated words in document %d", doc)
	}
	t.Logf("%d words in %d parts", len(words), len(parts))

	type word struct {
		Part   int
		T0, T1 float64 // in the part's audio file
		Page   int
		Boxes  [][4]int
	}
	var list []word
	used := map[int]bool{}
	partOf := map[string]int{}
	for i, pt := range parts {
		partOf[pt.file] = i
	}
	for _, c := range words {
		page, boxes := r.book.Find(c.doc, c.id)
		if page < 0 || len(boxes) == 0 {
			continue
		}
		used[page] = true
		wd := word{Part: partOf[c.file], T0: c.begin, T1: c.end, Page: page}
		for _, b := range boxes {
			wd.Boxes = append(wd.Boxes, [4]int{b.Min.X, b.Min.Y, b.Max.X, b.Max.Y})
		}
		list = append(list, wd)
	}
	var pages []int
	for p := range used {
		pages = append(pages, p)
	}
	sort.Ints(pages)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	var files []string
	index := map[int]int{}
	for i, p := range pages {
		img := image.NewGray(image.Rect(0, 0, w, h))
		r.book.Draw(p, img)
		name := "page-" + strconv.Itoa(i) + ".png"
		f, _ := os.Create(filepath.Join(out, name))
		png.Encode(f, img)
		f.Close()
		files = append(files, name)
		index[p] = i
	}
	for i := range list {
		list[i].Page = index[list[i].Page]
	}
	var bytes int
	for i, pt := range parts {
		audio, err := r.read(pt.file)
		if err != nil {
			t.Fatal(err)
		}
		pt.Audio = "audio-" + strconv.Itoa(i) + ".mp3"
		bytes += len(audio)
		if err := os.WriteFile(filepath.Join(out, pt.Audio), audio, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var po []map[string]any
	for _, pt := range parts {
		po = append(po, map[string]any{"audio": pt.Audio, "begin": pt.begin, "end": pt.end})
	}
	data, _ := json.MarshalIndent(map[string]any{
		"width": w, "height": h, "pages": files, "parts": po, "words": list,
	}, "", " ")
	os.WriteFile(filepath.Join(out, "words.json"), data, 0o644)
	t.Logf("%d pages, %d words with boxes, %d audio files (%d KB) -> %s", len(pages), len(list), len(parts), bytes/1024, out)
}
