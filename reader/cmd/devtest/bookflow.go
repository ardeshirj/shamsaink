package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A book opened for the walk: where it opened, and which way it reads (told by the reader's own log, not guessed from the
// screen: both layouts have ink in many of the same boxes).
type book struct {
	d           *dev
	lib         *boxes
	lang        string // "fa" for a right-to-left book, else "en": the renders its screens are held against
	rtl         bool
	doc, page   int
	w           int
	report      func(good bool, what, detail string)
	page0, menu *screen // the page as opened, and the bar and dock as last shown
	bar, dock   *boxes
}

func pause() { time.Sleep(2500 * time.Millisecond) }

var reOpened = regexp.MustCompile(`document (\d+) page (\d+) shown in .*right to left (true|false)`)

// openFirst taps the library's first row (the book read last) and waits for it: its place and its direction.
func (d *dev) openFirst(lib *boxes) (*book, error) {
	b := &book{d: d, lib: lib, w: d.prof.Screen.Width}
	if d.book == "" { // from the library; a book named by the scenario was opened by the launcher already
		if err := d.tap(lib, "e0"); err != nil {
			return nil, err
		}
		if _, err := d.expect(`opened \d+ ms`, 40*time.Second); err != nil {
			return nil, err
		}
	}
	line, err := d.expect(`document \d+ page \d+ shown in`, 20*time.Second)
	if err != nil {
		return nil, err
	}
	m := reOpened.FindStringSubmatch(line)
	if m == nil {
		return nil, fmt.Errorf("the reader's opening line is not as expected: %q", line)
	}
	b.doc, _ = strconv.Atoi(m[1])
	b.page, _ = strconv.Atoi(m[2])
	b.rtl = m[3] == "true"
	b.lang = map[bool]string{true: "fa", false: "en"}[b.rtl]
	b.bar, _ = d.boxes("topbar-book-" + b.lang)
	b.dock, _ = d.boxes("dock-narrated-" + b.lang)
	if b.bar == nil || b.dock == nil {
		return nil, fmt.Errorf("no kept renders for a book in %s", b.lang)
	}
	pause()
	return b, nil
}

// menuIsOpen says whether the bar and the dock are on the screen: ink in the bar's menu button and the dock's Contents.
func (b *book) menuIsOpen(s *screen) bool {
	m, _ := box(b.bar, "topbar-menu")
	c, _ := box(b.dock, "contents")
	return s.dark(m) >= 20 && s.dark(c) >= 20
}

// open brings the bar and the dock by a tap in the page's top tenth, if they are not there; false if they did not come.
// It is the one tap that never turns a page.
func (b *book) open() bool {
	if err := b.d.tapAt(b.w/2, 80); err != nil {
		b.report(false, "the bar and the dock", err.Error())
		return false
	}
	if _, err := b.d.expect(`menu drawn in`, 10*time.Second); err != nil {
		b.report(false, "reading ›tap the top tenth› the bar and the dock", err.Error())
		return false
	}
	pause()
	s, err := b.d.screen()
	if err != nil {
		b.report(false, "the screen read", err.Error())
		return false
	}
	b.menu = s
	return true
}

// closeOnPage taps the middle of the page to close the bar, the dock and the sheet, but only when they are on the screen:
// a tap on the page without them turns it.
func (b *book) closeOnPage(what string) bool {
	s, err := b.d.screen()
	if err != nil || !b.menuIsOpen(s) {
		b.report(false, what, "the bar and the dock are not on the screen: no tap, a tap on the page would turn it")
		return false
	}
	b.d.tapAt(b.w/2, 600)
	return true
}

func missing(r *boxes, s *screen, optional ...string) []string {
	var out []string
	for id, bx := range r.Boxes {
		skip := false
		for _, o := range optional {
			skip = skip || o == id
		}
		if n := s.dark([4]int{bx[0], bx[1], bx[2], bx[3]}); n < 20 && !skip {
			out = append(out, fmt.Sprintf("%s (%d dark pixels)", id, n))
		}
	}
	return out
}

// shows says whether the screen now shows the buttons of a kept render, new in their area (not ink that was there).
func (b *book) shows(what, render string, before *screen, optional ...string) bool {
	r, err := b.d.boxes(render)
	if err != nil {
		b.report(false, what, err.Error())
		return false
	}
	pause()
	now, err := b.d.screen()
	if err != nil {
		b.report(false, what, err.Error())
		return false
	}
	m := missing(r, now, optional...)
	area := [4]int{1 << 30, 1 << 30, 0, 0}
	for _, bx := range r.Boxes {
		area = [4]int{min(area[0], bx[0]), min(area[1], bx[1]), max(area[2], bx[2]), max(area[3], bx[3])}
	}
	if c := regionDiff(before, now, area); c < 0.02 {
		m = append(m, fmt.Sprintf("a change in its area (%.1f%% differs)", 100*c))
	}
	b.report(len(m) == 0, what, fmt.Sprintf("%d buttons held against %s%s", len(r.Boxes), render,
		map[bool]string{true: "", false: "; missing: " + strings.Join(m, ", ")}[len(m) == 0]))
	return len(m) == 0
}

// same says whether the screen is as it was in want.
func (b *book) same(what string, want *screen, as string) bool {
	pause()
	now, err := b.d.screen()
	if err != nil {
		b.report(false, what, err.Error())
		return false
	}
	share := differs(want, now)
	b.report(share < 0.005, "  "+what, fmt.Sprintf("%.2f%% of the screen differs from %s", 100*share, as))
	return share < 0.005
}

// bookFlow walks the flow inside a book, from the library's first row: the bar and the dock opened by a tap in the page's top
// tenth; Contents (and a jump to an entry, and the way back from it), the Text sheet, Light, the battery and the menu
// opened by name, each held against its kept render and closed; the highlight's menu opened by a hold on a word and closed;
// the way back to the library. It stops at the first step that leaves the screen not as expected, since the next tap would
// be at a place that is not there, and no tap that is not on the bar, the dock or a panel is made unless they are open: it
// turns no page, and the place the reader keeps ends where it began.
func (d *dev) bookFlow(lib *boxes, report func(good bool, what, detail string)) bool {
	b, err := d.openFirst(lib)
	if err != nil {
		report(false, "library ›e0› reading", err.Error())
		return false
	}
	b.report = report
	report(true, "library ›e0› reading", fmt.Sprintf("document %d page %d, the book reads %s", b.doc, b.page,
		map[bool]string{true: "right to left (Farsi renders)", false: "left to right"}[b.rtl]))
	if b.page0, err = d.screen(); err != nil {
		report(false, "the page read", err.Error())
		return false
	}

	// the bar and the dock
	if !b.open() {
		return false
	}
	m := append(missing(b.bar, b.menu, "topbar-phones"), missing(b.dock, b.menu, "back", "read-aloud")...)
	report(len(m) == 0, "reading ›tap the top tenth› topbar and dock", fmt.Sprintf("%d + %d buttons held against the renders%s", len(b.bar.Boxes), len(b.dock.Boxes),
		map[bool]string{true: "", false: "; no ink at " + strings.Join(m, ", ")}[len(m) == 0]))
	if len(m) > 0 {
		return false // not the layout the render has: no tap is made
	}

	// Contents: open, then a jump to the third entry, and the way back from it by the dock's Back
	if err := d.tap(b.dock, "contents"); err != nil {
		report(false, "dock ›contents› contents", err.Error())
		return false
	}
	line, err := d.expect(`contents: \d+ entries`, 10*time.Second)
	if err != nil {
		report(false, "dock ›contents› contents", err.Error())
		return false // the next taps would be on a screen that is not there
	}
	if !b.shows("dock ›contents› contents ("+strings.TrimSpace(line)+")", "contents-list-"+b.lang, b.menu, "e0", "e1", "e2", "e3") {
		return false
	}
	cont, _ := d.boxes("contents-list-" + b.lang)
	if err := d.tap(cont, "e2"); err != nil {
		report(false, "contents ›e› reading", err.Error())
		return false
	}
	jump, err := d.expect(`document \d+ page \d+`, 15*time.Second)
	if err != nil {
		report(false, "contents ›e2› reading (a jump)", err.Error())
		return false
	}
	report(true, "contents ›e2› reading (a jump)", strings.TrimSpace(jump))
	pause()
	if !b.open() {
		return false
	}
	back, _ := box(b.dock, "back")
	if n := b.menu.dark(back); n < 20 {
		report(false, "the dock after a jump", fmt.Sprintf("no Back button: %d dark pixels at %v", n, back))
		return false
	}
	report(true, "the dock after a jump", "Back is there")
	if err := d.tap(b.dock, "back"); err != nil {
		report(false, "dock ›back› reading", err.Error())
		return false
	}
	if line, err = d.expect(fmt.Sprintf(`document %d page %d`, b.doc, b.page), 15*time.Second); err != nil {
		report(false, "dock ›back› reading (the way back)", err.Error())
		return false
	}
	report(true, "dock ›back› reading (the way back)", "to document "+strconv.Itoa(b.doc)+" page "+strconv.Itoa(b.page)+": "+strings.TrimSpace(line))
	if !b.open() {
		return false
	}

	// the Text sheet, in the dock's place
	if err := d.tap(b.dock, "text"); err != nil {
		report(false, "dock ›text› text", err.Error())
		return false
	}
	if !b.shows("dock ›text› text", "text-text-"+b.lang, b.menu) {
		return false
	}
	if !b.closeOnPage("text ›tap the page› reading") {
		return false
	}
	if !b.same("text ›tap the page› reading", b.page0, "the page as opened") {
		return false
	}
	if !b.open() {
		return false
	}

	// the panels from the bar, each closed by a tap on the page, the bar and the dock staying
	for _, p := range []struct{ control, to, render string }{{"topbar-light", "light", "light-panel-"}, {"topbar-battery", "battery", "battery-level-"},
		{"topbar-menu", "menu", "menu-book-"}} {
		what := "topbar ›" + p.control + "› " + p.to
		if err := d.tap(b.bar, p.control); err != nil {
			report(false, what, err.Error())
			return false
		}
		if !b.shows(what, p.render+b.lang, b.menu) {
			return false
		}
		if !b.closeOnPage(what + " closed") {
			return false
		}
		if !b.same("and back to the bar and the dock", b.menu, "the bar and the dock before") {
			return false
		}
	}

	// the highlight's menu: the bar and the dock closed, then a hold on a word of the page, dragged and lifted, and a tap off it
	if !b.closeOnPage("the bar and the dock closed") {
		return false
	}
	pause()
	if err := d.holdAt(b.w*7/10, 700, b.w*4/10, 700); err != nil {
		report(false, "reading ›hold a word› highlight-menu", err.Error())
		return false
	}
	if line, err := d.expect(`highlight menu: words chosen`, 8*time.Second); err != nil {
		report(false, "reading ›hold a word› highlight-menu", "no menu: no word at that place, or a gap in the text; "+err.Error())
	} else {
		report(true, "reading ›hold a word› highlight-menu", line)
		d.tapAt(b.w/2, 300) // off the menu: the words left as they were (the menu is open, so no page turns)
		b.same("and back to the page (the words left as they were)", b.page0, "the page as opened")
	}

	// the way back to the library, by the bar's back arrow
	if !b.open() {
		return false
	}
	if err := d.tap(b.bar, "topbar-start"); err != nil {
		report(false, "topbar ›topbar-start› library", err.Error())
		return false
	}
	if line, err := d.expect(`library: \d+ books.*shown in`, 20*time.Second); err != nil {
		report(false, "topbar ›topbar-start› library", err.Error())
		return false
	} else {
		report(true, "topbar ›topbar-start› library", line)
	}
	pause()
	return true
}

// turnPage opens the first book, turns one page (back or forward, by the book's own direction) and leaves the book: for a
// place that a failed run moved. It reads the place from the reader's log.
func turnPage(name, way string) bool {
	d, err := load(name)
	if err != nil {
		fmt.Println(err)
		return false
	}
	lib, err := d.boxes("library-reading-en")
	if err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	stop, _, err := d.start()
	if stop != nil {
		defer stop()
	}
	if err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	b, err := d.openFirst(lib)
	if err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	fmt.Printf("opened at document %d page %d (reads %s)\n", b.doc, b.page, b.lang)
	// back is the quarter by the book's start edge, forward the rest
	left := (way == "back") != b.rtl
	x := b.w * 9 / 10
	if left {
		x = b.w / 10
	}
	if err := d.tapAt(x, 900); err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	line, err := d.expect(`document \d+ page \d+: drawn`, 15*time.Second)
	if err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	fmt.Println("ok  ", strings.TrimSpace(line))
	pause()
	b.report = func(good bool, what, detail string) { fmt.Println(good, what, detail) }
	if !b.open() {
		return false
	}
	if err := d.tap(b.bar, "topbar-start"); err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	if _, err := d.expect(`library: \d+ books.*shown in`, 20*time.Second); err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	pause()
	menu, _ := d.boxes("menu-library-en")
	if err := d.leave(lib, menu); err != nil {
		fmt.Println("FAIL", err)
		return false
	}
	fmt.Println("ok   left")
	return true
}
