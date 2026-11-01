package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"shamsainkreader/design"
)

// A step of the flow: the control tapped on a screen, and the screen it opens.
type step struct{ from, control, to string }

// bestRender is the kept render of a screen (in English) that has the most buttons, with its boxes: the variant with
// every entry (the menu of the library, not of a book).
func (d *dev) bestRender(screen string) (string, *boxes) { return d.bestRenderIn(screen, "en") }

// bestRenderIn is bestRender in a language ("en", or "fa" for a book that reads right to left).
func (d *dev) bestRenderIn(screen, lang string) (string, *boxes) {
	files, _ := filepath.Glob(filepath.Join("reader", "design", "golden", d.name, screen+"-*-"+lang+".json"))
	best, name := (*boxes)(nil), ""
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".json")
		if owner := strings.TrimPrefix(base, screen+"-"); strings.Count(owner, "-") > 1 {
			continue // a render of another screen whose name starts with this one's (menu-… and library-…)
		}
		r, err := d.boxes(base)
		if err != nil {
			continue
		}
		if best == nil || len(r.Boxes) > len(best.Boxes) {
			best, name = r, base
		}
	}
	return name, best
}

// paths are the ways the spec opens screens from the one the app starts at, a control at a time, through screens that
// have a kept render of where their buttons are (a panel hung by a row is skipped, a control that does something is not a
// step), at most two deep.
func (d *dev) paths(spec *design.Spec, entry string) [][]step {
	var out [][]step
	var walk func(at string, path []step)
	walk = func(at string, path []step) {
		sc := spec.Screens[at]
		for _, c := range sc.Controls {
			if c.Action != "goto" && c.Action != "replace" || c.Dynamic {
				continue
			}
			t := spec.Screens[c.To]
			if t == nil || t.Layer == "page" || t.Layer == "screen" {
				continue
			}
			if _, r := d.bestRender(c.To); r == nil || r.Placement != "screen" {
				continue
			}
			p := append(append([]step(nil), path...), step{at, c.ID, c.To})
			out = append(out, p)
			if len(p) < 2 {
				walk(c.To, p)
			}
		}
	}
	walk(entry, nil)
	return out
}

// differs is the share of the screen's pixels (under the bar's clock strip) that two readings of it do not share.
func differs(a, b *screen) float64 {
	n := 0
	for i := a.w * 140; i < len(a.pix); i++ {
		if d := int(a.pix[i]) - int(b.pix[i]); d > 40 || d < -40 {
			n++
		}
	}
	return float64(n) / float64(len(a.pix)-a.w*140)
}

// regionDiff is the share of the box's pixels that two readings of the screen do not share.
func regionDiff(a, b *screen, box [4]int) float64 {
	n, total := 0, 0
	for y := max(box[1], 0); y < min(box[3], a.h); y++ {
		for x := max(box[0], 0); x < min(box[2], a.w); x++ {
			total++
			if d := int(a.pix[y*a.w+x]) - int(b.pix[y*a.w+x]); d > 40 || d < -40 {
				n++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}

// flow launches the reader and walks the spec's flow from the library: each control that opens a panel is tapped by name,
// the panel's buttons are held against its kept render, the panel is dismissed by a tap on the bar's clock (nothing there to
// press), and the library must be as it was. Changes nothing the reader keeps.
func flow(name string) bool {
	d, err := load(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	spec, _ := design.Load()
	entry := ""
	for n, sc := range spec.Screens {
		if sc.Entry {
			entry = n
		}
	}
	lib, err := d.boxes("library-reading-en")
	if err != nil {
		fmt.Println("FAIL the kept boxes of the library:", err)
		return false
	}
	_, menu := d.bestRender("menu")
	ok := true
	report := func(good bool, what, detail string) {
		mark := "ok  "
		if !good {
			mark, ok = "FAIL", false
		}
		fmt.Printf("%s %s", mark, what)
		if detail != "" {
			fmt.Printf(": %s", detail)
		}
		fmt.Println()
	}
	stop, line, err := d.start()
	if stop != nil {
		defer stop()
	}
	if err != nil {
		report(false, "the library drawn", err.Error())
		return false
	}
	report(true, "the library drawn", line)
	base, err := d.screen()
	if err != nil {
		report(false, "the screen read", err.Error())
		return false
	}

	paths := d.paths(spec, entry)
	sort.SliceStable(paths, func(i, j int) bool { return len(paths[i]) < len(paths[j]) })
	for _, p := range paths {
		var label []string
		for _, s := range p {
			label = append(label, s.from+" ›"+s.control+"› "+s.to)
		}
		what := strings.Join(label, ", ")
		var shown *screen
		good := true
		for _, s := range p {
			fromName, from := d.bestRender(s.from)
			if s.from == entry {
				fromName, from = "library-reading-en", lib
			}
			if from == nil {
				report(false, what, "no kept render of "+s.from)
				good = false
				break
			}
			if err := d.tap(from, s.control); err != nil {
				report(false, what, fromName+": "+err.Error())
				good = false
				break
			}
			time.Sleep(2500 * time.Millisecond)
		}
		if !good {
			d.dismiss()
			continue
		}
		last := p[len(p)-1]
		toName, to := d.bestRender(last.to)
		if shown, err = d.screen(); err != nil {
			report(false, what, err.Error())
			continue
		}
		var missing []string
		for id, b := range to.Boxes {
			if n := shown.dark([4]int{b[0], b[1], b[2], b[3]}); n < 20 && !(id == "topbar-phones") {
				missing = append(missing, fmt.Sprintf("%s (%d dark pixels)", id, n))
			}
		}
		sort.Strings(missing)
		// and it is new: the screen changed inside the panel's area (not ink the library had there)
		area := [4]int{1 << 30, 1 << 30, 0, 0}
		for _, b := range to.Boxes {
			area = [4]int{min(area[0], b[0]), min(area[1], b[1]), max(area[2], b[2]), max(area[3], b[3])}
		}
		if changed := regionDiff(base, shown, area); changed < 0.02 {
			missing = append(missing, fmt.Sprintf("a change in its area (%.1f%% of it differs from the library)", 100*changed))
		}
		report(len(missing) == 0, what, fmt.Sprintf("%s: %d buttons held against %s%s", last.to, len(to.Boxes), toName,
			map[bool]string{true: "", false: "; missing: " + strings.Join(missing, ", ")}[len(missing) == 0]))
		// out: a tap on the bar's clock closes the panel, and the library is as it was
		d.dismiss()
		time.Sleep(2500 * time.Millisecond)
		after, err := d.screen()
		if err != nil {
			report(false, "back to the library", err.Error())
			continue
		}
		share := differs(base, after)
		report(share < 0.005, "  and back to the library", fmt.Sprintf("%.2f%% of the screen differs from before", 100*share))
	}
	if len(os.Args) > 3 && os.Args[3] == "book" || len(os.Args) > 3 && os.Args[3] == "all" {
		d.bookFlow(lib, report)
	}
	if err := d.leave(lib, menu); err != nil {
		report(false, "leave", err.Error())
	} else {
		report(true, "left, the device's own interface back", "")
	}
	return ok
}

// dismiss taps the bar's clock, which is no button: it closes a panel that is open and does nothing otherwise.
func (d *dev) dismiss() {
	d.tapAt(d.prof.Screen.Width/2, 80)
}
