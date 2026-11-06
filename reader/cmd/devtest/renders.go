package main

import (
	"path/filepath"
	"strings"
)

// bestRenderIn is the kept render of a screen in a language ("en", or "fa" for a book that reads right to left) that has
// the most buttons, with its boxes: the variant with every entry.
func (d *dev) bestRenderIn(screen, lang string) (string, *boxes) {
	files, _ := filepath.Glob(filepath.Join("reader", "design", "golden", d.name, screen+"-*-"+lang+".json"))
	best, name := (*boxes)(nil), ""
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".json")
		if owner := strings.TrimPrefix(base, screen+"-"); strings.Count(owner, "-") > 1 {
			continue // a render of another screen whose name starts with this one's (menu-… and library-…)
		}
		r, err := d.boxes(base)
		if err != nil {
			continue
		}
		if best == nil || len(r.Boxes) > len(best.Boxes) {
			best, name = r, base
		}
	}
	return name, best
}

// differs is the share of the screen's pixels (under the bar's clock strip) that two readings of it do not share.
func differs(a, b *screen) float64 {
	n := 0
	for i := a.w * 140; i < len(a.pix); i++ {
		if d := int(a.pix[i]) - int(b.pix[i]); d > 40 || d < -40 {
			n++
		}
	}
	return float64(n) / float64(len(a.pix)-a.w*140)
}

// regionDiff is the share of the box's pixels that two readings of the screen do not share.
func regionDiff(a, b *screen, box [4]int) float64 {
	n, total := 0, 0
	for y := max(box[1], 0); y < min(box[3], a.h); y++ {
		for x := max(box[0], 0); x < min(box[2], a.w); x++ {
			total++
			if d := int(a.pix[y*a.w+x]) - int(b.pix[y*a.w+x]); d > 40 || d < -40 {
				n++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}
