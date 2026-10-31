package ui

import (
	"fmt"
	"hash/fnv"
	"image"
	"os"
	"path/filepath"
	"time"

	blitz "shamsainkengine"
)

// The opening screen (roadmap, "An opening screen"): shown when a book is not open within SplashAfter, so a quick
// open goes straight to its page (the screen's flash would hold the page back ~0.4 s, and an opening up to 3 s is not worth a screen). Drawn once from
// templates/splash.html and kept as a picture in DIR/cache (a new one whenever the template or the screen's size changes),
// so showing it is a copy and a refresh, the ink moving while the book opens.
const SplashAfter = 3 * time.Second

type splashData struct{ Top int }

// Splash is the opening screen for a screen w × h: read from DIR/cache, else drawn and kept there.
func Splash(dir string, fonts *blitz.Fonts, w, h int) *image.Gray {
	return opening("splash", dir, fonts, w, h)
}

// Loading is the screen for a book that is slow to open from the library (templates/loading.html): the mark and
// "Opening…", kept like Splash.
func Loading(dir string, fonts *blitz.Fonts, w, h int) *image.Gray {
	return opening("loading", dir, fonts, w, h)
}

func opening(name, dir string, fonts *blitz.Fonts, w, h int) *image.Gray {
	// the logos, in brand/, made from FarsiEPUB's brand folder (our Blitz draws no SVG), read by the engine as files
	// beside the page: ShamsaInk's mark, from brand/shamsaink/mark-1bit.svg (rsvg-convert -w 240 -h 240 -b white),
	// and FarsiEPUB's logo, from brand/farsiepub/logo.svg (rsvg-convert -h 60 -b white)
	mark, _ := files.ReadFile("templates/brand/shamsaink.png")
	logo, _ := files.ReadFile("templates/brand/farsiepub.png")
	html, _ := render(name+".html", splashData{Top: h * 3 / 10})
	sum := fnv.New32a()
	sum.Write([]byte(html))
	sum.Write(mark)
	sum.Write(logo)
	path := filepath.Join(dir, "cache", fmt.Sprintf("%s-%dx%d-%08x.gray", name, w, h, sum.Sum32()))
	img := image.NewGray(image.Rect(0, 0, w, h))
	if b, err := os.ReadFile(path); err == nil && len(b) == len(img.Pix) {
		copy(img.Pix, b)
		return img
	}
	t := time.Now()
	rest := readFor(html)
	read := func(path string) ([]byte, error) {
		switch path {
		case "brand/shamsaink.png":
			return mark, nil
		case "brand/farsiepub.png":
			return logo, nil
		}
		return rest(path)
	}
	b := blitz.New([]string{name + ".html"}, read, blitz.Options{Width: w, Height: h, FontSize: 40}, fonts)
	defer b.Close()
	b.Layout(0)
	b.Draw(0, img)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, img.Pix, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: the opening screen not kept:", err)
	}
	fmt.Printf("  opening screen drawn in %.0f ms\n", ms(t))
	return img
}
