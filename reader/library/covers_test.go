package library

import (
	"archive/zip"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// A cover is kept in colour (2026-11-24): its file read back is the picture made small, its shape kept, red where the
// picture is red and blue where it is blue; its grey is the colours' light; a grey PNG kept before is removed, and the
// listing of kept covers finds the new one.
func TestCoverKeptInColour(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "books"), 0o755)
	f, _ := os.Create(filepath.Join(dir, "books", "c.epub"))
	z := zip.NewWriter(f)
	w, _ := z.Create("OEBPS/cover.png")
	pic := image.NewRGBA(image.Rect(0, 0, 200, 300)) // red above, blue below
	for y := range 300 {
		for x := range 200 {
			c := color.RGBA{220, 20, 20, 255}
			if y >= 150 {
				c = color.RGBA{20, 20, 220, 255}
			}
			pic.Set(x, y, c)
		}
	}
	png.Encode(w, pic)
	z.Close()
	f.Close()
	os.MkdirAll(filepath.Join(dir, "covers"), 0o755)
	os.WriteFile(oldCoverFile(dir, "h"), []byte("a grey PNG"), 0o644)

	b := Book{Path: "books/c.epub", Cover: "OEBPS/cover.png", Hash: "h"}
	if err := MakeCover(dir, b, 20, 30); err != nil {
		t.Fatal(err)
	}
	c, err := ReadCover(CoverFile(dir, "h"))
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 20 || c.H != 30 {
		t.Fatalf("kept %d by %d, want 20 by 30", c.W, c.H)
	}
	at := func(x, y int) []byte { return c.RGB[(y*c.W+x)*3 : (y*c.W+x)*3+3] }
	if p := at(10, 5); p[0] != 220 || p[2] != 20 {
		t.Errorf("top %v, want red", p)
	}
	if p := at(10, 25); p[0] != 20 || p[2] != 220 {
		t.Errorf("bottom %v, want blue", p)
	}
	if g, want := c.Grey().GrayAt(10, 5).Y, color.GrayModel.Convert(color.RGBA{220, 20, 20, 255}).(color.Gray).Y; g != want {
		t.Errorf("red's grey %d, want %d", g, want)
	}
	if _, err := os.Stat(oldCoverFile(dir, "h")); err == nil {
		t.Error("the grey PNG kept before is still there")
	}
	if !CoversMade(dir)["h"] {
		t.Error("CoversMade does not list it")
	}
	// a file cut short is not read as a cover (made again)
	os.WriteFile(CoverFile(dir, "cut"), []byte{20, 0, 0, 0, 30, 0, 0, 0, 1, 2, 3}, 0o644)
	if _, err := ReadCover(CoverFile(dir, "cut")); err == nil {
		t.Error("a cut file read as a cover")
	}
}
