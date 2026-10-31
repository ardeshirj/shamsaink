//go:build blitz

package ui

import (
	"image/png"
	"os"
	"testing"

	blitz "shamsainkengine"
)

// The panel's two buttons are found, apart. FONTS and PANEL_PNG as TestMenu's.
func TestRemovePanel(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	m := DrawRemove(fonts, 1236)
	a, b := m.buttons["remove-highlight"], m.buttons["keep-highlight"]
	if a.Empty() || b.Empty() || a.Overlaps(b) || !a.In(m.img.Bounds()) || !b.In(m.img.Bounds()) {
		t.Errorf("buttons %v", m.buttons)
	}
	if p := os.Getenv("PANEL_PNG"); p != "" {
		f, _ := os.Create(p)
		defer f.Close()
		png.Encode(f, m.img)
	}
}
