//go:build blitz

// open: a book opened as the reader will (experiment 08's way, through the Blitz engine's C interface, engine/go): go-toolkit
// reads the EPUB, the document DOC is laid out and its first page drawn, then the other documents one by one, the
// cache is written (unless it was restored), and every page is drawn in order, laying its document out first.
// Prints for every page its index, spine document and a hash of its pixels, as `blitz-open` does with PAGE_HASHES
// set, so the two outputs can be diffed, then the timings in ms.
//
//	open BOOK.epub FONTS_DIR|- DOC CACHE
package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"image"
	"os"
	"strconv"
	"time"

	"einkengine/epub"
	blitz "indiumengine"
)

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: open BOOK.epub FONTS_DIR|- DOC CACHE")
		os.Exit(2)
	}
	doc, err := strconv.Atoi(os.Args[3])
	check(err)
	t0 := time.Now()
	b, err := epub.Open(context.Background(), os.Args[1])
	check(err)
	var fonts *blitz.Fonts
	if os.Args[2] != "-" {
		fonts = blitz.LoadFonts(os.Args[2])
		defer fonts.Free()
	}
	// blitz-open's defaults
	o := blitz.Options{Width: 1072, Height: 1448, Margin: 60, FontSize: 34}
	img := image.NewGray(image.Rect(0, 0, o.Width, o.Height))

	book := blitz.New(b.Spine(), b.Read, o, fonts)
	defer book.Close()
	restored := false
	if data, err := os.ReadFile(os.Args[4]); err == nil {
		restored = book.Restore(data)
	}
	first, _ := book.Layout(doc)
	book.Draw(first, img)
	firstMs := ms(t0)

	s := time.Now()
	for book.LayoutNext() {
	}
	restMs := ms(s)
	cache := "restored"
	if !restored {
		data := book.Save()
		if data == nil {
			check(fmt.Errorf("every document paginated, but no cache"))
		}
		check(os.WriteFile(os.Args[4], data, 0o644))
		cache = fmt.Sprintf("written, %d KB", len(data)/1024)
	}

	infos := book.PageLines()
	cur := doc
	for n, p := range infos {
		if p.Spine != cur {
			book.Layout(p.Spine)
			cur = p.Spine
		}
		book.Draw(n, img)
		h := fnv.New64a()
		h.Write(img.Pix)
		fmt.Printf("page %d doc %d hash %016x\n", n, p.Spine, h.Sum64())
	}
	fmt.Fprintf(os.Stderr, "document %d: first page %.1f ms, then the rest %.0f ms, whole %.0f ms; %d pages; cache %s\n",
		doc, firstMs, restMs, ms(t0), len(infos), cache)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
