package main

import "testing"

// Wi-Fi in the usage lines (2026-11-20): read from wifi=, and an awake stretch's Wi-Fi from both its ends.
func TestUsageWiFi(t *testing.T) {
	ss := stretches(parseUsage(`usage: start at=2026-11-20T08:00:00Z battery=90 plugged=false wifi=on
usage: sleep at=2026-11-20T09:00:00Z battery=88 plugged=false wifi=on
usage: wake at=2026-11-20T17:00:00Z battery=87 plugged=false asleep=28800 wifi=on
usage: stop at=2026-11-20T18:00:00Z battery=85 plugged=false wifi=off
usage: start at=2026-11-20T19:00:00Z battery=85 plugged=false
usage: stop at=2026-11-20T19:30:00Z battery=84 plugged=false`))
	want := []string{"on", "", "on→off", ""}
	if len(ss) != len(want) {
		t.Fatalf("%d stretches, want %d", len(ss), len(want))
	}
	for i, s := range ss {
		if s.wifi() != want[i] {
			t.Errorf("stretch %d (%s): wi-fi %q, want %q", i, s.what, s.wifi(), want[i])
		}
	}
}
