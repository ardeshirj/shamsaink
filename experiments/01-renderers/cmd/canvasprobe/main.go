package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tdewolff/canvas"
)

// Usage: canvasprobe FONTDIR TEXTFILE
func main() {
	family := canvas.NewFontFamily("BookFont")
	family.MustLoadFontFile(os.Args[1]+"/Parastoo.ttf", canvas.FontRegular)
	face := family.Face(34*72/300.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	// The text to wrap: a paragraph from the local test book, passed as a file (book text stays out of git).
	b, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	s := strings.TrimSpace(string(b))
	for _, al := range []canvas.TextAlign{canvas.Left, canvas.Right, canvas.Justify} {
		rt := canvas.NewRichText(face)
		rt.WriteString(s)
		t := rt.ToText(77.7, 0, al, canvas.Top, nil)
		fmt.Println("align", al, "lines", t.Lines(), "W", t.Width)
		t.WalkLines(func(y float64, spans []canvas.TextSpan) {
			for _, sp := range spans {
				fmt.Printf("  y=%.2f X=%.2f W=%.2f end=%.2f dir=%v level=%d\n", y, sp.X, sp.Width, sp.X+sp.Width, sp.Direction, sp.Level)
			}
		})
	}
}
