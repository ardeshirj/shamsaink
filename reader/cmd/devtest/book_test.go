package main

import "testing"

// The reader's opening line, as it logs it (2026-11-22, the Kindle; one from before the interface was logged): the book's
// file, its place, its direction and its interface read from it.
func TestOpenedLine(t *testing.T) {
	for _, c := range []struct {
		line, file string
		doc, page  string
		rtl, lang  string
	}{
		{"holmes.epub: document 0 page 0 shown in 464 ms (cache false, right to left false; text 50 px, lines 0, margin 60; interface en)",
			"holmes.epub", "0", "0", "false", "en"},
		{"fa-basics.epub: document 3 page 12 shown in 120 ms (cache true, right to left true)", "fa-basics.epub", "3", "12", "true", ""},
	} {
		m := reOpened.FindStringSubmatch(c.line)
		if m == nil || m[1] != c.file || m[2] != c.doc || m[3] != c.page || m[4] != c.rtl || m[5] != c.lang {
			t.Errorf("%q: %q", c.line, m)
		}
	}
}
