//go:build blitz

package blitz

import (
	"math"
	"os"
	"testing"
)

// readAll reads every sample left and returns how many there were.
func readAll(a *Audio) int {
	buf := make([]int16, 65536)
	n := 0
	for {
		k := a.Read(buf)
		if k == 0 {
			return n
		}
		n += k
	}
}

// TestAudio decodes a narration MP3 (TEST_MP3: its path) whole, then from a seek to 100 s.
func TestAudio(t *testing.T) {
	path := os.Getenv("TEST_MP3")
	if path == "" {
		t.Skip("TEST_MP3 not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const length = 153.887

	a := OpenAudio(data)
	if a == nil {
		t.Fatal("not decodable")
	}
	if a.Rate() != 44100 || a.Channels() != 1 {
		t.Fatalf("rate %d, channels %d; want 44100, 1", a.Rate(), a.Channels())
	}
	per := float64(a.Rate() * a.Channels()) // samples per second, all channels
	n := readAll(a)
	a.Close()
	secs := float64(n) / per
	t.Logf("whole: %d samples, %.3f s", n, secs)
	if math.Abs(secs-length) > 0.05 {
		t.Errorf("whole length %.3f s; want %.3f", secs, length)
	}

	b := OpenAudio(data)
	if b == nil {
		t.Fatal("not decodable on the second open")
	}
	defer b.Close()
	got := b.Seek(100)
	t.Logf("seek to 100: reached %.3f s", got)
	if got < 0 || math.Abs(got-100) > 0.1 {
		t.Errorf("seek to 100 reached %.3f s", got)
	}
	m := readAll(b)
	rest := float64(m) / per
	t.Logf("after the seek: %d samples, %.3f s", m, rest)
	if math.Abs(rest-(length-100)) > 0.1 {
		t.Errorf("after the seek %.3f s; want %.3f", rest, length-100)
	}
}
