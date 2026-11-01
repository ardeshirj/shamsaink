// Command gen writes what the design spec makes, from reader/ (`go run ./design/cmd/gen`, or `make design`): the
// variables of ui/templates/ui.css, ui/tokens_gen.go, and the page docs/design/system.html from the spec and the renders
// kept in design/golden.
package main

import (
	"fmt"
	"os"

	"shamsainkreader/design"
)

func main() {
	what := "all"
	if len(os.Args) > 1 {
		what = os.Args[1] // tokens: ui.css and tokens_gen.go; page: system.html (after the renders are kept)
	}
	s, err := design.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var css []byte
	if what != "page" {
		css, err = os.ReadFile("ui/templates/ui.css")
	}
	if what != "page" && err == nil {
		var out string
		if out, err = design.TokensCSS(string(css), s.Tokens); err == nil {
			err = os.WriteFile("ui/templates/ui.css", []byte(out), 0o644)
		}
	}
	if what != "page" && err == nil {
		err = os.WriteFile("ui/tokens_gen.go", []byte(design.TokensGo(s.Tokens)), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if what == "tokens" {
		return
	}
	page, err := design.Site(s, os.DirFS("design/golden"), "../../reader/design/golden")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll("../docs/design", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile("../docs/design/system.html", page, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
