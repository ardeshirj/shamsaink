//go:build blitz

package main

import (
	"image"
	"image/png"
	"os"
	"testing"

	blitz "indiumengine"
)

// Every button of the menu is found, on the panel, apart from the others. FONTS: a fallback fonts folder (as on the
// device); MENU_PNG: where to write the panel, the buttons framed, to look at.
func TestMenu(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	s := &settings{FontSize: 50, LineHeight: 1.35, Margin: 60, Direction: map[string]string{"k": "rtl"}}
	m := drawMenu(s, "k", 1.16, fonts, 1236)
	for _, id := range menuButtons {
		r, ok := m.buttons[id]
		if !ok || r.Empty() || !r.In(m.img.Bounds()) {
			t.Errorf("%s: %v (found %v)", id, r, ok)
		}
		for other, q := range m.buttons {
			if other != id && r.Overlaps(q) {
				t.Errorf("%s overlaps %s", id, other)
			}
		}
	}
	if p := os.Getenv("MENU_PNG"); p != "" {
		img := image.NewGray(m.img.Bounds())
		copy(img.Pix, m.img.Pix)
		for _, r := range m.buttons {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.Pix[r.Min.Y*img.Stride+x], img.Pix[(r.Max.Y-1)*img.Stride+x] = 128, 128
			}
		}
		f, _ := os.Create(p)
		defer f.Close()
		png.Encode(f, img)
	}
	t.Log(m.buttons)
}
