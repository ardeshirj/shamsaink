// Command simvideo makes a video of the simulated Kobo from a recording (e2e/video/README.md): the refreshes the reader
// sent (SHAMSAINK_RECORD's DIR/log and its regions), shown on a drawn e-reader as the ink would show them, the touches
// devtest made, the acts' cards and the steps' titles it wrote, and the sound the reader played. It writes the frames as
// raw RGB to its standard output (for ffmpeg), the sound on the video's clock as DIR/sound.wav, and the times mix.sh
// lays the music by as DIR/times.
//
//	simvideo -rec DIR -fonts DIR -brand DIR [-music PCM] [-fps 30] > frames.rgb
//
// The frame (1920×1080): an act opens with a card of its own (the recording's clock stopped while it is up, so nothing
// is missed), then the e-reader at the right, the stacked logo at the left and the step's name under it. The opening
// and end cards carry the stacked logo (BRAND.md: large title surfaces); the brand's pictures are drawn from its SVGs
// at these sizes (brand.sh), never redrawn. With -music (the take as raw mono S16LE at 22050 Hz) the cards start and
// end on its beats and the end card's list comes a beat at a time. How the ink moves is approximated: a region fades
// from what it showed to what was sent over its waveform's time (measured on the devices: A2 120 ms, GL16 and REAGL
// 300 ms), and a flash (GC16, 370 ms) goes through black and white first.
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	frameW, frameH = 1920, 1080
	screenW        = 1264 // the Kobo Libra Colour's
	screenH        = 1680
	rate           = 48000 // the sink's sound: S16LE, stereo
	sideX          = 440   // the middle of the frame's left part, beside the device: the stacked logo, the step under it
	stackY         = 300   // the stacked logo's top there
	stepY          = 650   // and the step's box under it
)

var (
	bg      = color.RGBA{0xf4, 0xf4, 0xf2, 0xff}
	ink     = color.RGBA{0x16, 0x18, 0x1d, 0xff}
	muted   = color.RGBA{0x5b, 0x61, 0x6b, 0xff}
	saffron = color.RGBA{0xf2, 0xb5, 0x44, 0xff} // the mark's centre: the touches
)

// waveform times, in ms
var duration = map[string]float64{"A2": 120, "GL16": 300, "REAGL": 300, "GC16": 370}

type event struct {
	t         int64 // Unix ms
	kind      string
	rect      image.Rectangle
	wave      string
	flash     bool
	file      string
	words     string // a caption's or a title's
	soundFile string
	touch     string // tap, hold; for a button: upper, lower
	xy        [4]int
}

// card is an act's own screen: the recording's clock stops from the moment it comes (at) until it has gone, so nothing
// of the screen is missed. With the music's beats it waits for the next one (pre: the screen held still), and lasts
// whole beats.
type card struct {
	at    int64   // the recording's time it comes at (Unix ms)
	pre   float64 // s the screen is held before it, to the next beat
	skip  int64   // ms of the recording left out after it: to its resume, else to just before the next thing done
	hold  float64 // s the screen is held still after it, when the next thing done comes sooner than afterCard
	start float64 // its start in the video, s
	secs  float64
	img   *image.RGBA
	items []*image.RGBA // drawn over img a beat apart (the end card's list), each the frame's size
	from  *image.RGBA   // the card just before it, when it follows straight on (faded from, not from the screen)
	last  bool
}

type fonts struct{ regular, bold *opentype.Font }

func main() {
	rec := flag.String("rec", "", "the recording's folder")
	fontDir := flag.String("fonts", "", "the reader's fonts folder")
	brandDir := flag.String("brand", "", "the brand's pictures at the video's sizes (brand.sh)")
	musicPCM := flag.String("music", "", "the music take as raw mono S16LE at 22050 Hz, for its beats")
	fps := flag.Int("fps", 30, "frames a second")
	cardSecs := flag.Float64("card", 2.4, "seconds an act's card is shown, at least")
	list := flag.Bool("list", false, "write the sound and the times, and list the sections (no frames)")
	section := flag.Int("section", -1, "draw only this section (from -list); -1: all")
	flag.Parse()
	if *rec == "" || *fontDir == "" || *brandDir == "" {
		flag.Usage()
		os.Exit(2)
	}
	events, err := readLog(filepath.Join(*rec, "log"))
	check(err)
	if len(events) == 0 {
		check(fmt.Errorf("nothing recorded in %s", *rec))
	}
	t0 := events[0].t
	f := fonts{loadFont(filepath.Join(*fontDir, "Vazirmatn-Regular.ttf")), loadFont(filepath.Join(*fontDir, "Vazirmatn-Bold.ttf"))}
	brand := func(name string) image.Image { return loadPNG(filepath.Join(*brandDir, name)) }
	side, stack, stackEnd := brand("stack-side.png"), brand("stack.png"), brand("stack-end.png")

	// the beats: none without music, else the take's (its first at phase, one each period)
	period, phase := 0.0, 0.0
	if *musicPCM != "" {
		period, phase = beats(*musicPCM)
		fmt.Fprintf(os.Stderr, "simvideo: the music's beat %.3f s (%.1f bpm), the first at %.3f s\n", period, 60/period, phase)
	}
	onBeat := func(t float64) float64 { // the first beat at t or after
		if period == 0 {
			return t
		}
		return phase + math.Ceil((t-phase)/period-1e-6)*period
	}
	whole := func(secs float64) float64 { // secs, as whole beats (at least as long)
		if period == 0 {
			return secs
		}
		return math.Ceil(secs/period-1e-6) * period
	}
	beat := period
	if beat == 0 {
		beat = 0.5
	}

	// the cards and their times in the video: the recording's, plus the cards (and their waits) before them
	var cards []*card
	var endAt int64 = -1
	shift := 0.0
	readalong, readalongSecs := -1.0, 0.0
	for i, e := range events {
		if e.kind != "caption" {
			continue
		}
		natural := float64(e.t-t0)/1000 + shift
		c := &card{at: e.t, start: onBeat(natural)}
		c.pre = c.start - natural
		parts := splitBar(e.words)
		switch {
		case parts[0] == "end":
			c.last, endAt = true, e.t
			c.img, c.items = endCard(f, stackEnd, parts[1:])
			c.secs = whole(beat*float64(2+len(c.items)) + 3.5) // the list a beat at a time, then held
		case parts[0] == "ShamsaInk":
			c.img, c.secs = openingCard(f, stack, parts[1:]), whole(math.Max(*cardSecs+0.6, readSecs(parts)))
		default:
			c.img, c.secs = actCard(f, parts[0], parts[1:]), whole(math.Max(*cardSecs, readSecs(parts)))
		}
		if n := len(cards); n > 0 && e.t-cards[n-1].at < 400 { // straight after the one before: one into the other, at once
			c.from = cards[n-1].img
			c.start, c.pre = cards[n-1].start+cards[n-1].secs, 0
		}
		c.skip, c.hold = skipAfter(events[i+1:], e.t)
		shift += c.pre + c.secs + c.hold - float64(c.skip)/1000
		cards = append(cards, c)
		if c.last {
			break
		}
	}
	if endAt < 0 {
		endAt = events[len(events)-1].t + 1500
	}
	total := float64(endAt-t0)/1000 + shift
	if n := len(cards); n > 0 && cards[n-1].last { // the end card is the video's last frame: no screen held after it
		total = cards[n-1].start + cards[n-1].secs
	}
	// the read-along's card: the last before the narration's sound; the music comes back with the card after it
	readalongCard := -1
	for _, e := range events {
		if e.kind == "sound" {
			for i, c := range cards {
				if c.at <= e.t {
					readalongCard = i
				}
			}
			break
		}
	}
	if readalongCard >= 0 {
		readalong, readalongSecs = cards[readalongCard].start, cards[readalongCard].secs
	}
	videoAt := func(t int64) float64 { // a time of the recording on the video's clock (outside the cards)
		v := float64(t-t0) / 1000
		for _, c := range cards {
			if c.at <= t {
				v += c.pre + c.secs + c.hold - float64(c.skip)/1000
			}
		}
		return v
	}
	outro := -1.0 // where the music comes back after the narration: the next card, else the next step
	if readalongCard >= 0 && readalongCard+1 < len(cards) && !cards[readalongCard+1].last {
		outro = cards[readalongCard+1].start
	}
	for _, e := range events {
		if e.kind == "title" && readalong >= 0 && videoAt(e.t) > readalong+readalongSecs+1 && outro < 0 {
			outro = videoAt(e.t)
		}
	}
	last := cards[len(cards)-1]
	if outro < 0 {
		outro = last.start
	}
	// the sections: one from each card (one following another straight on stays in its section) to the next, each
	// frames f0 to f1; frame f is at f/fps s. A section's frames are what a whole render draws there.
	frames := int(total * float64(*fps))
	type part struct {
		f0, f1 int
		start  float64
		name   string
	}
	var parts []part
	for i, c := range cards {
		if i > 0 && c.from != nil {
			continue
		}
		start := c.start
		if i == 0 {
			start = 0
		}
		f0 := int(math.Ceil(start*float64(*fps) - 1e-9))
		if n := len(parts); n > 0 {
			parts[n-1].f1 = f0
		}
		parts = append(parts, part{f0: f0, f1: frames, start: start, name: splitBar(eventWords(events, c.at))[0]})
	}
	if *list || *section < 0 {
		check(writeSound(*rec, events, t0, endAt, cards))
		check(os.WriteFile(filepath.Join(*rec, "times"), fmt.Appendf(nil, "readalong %.3f %.3f\nend %.3f %.3f\noutro %.3f\ntotal %.3f\n",
			readalong, readalongSecs, last.start, last.secs, outro, total), 0o644))
	}
	if *list {
		for i, p := range parts {
			fmt.Printf("%d %d %d %s\n", i, p.f0, p.f1, p.name)
		}
		return
	}
	f0, f1 := 0, frames
	if *section >= 0 {
		if *section >= len(parts) {
			check(fmt.Errorf("no section %d of %d", *section, len(parts)))
		}
		f0, f1 = parts[*section].f0, parts[*section].f1
	}

	still, lay := backdrop(side)
	scr := lay.screen
	titles := map[string]*image.RGBA{}
	shown := image.NewRGBA(image.Rect(0, 0, screenW, screenH))
	draw.Draw(shown, shown.Bounds(), image.White, image.Point{}, draw.Src)
	type move struct {
		e        event
		from, to *image.RGBA
	}
	var moving []*move
	var touches []event
	scaled := image.NewRGBA(image.Rect(0, 0, scr.Dx(), scr.Dy()))
	view := image.NewRGBA(shown.Bounds())
	device := image.NewRGBA(image.Rect(0, 0, frameW, frameH))
	frame := image.NewRGBA(image.Rect(0, 0, frameW, frameH))
	blank := paper() // what the first card fades in from
	out := bufio.NewWriterSize(os.Stdout, 1<<20)
	rgb := make([]byte, frameW*frameH*3)
	next, nextTouch := 0, 0
	dirty := true
	title, oldTitle, titleAt := "", "", -10.0 // the step's title, the one before it, and when it changed (video s)
	const fade = 0.5                          // s, every fade: a card in and out, one title into the next
	plain := false                            // the frame before was the device alone, as drawn (no card, no touch): sent again if nothing changed
	for fr := 0; fr < f1; fr++ {
		tv := float64(fr) / float64(*fps)
		draws := fr >= f0 // before the section, the recording is only replayed: the screen as it is, nothing drawn
		if fr == f0 {
			dirty, plain = true, false
		}
		// the card up now (or waiting for its beat), and the recording's time: the video's less the cards before it
		var up, held *card
		shift := 0.0
		for _, c := range cards {
			if tv >= c.start+c.secs+c.hold {
				shift += c.pre + c.secs + c.hold - float64(c.skip)/1000
				continue
			}
			if tv >= c.start-c.pre {
				held = c
				if tv < c.start+c.secs && tv >= c.start {
					up = c
				}
			}
			break
		}
		now := t0 + int64((tv-shift)*1000)
		tnow := now     // the touches' clock: in a card's time it runs up to where the recording goes on, for their lead-in
		titleNow := now // a step's title is shown from the card on, though logged as the screen is held after it
		if c := held; c != nil {
			now, titleNow = c.at, c.at  // frozen while the card waits for its beat, the title too: the next step's never before it
			if tv >= c.start+c.secs/2 { // the card covers the screen: what it left out is done behind it
				now = c.at + c.skip
				titleNow = now + int64(c.hold*1000) + 1
			}
			tnow = c.at + c.skip - int64((c.start+c.secs+c.hold-tv)*1000)
		}
		for next < len(events) && (events[next].t <= now || events[next].kind == "title" && events[next].t <= titleNow) {
			e := events[next]
			next++
			switch e.kind {
			case "title":
				oldTitle, title, titleAt, dirty = title, e.words, tv, true
			case "touch", "button":
				// drawn from their lead-in on (below)
			case "refresh":
				to := loadPPM(filepath.Join(*rec, e.file))
				// a region still moving under this one ends at once: the driver merges them
				kept := moving[:0]
				for _, m := range moving {
					if m.e.rect.Overlaps(e.rect) {
						draw.Draw(shown, m.e.rect, m.to, image.Point{}, draw.Src)
						continue
					}
					kept = append(kept, m)
				}
				moving = kept
				from := image.NewRGBA(image.Rect(0, 0, e.rect.Dx(), e.rect.Dy()))
				draw.Draw(from, from.Bounds(), shown, e.rect.Min, draw.Src)
				moving = append(moving, &move{e, from, to})
				dirty = true
			}
		}
		composed := false
		if dirty && !draws { // replayed: the regions' moves ended as they would have, nothing drawn
			kept := moving[:0]
			for _, m := range moving {
				d := duration[m.e.wave]
				if d == 0 {
					d = 300
				}
				if float64(now-m.e.t) >= d {
					draw.Draw(shown, m.e.rect, m.to, image.Point{}, draw.Src)
					continue
				}
				kept = append(kept, m)
			}
			moving = kept
		} else if dirty {
			composed = true
			copy(view.Pix, shown.Pix)
			kept := moving[:0]
			for _, m := range moving {
				d := duration[m.e.wave]
				if d == 0 {
					d = 300
				}
				p := float64(now-m.e.t) / d
				if p >= 1 {
					draw.Draw(shown, m.e.rect, m.to, image.Point{}, draw.Src)
					draw.Draw(view, m.e.rect, m.to, image.Point{}, draw.Src)
					continue
				}
				paint(view, m.e.rect, m.from, m.to, p, m.e.flash)
				kept = append(kept, m)
			}
			moving = kept
			draw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), view, view.Bounds(), draw.Src, nil)
			copy(device.Pix, still.Pix)
			draw.Draw(device, scr, scaled, image.Point{}, draw.Src)
			k := ease(math.Min((tv-titleAt)/fade, 1)) // one title into the next
			for _, w := range []struct {
				s string
				a float64
			}{{oldTitle, 1 - k}, {title, k}} {
				if w.s == "" || w.a <= 0 {
					continue
				}
				if titles[w.s] == nil {
					titles[w.s] = sideTitle(f, w.s)
				}
				t := titles[w.s]
				draw.DrawMask(device, t.Bounds().Add(image.Pt(sideX-t.Bounds().Dx()/2, stepY)), t, image.Point{},
					image.NewUniform(color.Alpha{uint8(255 * w.a)}), image.Point{}, draw.Over)
			}
			dirty = len(moving) > 0 || k < 1
		}
		for nextTouch < len(events) && events[nextTouch].t <= tnow+touchLead {
			e := events[nextTouch]
			early := false // after a card that has not come yet: its lead-in waits for the card (never before it)
			for _, c := range cards {
				early = early || e.t >= c.at && tv < c.start
			}
			if early {
				break
			}
			hidden := false // done behind a card (what it leaves out): never drawn
			for _, c := range cards {
				hidden = hidden || e.t >= c.at && e.t < c.at+c.skip
			}
			if (e.kind == "touch" || e.kind == "button") && !hidden {
				touches = append(touches, e)
			}
			nextTouch++
		}
		if !draws {
			kept := touches[:0]
			for _, t := range touches {
				if tnow-t.t < 3000 {
					kept = append(kept, t)
				}
			}
			touches = kept
			continue
		}
		if !composed && plain && up == nil && len(touches) == 0 && tv >= cards[0].start {
			if _, err := out.Write(rgb); err != nil { // the same as the frame before
				check(err)
			}
			continue
		}
		plain = up == nil && len(touches) == 0 && tv >= cards[0].start
		copy(frame.Pix, device.Pix)
		if tv < cards[0].start { // before the first card: the paper alone, not an empty device
			copy(frame.Pix, blank.Pix)
		}
		kept := touches[:0]
		for _, t := range touches {
			if drawTouch(frame, lay, t, tnow) {
				kept = append(kept, t)
			}
		}
		touches = kept
		if up != nil {
			k := 1.0 // the card's share of the frame
			in, left := tv-up.start, up.start+up.secs-tv
			under := frame // what it fades out to: the screen; and in from: the card before it, or the paper at first
			switch {
			case in >= fade:
			case up.from != nil:
				under = up.from
			case up == cards[0]:
				under = blank
			}
			if in < fade {
				k = in / fade
			}
			nextFollows := false
			for i, c := range cards {
				if c == up && i+1 < len(cards) && cards[i+1].from != nil {
					nextFollows = true
				}
			}
			if left < fade && !up.last && !nextFollows {
				k = left / fade
			}
			k8 := int(ease(k)*256 + 0.5)
			img := up.img
			par(len(frame.Pix), func(a, b int) {
				for i := a; i < b; i++ {
					frame.Pix[i] = uint8((int(under.Pix[i])*(256-k8) + int(img.Pix[i])*k8) >> 8)
				}
			})
			for i, it := range up.items { // the list, a beat apart from the second beat
				a := math.Min(math.Max((in-beat*float64(2+i))/0.35, 0), 1)
				if a > 0 {
					over(frame, it, a)
				}
			}
		}
		par(frameW*frameH, func(a, b int) {
			for p := a; p < b; p++ {
				rgb[p*3], rgb[p*3+1], rgb[p*3+2] = frame.Pix[p*4], frame.Pix[p*4+1], frame.Pix[p*4+2]
			}
		})
		if _, err := out.Write(rgb); err != nil {
			check(err)
		}
	}
	check(out.Flush())
	fmt.Fprintf(os.Stderr, "simvideo: frames %d to %d of %d, %.1f s, %d events, %d cards\n", f0, f1, frames, total, len(events), len(cards))
}

// splitBar splits "A | B | C" into its parts, trimmed.
func splitBar(s string) []string {
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// touchLead is how long before a touch lands its dot is drawn coming in, so the eye is there when the screen answers.
const touchLead = 350

// drawTouch draws a touch or a page button's press on the frame as it is at the recording's time now, in the mark's
// saffron. A tap: its dot coming in (touchLead), then a ring that widens and fades (0.9 s). A hold: the dot, its ring
// closing over the 1.5 s held, then the dot following the finger to where it moved (0.9 s) and fading (0.5 s). A
// button: lit, then let go (0.8 s). It says whether the touch is still to be drawn.
func drawTouch(img *image.RGBA, lay layout, t event, now int64) bool {
	scr := lay.screen
	s := float64(scr.Dx()) / screenW
	at := func(x, y int) (float64, float64) {
		return float64(scr.Min.X) + float64(x)*s, float64(scr.Min.Y) + float64(y)*s
	}
	dt := float64(now-t.t) / 1000
	lead := float64(touchLead) / 1000
	if t.kind == "button" {
		if dt > 0.8 || t.touch == "power" { // the power button is not drawn: the device's top edge, out of the frame
			return false
		}
		a := 0.85
		switch {
		case dt < 0:
			a *= ease(1 + dt/lead)
		case dt > 0.3:
			a *= 1 - ease((dt-0.3)/0.5)
		}
		b := lay.lower
		if t.touch == "upper" {
			b = lay.upper
		}
		roundRect(img, b[0], b[1], b[2], b[3], b[2]*0.4, func(fx, fy float64) color.RGBA { return saffron }, a)
		return true
	}
	x, y := at(t.xy[0], t.xy[1])
	if dt < 0 { // coming in: from wider and faint to the touch
		p := ease(1 + dt/lead)
		disc(img, x, y, 22+14*(1-p), saffron, 0.7*p)
		return true
	}
	switch t.touch {
	case "tap":
		if dt > 0.9 {
			return false
		}
		p := dt / 0.9
		disc(img, x, y, 22, saffron, 0.7*(1-ease(p)))
		ring(img, x, y, 22+30*ease(p), 4, saffron, 0.9*(1-p))
		return true
	case "hold":
		if dt > 2.9 {
			return false
		}
		a := 0.7
		switch {
		case dt < 1.5:
			ring(img, x, y, 46-20*dt/1.5, 4, saffron, 0.9)
		case dt < 2.4:
			x2, y2 := at(t.xy[2], t.xy[3])
			p := ease((dt - 1.5) / 0.9)
			x, y = x+(x2-x)*p, y+(y2-y)*p
		default:
			x, y = at(t.xy[2], t.xy[3])
			a *= 1 - ease((dt-2.4)/0.5)
		}
		disc(img, x, y, 22, saffron, a)
		return true
	}
	return false
}

// ease is a smooth step from 0 to 1, for every fade and move.
func ease(k float64) float64 {
	k = math.Min(math.Max(k, 0), 1)
	return k * k * (3 - 2*k)
}

// afterCard is how long the screen shows, once a card has gone, before the next thing done: the card's fade out, then
// the touch's lead-in, never at once.
const afterCard = 1300

// skipAfter is how much of the recording after a card at t is left out (events: the log after it): to the resume that
// follows it before another card (the reader's start and splash, after the tour's first act's card), else to afterCard
// before the next thing done (a touch, a button, a refresh), so the screen never waits idle after a card; when the
// next thing is sooner, the screen is held still after the card instead (hold). A card straight before another leaves
// nothing out.
func skipAfter(events []event, t int64) (skip int64, hold float64) {
	for _, e := range events {
		if e.kind == "caption" {
			if e.t-t < 400 {
				return 0, 0
			}
			break
		}
		if e.kind == "resume" { // and a little after it: the refreshes just before it done behind the card
			return e.t - t + 400, 0
		}
	}
	for _, e := range events {
		if e.t > t && (e.kind == "touch" || e.kind == "button" || e.kind == "refresh") {
			d := e.t - t - afterCard
			return max(d, 0), float64(max(-d, 0)) / 1000 // too soon: the screen held still until it is afterCard away
		}
	}
	return 0, 0
}

// disc and ring draw a filled circle and a ring of width w, at alpha a, their edges smoothed.
func disc(img *image.RGBA, cx, cy, r float64, c color.RGBA, a float64) {
	shape(img, cx, cy, r+1, c, a, func(d float64) float64 { return math.Min(math.Max(r-d+0.5, 0), 1) })
}

func ring(img *image.RGBA, cx, cy, r, w float64, c color.RGBA, a float64) {
	shape(img, cx, cy, r+w, c, a, func(d float64) float64 { return math.Min(math.Max(w/2-math.Abs(d-r)+0.5, 0), 1) })
}

func shape(img *image.RGBA, cx, cy, reach float64, c color.RGBA, a float64, cover func(d float64) float64) {
	b := img.Bounds()
	for y := max(int(cy-reach), b.Min.Y); y <= min(int(cy+reach)+1, b.Max.Y-1); y++ {
		for x := max(int(cx-reach), b.Min.X); x <= min(int(cx+reach)+1, b.Max.X-1); x++ {
			k := a * cover(math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy))
			if k <= 0 {
				continue
			}
			o := img.PixOffset(x, y)
			for i, v := range []uint8{c.R, c.G, c.B} {
				img.Pix[o+i] = uint8(float64(img.Pix[o+i])*(1-k) + float64(v)*k + 0.5)
			}
		}
	}
}

// over draws the layer it (the frame's size, transparent where empty) over img at alpha a.
func over(img, it *image.RGBA, a float64) {
	a8 := int(a*256 + 0.5)
	par(len(img.Pix)/4, func(from, to int) {
		for p := from * 4; p < to*4; p += 4 {
			if al := int(it.Pix[p+3]); al != 0 { // the layer's colours are premultiplied
				w := a8 * al / 255
				for c := 0; c < 3; c++ {
					img.Pix[p+c] = uint8((int(img.Pix[p+c])*(256-w) + int(it.Pix[p+c])*a8) >> 8)
				}
			}
		}
	})
}

// par runs fn over 0..n in a part for each core.
func par(n int, fn func(a, b int)) {
	k := runtime.NumCPU()
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		a, b := n*i/k, n*(i+1)/k
		wg.Add(1)
		go func() { defer wg.Done(); fn(a, b) }()
	}
	wg.Wait()
}

// eventWords is the words of the caption at t.
func eventWords(events []event, t int64) string {
	for _, e := range events {
		if e.kind == "caption" && e.t == t {
			return e.words
		}
	}
	return ""
}

// beats finds the music's beat in a take (raw mono S16LE at 22050 Hz): the onsets (the rise of its loudness, 512
// samples a step), their period between 100 and 140 beats a minute (the strongest self-likeness), and their phase
// (the offset where the onsets a period apart add up most).
func beats(path string) (period, phase float64) {
	b, err := os.ReadFile(path)
	check(err)
	const sr, hop = 22050, 512
	n := len(b) / 2 / hop
	energy := make([]float64, n)
	for i := range n {
		var e float64
		for j := 0; j < hop; j++ {
			v := float64(int16(binary.LittleEndian.Uint16(b[(i*hop+j)*2:])))
			e += v * v
		}
		energy[i] = math.Log1p(e / hop)
	}
	onset := make([]float64, n)
	for i := 1; i < n; i++ {
		onset[i] = math.Max(energy[i]-energy[i-1], 0)
	}
	step := float64(hop) / sr
	best, bestLag := -1.0, 0
	for lag := int(60 / 140.0 / step); lag <= int(60/100.0/step)+1; lag++ {
		var s float64
		for i := lag; i < n; i++ {
			s += onset[i] * onset[i-lag]
		}
		s /= float64(n - lag)
		if s > best {
			best, bestLag = s, lag
		}
	}
	// the period finer than a step: the one whose multiples line up best over the whole take
	bestP, bestS := float64(bestLag)*step, -1.0
	for p := (float64(bestLag) - 1) * step; p <= (float64(bestLag)+1)*step; p += step / 20 {
		for off := 0.0; off < p; off += step {
			var s float64
			for t := off; t < float64(n)*step; t += p {
				s += onset[int(t/step)]
			}
			if s > bestS {
				bestS, bestP, phase = s, p, off
			}
		}
	}
	return bestP, phase
}

// paint draws region r of view at share p (0 to 1) of its move from what it showed to what was sent: a fade, or with
// a flash black, then white, then the new picture.
func paint(view *image.RGBA, r image.Rectangle, from, to *image.RGBA, p float64, flash bool) {
	mix := func(a, b uint8, k float64) uint8 { return uint8(float64(a)*(1-k) + float64(b)*k + 0.5) }
	for y := 0; y < r.Dy(); y++ {
		vo := (r.Min.Y+y)*view.Stride + r.Min.X*4
		so := y * from.Stride
		for x := 0; x < r.Dx()*4; x += 4 {
			for c := 0; c < 3; c++ {
				a, b := from.Pix[so+x+c], to.Pix[so+x+c]
				var v uint8
				switch {
				case !flash:
					v = mix(a, b, p)
				case p < 0.3:
					v = mix(a, 0, p/0.3)
				case p < 0.55:
					v = mix(0, 255, (p-0.3)/0.25)
				default:
					v = mix(255, b, (p-0.55)/0.45)
				}
				view.Pix[vo+x+c] = v
			}
		}
	}
}

// backdrop is the still part of a frame of the tour: the paper, the e-reader's body with its page buttons at the right
// (a book read right to left: the eye starts there), the stacked logo at the left (BRAND.md: a title surface, as the
// read-along video's one screen had it; the credit once); and where the screen goes in it.
func backdrop(logo image.Image) (*image.RGBA, layout) {
	img := paper()
	// a generic e-reader after the Libra's proportions (the screen 73.7% of the body's width, the grip at the right
	// with two page buttons); no maker's name or logo: the read-along video's body
	bodyH := 1000.0
	bodyW := bodyH / 1.1176
	bx, by := frameW-150-bodyW, (frameH-bodyH)/2
	roundRect(img, bx, by, bodyW, bodyH, bodyW*0.05, func(fx, fy float64) color.RGBA {
		k := (fx*0.35 + fy*0.65) // a light from the top left
		lerp := func(a, b float64) uint8 { return uint8(a + (b-a)*k) }
		return color.RGBA{lerp(0x3e, 0x22), lerp(0x41, 0x24), lerp(0x47, 0x28), 0xff}
	}, 1)
	sw := int(math.Round(bodyW * 0.737))
	sh := sw * screenH / screenW
	sx := int(math.Round(bx + bodyW*0.076))
	sy := int(math.Round(by + (bodyH-float64(sh))/2))
	draw.Draw(img, image.Rect(sx-2, sy-2, sx+sw+2, sy+sh+2), &image.Uniform{color.RGBA{0x0e, 0x0f, 0x11, 0xff}}, image.Point{}, draw.Src)
	btnW, btnH := bodyW*0.052, bodyH*0.115
	var btn [2][4]float64 // the upper, the lower: x, y, w, h
	for i, top := range []float64{0.5 - 0.135, 0.5 + 0.02} {
		btn[i] = [4]float64{bx + bodyW - bodyW*0.072 - btnW, by + top*bodyH, btnW, btnH}
		roundRect(img, btn[i][0], btn[i][1], btnW, btnH, btnW*0.4, func(fx, fy float64) color.RGBA {
			return color.RGBA{0x4c, 0x4f, 0x56, 0xff}
		}, 1)
	}
	multiply(img, logo, sideX-logo.Bounds().Dx()/2, stackY)
	return img, layout{image.Rect(sx, sy, sx+sw, sy+sh), btn[0], btn[1]}
}

// layout is where the device's parts are in the frame: the screen, and the upper and lower page buttons (x, y, w, h).
type layout struct {
	screen       image.Rectangle
	upper, lower [4]float64
}

func paper() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, frameW, frameH))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	return img
}

// sideTitle is a step's name as it stands under the logo beside the device: a label in a pill outlined in the ink's
// grey, on the paper of a box 740 wide (opaque: one title fades into the next over the same paper).
func sideTitle(f fonts, title string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 740, 120))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	face := newFace(f.regular, 40)
	w := float64((&font.Drawer{Face: face}).MeasureString(title).Round()) + 2*40
	h := 80.0
	x, y := 370-w/2, 60-h/2
	roundRect(img, x, y, w, h, h/2, func(_, _ float64) color.RGBA { return muted }, 0.55)
	roundRect(img, x+2.5, y+2.5, w-5, h-5, h/2-2.5, func(_, _ float64) color.RGBA { return color.RGBA{0xfc, 0xfc, 0xfb, 0xff} }, 1)
	text(img, face, title, 370, 74, ink)
	return img
}

// openingCard is the first screen: the stacked logo (a large title surface), the lines under it.
func openingCard(f fonts, stack image.Image, lines []string) *image.RGBA {
	img := paper()
	h := stack.Bounds().Dy() + 60 + 64*len(lines)
	y := (frameH - h) / 2
	multiply(img, stack, frameW/2-stack.Bounds().Dx()/2, y)
	y += stack.Bounds().Dy() + 100
	face := newFace(f.regular, 46)
	for _, l := range lines {
		text(img, face, l, frameW/2, y, muted)
		y += 64
	}
	return img
}

// actCard is an act's card: its title, the lines under it, all centred.
func actCard(f fonts, title string, lines []string) *image.RGBA {
	img := paper()
	h := 120 + 70*len(lines)
	y := (frameH-h)/2 + 60
	text(img, newFace(f.bold, 104), title, frameW/2, y, ink)
	y += 100
	face := newFace(f.regular, 46)
	for _, l := range lines {
		text(img, face, l, frameW/2, y, muted)
		y += 70
	}
	return img
}

// endCard is the last screen: the stacked logo on top (a large title surface), the note at the foot; what ShamsaInk
// does, a point a line, centred (a line "and more…" muted); and last a call to try it, "WORDS — LINK", the link large.
// Each line is its own layer, to come a beat after the one before.
func endCard(f fonts, stack image.Image, lines []string) (*image.RGBA, []*image.RGBA) {
	img := paper()
	multiply(img, stack, frameW/2-stack.Bounds().Dx()/2, 50)
	text(img, newFace(f.regular, 24), "Simulated on a Kobo Libra Colour: the reader's own screens, timings and sound; the ink's movement approximated",
		frameW/2, frameH-36, muted)
	face := newFace(f.regular, 38)
	y := 420
	var items []*image.RGBA
	for _, l := range lines {
		it := image.NewRGBA(img.Bounds())
		if words, link, ok := strings.Cut(l, "—"); ok {
			y += 60
			text(it, newFace(f.regular, 36), strings.TrimSpace(words), frameW/2, y, muted)
			text(it, newFace(f.bold, 58), strings.TrimSpace(link), frameW/2, y+76, ink)
		} else {
			c := ink
			if strings.HasPrefix(l, "and more") {
				c = muted
			}
			text(it, face, l, frameW/2, y, c)
			y += 62
		}
		items = append(items, it)
	}
	return img, items
}

// multiply draws a picture at its own size at x, y, its white taken as the paper (multiplied).
func multiply(img *image.RGBA, pic image.Image, x, y int) {
	b := pic.Bounds()
	for py := 0; py < b.Dy(); py++ {
		for px := 0; px < b.Dx(); px++ {
			r, g, bl, _ := pic.At(b.Min.X+px, b.Min.Y+py).RGBA()
			d := img.RGBAAt(x+px, y+py)
			img.SetRGBA(x+px, y+py, color.RGBA{uint8(int(r>>8) * int(d.R) / 255), uint8(int(g>>8) * int(d.G) / 255),
				uint8(int(bl>>8) * int(d.B) / 255), 0xff})
		}
	}
}

func text(img *image.RGBA, face font.Face, s string, cx, baseline int, c color.RGBA) {
	d := &font.Drawer{Dst: img, Src: &image.Uniform{c}, Face: face}
	w := d.MeasureString(s)
	d.Dot = fixed.Point26_6{X: fixed.I(cx) - w/2, Y: fixed.I(baseline)}
	d.DrawString(s)
}

// roundRect fills a rectangle with corners of radius rad at alpha a, its colour by where in it a pixel is (0 to 1 each
// way), its edge smoothed.
func roundRect(img *image.RGBA, x, y, w, h, rad float64, fill func(fx, fy float64) color.RGBA, a float64) {
	for py := int(y); py < int(math.Ceil(y+h)); py++ {
		for px := int(x); px < int(math.Ceil(x+w)); px++ {
			fx, fy := float64(px)+0.5, float64(py)+0.5
			qx := math.Max(math.Max(x+rad-fx, fx-(x+w-rad)), 0)
			qy := math.Max(math.Max(y+rad-fy, fy-(y+h-rad)), 0)
			d := math.Hypot(qx, qy) - rad // < 0 inside
			cover := a * math.Min(math.Max(0.5-d, 0), 1)
			if fx < x || fx > x+w || fy < y || fy > y+h {
				cover = 0
			}
			if cover == 0 {
				continue
			}
			c := fill((fx-x)/w, (fy-y)/h)
			b := img.RGBAAt(px, py)
			m := func(a, b uint8) uint8 { return uint8(float64(b)*(1-cover) + float64(a)*cover) }
			img.SetRGBA(px, py, color.RGBA{m(c.R, b.R), m(c.G, b.G), m(c.B, b.B), 0xff})
		}
	}
}

func readLog(path string) ([]event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		a := strings.Fields(sc.Text())
		if len(a) < 2 {
			continue
		}
		t, err := strconv.ParseInt(a[0], 10, 64)
		if err != nil {
			continue
		}
		e := event{t: t, kind: a[1]}
		switch a[1] {
		case "refresh": // refresh X Y W H WAVEFORM FLASH FILE
			if len(a) != 9 {
				continue
			}
			n := make([]int, 4)
			for i := range n {
				n[i], _ = strconv.Atoi(a[2+i])
			}
			e.rect = image.Rect(n[0], n[1], n[0]+n[2], n[1]+n[3])
			e.wave, e.flash, e.file = a[6], a[7] == "true", a[8]
		case "caption", "title":
			e.words = strings.Join(a[2:], " ")
		case "touch": // touch tap X Y, touch hold X Y X2 Y2
			if len(a) < 5 {
				continue
			}
			e.touch = a[2]
			for i := 0; i < 4 && 3+i < len(a); i++ {
				e.xy[i], _ = strconv.Atoi(a[3+i])
			}
		case "button": // button upper|lower
			if len(a) != 3 {
				continue
			}
			e.touch = a[2]
		case "resume":
		case "sound": // sound s16le RATE CHANNELS FILE
			if len(a) != 6 {
				continue
			}
			e.soundFile = a[5]
		default:
			continue
		}
		out = append(out, e)
	}
	// devtest and the reader append to the log apart: in time order
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].t < out[j-1].t; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, sc.Err()
}

// writeSound lays each sound the reader played at its time on the recording's clock, silence between, and then
// silence in at each card (its wait for the beat and its time), as the video stops the clock there: DIR/sound.wav.
func writeSound(dir string, events []event, t0, end int64, cards []*card) error {
	at := func(t int64) int { return int((t-t0)*rate/1000) * 4 }
	pcm := make([]byte, at(end))
	for _, e := range events {
		if e.kind != "sound" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.soundFile))
		if err != nil {
			return err
		}
		if a := at(e.t); a < len(pcm) {
			copy(pcm[a:], b)
		}
	}
	var out []byte
	from := 0
	for _, c := range cards {
		cut := min(at(c.at), len(pcm))
		out = append(out, pcm[from:cut]...)
		out = append(out, make([]byte, int((c.pre+c.secs+c.hold)*rate)*4)...)
		from = min(at(c.at+c.skip), len(pcm))
	}
	out = append(out, pcm[from:]...)
	f, err := os.Create(filepath.Join(dir, "sound.wav"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := func(v any) { binary.Write(f, binary.LittleEndian, v) }
	f.WriteString("RIFF")
	w(uint32(36 + len(out)))
	f.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(2))
	w(uint32(rate))
	w(uint32(rate * 4))
	w(uint16(4))
	w(uint16(16))
	f.WriteString("data")
	w(uint32(len(out)))
	_, err = f.Write(out)
	return err
}

func loadPPM(path string) *image.RGBA {
	f, err := os.Open(path)
	check(err)
	defer f.Close()
	r := bufio.NewReader(f)
	var w, h, max int
	_, err = fmt.Fscanf(r, "P6\n%d %d\n%d\n", &w, &h, &max)
	check(err)
	buf := make([]byte, w*h*3)
	_, err = io.ReadFull(r, buf)
	check(err)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, j := 0, 0; j < len(buf); i, j = i+4, j+3 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = buf[j], buf[j+1], buf[j+2], 0xff
	}
	return img
}

func loadPNG(path string) image.Image {
	f, err := os.Open(path)
	check(err)
	defer f.Close()
	img, err := png.Decode(f)
	check(err)
	return img
}

func loadFont(path string) *opentype.Font {
	b, err := os.ReadFile(path)
	check(err)
	f, err := opentype.Parse(b)
	check(err)
	return f
}

func newFace(f *opentype.Font, size float64) font.Face {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	check(err)
	return face
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "simvideo:", err)
		os.Exit(1)
	}
}

// readSecs is how long a card's words take to read: a second to see it, then 0.3 s a word.
func readSecs(parts []string) float64 {
	n := 0
	for _, p := range parts {
		n += len(strings.Fields(p))
	}
	return 1 + 0.3*float64(n)
}
