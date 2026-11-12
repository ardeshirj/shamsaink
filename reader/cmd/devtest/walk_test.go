package main

import (
	"strings"
	"testing"

	"shamsainkreader/design"
)

// On each device's profile, every built screen of the spec is walked, or shown by the reader itself, or dev mode's, or
// reached only through what that device has not (the Kindle's Sound panel); a screen added with no way the walk can make
// to it fails here, in make check. Every route's gesture is one the walk can make.
func TestWalkPlan(t *testing.T) {
	spec, err := design.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, dev := range spec.Devices {
		routes, left := plan(spec, dev.Has)
		for n, why := range left {
			if !strings.HasPrefix(why, "shown by the reader itself") && !strings.HasPrefix(why, "dev mode only") &&
				!strings.HasPrefix(why, "not on this device") {
				t.Errorf("%s: %s: %s", dev.Name, n, why)
			}
		}
		for _, r := range routes {
			t.Logf("%s: from %-8s %s", dev.Name, r.base, r)
		}
		for n, why := range left {
			t.Logf("%s: left     %s: %s", dev.Name, n, why)
		}
	}
}
