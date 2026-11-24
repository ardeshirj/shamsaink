// Experiment 19: the library's covers in colour on the Kobo Libra Colour, against the grey ones it keeps
// (reader/library/covers.go). For each book with a cover picture under DIR/books: its small cover made grey (the
// library's own MakeCover) and in colour (the same scaling, its colour kept), timed, their sizes; each kept one read
// back, timed. With -show, a page like the library's (twelve covers, four a row) on the screen: grey, then in colour
// with the library's refresh modes, then a panel opened and closed over the colour covers; each step timed to the
// ink, a pause after it to look.
//
//	colourcovers -dir /mnt/onboard/.adds/colourcovers [-show] [-pause 5]
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/library"
)

const (
	screenW = 1264
	margin  = 40
	gap     = 36
)

var tw = (screenW - 2*margin - 3*gap) / 4 // the library's tile (reader/librarycovers.go)
var th = tw * 3 / 2

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func main() {
	dir := flag.String("dir", "", "a folder with books/ in it")
	show := flag.Bool("show", false, "show the covers on the screen")
	formats := flag.Bool("formats", false, "the colour covers kept as raw RGBA, raw RGB and JPEG, each read back, timed")
	pause := flag.Int("pause", 5, "seconds to look after each screen step")
	flag.Parse()
	var books []library.Book
	for _, b := range library.Scan(*dir) {
		if b.Cover != "" {
			books = append(books, b)
		}
	}
	fmt.Printf("%d books with a cover picture; tiles %d×%d\n", len(books), tw, th)
	fmt.Printf("%-50s %8s %8s %8s %8s %8s %8s\n", "book", "grey ms", "col ms", "grey KB", "col KB", "rd grey", "rd col")
	var greys, colours []image.Image
	var sum [6]float64
	for _, b := range books {
		t := time.Now()
		if err := library.MakeCover(*dir, b, tw, th); err != nil {
			fmt.Println(err)
			continue
		}
		gm := ms(t)
		t = time.Now()
		cf, err := makeColour(*dir, b)
		if err != nil {
			fmt.Println(err)
			continue
		}
		cm := ms(t)
		gf := library.CoverFile(*dir, b.Hash)
		t = time.Now()
		g := readPNG(gf)
		gr := ms(t)
		t = time.Now()
		c := readPNG(cf)
		cr := ms(t)
		gs, cs := size(gf), size(cf)
		fmt.Printf("%-50s %8.0f %8.0f %8.0f %8.0f %8.1f %8.1f\n", filepath.Base(b.Path), gm, cm, gs, cs, gr, cr)
		for i, v := range []float64{gm, cm, gs, cs, gr, cr} {
			sum[i] += v
		}
		greys, colours = append(greys, g), append(colours, c)
	}
	if n := float64(len(greys)); n > 0 {
		fmt.Printf("%-50s %8.0f %8.0f %8.0f %8.0f %8.1f %8.1f\n", "mean", sum[0]/n, sum[1]/n, sum[2]/n, sum[3]/n, sum[4]/n, sum[5]/n)
	}
	if *formats {
		compareFormats(*dir, books, colours)
	}
	if *show && len(greys) > 0 {
		screen(greys, colours, time.Duration(*pause)*time.Second)
	}
}

// makeColour is MakeCover in colour: the cover picture scaled the same way (the least size filling the tile, each
// pixel the mean of every other source pixel under it each way), kept as DIR/covers-colour/HASH.png.
func makeColour(dir string, b library.Book) (string, error) {
	z, err := zip.OpenReader(filepath.Join(dir, b.Path))
	if err != nil {
		return "", err
	}
	defer z.Close()
	var pic image.Image
	for _, f := range z.File {
		if f.Name == b.Cover {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			pic, _, err = image.Decode(io.LimitReader(rc, 32<<20))
			rc.Close()
			if err != nil {
				return "", err
			}
		}
	}
	if pic == nil {
		return "", fmt.Errorf("%s: no %s", b.Path, b.Cover)
	}
	small := shrinkRGBA(pic, tw, th)
	os.MkdirAll(filepath.Join(dir, "covers-colour"), 0o755)
	out := filepath.Join(dir, "covers-colour", b.Hash+".png")
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return out, png.Encode(f, small)
}

func shrinkRGBA(pic image.Image, w, h int) *image.RGBA {
	b := pic.Bounds()
	s := max(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	s = min(s, 1)
	ow, oh := max(int(float64(b.Dx())*s+0.5), 1), max(int(float64(b.Dy())*s+0.5), 1)
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := range oh {
		y0, y1 := b.Min.Y+y*b.Dy()/oh, b.Min.Y+(y+1)*b.Dy()/oh
		for x := range ow {
			x0, x1 := b.Min.X+x*b.Dx()/ow, b.Min.X+(x+1)*b.Dx()/ow
			var r, g, bl, n uint32
			for yy := y0; yy < max(y1, y0+1); yy += 2 {
				for xx := x0; xx < max(x1, x0+1); xx += 2 {
					cr, cg, cb, _ := pic.At(xx, yy).RGBA()
					r, g, bl, n = r+cr>>8, g+cg>>8, bl+cb>>8, n+1
				}
			}
			out.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255})
		}
	}
	return out
}

func readPNG(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	p, _ := png.Decode(f)
	return p
}

func size(path string) float64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return float64(st.Size()) / 1024
}

// tile is where cover i goes on the page: four a row from under a head, as the library lays them out.
func tile(i int) image.Rectangle {
	x := margin + (i%4)*(tw+gap)
	y := 300 + (i/4)*(th+70)
	return image.Rect(x, y, x+tw, y+th)
}

// fit is the middle of pic cut to r's size (the tile cuts what is over, as the library's).
func fit(pic image.Image, r image.Rectangle) image.Point {
	b := pic.Bounds()
	return image.Pt(b.Min.X+(b.Dx()-r.Dx())/2, b.Min.Y+(b.Dy()-r.Dy())/2)
}

func screen(greys, colours []image.Image, pause time.Duration) {
	d, err := device.Open()
	if err != nil {
		fmt.Println("screen:", err)
		return
	}
	scr := d.Screen
	full := image.Rect(0, 0, scr.W, scr.H)
	grey := image.NewGray(full)
	col := image.NewRGBA(full)
	draw.Draw(grey, full, image.White, image.Point{}, draw.Src)
	draw.Draw(col, full, image.White, image.Point{}, draw.Src)
	headLine := image.Rect(0, 240, scr.W, 244)
	draw.Draw(grey, headLine, image.Black, image.Point{}, draw.Src)
	draw.Draw(col, headLine, image.Black, image.Point{}, draw.Src)
	for i := range min(len(greys), 12) {
		r := tile(i)
		draw.Draw(grey, r, greys[i], fit(greys[i], r), draw.Src)
		draw.Draw(col, r, colours[i], fit(colours[i], r), draw.Src)
	}
	step := func(name string, f func()) {
		t := time.Now()
		f()
		scr.Wait()
		fmt.Printf("screen: %-44s %6.0f ms to the ink\n", name, ms(t))
		time.Sleep(pause)
	}
	step("grey page, GC16 with a flash", func() { scr.Show(grey, device.WfGC16, true) })
	step("colour page, GC16 with a flash", func() { scr.ShowRGBA(col, full, device.WfGC16, true) })
	step("colour page again, REAGL (a page turn)", func() { scr.ShowRGBA(col, full, device.WfREAGL, false) })
	// a panel under the head, over the first row of covers, opened and closed as the library's are (REAGL)
	band := image.Rect(0, 0, scr.W, 760)
	open := image.NewRGBA(full)
	draw.Draw(open, full, col, image.Point{}, draw.Src)
	draw.Draw(open, band, image.White, image.Point{}, draw.Src)
	draw.Draw(open, image.Rect(0, band.Max.Y-4, scr.W, band.Max.Y), image.Black, image.Point{}, draw.Src)
	step("panel opened over the covers, REAGL", func() { scr.ShowRGBA(open, band, device.WfREAGL, false) })
	step("panel closed: the covers back, REAGL", func() { scr.ShowRGBA(col, band, device.WfREAGL, false) })
	step("panel opened, GL16 (partial)", func() { scr.ShowRGBA(open, band, device.WfGL16, false) })
	step("panel closed, GL16 (partial)", func() { scr.ShowRGBA(col, band, device.WfGL16, false) })
	step("the same on the grey page: panel opened, REAGL", func() {
		g := image.NewGray(full)
		draw.Draw(g, full, grey, image.Point{}, draw.Src)
		draw.Draw(g, band, image.White, image.Point{}, draw.Src)
		scr.Show(grey, device.WfGC16, true)
		scr.Wait()
		scr.ShowPart(g, band, device.WfREAGL, false)
	})
}

// compareFormats keeps each colour cover three more ways, then, the file cache emptied (as after a restart), reads each
// back as the library would need it (an *image.RGBA), the colour PNG too: raw RGBA (8 bytes of width and height, then
// the pixels: read straight), raw RGB (three bytes a pixel, made RGBA as it is read), JPEG at quality 90 (decoded, then
// made RGBA). Sizes and read times, the mean of all.
func compareFormats(dir string, books []library.Book, colours []image.Image) {
	os.MkdirAll(filepath.Join(dir, "covers-formats"), 0o755)
	var bases []string
	for i, c := range colours {
		if c == nil {
			continue
		}
		src := image.NewRGBA(c.Bounds())
		draw.Draw(src, src.Rect, c, c.Bounds().Min, draw.Src)
		w, h := src.Rect.Dx(), src.Rect.Dy()
		base := filepath.Join(dir, "covers-formats", books[i].Hash)
		hdr := []byte{byte(w >> 24), byte(w >> 16), byte(w >> 8), byte(w), byte(h >> 24), byte(h >> 16), byte(h >> 8), byte(h)}
		os.WriteFile(base+".rgba", append(hdr, src.Pix...), 0o644)
		rgb := make([]byte, 0, len(hdr)+w*h*3)
		rgb = append(rgb, hdr...)
		for j := 0; j < len(src.Pix); j += 4 {
			rgb = append(rgb, src.Pix[j], src.Pix[j+1], src.Pix[j+2])
		}
		os.WriteFile(base+".rgb", rgb, 0o644)
		f, _ := os.Create(base + ".jpg")
		jpeg.Encode(f, src, &jpeg.Options{Quality: 90})
		f.Close()
		if b, err := os.ReadFile(filepath.Join(dir, "covers-colour", books[i].Hash+".png")); err == nil { // a copy: FAT has no links
			os.WriteFile(base+".png", b, 0o644)
		}
		bases = append(bases, base)
	}
	dims := func(b []byte) (int, int) {
		return int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3]), int(b[4])<<24 | int(b[5])<<16 | int(b[6])<<8 | int(b[7])
	}
	toRGBA := func(p image.Image) *image.RGBA {
		out := image.NewRGBA(p.Bounds())
		draw.Draw(out, out.Rect, p, p.Bounds().Min, draw.Src)
		return out
	}
	exts := []string{".png", ".rgba", ".rgb", ".jpg"}
	reads := []func(string) *image.RGBA{
		func(f string) *image.RGBA { return toRGBA(readPNG(f)) },
		func(f string) *image.RGBA {
			b, _ := os.ReadFile(f)
			w, h := dims(b)
			return &image.RGBA{Pix: b[8:], Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
		},
		func(f string) *image.RGBA {
			b, _ := os.ReadFile(f)
			w, h := dims(b)
			out := image.NewRGBA(image.Rect(0, 0, w, h))
			for j, k := 8, 0; j < len(b); j, k = j+3, k+4 {
				out.Pix[k], out.Pix[k+1], out.Pix[k+2], out.Pix[k+3] = b[j], b[j+1], b[j+2], 255
			}
			return out
		},
		func(f string) *image.RGBA {
			r, _ := os.Open(f)
			defer r.Close()
			p, _ := jpeg.Decode(r)
			return toRGBA(p)
		},
	}
	var sz, cold, warm [4]float64
	for pass, into := range []*[4]float64{&cold, &warm} {
		if pass == 0 { // cold: as after a restart
			exec.Command("sync").Run()
			os.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0o200)
		}
		for _, base := range bases {
			for k, ext := range exts {
				t := time.Now()
				reads[k](base + ext)
				into[k] += ms(t)
				if pass == 0 {
					sz[k] += size(base + ext)
				}
			}
		}
	}
	n := float64(len(bases))
	fmt.Printf("\nformats (mean of %d covers, each read as the library needs it):\n", len(bases))
	for k, name := range []string{"PNG", "raw RGBA", "raw RGB", "JPEG q90"} {
		fmt.Printf("  %-10s %6.0f KB  read cold %5.1f ms  warm %5.1f ms\n", name, sz[k]/n, cold[k]/n, warm[k]/n)
	}
}
