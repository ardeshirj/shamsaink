package design

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Sources are the files of reader/ui the spec is held against.
type Sources struct {
	UI fs.FS // reader/ui: templates/, words/, icons/codepoints.txt, panel.go
}

var (
	reComment = regexp.MustCompile(`(?s)\{\{/\*.*?\*/\s*-?\}\}`)
	reRoot    = regexp.MustCompile(`(?s):root\s*\{(.*?)\}`)
	reVar     = regexp.MustCompile(`(--[\w-]+)\s*:\s*([^;/]+?)\s*;`)
	reSlide   = regexp.MustCompile(`\bslide\s+\([^)]*\)\s+"([^"]+)"\s+"[^"]*"\s+\.\w+\s+"([^"]+)"\s+"([^"]+)"`)
	reIcon    = regexp.MustCompile(`\bicon\s+"([^"]+)"|\bibtn\s+"[^"]+"\s+"([^"]+)"`)
	reWord    = regexp.MustCompile(`\bt\s+"([^"]+)"`)
	reRoom    = regexp.MustCompile(`const room = (\d+)`)
)

// roles are what a control is to someone using it, as accessibility and test tools name them: a test says
// "the menuitem Contents", not a position.
var roles = map[string]bool{"button": true, "slider": true, "tab": true, "menuitem": true, "switch": true, "listitem": true,
	"progressbar": true, "radio": true, "spinbutton": true}

func em(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "em"), 64)
	return v, err == nil && strings.HasSuffix(s, "em")
}

// Validate holds the spec against the templates, the stylesheet, the words and the icons, and says every way they
// disagree (none: it is in step). It needs no device and no engine.
func Validate(s *Spec, src Sources) []string {
	var bad []string
	f := func(format string, a ...any) { bad = append(bad, fmt.Sprintf(format, a...)) }
	read := func(name string) string {
		b, err := fs.ReadFile(src.UI, name)
		if err != nil {
			f("%v", err)
		}
		return string(b)
	}

	// what a device has and what a control requires are of the known capabilities
	for _, d := range s.Devices {
		for _, h := range d.Has {
			if !contains(Capabilities, h) {
				f("device %s has %q, which is not a capability (%v)", d.Name, h, Capabilities)
			}
		}
	}
	for _, sc := range s.Screens {
		for _, c := range sc.Controls {
			for _, r := range c.Requires {
				if !contains(Capabilities, r) {
					f("screen %s, control %s requires %q, which is not a capability (%v)", sc.Name, c.ID, r, Capabilities)
				}
			}
		}
	}

	// tokens against ui.css
	css := read("templates/ui.css")
	have := map[string]string{}
	if m := reRoot.FindStringSubmatch(css); m != nil {
		for _, v := range reVar.FindAllStringSubmatch(m[1], -1) {
			have[v[1]] = v[2]
		}
	}
	for k, v := range s.Tokens.CSS {
		if have[k] != v {
			f("token %s: spec %q, ui.css %q", k, v, have[k])
		}
	}
	for k := range have {
		if _, ok := s.Tokens.CSS[k]; !ok {
			f("ui.css has %s, which tokens.yaml lacks", k)
		}
	}

	// devices
	known := map[string]bool{}
	panelGo := read("panel.go")
	for _, d := range s.Devices {
		for _, w := range d.Waveforms {
			known[w] = true
		}
		if m := reRoom.FindStringSubmatch(panelGo); m == nil || m[1] != strconv.Itoa(d.Tap.SlackPx) {
			f("device %s: tap slack %d px, panel.go has %v", d.Name, d.Tap.SlackPx, m)
		}
	}

	// components
	comps := read("templates/components.html")
	names := sortedKeys(s.Components)
	for _, n := range names {
		c := s.Components[n]
		if c.Template != "" && !strings.Contains(comps, `{{define "`+c.Template+`"}}`) {
			f("component %s: no {{define %q}} in components.html", n, c.Template)
		}
		if !roles[c.Role] {
			f("component %s: role %q is not one of %s", n, c.Role, strings.Join(sortedKeys(roles), ", "))
		}
		if c.Refresh.Waveform == "" {
			f("component %s: no refresh waveform", n)
		}
		for _, d := range s.Devices {
			if c.Refresh.Waveform != "" && !contains(d.Waveforms, c.Refresh.Waveform) {
				f("component %s: waveform %s not on %s", n, c.Refresh.Waveform, d.Name)
			}
		}
		if c.CSS == nil {
			continue
		}
		rule := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(c.CSS.Selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
		if rule == nil {
			f("component %s: no rule %q in ui.css", n, c.CSS.Selector)
			continue
		}
		for _, p := range [][2]string{{"width", c.CSS.Width}, {"height", c.CSS.Height}} {
			m := regexp.MustCompile(`(?:^|[;\s])` + p[0] + `:\s*([^;]+);`).FindStringSubmatch(rule[1])
			if m == nil || strings.TrimSpace(m[1]) != p[1] {
				f("component %s: %s %s in the spec, ui.css has %v", n, p[0], p[1], m)
			}
		}
		// a finger's room: the box and the reader's slack round it, in mm on each device
		w, ok1 := em(c.CSS.Width)
		h, ok2 := em(c.CSS.Height)
		if ok1 && ok2 {
			for _, d := range s.Devices {
				px := min(w, h)*float64(s.Tokens.Panel.FontPx) + 2*float64(d.Tap.SlackPx)
				if mm := px / float64(d.Screen.PPI) * 25.4; mm < d.Tap.MinMM {
					f("component %s on %s: a target %.1f mm, at least %.1f wanted", n, d.Name, mm, d.Tap.MinMM)
				}
			}
		}
	}

	// screens and their flow
	layers := map[string]bool{}
	for _, l := range s.Tokens.Layers {
		layers[l] = true
	}
	icons := map[string]bool{}
	for _, l := range strings.Fields(read("icons/codepoints.txt")) {
		icons[l] = true
	}
	words := map[string]map[string]string{}
	for _, l := range []string{"en", "fa"} {
		words[l] = map[string]string{}
		decodeJSON(read("words/"+l+".json"), words[l])
	}
	entry := ""
	for _, n := range sortedKeys(s.Screens) {
		sc := s.Screens[n]
		if !layers[sc.Layer] {
			f("screen %s: layer %q is not one of the layers", n, sc.Layer)
		}
		if sc.Status != "built" && sc.Status != "planned" {
			f("screen %s: status %q (built or planned)", n, sc.Status)
		}
		if sc.Entry {
			if entry != "" {
				f("two entry screens: %s and %s", entry, n)
			}
			entry = n
		} else if sc.Dismiss == "" && !sc.Page {
			f("screen %s: no way out (dismiss)", n)
		}
		// the page buttons: a map of their own on each screen when any device has them
		for _, d := range s.Devices {
			if d.PageButtons && sc.Buttons == "" {
				f("screen %s: no page-button map, and %s has page buttons", n, d.Name)
			}
		}
		if sc.Buttons != "" {
			m, ok := s.Buttons[sc.Buttons]
			if !ok {
				f("screen %s: page-button map %q does not exist", n, sc.Buttons)
			}
			if ok && (m.Forward == "" || m.Back == "" || m.HoldForward == "" || m.HoldBack == "") {
				f("page-button map %s: a gesture does nothing; every one needs a line", sc.Buttons)
			}
		}
		// every way out lands on the page: a screen is left by a tap off it, or by a control that goes to reading
		if sc.Dismiss != "" && sc.Dismiss != "tap-outside" && sc.Dismiss != "automatic" {
			ok := false
			for _, c := range sc.Controls {
				ok = ok || c.ID == sc.Dismiss && c.Action == "goto" && (c.To == pageScreen(s) || enteredFrom(sc, c.To))
			}
			if !ok {
				f("screen %s: dismiss %q is neither tap-outside nor a control that goes to the page or back to where it came from", n, sc.Dismiss)
			}
		}
		// two levels at most: a panel does not open another
		if sc.Layer == "panel" || sc.Layer == "overlay" {
			for _, c := range sc.Controls {
				if t := s.Screens[c.To]; c.Action == "goto" && t != nil && (t.Layer == "panel" || t.Layer == "overlay") {
					f("screen %s: control %s opens %s, a panel from a panel", n, c.ID, c.To)
				}
			}
		}
		if !sc.Entry && len(sc.Enter) == 0 {
			f("screen %s: nothing enters it", n)
		}
		for _, c := range sc.Controls {
			comp := s.Components[c.Component]
			if comp == nil {
				f("screen %s, control %s: no component %q", n, c.ID, c.Component)
				continue
			}
			switch c.Action {
			case "goto":
				if s.Screens[c.To] == nil {
					f("screen %s, control %s: goes to %q, which is no screen", n, c.ID, c.To)
				}
			case "replace": // a panel swapped for another at the same level, in its place
				if t := s.Screens[c.To]; t == nil {
					f("screen %s, control %s: replaces itself with %q, which is no screen", n, c.ID, c.To)
				} else if t.Layer != sc.Layer {
					f("screen %s, control %s: replaces with %s, which is in another layer", n, c.ID, c.To)
				}
			case "act":
				if c.Does == "" {
					f("screen %s, control %s: acts, but does not say what", n, c.ID)
				}
			default:
				f("screen %s, control %s: action %q (goto, replace or act)", n, c.ID, c.Action)
			}
		}
		for _, e := range sc.Enter {
			if e.Gesture == "app-start" && e.From == "" { // the program starting: no screen before it
				continue
			}
			from := s.Screens[e.From]
			if from == nil {
				f("screen %s: entered from %q, which is no screen", n, e.From)
				continue
			}
			if e.Control == "" {
				continue
			}
			ok := false
			for _, c := range from.Controls {
				ok = ok || c.ID == e.Control && (c.Action == "goto" || c.Action == "replace") && c.To == n
			}
			if !ok {
				f("screen %s: entered by %s on %s, which does not go to it", n, e.Control, e.From)
			}
		}
		if sc.Status != "built" || len(sc.Templates) == 0 {
			continue
		}

		// the screen's templates against its controls, icons and words
		var tpl string
		for _, t := range sc.Templates {
			tpl += reComment.ReplaceAllString(read("templates/"+t), "")
		}
		for _, m := range reSlide.FindAllStringSubmatch(tpl, -1) {
			for _, ic := range m[2:] {
				if !icons[ic] {
					f("screen %s: icon %q is not in the icon font", n, ic)
				}
			}
		}
		for _, m := range reIcon.FindAllStringSubmatch(tpl, -1) {
			if ic := m[1] + m[2]; !icons[ic] {
				f("screen %s: icon %q is not in the icon font", n, ic)
			}
		}
		used := map[string]bool{}
		for _, w := range sc.Words {
			used[w] = true
		}
		for _, m := range reWord.FindAllStringSubmatch(tpl, -1) {
			used[m[1]] = true
		}
		for w := range used {
			for l, tab := range words {
				if _, ok := tab[w]; !ok {
					f("screen %s: word %q is not in the %s table", n, w, l)
				}
			}
		}
	}
	if entry == "" {
		f("no entry screen")
		return bad
	}
	if pageScreen(s) == "" {
		f("no page screen")
	}

	// every screen reachable from the entry, by a gesture on the screen before it
	reach := map[string]bool{entry: true}
	for grew := true; grew; {
		grew = false
		for n, sc := range s.Screens {
			for _, e := range sc.Enter {
				if (reach[e.From] || e.Gesture == "app-start" && e.From == "") && !reach[n] {
					reach[n], grew = true, true
				}
			}
		}
	}
	for _, n := range sortedKeys(s.Screens) {
		if !reach[n] {
			f("screen %s cannot be reached from %s", n, entry)
		}
	}
	return bad
}

// enteredFrom says whether sc can be entered from the screen named from.
func enteredFrom(sc *Screen, from string) bool {
	for _, e := range sc.Enter {
		if e.From == from {
			return true
		}
	}
	return false
}

// pageScreen is the name of the page screen, "" if there is none.
func pageScreen(s *Spec) string {
	for n, sc := range s.Screens {
		if sc.Page {
			return n
		}
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

// CheckControls holds what a screen's renders drew for a device (found: the ids of every button, over all its states) against the
// controls the spec declares for it: each declared control was drawn, and each button drawn is declared. A dynamic
// control is a prefix (a list's rows e0, e1, ...). It says every way they disagree.
func CheckControls(s *Spec, device, screen string, found []string) []string {
	var bad []string
	var has []string
	for _, d := range s.Devices {
		if d.Name == device {
			has = d.Has
		}
	}
	sc := s.Screens[screen]
	if sc == nil {
		return []string{fmt.Sprintf("no screen %q", screen)}
	}
	exact, prefixes := map[string]bool{}, []string{}
	for _, id := range sc.Internal {
		exact[id] = true
	}
	for _, c := range sc.Controls {
		comp := s.Components[c.Component]
		if comp == nil {
			continue
		}
		if !hasAll(has, c.Requires) { // the device lacks what it needs: not on this device, and so not drawn for it
			continue
		}
		if c.Dynamic {
			prefixes = append(prefixes, c.ID)
			continue
		}
		for _, suf := range comp.IDs {
			exact[c.ID+suf] = true
		}
	}
	got := map[string]bool{}
	for _, id := range found {
		got[id] = true
		ok := exact[id]
		for _, p := range prefixes {
			ok = ok || strings.HasPrefix(id, p)
		}
		if !ok {
			bad = append(bad, fmt.Sprintf("screen %s: drew %q, which the spec does not declare", screen, id))
		}
	}
	for id := range exact {
		if !got[id] && !contains(sc.Internal, id) {
			bad = append(bad, fmt.Sprintf("screen %s: the spec declares %q, which was not drawn", screen, id))
		}
	}
	for _, p := range prefixes {
		hit := false
		for id := range got {
			hit = hit || strings.HasPrefix(id, p)
		}
		if !hit {
			bad = append(bad, fmt.Sprintf("screen %s: the spec declares %s..., none of which was drawn", screen, p))
		}
	}
	sort.Strings(bad)
	return bad
}

// hasAll says whether have holds every one of need.
func hasAll(have, need []string) bool {
	for _, n := range need {
		if !contains(have, n) {
			return false
		}
	}
	return true
}
