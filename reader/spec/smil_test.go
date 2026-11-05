// Package spec holds tests of what the EPUB specification defines for narration, which our dependencies and our code must
// do (EPUB 3.3, media overlays, and Reading Systems 3.3, section 9; see docs/design.md).
package spec

import (
	"math"
	"testing"

	"github.com/readium/go-toolkit/pkg/parser/epub"
)

// The clock values of clipBegin and clipEnd (SMIL 3.0 clock value syntax: the spec's own examples "23s", "10s",
// "0:23:22.000", and the other forms of the grammar: a full clock, a partial one, and timecounts with h, min, s, ms).
func TestClockValues(t *testing.T) {
	for raw, want := range map[string]float64{
		"23s":          23,
		"10s":          10,
		"0:23:22.000":  23*60 + 22,
		"0:00:24.500":  24.5,
		"01:02":        62,
		"1:02:03.5":    3723.5,
		"1.5h":         5400,
		"2min":         120,
		"500ms":        0.5,
		"7.2":          7.2,
		"  12.25s  ":   12.25,
		"0:00:29.268":  29.268,
		"00:00:00.000": 0,
	} {
		got := epub.ParseClockValue(raw)
		if got == nil || math.Abs(*got-want) > 1e-9 {
			t.Errorf("%q: got %v, want %v", raw, got, want)
		}
	}
	// no value: the clip's begin is 0 and its end the end of the file (the caller's rule, not the parser's)
	if got := epub.ParseClockValue(""); got != nil {
		t.Errorf("empty: got %v, want none", *got)
	}
	for _, bad := range []string{"abc", "5x", "s"} {
		if got := epub.ParseClockValue(bad); got != nil {
			t.Errorf("%q: got %v, want none", bad, *got)
		}
	}
}
