package ui

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"time"

	blitz "shamsainkengine"
)

// The opening screen (plan, "An opening screen"): shown when a book is not open within SplashAfter, so a quick
// open goes straight to its page (the screen's flash would hold the page back ~0.4 s, and an opening up to 3 s is not worth a screen). Drawn once from
// templates/splash.html and kept as a picture in DIR/cache (a new one whenever the template or the screen's size changes),
// so showing it is a copy and a refresh, the ink moving while the book opens.
const SplashAfter = 3 * time.Second

// SplashFor is how long the opening screen shown when the reader starts stays at least, so it is seen, not flashed by
// when the library is ready sooner. 1 s since 2026-11-06 (2 s before): the library takes ~0.7 s behind it on the
// Kindle, so the logo is still seen and the start no longer waits ~1.3 s for nothing (2.8 s to the library, now ~1.8).
const SplashFor = 1 * time.Second

type splashData struct{ Top int }

// Opening is an opening screen: the grey picture (always, also the Kobo's sleep screen), and on a colour screen the colour
// one, the grey layout with the logos in their colours.
type Opening struct {
	Gray *image.Gray
	RGBA *image.RGBA // nil: a grey screen
}

// Splash is the opening screen for a screen w × h: read from DIR/cache, else drawn and kept there; with colour, in colour
// as well.
func Splash(dir string, fonts *blitz.Fonts, w, h int, colour bool) *Opening {
	return opening("splash", dir, fonts, w, h, colour)
}

// Loading is the screen for a book that is slow to open from the library (templates/loading.html): the mark and
// "Opening…", kept like Splash.
func Loading(dir string, fonts *blitz.Fonts, w, h int, colour bool) *Opening {
	return opening("loading", dir, fonts, w, h, colour)
}

// Leaving is the screen drawn as the reader is left (templates/leaving.html): the mark and "Exiting…", kept like Splash. It stays until the device's own interface draws over it: on the Kobo, Nickel's home screen 8 to 10 s later
// (2026-11-12), where the screen was blank or still the library before.
func Leaving(dir string, fonts *blitz.Fonts, w, h int, colour bool) *Opening {
	return opening("leaving", dir, fonts, w, h, colour)
}

// The logos, in brand/, from FarsiEPUB's brand folder, in colour, one set for every screen (our Blitz draws no SVG): ShamsaInk's
// mark from brand/shamsaink/mark.svg (rsvg-convert -w 240 -h 240 -b white), for the Loading screen, and the stacked logo
// (the mark, the name and "by FarsiEPUB") from brand/shamsaink/logo-stack.svg (rsvg-convert -w 400 -b white), for the
// opening screen. A grey screen shows them in grey, as the engine draws them; a colour screen has each put
// over the box of the image with its id in the page, in its colours.
// Build is the reader's version (main sets it), part of the opening screens' cache key: a new build may draw them
// differently (an engine or font change), and a package installed over USB cannot clear the cache as an SSH install does.
var Build string

var colourLogos = []struct{ id, file string }{{"mark", "templates/brand/shamsaink.png"}, {"logo", "templates/brand/shamsaink-stack.png"}}

func opening(name, dir string, fonts *blitz.Fonts, w, h int, colour bool) *Opening {
	// the logos (colourLogos), read by the engine as files beside the page
	mark, _ := files.ReadFile("templates/brand/shamsaink.png")
	stack, _ := files.ReadFile("templates/brand/shamsaink-stack.png")
	html, _ := render(name+".html", splashData{Top: h * 3 / 10})
	sum := fnv.New32a()
	sum.Write([]byte(Build))
	sum.Write([]byte(html))
	sum.Write(mark)
	sum.Write(stack)
	grey := filepath.Join(dir, "cache", fmt.Sprintf("%s-%dx%d-%08x.gray", name, w, h, sum.Sum32()))
	rgbaPath := filepath.Join(dir, "cache", fmt.Sprintf("%s-%dx%d-%08x.rgba", name, w, h, sum.Sum32()))
	img := image.NewGray(image.Rect(0, 0, w, h))
	var rgba *image.RGBA
	if colour {
		rgba = image.NewRGBA(img.Rect)
	}
	// both kept: the grey one is the sleep screen's, and what a colour one is made of
	if b, err := os.ReadFile(grey); err == nil && len(b) == len(img.Pix) {
		copy(img.Pix, b)
		if !colour {
			return &Opening{Gray: img}
		}
		if c, err := os.ReadFile(rgbaPath); err == nil && len(c) == len(rgba.Pix) {
			copy(rgba.Pix, c)
			return &Opening{img, rgba}
		}
	}
	t := time.Now()
	rest := readFor(html)
	read := func(path string) ([]byte, error) {
		switch path {
		case "brand/shamsaink.png":
			return mark, nil
		case "brand/shamsaink-stack.png":
			return stack, nil
		}
		return rest(path)
	}
	b := blitz.New([]string{name + ".html"}, read, blitz.Options{Width: w, Height: h, FontSize: 40}, fonts)
	defer b.Close()
	b.Layout(0)
	b.Draw(0, img)
	os.MkdirAll(filepath.Dir(grey), 0o755)
	// an older build's or design's copies go (2 MB grey and 8 MB colour each on the Kobo)
	old, _ := filepath.Glob(filepath.Join(dir, "cache", fmt.Sprintf("%s-%dx%d-*", name, w, h)))
	for _, f := range old {
		os.Remove(f)
	}
	if err := os.WriteFile(grey, img.Pix, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: the opening screen not kept:", err)
	}
	o := &Opening{Gray: img}
	if colour {
		o.RGBA = colourOf(img, func(id string) image.Rectangle { _, rs := b.Find(0, id); return union(rs) })
		if err := os.WriteFile(rgbaPath, o.RGBA.Pix, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: the colour opening screen not kept:", err)
		}
	}
	fmt.Printf("  opening screen drawn in %.0f ms\n", ms(t))
	return o
}

// colourOf is the grey picture as colour, with each logo that has a colour one put over the box of its image (where finds
// it by id): a logo whose box is not about the picture's size is left grey.
func colourOf(img *image.Gray, where func(id string) image.Rectangle) *image.RGBA {
	rgba := image.NewRGBA(img.Rect)
	for i, g := range img.Pix {
		rgba.Pix[4*i], rgba.Pix[4*i+1], rgba.Pix[4*i+2], rgba.Pix[4*i+3] = g, g, g, 255
	}
	for _, l := range colourLogos {
		data, err := files.ReadFile(l.file)
		if err != nil {
			continue
		}
		pic, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		box := where(l.id)
		// the layout's box may be a pixel off the picture's size (rounding); more than two is another picture
		pw, ph := pic.Bounds().Dx(), pic.Bounds().Dy()
		if box.Empty() || max(box.Dx()-pw, pw-box.Dx()) > 2 || max(box.Dy()-ph, ph-box.Dy()) > 2 {
			continue
		}
		draw.Draw(rgba, image.Rect(box.Min.X, box.Min.Y, box.Min.X+pw, box.Min.Y+ph), pic, pic.Bounds().Min, draw.Src)
	}
	return rgba
}
