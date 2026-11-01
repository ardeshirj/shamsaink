// shamsaink: the minimal reader (experiment 11; docs/roadmap.md, stage 2). Opens an EPUB at its place, turns pages by
// tap or swipe (in the book's direction: a right-to-left book turns forward on the left), keeps its place and the
// book's cache in DIR/places and DIR/cache, and lays the rest of the book out between touches. A tap on the top tenth
// of the screen opens the menu: the settings (DIR/settings.json) and a way back. Run by KOReader's plugin
// (shamsaink.koplugin), which lets go of the touch screen first and redraws after.
//
//	shamsaink BOOK.epub [FONTS_DIR|-]       one book, then exit
//	shamsaink -library [FONTS_DIR|-]       the library (DIR/books): a book chosen is read, and leaving it comes back;
//	                                        exits when the library is left (the Kobo's launcher)
//	shamsaink -serve PIPES [FONTS_DIR|-]    kept running: a book for each message written to PIPES/in, "left" written
//	                                        to PIPES/out when it is left (the plugin's way: the program starts once)
//
// A message is lines of "key value": "book PATH". DIR: $SHAMSAINK_DIR, or the folder the program is in.
package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	blitz "shamsainkengine"
	"shamsainkreader/device"
	"shamsainkreader/ui"
)

var (
	// the settings' defaults, until set in the menu
	fontSize   = env("FONT_SIZE", "50") // px (09: 50 looks right at 300 ppi)
	lineHeight = env("LINE_HEIGHT", "0")
	margin     = env("MARGIN", "60")
	leadMs     = env("LEAD_MS", "250") // experiment 12: by ear, for the maintainer's headphones
)

// What stays between books: the screen, the fallback fonts, the three pages' buffers (the engine's pixels only) and
// one for the screen put together, a page and what is over it (layers.go).
type reader struct {
	dev   *device.Device
	fonts *blitz.Fonts
	dir   string
	pages drawnPages
	over  *image.Gray
	quit  bool // Leave chosen in a book's ≡: out of the library too
	lib   bool // books are opened from the library: a slow one shows the loading screen, not the opening one

	splashUntil time.Time // the opening screen shown at the start stays at least until then (ui.SplashFor)
}

func main() {
	serve := len(os.Args) > 2 && os.Args[1] == "-serve"
	if len(os.Args) < 2 || os.Args[1] == "-serve" && !serve {
		fmt.Fprintln(os.Stderr, "shamsaink BOOK.epub [FONTS_DIR|-]\nshamsaink -library [FONTS_DIR|-]\nshamsaink -serve PIPES [FONTS_DIR|-]")
		os.Exit(2)
	}
	t := time.Now()
	started := sinceStart()
	exe, _ := os.Executable()
	rd := &reader{dir: env("SHAMSAINK_DIR", filepath.Dir(exe))}
	var err error
	rd.dev, err = device.Open()
	check(err)
	set := loadSettings(rd.dir)
	rd.dev.Screen.SetInvert(set.Dark)
	rd.dev.SetSleepAfter(time.Duration(set.SleepMin) * time.Minute)
	if set.Light != nil && set.Warmth != nil { // the reader's own light (light.go)
		rd.dev.SetLight(*set.Light)
		rd.dev.SetWarmth(*set.Warmth)
	}
	fontsArg := 2
	if serve {
		fontsArg = 3
	}
	if len(os.Args) > fontsArg && os.Args[fontsArg] != "-" {
		rd.fonts = blitz.LoadFonts(os.Args[fontsArg])
	}
	rd.pages = make(drawnPages, 3)
	for i := range rd.pages {
		rd.pages[i] = &drawn{img: image.NewGray(image.Rect(0, 0, rd.dev.Screen.W, rd.dev.Screen.H))}
	}
	rd.over = image.NewGray(image.Rect(0, 0, rd.dev.Screen.W, rd.dev.Screen.H))
	// started: from the process's start to main (the program read, Go's runtime and every package's init), to 10 ms
	fmt.Printf("started: main after %s, fonts and screen %.0f ms more\n", started, ms(t))
	if !serve {
		// RESTORE_SCREEN: the screen found put back on leaving, for a host paused meanwhile that does not redraw once
		// resumed (Kobo's interface, from reader/kobo/launch.sh)
		var found device.Saved
		if os.Getenv("RESTORE_SCREEN") != "" {
			found = rd.dev.Screen.Save()
		}
		var err error
		if os.Args[1] == "-library" {
			err = rd.browse(t)
		} else {
			_, err = rd.read(map[string]string{"book": os.Args[1]}, t)
		}
		if found != nil {
			rd.dev.Screen.Restore(found)
		}
		check(err)
		return
	}
	pipes := os.Args[2]
	check(os.WriteFile(filepath.Join(pipes, "pid"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644))
	for {
		// blocks until the plugin writes a message, read to its end when the plugin closes the pipe
		in, err := os.ReadFile(filepath.Join(pipes, "in"))
		check(err)
		m := map[string]string{}
		for _, l := range strings.Split(string(in), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(l), " "); ok {
				m[k] = v
			}
		}
		if m["book"] == "" {
			continue
		}
		t := time.Now()
		// held open while the book is read: if we die, the plugin reads the end of the pipe and goes on
		out, err := os.OpenFile(filepath.Join(pipes, "out"), os.O_WRONLY, 0)
		check(err)
		asleep, err := rd.read(m, t)
		if err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink:", err)
		}
		// "asleep": left as the Kindle went to sleep, to be opened again when it wakes
		fmt.Fprintln(out, map[bool]string{false: "left", true: "asleep"}[asleep])
		out.Close()
	}
}

// A page drawn, its place, and when it was last shown or drawn.
type drawn struct {
	img     *image.Gray
	doc, at int
	last    int
}

type drawnPages []*drawn

var clock int

func (ps drawnPages) find(doc, at int) *drawn {
	for _, p := range ps {
		if p.doc == doc && p.at == at {
			return p
		}
	}
	return nil
}

func (ps drawnPages) touch(p *drawn) { clock++; p.last = clock }

// get is the page at (doc, at), number n now: drawn already, or drawn into the page used longest ago.
func (ps drawnPages) get(r *reading, doc, at, n int) *drawn {
	if p := ps.find(doc, at); p != nil {
		return p
	}
	p := ps[0]
	for _, q := range ps[1:] {
		if q.last < p.last {
			p = q
		}
	}
	r.book.Draw(n, p.img)
	p.doc, p.at = doc, at
	ps.touch(p)
	return p
}

// apply turns a page for a gesture, or says to open the menu (a tap on the top tenth of the screen).
func apply(r *reading, g device.Gesture, w, h int) (turned, menu bool) {
	switch {
	case !g.Swipe && g.Y < h/10:
		return false, true
	case g.Swipe && abs(g.DX) > abs(g.DY):
		// a swipe towards the book's start edge turns forward, as a paper page
		if (g.DX < 0) != r.rtl {
			return r.next(), false
		}
		return r.prev(), false
	case !g.Swipe:
		// back on the quarter by the book's start edge, forward on the rest
		back := g.X < w/4
		if r.rtl {
			back = g.X >= w-w/4
		}
		if back {
			return r.prev(), false
		}
		return r.next(), false
	}
	return false, false
}

// sinceStart is the time since the process started, from /proc (to the kernel's clock tick, 10 ms).
func sinceStart() string {
	stat, err1 := os.ReadFile("/proc/self/stat")
	up, err2 := os.ReadFile("/proc/uptime")
	if err1 != nil || err2 != nil {
		return "?"
	}
	// the fields after the command's name, which is in parentheses and may hold spaces; starttime is field 22
	f := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	ticks, err1 := strconv.ParseFloat(f[19], 64)
	now, err2 := strconv.ParseFloat(strings.Fields(string(up))[0], 64)
	if err1 != nil || err2 != nil {
		return "?"
	}
	return fmt.Sprintf("%.0f ms", (now-ticks/100)*1000)
}

func abs(v int) int { return max(v, -v) }

// uiEntries are the table of contents as the contents view shows it.
func uiEntries(es []entry) []ui.Entry {
	out := make([]ui.Entry, len(es))
	for i, e := range es {
		out[i] = ui.Entry{Title: e.title, Depth: e.depth}
	}
	return out
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	check(err)
	return n
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink:", err)
		os.Exit(1)
	}
}
