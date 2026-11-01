// Command site writes the design system as one HTML page (docs/design/system.html) from the spec and the renders kept in
// reader/design/golden: go run ./design/cmd/site (from reader/), or `make design`.
package main

import (
	"fmt"
	"os"

	"shamsainkreader/design"
)

func main() {
	s, err := design.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
