package main

import "testing"

// A book is in Farsi by its stated language; with none, by its direction.
func TestPersian(t *testing.T) {
	for _, c := range []struct {
		langs    []string
		rtl, out bool
	}{{[]string{"fa"}, false, true}, {[]string{"fa-IR"}, false, true}, {[]string{"per"}, false, true},
		{[]string{"en"}, true, false}, {[]string{"ar"}, true, false}, {[]string{"en", "fa"}, false, true},
		{nil, true, true}, {nil, false, false}} {
		if got := persian(c.langs, c.rtl); got != c.out {
			t.Errorf("persian(%v, %v) = %v", c.langs, c.rtl, got)
		}
	}
}
