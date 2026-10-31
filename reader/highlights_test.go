//go:build blitz

package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	blitz "shamsainkengine"
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

// A highlight made over others takes them in, a chain of them too (one reached only once the span grew); one beside
// it, in another document, or apart is kept; the text is read again only for the one that took others in.
func TestAddHighlight(t *testing.T) {
	h := &highlights{list: []highlight{
		{Doc: 1, From: [2]int{11, 0}, To: [2]int{11, 10}, Text: "a"},  // reached once the span takes in the next
		{Doc: 1, From: [2]int{11, 8}, To: [2]int{11, 25}, Text: "b"},  // overlaps the new one
		{Doc: 1, From: [2]int{12, 8}, To: [2]int{12, 12}, Text: "c"},  // after it, a space apart
		{Doc: 2, From: [2]int{11, 22}, To: [2]int{11, 26}, Text: "d"}, // another document
		{Doc: 1, From: [2]int{12, 0}, To: [2]int{12, 4}, Text: "e"},   // inside it, in its next block
	}}
	read := 0
	text := func(doc int, from, to [2]int) string { read++; return "new" }
	hl := h.add(highlight{Doc: 1, From: [2]int{11, 20}, To: [2]int{12, 6}}, text)
	if hl.From != [2]int{11, 0} || hl.To != [2]int{12, 6} || hl.Text != "new" || read != 1 {
		t.Errorf("kept %+v, text read %d times", hl, read)
	}
	var left []string
	for _, o := range h.list {
		left = append(left, o.Text)
	}
	if fmt.Sprint(left) != "[c d new]" {
		t.Errorf("left %v", left)
	}
	// saved overlapping before: tidied into one, the others' text not read
	h = &highlights{list: []highlight{{Doc: 1, From: [2]int{0, 0}, To: [2]int{0, 9}, Text: "x"},
		{Doc: 1, From: [2]int{0, 20}, To: [2]int{0, 29}, Text: "y"}, {Doc: 1, From: [2]int{0, 5}, To: [2]int{0, 7}, Text: "z"}}}
	read = 0
	if !h.tidy(text) || len(h.list) != 2 || h.list[1].From != [2]int{0, 0} || h.list[1].To != [2]int{0, 9} || read != 1 {
		t.Errorf("tidied %+v, text read %d times", h.list, read)
	}
}

func TestBadHighlights(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "highlights", "k.json")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("[{broken"), 0o644)
	h := loadHighlights(dir, "k")
	if len(h.list) != 0 {
		t.Errorf("list %v", h.list)
	}
	if b, err := os.ReadFile(p + ".bad"); err != nil || string(b) != "[{broken" {
		t.Errorf(".bad: %q, %v", b, err)
	}
	if _, err := os.Stat(p); err == nil {
		t.Error("bad file still in place")
	}
}
