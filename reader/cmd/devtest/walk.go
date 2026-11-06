package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"shamsainkreader/design"
)

// The walk: every screen the design spec says can be reached, opened the way the spec says, held against its kept render
// and closed the way the spec says, generated from the spec (docs/TESTING.md, section 6). Nobody lists its steps: a screen
// added to the spec, with its kept render, is walked the next time. It is a scenario's step (walk), so it has the
// scenario's known start, crash check and failure folder.
//
// Its routes start from one of two bases, where nothing is open over the screen: the library (the spec's entry) and the
// page of a book (the spec's page screen). A route is the gestures from its base to a screen, each an edge of the spec:
// a screen's enter: (a gesture on another screen, or a tap on one of its controls) or a control's goto or replace.

// An edge: screen to is reached from screen from by gesture (tap, with control; or a gesture of its own: hold-row, ...).
type edge struct {
	from, to, gesture, control, when string
}

func (e edge) String() string {
	if e.control != "" {
		return e.from + " ›" + e.control + "› " + e.to
	}
	return e.from + " ›" + e.gesture + "› " + e.to
}

// A route: a base, and the edges from it to a screen.
type route struct {
	base  string
	edges []edge
}

func (r route) target() string { return r.edges[len(r.edges)-1].to }

func (r route) String() string {
	var s []string
	for _, e := range r.edges {
		s = append(s, e.String())
	}
	return strings.Join(s, ", ")
}

// walkable are the gestures the walk can make; automatic, the ways in it waits for rather than makes (the screens they
// open are shown while the reader starts or opens a book, and close themselves).
var (
	walkable  = map[string]bool{"tap": true, "tap-top-tenth": true, "hold-row": true, "hold-word": true}
	automatic = map[string]bool{"app-start": true, "slow-open": true}
)

// edges are the spec's ways from screen to screen: enter: and the controls' goto and replace, once each.
func edges(spec *design.Spec) []edge {
	seen := map[string]bool{}
	var out []edge
	add := func(e edge) {
		k := e.from + "|" + e.to + "|" + e.gesture + "|" + e.control
		if !seen[k] {
			seen[k] = true
			out = append(out, e)
		}
	}
	names := make([]string, 0, len(spec.Screens))
	for n := range spec.Screens {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		sc := spec.Screens[n]
		for _, c := range sc.Controls {
			if c.Action == "goto" || c.Action == "replace" {
				add(edge{from: n, to: c.To, gesture: "tap", control: c.ID, when: c.When})
			}
		}
	}
	for _, n := range names {
		for _, en := range spec.Screens[n].Enter {
			if en.From == "" {
				continue
			}
			when := ""
			if en.Control != "" { // the control's own condition (menu's settings: in the library)
				for _, c := range spec.Screens[en.From].Controls {
					if c.ID == en.Control {
						when = c.When
					}
				}
			}
			add(edge{from: en.From, to: n, gesture: en.Gesture, control: en.Control, when: when})
		}
	}
	return out
}

// plan gives the walk's routes, one for each screen it can reach (the shortest, from the base the screen is first found
// from), and why each built screen it cannot reach is left (reached only automatically, or by nothing walkable).
func plan(spec *design.Spec) (routes []route, left map[string]string) {
	entry, page := bases(spec)
	all := edges(spec)
	reached := map[string]bool{entry: true, page: true} // by either base
	// a screen is walked from each base that reaches it: the menu or Light in a book are not the library's
	for _, base := range []string{entry, page} {
		here := map[string]bool{}
		queue := []route{{base: base}}
		for len(queue) > 0 {
			r := queue[0]
			queue = queue[1:]
			at := base
			if len(r.edges) > 0 {
				at = r.target()
			}
			for _, e := range all {
				if e.from != at || !walkable[e.gesture] || here[e.to] {
					continue
				}
				if e.when != "" && !fits(e.when, base, entry) {
					continue
				}
				if e.to == entry || e.to == page { // a base: the way between them is walked once, not as a route
					continue
				}
				n := route{base: base, edges: append(append([]edge(nil), r.edges...), e)}
				reached[e.to], here[e.to] = true, true
				routes = append(routes, n)
				if len(n.edges) < 4 {
					queue = append(queue, n)
				}
			}
		}
	}
	left = map[string]string{}
	for n, sc := range spec.Screens {
		if sc.Status != "built" || reached[n] {
			continue
		}
		why := "no walkable way in"
		for _, en := range sc.Enter {
			if automatic[en.Gesture] {
				why = "shown by the reader itself (" + en.Gesture + ")"
			}
		}
		left[n] = why
	}
	return routes, left
}

// bases are the spec's entry (the library) and its page (a book's).
func bases(spec *design.Spec) (entry, page string) {
	for n, sc := range spec.Screens {
		if sc.Entry {
			entry = n
		}
		if sc.Page {
			page = n
		}
	}
	return
}

// fits says whether a control's condition holds on the walk's way from base: in the library, or in a book.
func fits(when, base, entry string) bool {
	switch when {
	case "in the library":
		return base == entry
	case "in a book":
		return base != entry
	}
	return true // a condition of the book or the device: the control is there or not, and the check says which
}

// The walk on a device.
type walker struct {
	p         *player
	spec      *design.Spec
	lang      string // the renders' language: the library's is English; a book's, its direction's
	base      string // the base being walked from
	baseShot  *screen
	placed    map[string][2]int // where a screen hung by something was put, from the reader's log
	shown     map[string]string // the kept render each screen of the route matched (its case: the book menu of a new book)
	told      map[string]string // the case the reader said it drew, which a match by ink cannot tell (a menu's labels)
	hold      [4]int            // the word held for the highlight's menu: from x, y to x2, y2
	report    func(good bool, what, detail string)
	checked   map[string]bool
	failures  []string
	entry, pg string
}

var reBookPanel = regexp.MustCompile(`library: book (\w+) \((\w+)\) at (\d+),(\d+)`)

// walk plays every route of the plan: the library's from the library, then a book opened by its first row and the page's,
// then the library again, and the reader left. Its failures are listed together at the end: one screen drawn wrong does not
// hide the next.
func (p *player) walk(args []string) (string, error) {
	spec, err := design.Load()
	if err != nil {
		return "", err
	}
	routes, left := plan(spec)
	d := p.d
	w := &walker{p: p, spec: spec, lang: "en", placed: map[string][2]int{}, shown: map[string]string{}, told: map[string]string{}, checked: map[string]bool{},
		hold: [4]int{d.prof.Screen.Width * 7 / 10, 700, d.prof.Screen.Width * 4 / 10, 700}}
	w.entry, w.pg = bases(spec)
	if len(args) == 5 && args[0] == "word" { // walk word X Y X2 Y2: where a word is on the page the book opens at
		for i := range 4 {
			w.hold[i], _ = strconv.Atoi(args[i+1])
		}
	}
	w.report = func(good bool, what, detail string) {
		mark := "ok  "
		if !good {
			mark = "FAIL"
			w.failures = append(w.failures, what+": "+detail)
		}
		fmt.Printf("       %s %s: %s\n", mark, what, detail)
	}
	d.book = ""
	stop, _, err := d.start()
	p.stop = stop
	if err != nil {
		return "", err
	}
	if w.baseShot, err = w.settled(""); err != nil {
		return "", err
	}
	w.base = w.entry
	for _, r := range routes {
		if r.base == w.entry {
			w.play(r)
		}
	}
	// into a book: the library's first row
	before := d.fbHash()
	b, err := d.openFirst(p.lib)
	if err != nil {
		return "", fmt.Errorf("library ›e0› reading: %v", err)
	}
	p.b, w.lang, w.base = b, b.lang, w.pg
	if w.baseShot, err = w.settled(before); err != nil {
		return "", err
	}
	w.report(true, "library ›e0› reading", fmt.Sprintf("document %d page %d, %s", b.doc, b.page, map[bool]string{true: "right to left", false: "left to right"}[b.rtl]))
	for _, r := range routes {
		if r.base == w.pg {
			w.play(r)
		}
	}
	var notes []string
	for n, why := range left {
		notes = append(notes, n+": "+why)
	}
	sort.Strings(notes)
	for _, n := range notes {
		fmt.Printf("       note %s\n", n)
	}
	if len(w.failures) > 0 {
		return "", fmt.Errorf("%d of %d screens not as the spec has them: %s", len(w.failures), len(routes), strings.Join(w.failures, "; "))
	}
	return fmt.Sprintf("%d screens walked from the spec, %d shown by the reader itself", len(routes), len(left)), nil
}

// settled waits for the screen to settle (after it changed from a hash, if given) and reads it.
func (w *walker) settled(from string) (*screen, error) {
	if err := w.p.d.settle(from); err != nil {
		return nil, err
	}
	return w.p.d.screen()
}

// play walks one route from its base: each gesture made and its screen waited for, the last screen held against its kept
// render, closed, and the base as it was.
func (w *walker) play(r route) {
	what := r.String()
	w.shown = map[string]string{}
	for i, e := range r.edges {
		before := w.p.d.fbHash()
		if err := w.gesture(e); err != nil {
			w.report(false, what, err.Error())
			w.home()
			return
		}
		shot, err := w.settled(before)
		if err != nil {
			w.report(false, what, e.String()+": "+err.Error())
			w.home()
			return
		}
		w.shown[e.to] = w.match(e.to, shot)
		if i == len(r.edges)-1 {
			w.check(what, e.to, shot)
		}
	}
	w.effects(r)
	w.dismiss(r)
}

// effects presses each control of the route's screen that says what it does (the spec's emits:) and the walk may press:
// its log line waited for, then its undo pressed (the screen opened again first when pressing closed it) and the line
// waited for again. The base as it was is checked when the screen is closed (dismiss): an undo that does not put things
// back fails there.
func (w *walker) effects(r route) {
	screen := r.target()
	sc := w.spec.Screens[screen]
	if sc == nil {
		return
	}
	for _, c := range sc.Controls {
		if c.Emits == "" || c.Walk != "" || c.Dynamic {
			continue
		}
		what := "  " + screen + " ›" + c.ID + "›"
		press := c.ID
		if c.Press != "" {
			press = c.Press
		}
		shot, err := w.settled("")
		if err != nil {
			w.report(false, what, err.Error())
			return
		}
		if !w.inked(screen, press, shot) {
			fmt.Printf("       note %s not on the screen (%s), not pressed\n", what, c.When)
			continue
		}
		did, err := w.pressAndWait(screen, press, c.Emits)
		if err != nil {
			w.report(false, what, err.Error())
			w.home()
			w.replay(r)
			continue
		}
		if c.Undo == "" {
			w.report(true, what, did+" (no undo)")
			continue
		}
		if c.Closes {
			if err := w.replay(r); err != nil {
				w.report(false, what, did+"; opened again for the undo: "+err.Error())
				continue
			}
		}
		undid, err := w.pressAndWait(screen, c.Undo, c.Emits)
		if err != nil {
			w.report(false, what, did+"; undo "+c.Undo+": "+err.Error())
			w.home()
			w.replay(r)
			continue
		}
		w.report(true, what, did+"; undone by "+c.Undo+": "+undid)
		if c.Closes {
			w.replay(r) // the screen again, for the next control
		}
	}
}

// inked says whether a button of a screen has ink where the screen's kept render puts it.
func (w *walker) inked(screen, id string, shot *screen) bool {
	rr, _ := w.render(screen)
	if rr == nil {
		return false
	}
	b, ok := box(rr, id)
	at := w.placed[screen]
	return ok && shot.dark([4]int{at[0] + b[0], at[1] + b[1], at[0] + b[2], at[1] + b[3]}) >= 20
}

// pressAndWait taps a button of a screen by its kept render and waits for the line that says it was done.
func (w *walker) pressAndWait(screen, id, emits string) (string, error) {
	rr, name := w.render(screen)
	if rr == nil {
		return "", fmt.Errorf("no kept render of %s", screen)
	}
	b, ok := box(rr, id)
	if !ok {
		return "", fmt.Errorf("no button %s in %s", id, name)
	}
	at := w.placed[screen]
	before := w.p.d.fbHash()
	if err := w.p.d.tapAt(at[0]+(b[0]+b[2])/2, at[1]+(b[1]+b[3])/2); err != nil {
		return "", err
	}
	line, err := w.p.d.expect(emits, 15*time.Second)
	if err != nil {
		return "", err
	}
	if err := w.p.d.settle(before); err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// replay opens the route's screens again from where the screen is (its base, after a press that closed them), without
// holding them against their renders: they were, a moment ago.
func (w *walker) replay(r route) error {
	for _, e := range r.edges {
		before := w.p.d.fbHash()
		if err := w.gesture(e); err != nil {
			return err
		}
		shot, err := w.settled(before)
		if err != nil {
			return fmt.Errorf("%s: %v", e, err)
		}
		w.shown[e.to] = w.match(e.to, shot)
	}
	return nil
}

// gesture makes one edge's gesture on the device.
func (w *walker) gesture(e edge) error {
	d := w.p.d
	switch e.gesture {
	case "tap-top-tenth":
		return d.tapAt(d.prof.Screen.Width/2, 80)
	case "hold-word":
		if err := d.holdAt(w.hold[0], w.hold[1], w.hold[2], w.hold[3]); err != nil {
			return err
		}
		line, err := d.expect(`highlight menu: .* at \d+,\d+`, 8*time.Second)
		if err != nil {
			return fmt.Errorf("no highlight menu: no word at %d,%d (walk word X Y X2 Y2 names one): %v", w.hold[0], w.hold[1], err)
		}
		m := reMenuAt.FindStringSubmatch(line)
		x, _ := strconv.Atoi(m[1])
		y, _ := strconv.Atoi(m[2])
		w.placed[e.to] = [2]int{x, y}
		return nil
	case "hold-row":
		lib, _ := d.boxes("library-reading-en")
		b, ok := box(lib, "e0")
		if !ok {
			return fmt.Errorf("no first row in the library's kept render")
		}
		x, y := (b[0]+b[2])/2, (b[1]+b[3])/2
		if err := d.holdAt(x, y, x, y); err != nil {
			return err
		}
		return w.expectPlaced(e.to)
	case "tap":
		r, name := w.render(e.from)
		if r == nil {
			return fmt.Errorf("no kept render of %s", e.from)
		}
		bx, ok := box(r, e.control)
		if !ok {
			return fmt.Errorf("no control %s in %s", e.control, name)
		}
		at := w.placed[e.from]
		if err := d.tapAt(at[0]+(bx[0]+bx[2])/2, at[1]+(bx[1]+bx[3])/2); err != nil {
			return err
		}
		if w.relative(e.to) && e.from != w.entry && e.from != w.pg {
			return w.expectPlaced(e.to) // a panel hung where the one before it was
		}
		return nil
	}
	return fmt.Errorf("the walk has no way to make the gesture %q", e.gesture)
}

// expectPlaced waits for the reader's line saying where a hung panel was put.
func (w *walker) expectPlaced(screen string) error {
	line, err := w.p.d.expect(reBookPanel.String(), 8*time.Second)
	if err != nil {
		return err
	}
	m := reBookPanel.FindStringSubmatch(line)
	x, _ := strconv.Atoi(m[3])
	y, _ := strconv.Atoi(m[4])
	w.placed[screen] = [2]int{x, y}
	w.told[screen] = screen + "-" + m[2] + "-" + w.lang // the case the reader drew: its labels differ, not its ink
	return nil
}

// relative says whether a screen's kept renders are placed by what it hangs from, not on the screen.
func (w *walker) relative(screen string) bool {
	r, _ := w.render(screen)
	return r != nil && r.Placement != "screen"
}

// render is the kept render of a screen for where the walk is: its case for the library or a book when it has one
// (the menu), else the one with the most buttons, in the walk's language (the library's renders are English).
func (w *walker) render(screen string) (*boxes, string) {
	if name := w.shown[screen]; name != "" {
		if r, err := w.p.d.boxes(name); err == nil {
			return r, name
		}
	}
	lang := w.lang
	if screen == w.entry {
		lang = "en"
	}
	ctx := map[bool]string{true: "library", false: "book"}[w.base == w.entry]
	if r, err := w.p.d.boxes(screen + "-" + ctx + "-" + lang); err == nil {
		return r, screen + "-" + ctx + "-" + lang
	}
	name, r := w.p.d.bestRenderIn(screen, lang)
	return r, name
}

// match is the kept render of a screen that the screen shows: of its cases (a menu's for a new, opened or finished
// book; Text's tabs), the one with ink at every button and the most buttons; the walk's own choice (render) when it fits
// as well. Its taps are then at that case's places.
func (w *walker) match(screen string, shot *screen) string {
	if name := w.told[screen]; name != "" {
		if _, err := w.p.d.boxes(name); err == nil {
			return name
		}
	}
	ctx := map[bool]string{true: "library", false: "book"}[w.base == w.entry]
	if name := screen + "-" + ctx + "-" + w.lang; screen != w.entry {
		if _, err := w.p.d.boxes(name); err == nil {
			return name // a case for where the walk is (the library's menu): only it, so a wrong one is said as such
		}
	}
	pref, prefName := w.render(screen)
	lang := w.lang
	if screen == w.entry {
		lang = "en"
	}
	files, _ := filepath.Glob(filepath.Join("reader", "design", "golden", w.p.d.name, screen+"-*-"+lang+".json"))
	best, bestMiss, bestN := prefName, 1<<30, 0
	if pref != nil {
		bestMiss, bestN = w.misses(screen, pref, shot), len(pref.Boxes)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if owner := strings.TrimPrefix(name, screen+"-"); strings.Count(owner, "-") > 1 {
			continue // another screen's whose name starts with this one's
		}
		r, err := w.p.d.boxes(name)
		if err != nil {
			continue
		}
		if m := w.misses(screen, r, shot); m < bestMiss || m == bestMiss && len(r.Boxes) > bestN {
			best, bestMiss, bestN = name, m, len(r.Boxes)
		}
	}
	return best
}

// misses counts a render's buttons with no ink on the screen, where the screen is placed.
func (w *walker) misses(screen string, r *boxes, shot *screen) int {
	at, n := w.placed[screen], 0
	for _, b := range r.Boxes {
		if shot.dark([4]int{at[0] + b[0], at[1] + b[1], at[0] + b[2], at[1] + b[3]}) < 20 {
			n++
		}
	}
	return n
}

// optional are a screen's buttons that are there only sometimes: the spec's controls with a condition (when), a need of
// the device (requires), or made per item (a list's rows).
func (w *walker) optional(screen, id string) bool {
	sc := w.spec.Screens[screen]
	if sc == nil {
		return false
	}
	for _, c := range sc.Controls {
		match := id == c.ID
		if c.Dynamic && strings.HasPrefix(id, c.ID) {
			rest := strings.TrimPrefix(id, c.ID)
			_, err := strconv.Atoi(rest)
			match = err == nil
			for _, v := range c.Values {
				match = match || rest == v
			}
			if c.Dynamic && c.Component == "list-row" {
				return match
			}
		}
		if match && (c.When != "" || len(c.Requires) > 0 || c.Dynamic) {
			return true
		}
	}
	return false
}

// check holds the screen against the kept render: ink at each of its buttons there always, and the screen changed in
// their area from the base (the screen is new, not ink that was there).
func (w *walker) check(what, screen string, shot *screen) {
	r, name := w.render(screen)
	if r == nil {
		w.report(false, what, "no kept render of "+screen)
		return
	}
	at := w.placed[screen]
	if r.Placement != "screen" && at == ([2]int{}) {
		w.report(false, what, name+" is hung by something, and the reader did not say where")
		return
	}
	var miss []string
	area := [4]int{1 << 30, 1 << 30, 0, 0}
	n := 0
	for id, b := range r.Boxes {
		bx := [4]int{at[0] + b[0], at[1] + b[1], at[0] + b[2], at[1] + b[3]}
		area = [4]int{min(area[0], bx[0]), min(area[1], bx[1]), max(area[2], bx[2]), max(area[3], bx[3])}
		if w.optional(screen, id) {
			continue
		}
		n++
		if k := shot.dark(bx); k < 20 {
			miss = append(miss, fmt.Sprintf("%s (%d dark pixels)", id, k))
		}
	}
	if n == 0 && len(r.Boxes) > 0 && w.misses(screen, r, shot) == len(r.Boxes) { // all there only sometimes: one at least
		miss = append(miss, "ink at any of its buttons")
	}
	sort.Strings(miss)
	if r.Placement != "screen" { // hung by something: its area is its picture's, where the reader put it
		if wh, err := pngSize(filepath.Join("reader", "design", "golden", w.p.d.name, name+".png")); err == nil {
			area = [4]int{at[0], at[1], at[0] + wh[0], at[1] + wh[1]}
		}
	}
	if c := regionDiff(w.baseShot, shot, area); c < 0.02 {
		miss = append(miss, fmt.Sprintf("a change in its area (%.1f%% of it differs from %s)", 100*c, w.base))
	}
	w.checked[screen] = true
	w.report(len(miss) == 0, what, fmt.Sprintf("%d buttons held against %s%s", n, name,
		map[bool]string{true: "", false: "; missing: " + strings.Join(miss, ", ")}[len(miss) == 0]))
}

// outside is a tap that closes what is open and does nothing else: in the library the bar's clock (no button); on a
// book's page its middle, below the bar and above the dock, or higher when a menu hangs there. A tap on the page with
// nothing open would turn it, so dismiss taps only while the screen is not the base's.
func (w *walker) outside(screen string) (int, int) {
	W := w.p.d.prof.Screen.Width
	if w.base == w.entry {
		return W / 2, 80
	}
	if r, _ := w.render(screen); r != nil {
		at := w.placed[screen]
		for _, y := range []int{600, 1200, 300} {
			inside := false
			for _, b := range r.Boxes {
				inside = inside || y >= at[1]+b[1]-40 && y <= at[1]+b[3]+40
			}
			if !inside {
				return W / 2, y
			}
		}
	}
	return W / 2, 300
}

// dismiss closes the route's screens the way the spec says (dismiss:), one tap at a time while the screen is not the
// base's, and says whether the base is back as it was.
func (w *walker) dismiss(r route) {
	what := "  and back to the " + w.base
	for i := len(r.edges) - 1; i >= 0; i-- {
		now, err := w.settled("")
		if err != nil {
			w.report(false, what, err.Error())
			return
		}
		if differs(w.baseShot, now) < 0.0001 {
			break
		}
		screen := r.edges[i].to
		sc := w.spec.Screens[screen]
		before := w.p.d.fbHash()
		switch {
		case sc != nil && sc.Dismiss != "" && sc.Dismiss != "tap-outside" && sc.Dismiss != "automatic":
			rr, name := w.render(screen)
			if bx, ok := box(rr, sc.Dismiss); ok {
				w.p.d.tapAt((bx[0]+bx[2])/2, (bx[1]+bx[3])/2)
			} else {
				w.report(false, what, "no control "+sc.Dismiss+" in "+name+" to close it")
				return
			}
		default:
			x, y := w.outside(screen)
			w.p.d.tapAt(x, y)
		}
		if err := w.p.d.settle(before); err != nil {
			w.report(false, what, "closing "+screen+": "+err.Error())
			return
		}
	}
	now, err := w.settled("")
	if err != nil {
		w.report(false, what, err.Error())
		return
	}
	share := differs(w.baseShot, now)
	if share >= 0.0001 {
		w.report(false, what, fmt.Sprintf("%.2f%% of the screen differs from before", 100*share))
		w.home()
	}
}

// home is a failed route's way out, so the routes after it start from their base: taps outside, three at most, while
// the screen is not the base's (with something open, a tap on the page closes it; it turns no page).
func (w *walker) home() {
	for i := 0; i < 3; i++ {
		now, err := w.settled("")
		if err != nil || differs(w.baseShot, now) < 0.0001 {
			return
		}
		x, y := w.outside("")
		w.p.d.tapAt(x, y)
	}
}

// pngSize is a PNG's width and height, from its header.
func pngSize(path string) ([2]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return [2]int{}, err
	}
	defer f.Close()
	c, err := png.DecodeConfig(f)
	return [2]int{c.Width, c.Height}, err
}
