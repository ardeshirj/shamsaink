package device

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// The sleep screen, the same on every device: chosen from the YAML files in sleep/, drawn by the reader or a picture a
// file names (docs/SLEEP-SCREEN-FORMAT.md). Shown by the device's own doze (kobo.go, kindle.go).

// sleepScreen is one screen from DIR/sleep, chosen at random among the screens of its YAML files (.yml, .yaml; one
// screen or a set): drawn by text at the screen's own size, grey (*image.Gray: inverted in dark mode, as the page is),
// or a picture the screen names (layout picture, its path relative to the YAML file's folder: sleepPicture). Only YAML files count: a
// picture alone in the folder is not shown (2026-11-06, the maintainer: the YAML says what to show). With none, or a
// screen that fails to load, fallback (the opening screen). Each screen counts once, so a set of 30 comes up 30 times
// as often as a file of one.
func sleepScreen(dir string, fallback *image.Gray, text SleepText) image.Image {
	type screen struct {
		path string
		yml  []byte
		i    int // the screen in it
	}
	if text.Count == nil || text.Draw == nil {
		return fallback
	}
	var all []screen
	for _, ext := range []string{"*.yml", "*.yaml", "*.YML", "*.YAML"} {
		m, _ := filepath.Glob(filepath.Join(dir, "sleep", ext))
		for _, p := range m {
			if strings.HasPrefix(filepath.Base(p), ".") { // macOS's ._name beside a file copied from a Mac, as the library skips
				continue
			}
			b, err := os.ReadFile(p)
			if err != nil {
				fmt.Fprintln(os.Stderr, "shamsaink: sleep screen:", filepath.Base(p), err)
				continue
			}
			n, errs := text.Count(b)
			for _, err := range errs {
				fmt.Fprintln(os.Stderr, "shamsaink: sleep screen:", filepath.Base(p), err)
			}
			for i := range n {
				all = append(all, screen{p, b, i})
			}
		}
	}
	if len(all) == 0 {
		return fallback
	}
	s := all[rand.IntN(len(all))]
	if text.Picture != nil {
		if pic := text.Picture(s.yml, s.i); pic != "" {
			// relative to the YAML file: beside it (hafez.png) or in a folder beside it (pictures/hafez.png), never
			// out of its folder
			if !filepath.IsLocal(pic) {
				fmt.Fprintln(os.Stderr, "shamsaink: sleep screen:", filepath.Base(s.path), "screen", s.i+1, "image", pic,
					"is not inside the file's folder")
				return fallback
			}
			return sleepPicture(filepath.Join(filepath.Dir(s.path), pic), fallback)
		}
	}
	img, err := text.Draw(s.yml, s.i)
	if err == nil && img != nil {
		return img
	}
	fmt.Fprintln(os.Stderr, "shamsaink: sleep screen:", filepath.Base(s.path), "screen", s.i+1, err)
	return fallback
}

// sleepPicture is the picture at path (PNG or JPEG) fitted to the screen (fallback's size) on white, as large as fits,
// kept in proportion and centred; one already the screen's size is copied as it is (scaling is most of the time: 2.6
// s for a 1236 x 1648 PNG on the Kindle, 2026-11-06). Grey when it has no colour (greyIfGrey), else in colour
// (*image.RGBA: shown as it is in dark mode, and in colour on a colour screen). fallback if it does not load.
func sleepPicture(path string, fallback *image.Gray) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: sleep screen:", err)
		return fallback
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: sleep screen:", filepath.Base(path), err)
		return fallback
	}
	r := fallback.Rect
	img := image.NewRGBA(r)
	draw.Draw(img, r, image.NewUniform(color.White), image.Point{}, draw.Src)
	b := src.Bounds()
	if b.Dx() == r.Dx() && b.Dy() == r.Dy() {
		draw.Draw(img, r, src, b.Min, draw.Over)
		return greyIfGrey(img)
	}
	w, h := r.Dx(), b.Dy()*r.Dx()/b.Dx()
	if h > r.Dy() {
		w, h = b.Dx()*r.Dy()/b.Dy(), r.Dy()
	}
	at := image.Rect((r.Dx()-w)/2, (r.Dy()-h)/2, (r.Dx()+w)/2, (r.Dy()+h)/2)
	draw.CatmullRom.Scale(img, at, src, b, draw.Over, nil)
	return greyIfGrey(img)
}

// greyIfGrey is the picture as greys (*image.Gray) when it has no colour (each pixel's red, green and blue within 8 of
// each other, as a scan or a black and white picture has them), so it follows dark mode as the page does; a picture in
// colour stays as it is.
func greyIfGrey(img *image.RGBA) image.Image {
	p := img.Pix
	for i := 0; i < len(p); i += 4 {
		r, g, b := int(p[i]), int(p[i+1]), int(p[i+2])
		if max(r, g, b)-min(r, g, b) > 8 {
			return img
		}
	}
	gr := image.NewGray(img.Rect)
	for i, j := 0, 0; i < len(p); i, j = i+4, j+1 {
		gr.Pix[j] = uint8((299*uint32(p[i]) + 587*uint32(p[i+1]) + 114*uint32(p[i+2])) / 1000)
	}
	return gr
}
