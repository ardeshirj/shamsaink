//go:build blitz

package ui

import (
	"image"
	"image/png"
	"os"
	"testing"

	blitz "shamsainkengine"
)

// Every button of the Text sheet is found, on the panel, apart from the others, on both its tabs. FONTS: a fallback
// fonts folder (as on the device); MENU_PNG: where to write the sheet's Page tab, the buttons framed, to look at.
func TestText(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	const w, h = 1236, 1648
	var m *Panel
	for tab, want := range map[string][]string{"text": textIDs, "page": pageIDs} {
		m = DrawText(Text{Tab: tab, FontSize: 50, LineHeight: 1.35, Lines: 1.16, Margin: 60, Dir: "rtl"}, fonts, w, h)
		if r := m.Rect(); r.Max.Y != h || r.Min.Y <= h/3 {
			t.Errorf("%s: sheet at %v, want at the bottom of %d", tab, r, h)
		}
		for _, id := range want {
			r, ok := m.buttons[id]
			if !ok || r.Empty() || !r.In(m.img.Bounds()) {
				t.Errorf("%s: %s: %v (found %v)", tab, id, r, ok)
			}
			for other, q := range m.buttons {
				// a bar's segments share their border, no more
				if o := r.Intersect(q); other != id && !o.Empty() && min(o.Dx(), o.Dy()) > 3 {
					t.Errorf("%s: %s overlaps %s", tab, id, other)
				}
			}
		}
	}
	m = DrawText(Text{Tab: "page", FontSize: 50, Lines: 1.16, Margin: 60, Dir: "rtl"}, fonts, w, h)
	// a tap goes to the button under it, or the nearest: on either side of a shared border, near a button, not far
	rtl, ltr := m.buttons["dir-rtl"], m.buttons["dir-ltr"]
	mid := (rtl.Min.Y+rtl.Max.Y)/2 + m.y0
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
}

// The buttons the sheet's two tabs and the dock are expected to have.
var textIDs = []string{"tab-text", "tab-page", "size-dec", "size-inc", "lines-dec", "lines-inc", "margin-dec", "margin-inc",
	"reset"}
var pageIDs = []string{"tab-text", "tab-page", "dir-", "dir-rtl", "dir-ltr", "reset"}
var dockIDs = []string{"chapter-prev", "bar", "chapter-next", "back", "contents", "text", "read-aloud"}

// The dock has its buttons, found on it, at the bottom of the screen; the way back and Listen only when there are.
func TestDock(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	const w, h = 1236, 1648
	p := Progress{Chapter: "a chapter's title", At: 3, Of: 12, Page: 40, Total: 160, Share: 0.25, Back: 7, Ticks: []float64{0.5}, Narrated: true}
	dock := DrawDock(p, fonts, w, h)
	for _, id := range dockIDs {
		if b, ok := dock.buttons[id]; !ok || b.Empty() {
			t.Errorf("button %s not on the dock", id)
		}
	}
	if r := dock.Rect(); r.Max.Y != h {
		t.Errorf("dock at %v, want at the bottom of %d", r, h)
	}
	// in a right-to-left book the previous chapter's arrow is at the right
	p.RTL, p.Back, p.Narrated = true, 0, false
	dock = DrawDock(p, fonts, w, h)
	if prev, next := dock.buttons["chapter-prev"], dock.buttons["chapter-next"]; prev.Min.X <= next.Min.X {
		t.Errorf("rtl: previous at %v, next at %v", prev, next)
	}
	for _, id := range []string{"back", "read-aloud"} {
		if _, ok := dock.buttons[id]; ok {
			t.Errorf("%s on the dock without a jump or a narration", id)
		}
	}
}
