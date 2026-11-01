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
	reID      = regexp.MustCompile(`\b(?:id|btn|pick)\s+"([^"]+)"|\bibtn\s+"([^"]+)"`)
	reSlide   = regexp.MustCompile(`\bslide\s+\([^)]*\)\s+"([^"]+)"\s+"[^"]*"\s+\.\w+\s+"([^"]+)"\s+"([^"]+)"`)
	reIcon    = regexp.MustCompile(`\bicon\s+"([^"]+)"|\bibtn\s+"[^"]+"\s+"([^"]+)"`)
	reWord    = regexp.MustCompile(`\bt\s+"([^"]+)"`)
	reRoom    = regexp.MustCompile(`const room = (\d+)`)
)

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
		} else if sc.Dismiss == "" {
			f("screen %s: no way out (dismiss)", n)
		}
		if !sc.Entry && len(sc.Enter) == 0 {
			f("screen %s: nothing enters it", n)
		}
		declared := map[string]bool{}
		for _, id := range sc.Internal {
			declared[id] = true
		}
		for _, c := range sc.Controls {
			comp := s.Components[c.Component]
			if comp == nil {
				f("screen %s, control %s: no component %q", n, c.ID, c.Component)
				continue
			}
			for _, suf := range comp.IDs {
				declared[c.ID+suf] = true
			}
			switch c.Action {
			case "goto":
				if s.Screens[c.To] == nil {
					f("screen %s, control %s: goes to %q, which is no screen", n, c.ID, c.To)
				}
			case "act":
				if c.Does == "" {
					f("screen %s, control %s: acts, but does not say what", n, c.ID)
				}
			default:
				f("screen %s, control %s: action %q (goto or act)", n, c.ID, c.Action)
			}
		}
		for _, e := range sc.Enter {
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
				ok = ok || c.ID == e.Control && c.Action == "goto" && c.To == n
			}
			if !ok {
				f("screen %s: entered by %s on %s, which does not go to it", n, e.Control, e.From)
			}
		}
		if sc.Status != "built" || sc.Template == "" {
			continue
		}

		// the screen's template against its controls, icons and words
		tpl := reComment.ReplaceAllString(read("templates/"+sc.Template), "")
		seen := map[string]bool{}
		for _, m := range reID.FindAllStringSubmatch(tpl, -1) {
			seen[m[1]+m[2]] = true
		}
		for _, m := range reSlide.FindAllStringSubmatch(tpl, -1) {
			for _, suf := range s.Components["slider"].IDs {
				seen[m[1]+suf] = true
			}
			for _, ic := range m[2:] {
				if !icons[ic] {
					f("screen %s: icon %q is not in the icon font", n, ic)
				}
			}
		}
		for id := range seen {
			if !declared[id] {
				f("screen %s: the template makes %q, which the spec does not declare", n, id)
			}
		}
		for id := range declared {
			if !seen[id] {
				f("screen %s: the spec declares %q, which the template does not make", n, id)
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

	// every screen reachable from the entry, by a gesture on the screen before it
	reach := map[string]bool{entry: true}
	for grew := true; grew; {
		grew = false
		for n, sc := range s.Screens {
			for _, e := range sc.Enter {
				if reach[e.From] && !reach[n] {
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
