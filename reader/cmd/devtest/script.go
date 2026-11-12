package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// A scenario file (e2e/scenarios/NAME.txt): a header, then steps, one a line; # starts a comment.
//
//	on: sim kobo kindle    where it runs (required): the simulated Kobo (make e2e-sim), the Kobo or the Kindle (make e2e)
//	book: PATH             a book it needs, by its path under the reader's folder (books/farsiepub/...); checked before
//	                       the first step, so a missing book is said as such (one line each)
//	fixture: NAME          the reader's files it starts from, e2e/fixtures/NAME (default: default)
//
// The steps are listed in scenario.go. check holds every step against the design spec's kept renders without a device:
// go test ./cmd/devtest checks every scenario, so a step or control name mistyped fails in make check, not on the Kobo.
type script struct {
	path    string
	on      []string
	books   []string
	fixture string
	steps   []line
}

type line struct {
	n     int // 1-based, in the file
	text  string
	words []string
}

// where are the places a scenario can run: the simulated Kobo, the Kobo and the Kindle.
var where = map[string]bool{"sim": true, "kobo": true, "kindle": true}

func parseScenario(path string) (*script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &script{path: path, fixture: "default"}
	for n, raw := range strings.Split(string(data), "\n") {
		text := strings.TrimSpace(strings.SplitN(raw, " #", 2)[0])
		if strings.HasPrefix(text, "#") || text == "" {
			continue
		}
		w := strings.Fields(text)
		switch w[0] {
		case "on:", "book:", "fixture:":
			if len(s.steps) > 0 {
				return nil, fmt.Errorf("%s:%d: %s belongs in the header, before the first step", path, n+1, w[0])
			}
			if len(w) < 2 {
				return nil, fmt.Errorf("%s:%d: %s needs a value", path, n+1, w[0])
			}
			switch w[0] {
			case "on:":
				for _, p := range w[1:] {
					if !where[p] {
						return nil, fmt.Errorf("%s:%d: on: %q is not sim, kobo or kindle", path, n+1, p)
					}
				}
				s.on = w[1:]
			case "book:":
				s.books = append(s.books, w[1])
			case "fixture:":
				s.fixture = w[1]
			}
			continue
		}
		s.steps = append(s.steps, line{n + 1, text, w})
	}
	if len(s.on) == 0 {
		return nil, fmt.Errorf("%s: no on: line (on: sim kobo, on: kobo)", path)
	}
	if len(s.steps) == 0 {
		return nil, fmt.Errorf("%s: no steps", path)
	}
	return s, nil
}

func (s *script) runsOn(place string) bool {
	for _, p := range s.on {
		if p == place {
			return true
		}
	}
	return false
}

// koboOnly are the steps the simulator cannot do: the watchdog (the simulated launcher has none) and Kobo's own interface.
var koboOnly = map[string]bool{"freeze": true, "expect-nickel": true, "expect-online": true}

var reSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// check holds the scenario against what the steps need: each step known, its arguments, its regular expressions, the
// controls it taps by name in the device's kept renders, its fixture. Errors name the file's line.
func (s *script) check(d *dev) error {
	var errs []string
	fail := func(l line, f string, a ...any) {
		errs = append(errs, fmt.Sprintf("%s:%d: %s: %s", s.path, l.n, l.text, fmt.Sprintf(f, a...)))
	}
	if st, err := os.Stat(filepath.Join("e2e", "fixtures", s.fixture)); err != nil || !st.IsDir() {
		errs = append(errs, fmt.Sprintf("%s: no fixture e2e/fixtures/%s", s.path, s.fixture))
	}
	snaps := map[string]bool{}
	for _, l := range s.steps {
		a := l.words
		args := len(a) - 1
		isInt := func(w string) bool { _, err := strconv.Atoi(w); return err == nil }
		isNum := func(w string) bool { _, err := strconv.ParseFloat(w, 64); return err == nil }
		re := func(expr string) {
			if _, err := regexp.Compile(expr); err != nil {
				fail(l, "not a regular expression: %v", err)
			}
		}
		if koboOnly[a[0]] && s.runsOn("sim") {
			fail(l, "%s runs on the Kobo only (on: kobo)", a[0])
		}
		if a[0] == "press" && args == 1 && a[1] != "power" && s.runsOn("kindle") {
			fail(l, "the Kindle has no page buttons: a scenario with press %s is not on: kindle", a[1])
		}
		switch a[0] {
		case "open":
			if args > 1 {
				fail(l, "open [PATH]")
			}
		case "close", "freeze":
			if args != 0 {
				fail(l, "%s takes nothing", a[0])
			}
		case "tap":
			switch {
			case args == 1 && a[1] == "page-top":
			case args == 2 && isInt(a[1]) && isInt(a[2]):
			case args == 2:
				if err := d.hasControl(a[1], a[2]); err != nil {
					fail(l, "%v", err)
				}
			default:
				fail(l, "tap page-top, tap X Y or tap SCREEN CONTROL")
			}
		case "turn":
			if args != 1 || a[1] != "forward" && a[1] != "back" {
				fail(l, "turn forward|back")
			}
		case "press":
			if args != 1 || a[1] != "forward" && a[1] != "back" && a[1] != "power" {
				fail(l, "press forward|back|power")
			}
		case "tap-menu":
			if args != 1 {
				fail(l, "tap-menu CONTROL")
			} else if err := d.hasMenuControl(a[1]); err != nil {
				fail(l, "%v", err)
			}
		case "hold":
			if !(args == 2 && isInt(a[1]) && isInt(a[2]) || args == 5 && isInt(a[1]) && isInt(a[2]) && a[3] == "to" && isInt(a[4]) && isInt(a[5])) {
				fail(l, "hold X Y [to X2 Y2]")
			}
		case "expect":
			if args < 1 {
				fail(l, "expect REGEX")
			} else {
				re(strings.Join(a[1:], " "))
			}
		case "expect-in":
			if args < 2 || !isNum(a[1]) {
				fail(l, "expect-in SECONDS REGEX")
			} else {
				re(strings.Join(a[2:], " "))
			}
		case "expect-none":
			if args < 2 || !isNum(a[args]) {
				fail(l, "expect-none REGEX SECONDS")
			} else {
				re(strings.Join(a[1:args], " "))
			}
		case "expect-nickel", "expect-online", "wait":
			if args != 1 || !isNum(a[1]) {
				fail(l, "%s SECONDS", a[0])
			}
		case "fetch":
			if args != 3 || !strings.HasPrefix(a[1], "https://") || !reSHA256.MatchString(a[3]) {
				fail(l, "fetch https://URL PATH SHA256")
			}
		case "forget":
			if args != 1 {
				fail(l, "forget PATH")
			}
		case "env":
			for _, kv := range a[1:] {
				if !strings.Contains(kv, "=") {
					fail(l, "env KEY=VALUE...: %q", kv)
				}
			}
		case "resume":
			if args != 0 {
				fail(l, "resume takes nothing")
			}
		case "caption", "title":
			if args < 1 {
				fail(l, "%s TITLE ...", a[0])
			}
		case "walk":
			if !(args == 0 || args == 5 && a[1] == "word" && isInt(a[2]) && isInt(a[3]) && isInt(a[4]) && isInt(a[5])) {
				fail(l, "walk [word X Y X2 Y2]")
			}
		case "snap":
			if args != 1 || !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(a[1]) {
				fail(l, "snap NAME (a-z, 0-9, -)")
			} else if snaps[a[1]] {
				fail(l, "a second snap named %s", a[1])
			}
			if args == 1 {
				snaps[a[1]] = true
			}
		default:
			fail(l, "unknown step %q", a[0])
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// langs are the languages of the kept renders: a control is found in a book's direction when the scenario plays it.
var langs = []string{"en", "fa"}

// hasControl says whether some kept render of screen has the control (a list's row past those kept counts when the
// render has e0 and e1, as tapNamed finds it).
func (d *dev) hasControl(screen, id string) error {
	found := false
	for _, lang := range langs {
		_, r := d.bestRenderIn(screen, lang)
		if r == nil {
			continue
		}
		found = true
		if _, ok := box(r, id); ok {
			return nil
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "e")); err == nil && n >= 0 && strings.HasPrefix(id, "e") {
			_, ok0 := box(r, "e0")
			_, ok1 := box(r, "e1")
			if ok0 && ok1 {
				return nil
			}
		}
	}
	if !found {
		return fmt.Errorf("no kept render of the screen %q (reader/design/golden/%s)", screen, d.name)
	}
	return fmt.Errorf("no control %q in the kept renders of %s", id, screen)
}

func (d *dev) hasMenuControl(id string) error {
	for _, lang := range langs {
		for _, kind := range []string{"new", "made"} {
			if r, err := d.boxes("highlight-menu-" + kind + "-" + lang); err == nil {
				if _, ok := box(r, id); ok {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("no control %q in the highlight's menu", id)
}
