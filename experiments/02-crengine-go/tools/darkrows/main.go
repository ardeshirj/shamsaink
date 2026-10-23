// darkrows prints, for each PNG given, the first and last rows holding ink (text), to compare page layouts without looking.
package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
)

func main() {
	for _, p := range os.Args[1:] {
		f, _ := os.Open(p)
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			fmt.Println(p, err)
			continue
		}
		b := img.Bounds()
		first, last := -1, -1
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, _, _, _ := img.At(x, y).RGBA()
				if r < 0x8000 {
					if first < 0 {
						first = y
					}
					last = y
					break
				}
			}
		}
		fmt.Println(p, first, last)
	}
}
