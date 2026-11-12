// shamsaink: the minimal reader (experiment 11; docs/PLAN.md, stage 2). Opens an EPUB at its place, turns pages by
// tap or swipe (in the book's direction: a right-to-left book turns forward on the left), keeps its place and the
// book's cache in DIR/places and DIR/cache, and lays the rest of the book out between touches. A tap on the top tenth
// of the screen opens the menu: the settings (DIR/settings.json) and a way back. Started by each device's launcher
// (reader/kobo/launch.sh, reader/kindle/launch.sh), with the device's own interface stopped or paused meanwhile.
//
//	shamsaink BOOK.epub [FONTS_DIR|-]       one book, then exit
//	shamsaink -library [FONTS_DIR|-]       the library (DIR/books): a book chosen is read, and leaving it comes back;
//	                                        exits when the library is left (the launchers: reader/kobo, reader/kindle)
//
// DIR: $SHAMSAINK_DIR, or the folder the program is in. DIR/dev there, or $SHAMSAINK_DEV set: dev mode, a flask after the
// library's title and a Developer entry in its menu (devssh.go); SHAMSAINK_DEV=0 keeps it off, dev/ or not.
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
	markLog    = env("MARK_LOG", "") != "" // a log line for each word marked (the end-to-end tests read it)
	leadMs     = env("LEAD_MS", "250")     // experiment 12: by ear, for the maintainer's headphones
)

// version is set by the build (reader/scripts/build-armv7.sh): the release's tag (v0.0.1), or git's description of the
// commit built on the Mac (v0.0.1-3-gabc1234-dirty). `shamsaink -version` prints it; the log's start names it.
var version = "dev"

// What stays between books: the screen, the fallback fonts, the three pages' buffers (the engine's pixels only) and
// one for the screen put together, a page and what is over it (layers.go).
type reader struct {
	dev   *device.Device
	fonts *blitz.Fonts
	dir   string
	pages drawnPages
	over  *image.Gray
	lib   bool // books are opened from the library: a slow one shows the loading screen, not the opening one

	splashUntil time.Time // the opening screen shown at the start stays at least until then (ui.SplashFor)
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Println(version)
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "shamsaink BOOK.epub [FONTS_DIR|-]\nshamsaink -library [FONTS_DIR|-]")
		os.Exit(2)
	}
	t := time.Now()
	started := sinceStart()
	fmt.Printf("shamsaink %s\n", version)
	ui.Build = version
	exe, _ := os.Executable()
	rd := &reader{dir: env("SHAMSAINK_DIR", filepath.Dir(exe))}
	switch os.Getenv("SHAMSAINK_DEV") {
	case "": // dev mode while dev/ is there
		ui.DevMode = isDir(filepath.Join(rd.dir, "dev"))
	case "0": // off all the same: devtest's runs, which hold the screens to the readers' kept renders
	default:
		ui.DevMode = true
	}
	var err error
	rd.dev, err = device.Open()
	check(err)
	set := loadSettings(rd.dir)
	rd.dev.Screen.SetInvert(set.Dark)
	rd.dev.SetReverseButtons(set.ReverseButtons)
	rd.dev.SetSleepAfter(time.Duration(set.SleepMin) * time.Minute)
	if set.Light != nil && set.Warmth != nil { // the reader's own light (light.go)
		rd.dev.SetLight(*set.Light)
		rd.dev.SetWarmth(*set.Warmth)
	}
	if len(os.Args) > 2 && os.Args[2] != "-" {
		rd.fonts = blitz.LoadFonts(os.Args[2])
	}
	rd.pages = make(drawnPages, 3)
	for i := range rd.pages {
		rd.pages[i] = &drawn{img: image.NewGray(image.Rect(0, 0, rd.dev.Screen.W, rd.dev.Screen.H))}
	}
	rd.over = image.NewGray(image.Rect(0, 0, rd.dev.Screen.W, rd.dev.Screen.H))
	// started: from the process's start to main (the program read, Go's runtime and every package's init), to 10 ms
	fmt.Printf("started: main after %s, fonts and screen %.0f ms more\n", started, ms(t))
	// RESTORE_SCREEN: the screen found put back on leaving, for a host paused meanwhile that does not redraw once
	// resumed (Kobo's interface and the Kindle's, from their launchers)
	var found device.Saved
	if os.Getenv("RESTORE_SCREEN") != "" {
		found = rd.dev.Screen.Save()
	}
	rd.dev.Usage("start", "")
	if os.Args[1] == "-library" {
		err = rd.browse(t)
	} else {
		err = rd.read(map[string]string{"book": os.Args[1]}, t)
	}
	rd.dev.Usage("stop", "")
	if found != nil {
		rd.dev.Screen.Restore(found)
	} else if err == nil {
		// the leaving screen (the mark and "Exiting…"), up until the device's own interface draws over it: Kobo's
		// home screen comes 8 to 10 s after the reader ends, the screen blank or still the library meanwhile
		// (2026-11-12). Not where the screen found is put back (the Kindle's): its interface is back at once.
		// Always in English, as the library (the maintainer, 2026-11-12): a book opened by name ends the reader with
		// its own language still set.
		t := time.Now()
		ui.SetLanguage("en")
		showOpening(rd.dev.Screen, ui.Leaving(rd.dir, rd.fonts, rd.dev.Screen.W, rd.dev.Screen.H, rd.dev.Colour()),
			device.WfGC16, true)
		rd.dev.Screen.Wait()
		fmt.Printf("leaving screen shown in %.0f ms\n", ms(t))
	}
	check(err)
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

// shownNow is the time the reader shows (the bar's clock, when a book was added): now, or the fixed time SHAMSAINK_CLOCK
// gives (2026-01-01T10:00, local time), so the simulated device's screens are the same run after run (docs/TESTING.md).
// Durations and timers keep the real clock.
func shownNow() time.Time {
	if v := os.Getenv("SHAMSAINK_CLOCK"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", v, time.Local); err == nil {
			return t
		}
	}
	return time.Now()
}

// isDir tells whether p is a folder.
func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
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
