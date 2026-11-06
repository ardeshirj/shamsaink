package main

import (
	"strings"
	"testing"

	"shamsainkreader/design"
)

// Every built screen of the spec is walked, or shown by the reader itself (a screen added with no way the walk can
// make to it fails here, in make check); every route's gesture is one the walk can make.
func TestWalkPlan(t *testing.T) {
	spec, err := design.Load()
	if err != nil {
		t.Fatal(err)
	}
	routes, left := plan(spec)
	for n, why := range left {
		if !strings.HasPrefix(why, "shown by the reader itself") {
			t.Errorf("%s: %s", n, why)
		}
	}
	for _, r := range routes {
		t.Logf("from %-8s %s", r.base, r)
	}
	for n, why := range left {
		t.Logf("left     %s: %s", n, why)
	}
}
