package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A scenario is a file of steps, one a line (# starts a comment), played on the device by name where it can be:
//
//	open [PATH]               the reader launched and the library's first row opened: the book read last; with a path
//	                          under the device's folder (books/idpf/moby-dick-mo.epub) the launcher opens that book
//	                          at once, with no library
//	close                     the book left by the bar's back arrow, then the reader left from the library's menu
//	tap SCREEN CONTROL        a control of a screen of the spec, by its kept render: tap dock contents, tap contents e7
//	                          (a list's row n is the first row's box moved down by n rows), tap topbar topbar-light
//	tap page-top              the top tenth of the page: the bar and the dock
//	turn forward|back         a tap on the page that turns it, by the book's own direction
//	tap-menu CONTROL          an entry of the highlight's menu, found where the reader says it put it: tap-menu hl-make
//	tap X Y                   a tap at the screen's pixels (for what is the book's: a word)
//	swipe X Y X2 Y2, hold X Y [to X2 Y2]
//	expect REGEX              waits (10 s at most) for a line of the reader's log after the last one expected
//	expect-in SECONDS REGEX   the same, with that long to wait (a narration plays in real time)
//	snap NAME                 the screen copied: e2e/out/SCENARIO/NAME.pgm, compared byte for byte with refs/NAME.pgm
//	                          (--save keeps it as the reference), the clock's strip aside
//	wait SECONDS
//	env KEY=VALUE...          settings the reader is started with, before open: the heartbeat and the watcher on a faster
//	                          clock, HEARTBEAT_EVERY=1s WATCH_EVERY=4 (a freeze caught in 8 to 12 s, not 20 to 30)
//	fetch URL PATH SHA256     the file at PATH on the device, downloaded from URL (checked against the SHA256) if it is
//	                          not there: a public sample book for a scenario, never committed
//	forget PATH               the book's saved place removed on the device (its key, as library.Key makes it: a shell
//	                          recipe, for books over 128 KB), so the book opens at its start
//	freeze                    the reader stopped (SIGSTOP), as a hang that answers nothing
//	expect-none REGEX SECONDS waits that long and fails if the log has a line like it
//	expect-nickel SECONDS     waits (SSH may drop and come back) for Kobo's interface, the reader gone: the reader closed
//
// The references are drawn from a book: never committed (e2e/out/ is ignored).

// topRows are the rows of the clock's strip, left out of a comparison.
const topRows = 60

type player struct {
	d          *dev
	lib, menu  *boxes
	b          *book
	out, refs  string
	save       bool
	hlX, hlY   int // where the highlight's menu was put, from the reader's log
	hlSeen     bool
	stop       func()
	closedBook bool
}

var reMenuAt = regexp.MustCompile(`highlight menu: .* at (\d+),(\d+)`)

func scenario(name, path string, save bool) bool {
	d, err := load(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	lib, err := d.boxes("library-reading-en")
	if err != nil {
		fmt.Println("FAIL the kept boxes of the library:", err)
		return false
	}
	menu, _ := d.boxes("menu-library-en")
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	p := &player{d: d, lib: lib, menu: menu, out: filepath.Join("e2e", "out", base), save: save}
	p.refs = filepath.Join(p.out, "refs")
	os.MkdirAll(p.refs, 0o755)
	defer func() {
		if p.stop != nil {
			p.stop()
		}
	}()
	for n, line := range strings.Split(string(data), "\n") {
		text := strings.TrimSpace(strings.SplitN(line, " #", 2)[0])
		if strings.HasPrefix(text, "#") || text == "" {
			continue
		}
		t := time.Now()
		said, err := p.step(strings.Fields(text))
		if err != nil {
			fmt.Printf("FAIL %3d %s\n         %v\n", n+1, text, err)
			fmt.Println("stopped at the first failure: the reader may still be running on the device (it is stopped now, and its own interface started again)")
			return false
		}
		fmt.Printf("ok   %3d %-50.50s %5.1f s  %s\n", n+1, text, time.Since(t).Seconds(), truncate(said, 70))
	}
	return true
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// render is the kept render of a screen, for the book being read (its direction), with the boxes of its controls.
func (p *player) render(screen string) (*boxes, string, error) {
	lang := "en"
	if p.b != nil {
		lang = p.b.lang
	}
	name, r := p.d.bestRenderIn(screen, lang)
	if r == nil {
		return nil, "", fmt.Errorf("no kept render of %s in %s", screen, lang)
	}
	return r, name, nil
}

func (p *player) step(a []string) (string, error) {
	d := p.d
	need := func(n int) error {
		if len(a)-1 < n {
			return fmt.Errorf("%s needs %d argument(s)", a[0], n)
		}
		return nil
	}
	ints := func(from int, n int) ([]int, error) {
		var out []int
		for i := from; i < from+n; i++ {
			v, err := strconv.Atoi(a[i])
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	switch a[0] {
	case "open":
		d.book = ""
		if len(a) > 1 {
			d.book = a[1]
		}
		stop, _, err := d.start()
		p.stop = stop
		if err != nil {
			return "", err
		}
		b, err := d.openFirst(p.lib)
		if err != nil {
			return "", err
		}
		b.report = func(good bool, what, detail string) {
			if !good {
				fmt.Printf("     %s: %s\n", what, detail)
			}
		}
		p.b = b
		return fmt.Sprintf("document %d page %d, reads %s", b.doc, b.page, map[bool]string{true: "right to left", false: "left to right"}[b.rtl]), nil
	case "close":
		if p.b == nil {
			return "", fmt.Errorf("no book is open")
		}
		if !p.b.open() {
			return "", fmt.Errorf("the bar and the dock did not come")
		}
		if err := d.tap(p.b.bar, "topbar-start"); err != nil {
			return "", err
		}
		if d.book != "" { // a book opened by name has no library to go back to: the reader ends
			if _, err := d.expect(`left at document`, 20*time.Second); err != nil {
				return "", err
			}
			p.b, p.stop = nil, nil
			return p.expectNickel(60)
		}
		if _, err := d.expect(`library: \d+ books.*shown in`, 20*time.Second); err != nil {
			return "", err
		}
		pause()
		p.b = nil
		if err := d.leave(p.lib, p.menu); err != nil {
			return "", err
		}
		p.stop = nil
		return "Nickel started again", nil
	case "tap":
		if err := need(1); err != nil {
			return "", err
		}
		if a[1] == "page-top" {
			return "", d.tapAt(d.prof.Screen.Width/2, 80)
		}
		if v, err := strconv.Atoi(a[1]); err == nil { // pixels
			xy, err := ints(1, 2)
			if err != nil || len(xy) != 2 {
				return "", fmt.Errorf("tap X Y")
			}
			_ = v
			return "", d.tapAt(xy[0], xy[1])
		}
		if err := need(2); err != nil {
			return "", err
		}
		r, name, err := p.render(a[1])
		if err != nil {
			return "", err
		}
		return name, p.tapNamed(r, a[2])
	case "turn":
		if err := need(1); err != nil || p.b == nil {
			return "", fmt.Errorf("turn forward|back, with a book open")
		}
		w := d.prof.Screen.Width
		x := w / 2 // forward: not the start quarter
		if a[1] == "back" {
			x = w / 10
			if p.b.rtl {
				x = w * 9 / 10
			}
		}
		return "", d.tapAt(x, 900)
	case "tap-menu":
		if err := need(1); err != nil || !p.hlSeen {
			return "", fmt.Errorf("tap-menu CONTROL, after the highlight's menu opened")
		}
		r, err := d.boxes("highlight-menu-new-" + p.b.lang)
		if err != nil {
			return "", err
		}
		if _, ok := box(r, a[1]); !ok { // the menu for a highlight held has Remove
			if r, err = d.boxes("highlight-menu-made-" + p.b.lang); err != nil {
				return "", err
			}
		}
		bx, ok := box(r, a[1])
		if !ok {
			return "", fmt.Errorf("no control %q in the highlight's menu", a[1])
		}
		return "", d.tapAt(p.hlX+(bx[0]+bx[2])/2, p.hlY+(bx[1]+bx[3])/2)
	case "swipe":
		return "", fmt.Errorf("swipe is not in this tool yet")
	case "hold":
		if err := need(2); err != nil {
			return "", err
		}
		xy, err := ints(1, 2)
		if err != nil {
			return "", err
		}
		x2, y2 := xy[0], xy[1]
		if len(a) == 6 && a[3] == "to" {
			t, err := ints(4, 2)
			if err != nil {
				return "", err
			}
			x2, y2 = t[0], t[1]
		}
		return "", d.holdAt(xy[0], xy[1], x2, y2)
	case "expect":
		if err := need(1); err != nil {
			return "", err
		}
		line, err := d.expect(strings.Join(a[1:], " "), 10*time.Second)
		if err != nil {
			return "", err
		}
		if m := reMenuAt.FindStringSubmatch(line); m != nil {
			p.hlX, _ = strconv.Atoi(m[1])
			p.hlY, _ = strconv.Atoi(m[2])
			p.hlSeen = true
		}
		return line, nil
	case "fetch":
		if err := need(3); err != nil {
			return "", err
		}
		return d.fetch(a[1], a[2], a[3])
	case "forget":
		if err := need(1); err != nil {
			return "", err
		}
		out, err := d.sh("cd "+d.a.Dir+" && f="+a[1]+" && s=$(stat -c %s $f) && k=$({ echo $s; head -c 65536 $f; tail -c 65536 $f; } | sha256sum | cut -c1-12) && rm -f places/$k.place places/$k.mark && echo $k", nil)
		if err != nil {
			return "", err
		}
		return "place of " + strings.TrimSpace(string(out)) + " removed", nil
	case "env":
		d.env = strings.Join(a[1:], " ")
		return "", nil
	case "freeze":
		out, err := d.sh("P=$(pidof shamsaink) && kill -STOP $P && echo $P", nil)
		if err != nil {
			return "", err
		}
		return "reader " + strings.TrimSpace(string(out)) + " stopped", nil
	case "expect-none":
		if err := need(2); err != nil {
			return "", err
		}
		s, _ := strconv.ParseFloat(a[len(a)-1], 64)
		re := regexp.MustCompile(strings.Join(a[1:len(a)-1], " "))
		time.Sleep(time.Duration(s * float64(time.Second)))
		out, err := d.sh(fmt.Sprintf("tail -n +%d %s", d.mark+1, d.a.Log), nil)
		if err != nil {
			return "", err
		}
		for _, l := range strings.Split(string(out), "\n") {
			if re.MatchString(l) {
				return "", fmt.Errorf("a line like %q is in the log: %s", re, l)
			}
		}
		return "none in " + a[len(a)-1] + " s", nil
	case "expect-nickel":
		if err := need(1); err != nil {
			return "", err
		}
		s, _ := strconv.ParseFloat(a[1], 64)
		return p.expectNickel(s)
	case "expect-in":
		if err := need(2); err != nil {
			return "", err
		}
		secs, _ := strconv.ParseFloat(a[1], 64)
		line, err := d.expect(strings.Join(a[2:], " "), time.Duration(secs*float64(time.Second)))
		if err != nil {
			return "", err
		}
		return line, nil
	case "wait":
		if err := need(1); err != nil {
			return "", err
		}
		s, _ := strconv.ParseFloat(a[1], 64)
		time.Sleep(time.Duration(s * float64(time.Second)))
		return "", nil
	case "snap":
		if err := need(1); err != nil {
			return "", err
		}
		return p.snap(a[1])
	}
	return "", fmt.Errorf("unknown step %q", a[0])
}

// tapNamed taps a control of a kept render; a list's row beyond those the render has (e7 of e0 to e3) is the first row's box
// moved down by the rows' pitch.
func (p *player) tapNamed(r *boxes, id string) error {
	if _, ok := box(r, id); ok {
		return p.d.tap(r, id)
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(id, "e")); err == nil && strings.HasPrefix(id, "e") {
		e0, ok0 := box(r, "e0")
		e1, ok1 := box(r, "e1")
		if ok0 && ok1 {
			pitch := e1[1] - e0[1]
			return p.d.tapAt((e0[0]+e0[2])/2, (e0[1]+e0[3])/2+n*pitch)
		}
	}
	return fmt.Errorf("no control %q in the kept render", id)
}

// pgm is a grey picture as the P5 file.
func pgm(w, h int, pix []byte) []byte {
	return append([]byte(fmt.Sprintf("P5 %d %d 255\n", w, h)), pix...)
}

func (p *player) snap(name string) (string, error) {
	pause()
	s, err := p.d.screen()
	if err != nil {
		return "", err
	}
	data := pgm(s.w, s.h, s.pix)
	if err := os.WriteFile(filepath.Join(p.out, name+".pgm"), data, 0o644); err != nil {
		return "", err
	}
	ref := filepath.Join(p.refs, name+".pgm")
	switch {
	case p.save:
		return "kept as the reference", os.WriteFile(ref, data, 0o644)
	}
	old, err := os.ReadFile(ref)
	if err != nil {
		return "no reference (run with --save)", nil
	}
	if len(old) != len(data) {
		return "", fmt.Errorf("the reference is another size")
	}
	skip := len(data) - len(s.pix) + topRows*s.w
	diff := 0
	for i := skip; i < len(data); i++ {
		if data[i] != old[i] {
			diff++
		}
	}
	if diff > 0 {
		return "", fmt.Errorf("%d pixels differ from the reference, below the clock's strip", diff)
	}
	return "identical to the reference", nil
}

// expectNickel waits for Kobo's interface, the reader gone (SSH may drop and come back meanwhile).
func (p *player) expectNickel(seconds float64) (string, error) {
	end := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	for time.Now().Before(end) {
		out, err := p.d.sh("pidof nickel >/dev/null && ! pidof shamsaink >/dev/null && echo yes", nil)
		if err == nil && strings.TrimSpace(string(out)) == "yes" {
			p.stop, p.b = nil, nil
			return "Kobo's interface is back, the reader gone", nil
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("Kobo's interface did not come back in %.0f s", seconds)
}

// fetch puts the file at url on the device at path (under the device's folder) unless it is there: downloaded here,
// checked against its SHA-256, copied over SSH.
func (d *dev) fetch(url, path, sum string) (string, error) {
	full := d.a.Dir + "/" + path
	if out, _ := d.sh("test -f "+full+" && echo yes", nil); strings.TrimSpace(string(out)) == "yes" {
		return path + " is there", nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != sum {
		return "", fmt.Errorf("%s: SHA-256 is %s, not %s", url, got, sum)
	}
	if _, err := d.sh("mkdir -p $(dirname "+full+") && cat > "+full, data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s copied (%d bytes)", path, len(data)), nil
}
