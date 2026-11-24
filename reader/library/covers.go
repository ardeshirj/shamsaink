package library

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // the pictures a cover may be (EPUB's core media types)
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// A book's cover, made small (2026-11-23): its picture (Book.Cover, found by the scan) decoded once, at about a tile's
// size, kept as DIR/covers/HASH.rgb by the book's hash (a new edition has its own; identical copies share one), so the
// library reads a small file and never decodes a cover as it is shown (Holmes's 1400×2100 JPEG: 0.45 s to decode on the
// Kindle). Made in the background while the library is open (librarycovers.go); a missing one is made again.
// Kept in colour, as raw pixels (2026-11-24, experiment 19): a colour screen shows it in colour, a grey one its grey;
// read in 6–8 ms on the Kobo, quicker than the grey PNG it replaces (9 ms), for about 320 KB a book. A cover kept as a
// PNG before is made again. docs/DESIGN.md, "Covers".

// CoverFile is where a book's small cover is kept.
func CoverFile(dir, hash string) string { return filepath.Join(dir, "covers", hash+".rgb") }

// oldCoverFile is where it was kept before 2026-11-24 (grey, a PNG): removed once the new one is made.
func oldCoverFile(dir, hash string) string { return filepath.Join(dir, "covers", hash+".png") }

// CoversMade are the hashes whose small cover is kept, read in one listing of DIR/covers.
func CoversMade(dir string) map[string]bool {
	made := map[string]bool{}
	es, _ := os.ReadDir(filepath.Join(dir, "covers"))
	for _, e := range es {
		if h, ok := strings.CutSuffix(e.Name(), ".rgb"); ok && !strings.HasPrefix(h, ".") {
			made[h] = true
		}
	}
	return made
}

// A Cover is a small cover as kept: W by H pixels, three bytes each (red, green, blue), row by row.
type Cover struct {
	W, H int
	RGB  []byte
}

// coverHead is the kept file's start: its width and height, four bytes each, little-endian; the pixels follow.
const coverHead = 8

// ReadCover reads a kept cover.
func ReadCover(file string) (Cover, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return Cover{}, err
	}
	if len(b) < coverHead {
		return Cover{}, fmt.Errorf("%s: too short", file)
	}
	c := Cover{W: int(binary.LittleEndian.Uint32(b)), H: int(binary.LittleEndian.Uint32(b[4:])), RGB: b[coverHead:]}
	if c.W <= 0 || c.H <= 0 || len(c.RGB) != c.W*c.H*3 {
		return Cover{}, fmt.Errorf("%s: %d by %d, %d bytes", file, c.W, c.H, len(c.RGB))
	}
	return c, nil
}

// Grey is the cover's grey: each pixel's light, green weighing most (as image/color's GrayModel, and the screen's).
func (c Cover) Grey() *image.Gray {
	g := image.NewGray(image.Rect(0, 0, c.W, c.H))
	for i := range g.Pix {
		p := c.RGB[i*3 : i*3+3]
		g.Pix[i] = uint8((19595*uint32(p[0]) + 38470*uint32(p[1]) + 7471*uint32(p[2]) + 1<<15) >> 16)
	}
	return g
}

// MakeCover makes book b's small cover from its picture: its shape kept, scaled to the least size that fills a w by h
// box (the tile cuts what is over), written whole (a hidden name, then renamed) so a half-made one is never read.
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
	tmp := filepath.Join(dir, "covers", "."+b.Hash+".rgb")
	out := make([]byte, coverHead, coverHead+len(small.RGB))
	binary.LittleEndian.PutUint32(out, uint32(small.W))
	binary.LittleEndian.PutUint32(out[4:], uint32(small.H))
	if err := os.WriteFile(tmp, append(out, small.RGB...), 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, CoverFile(dir, b.Hash)); err != nil {
		return err
	}
	os.Remove(oldCoverFile(dir, b.Hash))
	return nil
}

// shrink is pic, its shape kept, at the least size filling w by h (never larger than it is), each pixel the mean of
// every other source pixel under it each way (enough for a picture this small, and four times quicker). A JPEG's
// colours are read from its planes straight; other pictures go through their colour model.
func shrink(pic image.Image, w, h int) Cover {
	b := pic.Bounds()
	s := max(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	s = min(s, 1)
	ow, oh := max(int(float64(b.Dx())*s+0.5), 1), max(int(float64(b.Dy())*s+0.5), 1)
	rgb := func(x, y int) (uint32, uint32, uint32) {
		r, g, b, _ := pic.At(x, y).RGBA()
		return r >> 8, g >> 8, b >> 8
	}
	switch p := pic.(type) {
	case *image.YCbCr:
		rgb = func(x, y int) (uint32, uint32, uint32) {
			c := p.COffset(x, y)
			r, g, b := color.YCbCrToRGB(p.Y[p.YOffset(x, y)], p.Cb[c], p.Cr[c])
			return uint32(r), uint32(g), uint32(b)
		}
	case *image.Gray:
		rgb = func(x, y int) (uint32, uint32, uint32) {
			g := uint32(p.Pix[p.PixOffset(x, y)])
			return g, g, g
		}
	}
	out := Cover{W: ow, H: oh, RGB: make([]byte, ow*oh*3)}
	for y := range oh {
		y0, y1 := b.Min.Y+y*b.Dy()/oh, b.Min.Y+max((y+1)*b.Dy()/oh, y*b.Dy()/oh+1)
		for x := range ow {
			x0, x1 := b.Min.X+x*b.Dx()/ow, b.Min.X+max((x+1)*b.Dx()/ow, x*b.Dx()/ow+1)
			var sr, sg, sb, n uint32
			for yy := y0; yy < y1; yy += 2 {
				for xx := x0; xx < x1; xx += 2 {
					r, g, b := rgb(xx, yy)
					sr, sg, sb, n = sr+r, sg+g, sb+b, n+1
				}
			}
			i := (y*ow + x) * 3
			out.RGB[i], out.RGB[i+1], out.RGB[i+2] = uint8(sr/n), uint8(sg/n), uint8(sb/n)
		}
	}
	return out
}
