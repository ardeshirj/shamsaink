package library

import (
	"fmt"
	"os"
	"path/filepath"

	"shamsainkreader/files"
)

// guide is the getting-started guide's name, as the package puts it beside the reader (reader/scripts/package.sh) and
// as it is copied into the books.
const guide = "getting-started.epub"

// Welcome copies the getting-started guide the package puts beside the reader (DIR/getting-started.epub) into
// DIR/books the first time the reader starts, when there is no DIR/library.json yet: so the library is not empty on a
// fresh install, and an upgrade, which replaces the copy beside the reader, does not bring back a guide that was removed.
// A book of that name already in the books is left as it is.
func Welcome(dir string) {
	if _, err := os.Stat(filepath.Join(dir, "library.json")); err == nil {
		return
	}
	to := filepath.Join(dir, "books", guide)
	if _, err := os.Stat(to); err == nil {
		return
	}
	b, err := os.ReadFile(filepath.Join(dir, guide))
	if os.IsNotExist(err) {
		return // a build with no guide (a development install)
	}
	if err == nil {
		err = os.MkdirAll(filepath.Dir(to), 0o755)
	}
	if err == nil {
		err = files.Write(to, b)
	}
	if err != nil {
		fmt.Printf("library: the guide not put in the books: %v\n", err)
		return
	}
	fmt.Printf("library: first start, the guide put in the books\n")
}
