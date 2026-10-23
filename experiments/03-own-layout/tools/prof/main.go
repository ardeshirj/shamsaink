// prof lays out the book N times under the CPU profiler: prof BOOK.epub N OUT.pprof
package main

import (
	"context"
	"os"
	"runtime/pprof"
	"strconv"

	"einklayout/epubsrc"
	"einklayout/layout"
)

func main() {
	src, err := epubsrc.Open(context.Background(), os.Args[1])
	if err != nil {
		panic(err)
	}
	n, _ := strconv.Atoi(os.Args[2])
	f, _ := os.Create(os.Args[3])
	pprof.StartCPUProfile(f)
	for i := 0; i < n; i++ {
		b, err := layout.Build(src, layout.Options{Width: 1072, Height: 1448, Margin: 60, FontSize: 34,
			Fallback: "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"})
		if err != nil {
			panic(err)
		}
		b.Shape()
		b.Paginate()
	}
	pprof.StopCPUProfile()
	f.Close()
}
