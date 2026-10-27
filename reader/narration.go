package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/pub"

	blitz "indiumengine"
)

// Read-along (roadmap, stage 4): the book's narration played to the Bluetooth headphones paired in the Kindle's own
// settings, each word underlined when the narrator reaches it (experiments 09 and 12). The sound goes through
// Amazon's mplay, from a raw file (it cannot play from a pipe: experiment 12): each part of the narration is decoded
// by the engine into RAM while the one before it plays (in the background: a long part takes ~2 s), and its mplay
// started at its time on the clock.

// A word timed by the book's media overlay: its element in spine document doc, and its clip of an audio file.
type cue struct {
	doc        int
	id         string
	file       string  // the audio file, by its path in the EPUB
	begin, end float64 // s, in the file
	at         float64 // s, on the narration's clock from where it started
}

// cues is the book's narration, the words in reading order (spine, then the SMIL's order); none if it has none.
func cues(ctx context.Context, p *pub.Publication) []cue {
	s, ok := p.FindService(pub.MediaOverlayService_Name).(pub.GuidedNavigationService)
	if !ok {
		return nil
	}
	var out []cue
	var walk func(doc int, objs []guidednavigation.GuidedNavigationObject)
	walk = func(doc int, objs []guidednavigation.GuidedNavigationObject) {
		for _, o := range objs {
			if c := o.AudioClip(); c != nil && c.Begin != nil && c.End != nil && o.TextRef != nil {
				out = append(out, cue{doc: doc, id: o.TextRef.Fragment(), file: o.AudioFile().String(),
					begin: c.Begin.Seconds(), end: c.End.Seconds()})
			}
			walk(doc, o.Children)
		}
	}
	for i, l := range p.Manifest.ReadingOrder {
		href := l.Href.String()
		if !s.HasGuideForResource(href) {
			continue
		}
		if g, err := s.GuideForResource(ctx, href); err == nil {
			walk(i, g.Guided)
		}
	}
	return out
}

// A part of the narration played by one mplay: a stretch of one audio file, from the first of its words to the last.
type part struct {
	file     string
	from, to float64 // s, in the file
	at       float64 // s, on the clock
	raw      string  // decoded, ready to play ("" until then)
	rate, ch int
	done     chan struct{} // closed once decoded (err says if it failed)
	err      error
}

// A narration being played from word first: its parts on one clock, started at start (the first mplay's start; the
// words are underlined lead later, for the headphones' own delay).
type narrator struct {
	cues    []cue
	next    int // the next word to underline
	parts   []*part
	started int // parts whose mplay has started
	start   time.Time
	lead    time.Duration
	read    blitz.ReadFunc
	plays   []*exec.Cmd
	bars    []image.Rectangle // the underline shown, to erase
	dir     string
	playing atomic.Int32  // parts whose mplay has started, for the decoder
	signal  chan struct{} // a part started
	quit    chan struct{}
}

// newNarrator plans the narration from word first: a new part where the file changes or the clips go back.
func newNarrator(all []cue, first int, read blitz.ReadFunc, lead time.Duration) *narrator {
	n := &narrator{cues: append([]cue(nil), all[first:]...), read: read, lead: lead, dir: "/var/tmp/indium",
		signal: make(chan struct{}, 1), quit: make(chan struct{})}
	var p *part
	for i := range n.cues {
		c := &n.cues[i]
		if p == nil || c.file != p.file || c.begin < p.to-0.01 {
			at := 0.0
			if p != nil {
				at = p.at + p.to - p.from
			}
			p = &part{file: c.file, from: c.begin, to: c.end, at: at, done: make(chan struct{})}
			n.parts = append(n.parts, p)
		}
		p.to = max(p.to, c.end)
		c.at = p.at + c.begin - p.from
	}
	return n
}

// begin decodes the first parts, until ~4 s of sound is ready (the first may be a title's second), then the rest in
// the background, each once the part before it has started (so at most two are in RAM, ~14 MB each at most here).
func (n *narrator) begin() error {
	k, ready := 0, 0.0
	for k < len(n.parts) && (k == 0 || ready < 4) {
		if err := n.decode(k); err != nil {
			return err
		}
		ready += n.parts[k].to - n.parts[k].from
		k++
	}
	go func() {
		for i := k; i < len(n.parts); i++ {
			for int(n.playing.Load()) < i {
				select {
				case <-n.signal:
				case <-n.quit:
					return
				}
			}
			n.decode(i)
		}
	}()
	return nil
}

// decode makes part i's raw file (16-bit, the file's own rate and channels), and says it is done.
func (n *narrator) decode(i int) error {
	p := n.parts[i]
	p.err = n.decodeRaw(i)
	close(p.done)
	return p.err
}

func (n *narrator) decodeRaw(i int) error {
	p := n.parts[i]
	data, err := n.read(p.file)
	if err != nil {
		return err
	}
	a := blitz.OpenAudio(data)
	if a == nil {
		return fmt.Errorf("%s: not decodable", p.file)
	}
	defer a.Close()
	p.rate, p.ch = a.Rate(), a.Channels()
	// a seek lands on the MP3 frame before the time (up to ~26 ms): the samples up to it dropped
	skip := 0
	if p.from > 0 {
		reached := a.Seek(p.from)
		if reached < 0 {
			return fmt.Errorf("%s: cannot seek to %.3f s", p.file, p.from)
		}
		skip = int((p.from-reached)*float64(p.rate)) * p.ch
	}
	want := int((p.to - p.from) * float64(p.rate*p.ch))
	buf := make([]int16, 1<<16)
	out := make([]byte, 0, 2*want)
	for len(out) < 2*want {
		k := a.Read(buf)
		if k == 0 {
			break
		}
		got := buf[:k]
		if skip > 0 {
			d := min(skip, len(got))
			got, skip = got[d:], skip-d
		}
		for _, s := range got[:min(len(got), want-len(out)/2)] {
			out = binary.LittleEndian.AppendUint16(out, uint16(s))
		}
	}
	os.MkdirAll(n.dir, 0o755)
	raw := filepath.Join(n.dir, fmt.Sprintf("sound-%d.raw", i))
	if err := os.WriteFile(raw, out, 0o644); err != nil {
		return err
	}
	p.raw = raw
	return nil
}

// play starts part i's mplay; its file is removed once it has played.
func (n *narrator) play(i int) {
	p := n.parts[i]
	cmd := exec.Command("mplay", "-r", strconv.Itoa(p.rate), "-c", strconv.Itoa(p.ch), p.raw)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "indium: mplay:", err)
		return
	}
	n.plays = append(n.plays, cmd)
	go func() { cmd.Wait(); os.Remove(p.raw) }()
	n.playing.Store(int32(i + 1))
	select {
	case n.signal <- struct{}{}:
	default:
	}
}

// wake plays a second of silence, so an idle Bluetooth link is woken (~0.4 s) before the narration's clock starts.
func (n *narrator) wake() {
	os.MkdirAll(n.dir, 0o755)
	raw := filepath.Join(n.dir, "silence.raw")
	if os.WriteFile(raw, make([]byte, 2*44100), 0o644) == nil {
		exec.Command("mplay", "-r", "44100", "-c", "1", raw).Run()
		os.Remove(raw)
	}
}

// stop ends the sound and the decoding, and removes what is left of it.
func (n *narrator) stop() {
	close(n.quit)
	for _, c := range n.plays {
		if c.ProcessState == nil {
			c.Process.Kill()
		}
	}
	for _, p := range n.parts {
		if p.raw != "" {
			os.Remove(p.raw)
		}
	}
}

// due is when the next thing is to be done: a part started or a word underlined; false when the narration is over.
func (n *narrator) due() (time.Time, bool) {
	var t time.Time
	ok := false
	at := func(s float64) time.Time { return n.start.Add(time.Duration(s * float64(time.Second))) }
	if n.started < len(n.parts) {
		t, ok = at(n.parts[n.started].at), true
	}
	if n.next < len(n.cues) {
		if w := at(n.cues[n.next].at).Add(n.lead); !ok || w.Before(t) {
			t, ok = w, true
		}
	}
	return t, ok
}
