// dump prints each paragraph's style and gap, for debugging the cascade: dump BOOK.epub [N]
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"einklayout/epubsrc"
	"einklayout/layout"
)

func main() {
	src, err := epubsrc.Open(context.Background(), os.Args[1])
	if err != nil {
		panic(err)
	}
	b, err := layout.Build(src, layout.Options{Width: 1072, Height: 1448, Margin: 60, FontSize: 34})
	if err != nil {
		panic(err)
	}
	n := 8
	if len(os.Args) > 2 {
		n, _ = strconv.Atoi(os.Args[2])
	}
	if len(os.Args) > 3 { // dump BOOK.epub 0 PAGE: the lines of a page
		b.Shape()
		b.Paginate()
		pg, _ := strconv.Atoi(os.Args[3])
		for _, l := range b.Pages[pg] {
			fmt.Printf("top=%.1f h=%.1f runes=%d..%d of %d glyphs=%d width %.2f of %.2f\n", l.Top, l.Height, l.A, l.B, len(l.Para.Text), len(l.Glyphs), l.Natural, l.Avail)
		}
	}
	for i, p := range b.Paras[:min(n, len(b.Paras))] {
		s := p.Style
		fmt.Printf("%d spine=%d gap=%.1f forced=%v x=%.1f..%.1f size=%.1f lh=%.1f align=%s rtl=%v bold=%v fam=%v runes=%d\n",
			i, p.Spine, p.Gap, p.Forced, p.X0, p.X1, s.Size, s.LineHeightPx(), s.Align, s.RTL, s.Bold, s.Family, len(p.Text))
	}
}
