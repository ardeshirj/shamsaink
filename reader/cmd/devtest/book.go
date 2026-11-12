package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// A book opened by a scenario: where it opened, and which way it reads (told by the reader's own log, not guessed from
// the screen: both layouts have ink in many of the same boxes).
type book struct {
	d         *dev
	lib       *boxes
	lang      string // the interface as the reader logs it ("en", "fa", "en-rtl"): the renders its screens are held against
	rtl       bool
	doc, page int
	w         int
	report    func(good bool, what, detail string)
	bar, dock *boxes
}

var reOpened = regexp.MustCompile(`document (\d+) page (\d+) shown in .*right to left (true|false)(?:.*interface ([a-z-]+))?`)

// openFirst taps the library's first row (the book read last) and waits for it: its place and its direction.
func (d *dev) openFirst(lib *boxes) (*book, error) {
	if d.book == "" { // from the library; a book named by the scenario was opened by the launcher already
		if err := d.tap(lib, "e0"); err != nil {
			return nil, err
		}
		return d.openedRow(lib)
	}
	return d.opened(lib)
}

// openedRow waits for the book a tap on a library's row opened, and gives it as openFirst does: its place and its
// direction, the renders its screens are held against from here on.
func (d *dev) openedRow(lib *boxes) (*book, error) {
	if _, err := d.expect(`opened \d+ ms`, 40*time.Second); err != nil {
		return nil, err
	}
	return d.opened(lib)
}

// opened reads the book just opened from the reader's opening line.
func (d *dev) opened(lib *boxes) (*book, error) {
	b := &book{d: d, lib: lib, w: d.prof.Screen.Width}
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
	b.lang = m[4]
	if b.lang == "" { // a reader from before the interface was logged: a right-to-left book's was Farsi
		b.lang = map[bool]string{true: "fa", false: "en"}[b.rtl]
	}
	b.bar, _ = d.boxes("topbar-book-" + b.lang)
	b.dock, _ = d.boxes("dock-narrated-" + b.lang)
	if b.bar == nil || b.dock == nil {
		return nil, fmt.Errorf("no kept renders for a book in %s", b.lang)
	}
	return b, d.settle("")
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
	if err := b.d.settle(""); err != nil {
		b.report(false, "the bar and the dock", err.Error())
		return false
	}
	return true
}
