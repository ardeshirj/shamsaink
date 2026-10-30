//go:build blitz

package main

import (
	"image"
	"testing"

	blitz "indiumengine"
)

// Two lines of words, the second right to left (its words' boxes from the right), two blocks.
var testWords = []blitz.Word{
	{Block: 0, Start: 0, End: 4, Box: image.Rect(60, 60, 160, 120)},
	{Block: 0, Start: 5, End: 9, Box: image.Rect(180, 60, 300, 120)},
	{Block: 1, Start: 0, End: 6, Box: image.Rect(900, 120, 1000, 180)},
	{Block: 1, Start: 7, End: 10, Box: image.Rect(700, 120, 880, 180)},
}

// A selection backwards across both blocks is one highlight from the first word to the last, one band a line.
func TestSelection(t *testing.T) {
	s := &selection{words: testWords, from: 3, to: 1}
	a, b := s.span()
	if a != testWords[1] || b != testWords[3] {
		t.Fatalf("span %v %v", a, b)
	}
	hl := highlight{Doc: 2, From: [2]int{a.Block, a.Start}, To: [2]int{b.Block, b.End}}
	var in []int
	for i, w := range testWords {
		if hl.covers(2, w) != s.in(w) {
			t.Errorf("word %d: highlight %v, selection %v", i, hl.covers(2, w), s.in(w))
		}
		if hl.covers(2, w) {
			in = append(in, i)
		}
	}
	if len(in) != 3 || in[0] != 1 {
		t.Errorf("covered %v", in)
	}
	if hl.covers(1, testWords[2]) {
		t.Error("covers another document")
	}
	bs := bands(testWords, func(w blitz.Word) bool { return hl.covers(2, w) })
	if len(bs) != 2 || bs[0] != image.Rect(180, 60, 300, 120) || bs[1] != image.Rect(700, 120, 1000, 180) {
		t.Errorf("bands %v", bs)
	}
}

func TestWordAt(t *testing.T) {
	for _, c := range []struct{ x, y, want int }{
		{100, 90, 0}, {165, 90, 0}, {175, 90, 1}, {350, 90, -1}, {890, 150, 2}, {100, 150, -1}, {100, 300, -1},
	} {
		if got := wordAt(testWords, c.x, c.y); got != c.want {
			t.Errorf("(%d, %d): %d, want %d", c.x, c.y, got, c.want)
		}
	}
}
