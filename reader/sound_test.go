package main

import (
	"encoding/binary"
	"testing"
)

// The Sound panel's buttons step the volume by 10, never under minVolume nor over 100, kept per pair of headphones and
// across starts.
func TestVolumeSteps(t *testing.T) {
	rd := &reader{dir: t.TempDir()}
	set := defaults()
	a, b := "00:11:22:33:44:55", "66:77:88:99:AA:BB"
	if v := set.volumeFor(a); v != 100 {
		t.Fatalf("a pair never set: %d, want 100", v)
	}
	if rd.soundTap(set, nil, a, "volume-inc", 0) {
		t.Error("over 100")
	}
	for range 3 {
		rd.soundTap(set, nil, a, "volume-dec", 0)
	}
	if v := set.volumeFor(a); v != 70 {
		t.Errorf("three steps down: %d, want 70", v)
	}
	for range 12 {
		rd.soundTap(set, nil, a, "volume-dec", 0)
	}
	if v := set.volumeFor(a); v != minVolume {
		t.Errorf("all the way down: %d, want %d", v, minVolume)
	}
	if v := set.volumeFor(b); v != 100 {
		t.Errorf("another pair: %d, want its own 100", v)
	}
	if v := loadSettings(rd.dir).volumeFor(a); v != minVolume {
		t.Errorf("after a start: %d, want %d kept", v, minVolume)
	}
}

// The gain: 100 leaves the sound as it is; 50 is a quarter of the amplitude (squared, about 12 dB quieter), negative
// samples too.
func TestVolumeGain(t *testing.T) {
	samples := []int16{0, 1000, -1000, 32767, -32768}
	in := make([]byte, 2*len(samples))
	for i, v := range samples {
		binary.LittleEndian.PutUint16(in[2*i:], uint16(v))
	}
	k := &sink{}
	for _, c := range []struct {
		volume int
		want   []int16
	}{
		{100, samples},
		{50, []int16{0, 250, -250, 8191, -8192}},
	} {
		k.setVolume(c.volume)
		out := in
		if g := k.gain.Load(); g < 1024 {
			out = scale(nil, in, g)
		}
		for i, w := range c.want {
			if got := int16(binary.LittleEndian.Uint16(out[2*i:])); got != w {
				t.Errorf("volume %d, sample %d: %d, want %d", c.volume, samples[i], got, w)
			}
		}
	}
}
