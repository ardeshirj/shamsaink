// pixdiff compares two folders of page PNGs, page by page in number order: how many pixels differ (by more than
// TOLERANCE grey levels, 0 by default).
//
//	pixdiff DIR_A DIR_B [TOLERANCE]
package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func load(p string) image.Image {
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, _ := image.Decode(f)
	return img
}

func main() {
	a, _ := filepath.Glob(os.Args[1] + "/page-*.png")
	b, _ := filepath.Glob(os.Args[2] + "/page-*.png")
	byNumber := func(s []string) { // page-9 before page-10
		sort.Slice(s, func(i, j int) bool { return len(s[i]) < len(s[j]) || len(s[i]) == len(s[j]) && s[i] < s[j] })
	}
	byNumber(a)
	byNumber(b)
	tol := 0
	if len(os.Args) > 3 {
		tol, _ = strconv.Atoi(os.Args[3])
	}
	same, total := 0, 0
	for i := range a {
		if i >= len(b) {
			break
		}
		x, y := load(a[i]), load(b[i])
		n := 0
		r := x.Bounds()
		for py := r.Min.Y; py < r.Max.Y; py++ {
			for px := r.Min.X; px < r.Max.X; px++ {
				p, _, _, _ := x.At(px, py).RGBA()
				q, _, _, _ := y.At(px, py).RGBA()
				d := int(p>>8) - int(q>>8)
				if d > tol || d < -tol {
					n++
				}
			}
		}
		total++
		if n == 0 {
			same++
		} else if total <= 200 {
			fmt.Printf("page %d: %d px differ\n", i+1, n)
		}
	}
	fmt.Printf("%d of %d pages identical (pages: %d vs %d)\n", same, total, len(a), len(b))
}
