package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	blitz "shamsainkengine"

	"shamsainkreader/design"
)

// componentRenders has, for each component of the spec drawn from components.html, its samples: the states shown on
// the design page (2026-11-23), each drawn alone from templates/samples.html, in English and Farsi (the components mirror
// themselves in right to left). Kept in reader/design/golden/DEVICE/components/, compared by make test as the screens'.
var componentRenders = map[string][]string{
	"button":      {"plain", "icon"},
	"dialog":      {"question", "notice"},
	"icon-button": {"off", "on", "pointing"},
	"segments":    {"first", "second"},
	"tabs":        {"first", "second"},
	"toggle":      {"off", "on", "row", "row-about"},
	"stepper":     {"value", "about"},
	"slider":      {"value"},
	"menu-item":   {"actions", "choices"},
	"bar":         {"share"},
	"list-row":    {"off", "current"},
	"dropdown":    {"plain", "open", "labelled", "labelled-open", "long"},
}

// sampleText is what the samples say that is no word of the reader's in both languages: the library's are English only
// (a catalogue, Settings' lines), and the values are invented. In Farsi, Farsi.
var sampleText = map[string]map[string]string{
	"en": {"dd-label": "Catalogue", "dd-value": "Standard Ebooks", "dd-short": "FarsiEPUB", "dd-long": "The Very Large Pu…",
		"step-about": "Minutes untouched before the reader sleeps", "bar-share": "42", "bar-side": "left", "switch-about": "Menus in a book's language; off: always English"},
	"fa": {"dd-label": "کاتالوگ", "dd-value": "کتاب‌های فارسی", "dd-short": "فارسی", "dd-long": "کتابخانه‌ی بسیار بزرگ آثار…",
		"step-about": "چند دقیقه دست‌نخورده، پیش از آن‌که دستگاه بخوابد", "bar-share": "42", "bar-side": "right", "switch-about": "منوها به زبان کتاب؛ خاموش: همیشه انگلیسی"},
}

// sampleRows are a contents' entries, invented, a part and its chapters (indented), in each language.
var sampleRows = map[string]map[string][]listRow{
	"en": {"rows": {{Title: "Part one", Pad: "0.3"}, {Title: "The lighthouse", Pad: "1.8"}, {Title: "Letters from the coast", Pad: "1.8"}, {Title: "Part two", Pad: "0.3"}}},
	"fa": {"rows": {{Title: "بخش یکم", Pad: "0.3"}, {Title: "فانوس دریایی", Pad: "1.8"}, {Title: "نامه‌هایی از ساحل", Pad: "1.8"}, {Title: "بخش دوم", Pad: "0.3"}}},
}

// drawSample draws a component's sample: the samples page cropped to its box.
func drawSample(name, c string, f *blitz.Fonts, w int) *Panel {
	html, ids := render("samples.html", struct {
		Name, Case string
		S          map[string]string
		T          map[string][]string
		R          map[string][]listRow
	}{name, c, sampleText[Language()], map[string][]string{"ticks": {"12", "30", "55", "81"}}, sampleRows[Language()]})
	return drawPanelIn(html, ids, "sample", f, w, 1000)
}

// Every component drawn from a shared template has samples, and every sample is of a component of the spec.
func TestEveryComponentIsRendered(t *testing.T) {
	s, err := design.Load()
	if err != nil {
		t.Fatal(err)
	}
	for n, c := range s.Components {
		if _, has := componentRenders[n]; c.Template != "" && !has {
			t.Errorf("component %s has a template and no samples in componentRenders", n)
		}
	}
	for n := range componentRenders {
		if s.Components[n] == nil {
			t.Errorf("samples of %s, which is no component of the spec", n)
		}
	}
}

// Every sample, per device and language, drawn and compared with the one kept (as TestScreenRenders).
func TestComponentRenders(t *testing.T) {
	dir := os.Getenv("FONTS")
	if dir == "" {
		t.Skip("FONTS not set: renders are compared with the fonts of reader/fonts")
	}
	fonts := blitz.LoadFonts(dir)
	s, err := design.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer SetLanguage("en")
	update := os.Getenv("GOLDEN") == "update"
	var names []string
	for n := range componentRenders {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, d := range s.Devices {
		for _, n := range names {
			for _, c := range componentRenders[n] {
				for _, lang := range enFa {
					setRenderLanguage(lang)
					name := fmt.Sprintf("%s-%s-%s", n, c, lang)
					p := drawSample(n, c, fonts, d.Screen.Width)
					compareGolden(t, filepath.Join("..", "design", "golden", d.Name, "components", name+".png"), name, p.img, update)
				}
			}
		}
	}
}
