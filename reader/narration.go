package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/pub"

	blitz "shamsainkengine"
	"shamsainkreader/device"
	"shamsainkreader/spec"
)

// Read-along (plan, stage 8): the book's narration played to the Bluetooth headphones paired in Kobo's own settings,
// each word underlined when the narrator reaches it (experiments 09, 12 and 13). The sound goes through Kobo's own
// player, one gst-launch-1.0 into the device's sink (PulseAudio on firmware 6) kept open for the session (sink.go), fed raw sound through a
// pipe: each part of the narration is decoded by the engine into RAM while the one before it plays, and the pipe
// takes it as the sink plays it, so the narration's clock is what the pipe has taken (see anchor), not a timer of ours.

// A word timed by the book's media overlay: its element in spine document doc, and its clip of an audio file.
type cue struct {
	doc        int
	id         string
	file       string  // the audio file, by its path in the EPUB
	begin, end float64 // s, in the file
	at         float64 // s, on the narration's clock from where it started
}

// narrated says whether the book has a narration, without reading it.
func narrated(p *pub.Publication) bool {
	s, ok := p.FindService(pub.MediaOverlayService_Name).(pub.GuidedNavigationService)
	return ok && slices.ContainsFunc(p.Manifest.ReadingOrder, func(l manifest.Link) bool {
		return s.HasGuideForResource(l.Href.String())
	})
}

// cues is the book's narration, the words in reading order (spine, then the SMIL's order); none if it has none, and
// those read so far if ctx is cancelled (Buried Alive's 23771 words: 2.7 s on the Kobo).
func cues(ctx context.Context, p *pub.Publication) []cue {
	s, ok := p.FindService(pub.MediaOverlayService_Name).(pub.GuidedNavigationService)
	if !ok {
		return nil
	}
	var out []cue
	openEnded := 0
	var walk func(doc int, objs []guidednavigation.GuidedNavigationObject)
	walk = func(doc int, objs []guidednavigation.GuidedNavigationObject) {
		for _, o := range objs {
			// the spec's defaults for a clip are in spec.Clip; a clip with no end (or none at all) runs to the end of the file,
			// which needs the file's length and is not done: those words are left out, and counted
			if file, begin, end, kind := spec.Clip(o); o.TextRef != nil {
				switch kind {
				case spec.Clipped:
					out = append(out, cue{doc: doc, id: o.TextRef.Fragment(), file: file, begin: begin, end: end})
				case spec.OpenEnded:
					openEnded++
				}
			}
			walk(doc, o.Children)
		}
	}
	for i, l := range p.Manifest.ReadingOrder {
		href := l.Href.String()
		if ctx.Err() != nil {
			break
		}
		if !s.HasGuideForResource(href) {
			continue
		}
		if g, err := s.GuideForResource(ctx, href); err == nil {
			walk(i, g.Guided)
		}
	}
	if openEnded > 0 {
		fmt.Fprintf(os.Stderr, "shamsaink: narration: %d clips without a clipEnd left out (not supported yet)\n", openEnded)
	}
	return out
}

// maxPart is the longest part, in seconds: a book with one long audio file (IDPF's Moby-Dick: two chapters, 9 MB) would
// otherwise be decoded whole, ~11 MB of sound a minute in RAM.
const maxPart = 60

// A part of the narration: a stretch of one audio file, from the first of its words to the last.
type part struct {
	file     string
	from, to float64       // s, in the file
	at       float64       // s, on the narration's clock
	pcm      []byte        // decoded to the sink's format (once done)
	done     chan struct{} // closed once decoded (err says if it failed)
	err      error
}

// A narration being played from word first: its parts one after the other on one clock.
type narrator struct {
	cues  []cue
	next  int // the next word to underline
	parts []*part
	lead  time.Duration // the words are underlined this much after the sound is taken, for the headphones' own delay
	read  blitz.ReadFunc
	bars  []image.Rectangle // the underline shown, to erase
	turn  *pageTurn         // a page turn due in the middle of the stretch underlined (underline), nil for none

	// the clock: anchor is when (UnixNano) the narration's second 0 was taken by the sink's pipe, found again after every
	// chunk written as now minus the narration's seconds written so far, so a stalled link stalls the clock, and a part
	// decoded late (silence put in its place) moves it. 0 until the first chunk is taken.
	anchor  atomic.Int64
	stopped atomic.Bool
	failed  atomic.Bool

	// the feeder's, used by the sink's goroutine only
	fp         int       // the part being fed
	off        int       // bytes of it fed
	fed        int64     // bytes of narration fed (silence put in not counted)
	decoding   int       // parts whose decoding has started
	lastWrite  time.Time // when the last chunk was taken by the sink's pipe
	lateLogged int       // the part (+1) whose late decoding was logged
	gcSeen     uint32    // Go's collections at the last gap logged

	// What a glitch heard may come from (2026-11-08, to find its cause), logged as it happens and summed up when the
	// narration stops: chunks of silence put in because a part was decoded late, and gaps between two chunks taken by
	// the sink's pipe over gapLog (a chunk is 20 ms and is taken as the sink plays it: a longer gap means the writer or
	// the player was held up, and the player may have run dry). started: when the narration was started.
	started time.Time
	late    atomic.Int64
	gaps    atomic.Int64
	maxGap  atomic.Int64 // ms
}

// gapLog is the gap between two chunks taken by the sink's pipe from which it is logged (60-65 ms is the player's
// usual rhythm, seen 2026-11-08).
const gapLog = 150 * time.Millisecond

// newNarrator plans the narration from word first: a new part where the file changes or the clips go back.
func newNarrator(all []cue, first int, read blitz.ReadFunc, lead time.Duration) *narrator {
	n := &narrator{cues: append([]cue(nil), all[first:]...), read: read, lead: lead}
	var p *part
	for i := range n.cues {
		c := &n.cues[i]
		if p == nil || c.file != p.file || c.begin < p.to-0.01 || c.end-p.from > maxPart {
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

// begin starts decoding the first part in the background (1.5 to 2.8 s on the Kobo: the screen must not wait for it);
// the sink is given silence until it is done, and a failure ends the narration (feed). The rest are decoded as the
// feeder comes to them.
func (n *narrator) begin() {
	n.started = time.Now()
	n.decoding = 1
	go n.decode(0)
}

// prefetch starts decoding the parts after the one being fed until a part's length (maxPart) of sound is ahead, what is
// left of the one being fed counted: the next part is decoded while this one plays, a minute to do it in, and at most
// two parts' sound is in RAM (~11 MB a minute). (Until 2026-11-08 4 s, and the whole of the part being fed counted:
// the next part's decoding started only when it was due, heard as a second of silence at each part's start.)
func (n *narrator) prefetch() {
	ahead := 0.0
	for i := n.fp; i < len(n.parts) && ahead < maxPart; i++ {
		if i >= n.decoding {
			n.decoding = i + 1
			go n.decode(i)
		}
		p := n.parts[i]
		left := p.to - p.from
		if i == n.fp {
			left -= float64(n.off) / sinkBytesPerSecond
		}
		ahead += left
	}
}

// decode makes part i's sound, and says it is done.
func (n *narrator) decode(i int) error {
	p := n.parts[i]
	t := time.Now()
	p.pcm, p.err = n.decodeParts(p)
	fmt.Printf("read aloud: part %d (%.0f s of sound) decoded in %.0f ms, at %.1f s\n", i, p.to-p.from, ms(t), time.Since(n.started).Seconds())
	close(p.done)
	return p.err
}

func (n *narrator) decodeParts(p *part) ([]byte, error) {
	data, err := n.read(p.file)
	if err != nil {
		return nil, err
	}
	a := blitz.OpenAudio(data)
	if a == nil {
		return nil, fmt.Errorf("%s: not decodable", p.file)
	}
	defer a.Close()
	rate, ch := a.Rate(), a.Channels()
	// a seek lands on the MP3 frame before the time (up to ~26 ms): the samples up to it dropped
	skip := 0
	if p.from > 0 {
		reached := a.Seek(p.from)
		if reached < 0 {
			return nil, fmt.Errorf("%s: cannot seek to %.3f s", p.file, p.from)
		}
		skip = int((p.from-reached)*float64(rate)) * ch
	}
	want := int((p.to - p.from) * float64(rate*ch))
	buf := make([]int16, 1<<16)
	in := make([]int16, 0, want)
	for len(in) < want {
		k := a.Read(buf)
		if k == 0 {
			break
		}
		got := buf[:k]
		if skip > 0 {
			d := min(skip, len(got))
			got, skip = got[d:], skip-d
		}
		in = append(in, got[:min(len(got), want-len(in))]...)
	}
	return sinkFormat(in, rate, ch), nil
}

// sinkFormat turns interleaved 16-bit sound of rate and ch channels into the sink's: 48 kHz stereo, little-endian,
// by linear interpolation (speech: no one hears it; a mono voice is put on both sides).
func sinkFormat(in []int16, rate, ch int) []byte {
	frames := len(in) / ch
	n := int(int64(frames) * sinkRate / int64(rate))
	out := make([]byte, 0, n*4)
	for j := 0; j < n; j++ {
		x := float64(j) * float64(rate) / sinkRate
		i := int(x)
		f := x - float64(i)
		i1 := min(i+1, frames-1)
		for c := 0; c < 2; c++ {
			cc := min(c, ch-1)
			a, b := float64(in[i*ch+cc]), float64(in[i1*ch+cc])
			out = binary.LittleEndian.AppendUint16(out, uint16(int16(a+(b-a)*f)))
		}
	}
	return out
}

// feed writes the next chunk to the sink (a chunk of silence while the part due is not decoded yet), and sets the
// clock; false once the narration has all been written (or has failed).
func (n *narrator) feed(w io.Writer) bool {
	if n.stopped.Load() || n.fp >= len(n.parts) {
		return false
	}
	n.prefetch()
	p := n.parts[n.fp]
	chunk, live := silence, false
	select {
	case <-p.done:
		if p.err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: read aloud:", p.err)
			n.failed.Store(true)
			return false
		}
		chunk, live = p.pcm[n.off:min(n.off+len(silence), len(p.pcm))], true
	default: // decoded late: silence until it is, the clock waiting
		if n.anchor.Load() != 0 { // the player started: heard as a pause
			n.late.Add(1)
			if n.lateLogged != n.fp+1 { // once a part
				n.lateLogged = n.fp + 1
				fmt.Printf("read aloud: part %d decoded late, silence put in at %.1f s\n", n.fp, time.Since(n.started).Seconds())
			}
		}
	}
	before := time.Now()
	if _, err := w.Write(chunk); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: read aloud: sound:", err)
		n.failed.Store(true)
		return false
	}
	now := time.Now()
	if n.anchor.Load() != 0 { // the player started: the gaps between the chunks it takes
		g := now.Sub(n.lastWrite)
		if g > gapLog { // split: the time before the write (ours: the writer late) and in it (the player not taking it)
			n.gaps.Add(1)
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("read aloud: a gap of %d ms at %.1f s: %d ms before the write, %d ms in it; Go's collections %d (+%d)\n",
				g.Milliseconds(), time.Since(n.started).Seconds(), before.Sub(n.lastWrite).Milliseconds(),
				now.Sub(before).Milliseconds(), m.NumGC, m.NumGC-n.gcSeen)
			n.gcSeen = m.NumGC
		}
		if g.Milliseconds() > n.maxGap.Load() {
			n.maxGap.Store(g.Milliseconds())
		}
	}
	n.lastWrite = now
	if live {
		n.off += len(chunk)
		n.fed += int64(len(chunk))
		if n.off >= len(p.pcm) {
			p.pcm, n.fp, n.off = nil, n.fp+1, 0
		}
	}
	n.anchor.Store(time.Now().Add(-time.Duration(n.fed) * time.Second / sinkBytesPerSecond).UnixNano())
	return true
}

// stop ends the narration: the feeder stops taking from it.
func (n *narrator) stop() { n.stopped.Store(true) }

// at is when narration second s is heard (by the pipe's count, the lead not added); false before the first chunk.
func (n *narrator) at(s float64) (time.Time, bool) {
	a := n.anchor.Load()
	if a == 0 {
		return time.Time{}, false
	}
	return time.Unix(0, a).Add(time.Duration(s * float64(time.Second))), true
}

// end is the narration's end, on its clock: its last word's end.
func (n *narrator) end() float64 {
	c := n.cues[len(n.cues)-1]
	return c.at + c.end - c.begin
}

// due is when the next thing is to be done: a word underlined, a page turned in the middle of a stretch, or the
// narration's end; false when it is over (or failed). Until the pipe has taken the first chunk (the player starting,
// ~2 s the first time) it asks again soon.
func (n *narrator) due() (time.Time, bool) {
	if n.failed.Load() {
		return time.Now(), false
	}
	s := n.end()
	if n.next < len(n.cues) {
		s = n.cues[n.next].at
	}
	if n.turn != nil && n.turn.at < s { // a page turn in the middle of a stretch comes first
		s = n.turn.at
	}
	t, ok := n.at(s)
	if !ok {
		return time.Now().Add(50 * time.Millisecond), true
	}
	return t.Add(n.lead), true
}

// readAloud starts the narration from the first narrated word on the page shown, or from the word from on it (the
// narrated word whose box meets its; the page's first if none does).
func (s *session) readAloud(from *blitz.Word) {
	r := s.r
	if s.cued != nil { // still being read: waited for
		s.narration = <-s.cued
		s.cued = nil
	}
	first := -1
	for i, c := range s.narration {
		if c.doc < r.doc {
			continue
		}
		p, rs := r.book.Find(c.doc, c.id)
		if c.doc > r.doc || p > r.page() {
			if first < 0 {
				first = i
			}
			break
		}
		if p < r.page() {
			continue
		}
		if first < 0 {
			first = i // the page's first
		}
		if from == nil {
			break
		}
		if slices.ContainsFunc(rs, from.Box.Overlaps) {
			first = i
			break
		}
	}
	if first < 0 {
		return
	}
	t := time.Now()
	if s.snd == nil || s.snd.dead.Load() {
		snd, err := openSink(s.dev)
		if err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: read aloud: no player:", err)
			return
		}
		s.snd = snd
	}
	s.snd.setVolume(s.set.volumeFor(s.dev.Headphones())) // the pair's own volume (sound.go)
	lead := time.Duration(s.set.LeadMs) * time.Millisecond
	if !device.Sim() { // Kobo's player, its buffer larger than the one lead_ms was set with (sink.go)
		lead += sinkLater
	}
	nar := newNarrator(s.narration, first, r.read, lead)
	nar.begin()
	s.nar = nar
	s.dev.Active()
	s.snd.play(nar)
	if !s.snd.settled.Load() { // the player still waking (sink.go): the first word underlined at once, so the press is seen
		s.underline(nar.cues[0]) // to be answered; the voice follows
	}
	fmt.Printf("read aloud from word %d (document %d): started after %.0f ms, %d parts\n",
		first, s.narration[first].doc, ms(t), len(nar.parts))
	s.due()
}

// due sets the word's timer to the next thing the narration has due, or ends it if nothing is.
func (s *session) due() {
	at, ok := s.nar.due()
	if ok && s.nar.next >= len(s.nar.cues) && !time.Now().Before(at) { // the last word has been said
		ok = false
	}
	if !ok {
		s.silence()
		return
	}
	s.word.Reset(time.Until(at))
}

// speak does what the narration has due, in time: the next part played once its time comes, and the word now
// spoken underlined (words due together: the last one).
func (s *session) speak() {
	nar := s.nar
	now := time.Now()
	s.dev.Active() // the narration is not idle time: the Kobo does not sleep in the middle of it
	w := -1
	for nar.next < len(nar.cues) {
		t, ok := nar.at(nar.cues[nar.next].at)
		if !ok || now.Before(t.Add(nar.lead)) {
			break
		}
		w, nar.next = nar.next, nar.next+1
	}
	if w >= 0 {
		s.underline(nar.cues[w])
	}
	if t := nar.turn; t != nil { // the voice has reached the stretch's lines on the next page
		if at, ok := nar.at(t.at); ok && !now.Before(at.Add(nar.lead)) {
			nar.turn = nil
			s.turnMidway(t)
		}
	}
	s.due()
}

// underline marks word c: a bar under each of its boxes, the last one erased, refreshed together with A2; the page
// turned first if the narration has crossed to another. A sentence or paragraph that runs onto the next page is marked
// on its first page, and its page turn is planned for when the voice reaches its first line there (nar.turn): its
// share of the stretch's time as its lines' share of the stretch's width.
func (s *session) underline(c cue) {
	nar := s.nar
	lines := s.r.book.FindAllLines(c.doc, c.id) // a paragraph in its lines, not as one box, on every page it is on
	if len(lines) == 0 {
		return
	}
	page := lines[0].Page
	var here, next []image.Rectangle
	hereW, allW := 0, 0
	for _, l := range lines {
		allW += l.Box.Dx()
		switch l.Page {
		case page:
			here = append(here, l.Box)
			hereW += l.Box.Dx()
		case page + 1:
			next = append(next, l.Box)
		}
	}
	if markLog { // for the end-to-end tests (MARK_LOG=1): what was marked, and in how many boxes (one a line)
		fmt.Printf("mark %s: document %d page %d, %d boxes\n", c.id, c.doc, page, len(here))
	}
	s.narratedPage(c.doc, page)
	s.markLines(here)
	nar.turn = nil
	if len(next) > 0 && allW > 0 {
		at := c.at + (c.end-c.begin)*float64(hereW)/float64(allW) - turnEarly.Seconds()
		nar.turn = &pageTurn{doc: c.doc, page: page + 1, boxes: next, at: max(at, c.at)}
	}
}

// turnEarly is how much before the voice reaches the next page's first line a page turn in the middle of a stretch
// comes (the maintainer, 2026-11-08: on time felt late; the ink takes ~0.3 s, and the eyes a moment to move to the top).
const turnEarly = time.Second

// A pageTurn is a page turn due in the middle of a narrated stretch (underline): at, on the narration's clock, the
// voice reaches the stretch's first line on page (the book's), whose lines there are boxes.
type pageTurn struct {
	doc, page int
	boxes     []image.Rectangle
	at        float64
}

// turnMidway turns the page planned by underline, and marks the stretch's lines on it.
func (s *session) turnMidway(t *pageTurn) {
	if markLog {
		fmt.Printf("mark: document %d page %d, %d boxes, the stretch's rest\n", t.doc, t.page, len(t.boxes))
	}
	s.narratedPage(t.doc, t.page)
	s.markLines(t.boxes)
}

// narratedPage shows page (the book's) of document doc, if it is not the page shown: a page the narration turns counts
// like one turned by hand, every Nth cleaned with a flash (the words due while the screen is busy are skipped, ~0.4 s).
func (s *session) narratedPage(doc, page int) {
	r, nar := s.r, s.nar
	first, _ := r.book.Layout(doc)
	if doc == r.doc && page == first+r.at {
		return
	}
	r.doc, r.at = doc, page-first
	s.shown = s.rd.pages.get(r, r.doc, r.at, page)
	s.rd.pages.touch(s.shown)
	nar.bars = nil
	s.turns++
	clean := s.set.cleanEvery() > 0 && s.turns >= s.set.cleanEvery()
	if clean {
		s.turns = 0
		s.showPage(device.WfGC16, true)
	} else {
		s.showPage(device.WfREAGL, false)
	}
	fmt.Printf("read aloud: page turned at %.1f s%s\n", time.Since(nar.started).Seconds(), map[bool]string{true: ", cleaned"}[clean])
	r.keep(s.rest)
}

// markLines underlines boxes: a bar under each, the bars before erased, refreshed together with A2.
func (s *session) markLines(boxes []image.Rectangle) {
	nar := s.nar
	region := union(nar.bars)
	nar.bars = nar.bars[:0]
	for _, b := range boxes {
		bar := image.Rect(b.Min.X, b.Max.Y+3, b.Max.X, min(b.Max.Y+9, s.scr.H))
		nar.bars = append(nar.bars, bar)
		region = region.Union(bar)
	}
	s.refresh(region, device.WfA2)
}

// silence ends the narration, the underline erased.
func (s *session) silence() {
	nar := s.nar
	nar.stop()
	s.word.Stop()
	region := union(nar.bars)
	nar.bars = nil
	s.refresh(region, device.WfA2)
	fmt.Printf("read aloud: stopped at word %d of %d after %.0f s: %d chunks late, %d gaps over %d ms, the longest %d ms; second core %s\n",
		nar.next, len(nar.cues), time.Since(nar.started).Seconds(), nar.late.Load(), nar.gaps.Load(), gapLog.Milliseconds(),
		nar.maxGap.Load(), cpuOnline(1))
	s.nar = nil
	s.ahead = true
}

// cpuOnline says whether CPU core n is switched on ("on", "off"; "?" where the kernel does not say: the Kindle's, the
// simulator's). The Kobo's second core is switched on by Kobo's own interface only (experiment 13).
func cpuOnline(n int) string {
	b, err := os.ReadFile(fmt.Sprintf("/sys/devices/system/cpu/cpu%d/online", n))
	switch {
	case err != nil:
		return "?"
	case strings.TrimSpace(string(b)) == "1":
		return "on"
	}
	return "off"
}
