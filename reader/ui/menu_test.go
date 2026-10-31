//go:build blitz

package ui

import (
	"image"
	"image/png"
	"os"
	"testing"

	blitz "shamsainkengine"
)

// Every button of the menu is found, on the panel, apart from the others. FONTS: a fallback fonts folder (as on the
// device); MENU_PNG: where to write the panel, the buttons framed, to look at.
func TestMenu(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	m := DrawMenu(Menu{FontSize: 50, LineHeight: 1.35, Lines: 1.16, Margin: 60, Dir: "rtl", BookRTL: true, Narrated: true}, fonts, 1236)
	for _, id := range menuIDs {
		r, ok := m.buttons[id]
		if !ok || r.Empty() || !r.In(m.img.Bounds()) {
			t.Errorf("%s: %v (found %v)", id, r, ok)
		}
		for other, q := range m.buttons {
			// a bar's segments share their border, no more
			if o := r.Intersect(q); other != id && !o.Empty() && min(o.Dx(), o.Dy()) > 3 {
				t.Errorf("%s overlaps %s", id, other)
			}
		}
	}
	// a tap goes to the button under it, or the nearest: on either side of a shared border, near a button, not far
	rtl, ltr := m.buttons["dir-rtl"], m.buttons["dir-ltr"]
	mid := (rtl.Min.Y + rtl.Max.Y) / 2
	for _, c := range []struct {
		x, y int
		id   string
	}{{(rtl.Min.X + rtl.Max.X) / 2, mid, "dir-rtl"}, {ltr.Min.X - 3, mid, "dir-rtl"}, {rtl.Max.X + 2, mid, "dir-ltr"},
		{ltr.Max.X + 10, mid, "dir-ltr"}, {ltr.Max.X + 40, mid, ""}} {
		if got := m.At(c.x, c.y); got != c.id {
			t.Errorf("tap at (%d, %d): %q, want %q", c.x, c.y, got, c.id)
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

// The buttons the menu and the bottom panel are expected to have.
var menuIDs = []string{"leave", "reset", "read-aloud", "size-dec", "size-inc", "lines-dec", "lines-inc", "margin-dec",
	"margin-inc", "dir-", "dir-rtl", "dir-ltr", "clock"}
var navIDs = []string{"contents", "chapter-prev", "chapter-next", "bar", "back"}

// The bottom panel has its buttons, found on it.
func TestNav(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	nav := DrawNav(Progress{Chapter: "a chapter's title", At: 3, Of: 12, Page: 40, Total: 160, Share: 0.25, Back: 7}, fonts, 1236, 1648)
	for _, id := range navIDs {
		if b, ok := nav.buttons[id]; !ok || b.Empty() {
			t.Errorf("button %s not on the panel", id)
		}
	}
}
