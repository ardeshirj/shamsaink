// side puts two page images next to each other at half size, for a quick look: side A.png B.png OUT.png
package main

import (
	"image"
	"image/draw"
	"image/png"
	"os"

	xdraw "golang.org/x/image/draw"
)

func load(p string) image.Image {
	f, err := os.Open(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	return img
}

func main() {
	a, b := load(os.Args[1]), load(os.Args[2])
	w, h := a.Bounds().Dx()/2, a.Bounds().Dy()/2
	out := image.NewGray(image.Rect(0, 0, 2*w+10, h))
	draw.Draw(out, out.Bounds(), image.NewUniform(image.Black), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(out, image.Rect(0, 0, w, h), a, a.Bounds(), draw.Src, nil)
	xdraw.CatmullRom.Scale(out, image.Rect(w+10, 0, 2*w+10, h), b, b.Bounds(), draw.Src, nil)
	f, _ := os.Create(os.Args[3])
	png.Encode(f, out)
	f.Close()
}
