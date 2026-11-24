package library

import (
	"archive/zip"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // the pictures a cover may be (EPUB's core media types)
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// A book's cover, made small (2026-11-23): its picture (Book.Cover, found by the scan) decoded once, in grey, at about
// a tile's size, kept as DIR/covers/HASH.png by the book's hash (a new edition has its own; identical copies share
// one), so the library reads a small file and never decodes a cover as it is shown (Holmes's 1400×2100 JPEG: 0.45 s to
// decode on the Kindle; its kept picture 20 ms to read). Made in the background while the library is open
// (libraryview.go); a missing one is made again. docs/DESIGN.md, "Covers".

// CoverFile is where a book's small cover is kept.
func CoverFile(dir, hash string) string { return filepath.Join(dir, "covers", hash+".png") }

// CoversMade are the hashes whose small cover is kept, read in one listing of DIR/covers.
func CoversMade(dir string) map[string]bool {
	made := map[string]bool{}
	es, _ := os.ReadDir(filepath.Join(dir, "covers"))
	for _, e := range es {
		if h, ok := strings.CutSuffix(e.Name(), ".png"); ok && !strings.HasPrefix(h, ".") {
			made[h] = true
		}
	}
	return made
}

// MakeCover makes book b's small cover from its picture: grey, its shape kept, scaled to the least size that fills a w
// by h box (the tile cuts what is over), written whole (a hidden name, then renamed) so a half-made one is never read.
func MakeCover(dir string, b Book, w, h int) error {
	if b.Cover == "" {
		return fmt.Errorf("%s: no cover", b.Path)
	}
	z, err := zip.OpenReader(filepath.Join(dir, b.Path))
	if err != nil {
		return err
	}
	defer z.Close()
	var pic image.Image
	for _, f := range z.File {
		if f.Name == b.Cover {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			pic, _, err = image.Decode(io.LimitReader(rc, 32<<20))
			rc.Close()
			if err != nil {
				return fmt.Errorf("%s: %s: %v", b.Path, b.Cover, err)
			}
			break
		}
	}
	if pic == nil {
		return fmt.Errorf("%s: %s not in the EPUB", b.Path, b.Cover)
	}
	small := shrink(pic, w, h)
	if err := os.MkdirAll(filepath.Join(dir, "covers"), 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "covers", "."+b.Hash+".png")
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := png.Encode(f, small); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, CoverFile(dir, b.Hash))
}

// shrink is pic in grey, its shape kept, at the least size filling w by h (never larger than it is), each pixel the
// mean of every other source pixel under it each way (enough for a picture this small, and four times quicker). A
// JPEG's grey is its Y plane, read straight; other pictures go through their colour model.
func shrink(pic image.Image, w, h int) *image.Gray {
	b := pic.Bounds()
	s := max(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	s = min(s, 1)
	ow, oh := max(int(float64(b.Dx())*s+0.5), 1), max(int(float64(b.Dy())*s+0.5), 1)
	grey := func(x, y int) uint8 { return color.GrayModel.Convert(pic.At(x, y)).(color.Gray).Y }
	switch p := pic.(type) {
	case *image.YCbCr:
		grey = func(x, y int) uint8 { return p.Y[p.YOffset(x, y)] }
	case *image.Gray:
		grey = func(x, y int) uint8 { return p.Pix[p.PixOffset(x, y)] }
	}
	out := image.NewGray(image.Rect(0, 0, ow, oh))
	for y := range oh {
		y0, y1 := b.Min.Y+y*b.Dy()/oh, b.Min.Y+max((y+1)*b.Dy()/oh, y*b.Dy()/oh+1)
		for x := range ow {
			x0, x1 := b.Min.X+x*b.Dx()/ow, b.Min.X+max((x+1)*b.Dx()/ow, x*b.Dx()/ow+1)
			var sum, n uint32
			for yy := y0; yy < y1; yy += 2 {
				for xx := x0; xx < x1; xx += 2 {
					sum += uint32(grey(xx, yy))
					n++
				}
			}
			out.Pix[y*ow+x] = uint8(sum / n)
		}
	}
	return out
}
