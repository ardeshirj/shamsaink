package main

import (
	"fmt"
	"os"

	"github.com/benoitkugler/webrender/html/document"
	"github.com/benoitkugler/webrender/html/tree"
	"github.com/benoitkugler/webrender/text"
	"github.com/benoitkugler/webrender/utils"
	"github.com/go-text/typesetting/fontscan"
)

func main() {
	fc := text.NewFontConfigurationGotext(fontscan.NewFontMap(nil))
	for _, in := range []utils.ContentInput{utils.InputString("<html><body><p>hello</p></body></html>"), utils.InputFilename(os.Args[1])} {
		h, err := tree.NewHTML(in, "", nil, "")
		fmt.Println("err", err, h.Root != nil, h.BaseUrl)
		if h.Root != nil {
			fmt.Println("root", h.Root.Data, h.Root.Type)
		}
		func() {
			defer func() { fmt.Println("recover:", recover()) }()
			d := document.Render(h, nil, true, fc)
			fmt.Println("pages", len(d.Pages))
		}()
	}
}
