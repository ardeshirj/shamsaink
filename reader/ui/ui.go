// Package ui draws the reader's panels: the bar, the dock and the Text sheet at the bottom, the panel for removing a highlight, the table of
// contents and the opening screen. Each is HTML (templates/) drawn by the engine; the reader (package main) hands
// this package plain data and gets pictures and the places of buttons back.
package ui

import (
	"embed"
	"fmt"
	"html/template"
	"image"
	"path"
	"strings"
	"time"
)

// The panels' HTML is in templates/: the shared rules (ui.css), the pieces (components.html) and a template for each
// panel. brand/ has the logos, icons/ the icon fonts and the names of their glyphs.
//
//go:embed templates brand icons
var files embed.FS

// The icon fonts by the file name the style sheet asks for, and where each is in files.
var iconFonts = map[string]string{"Phosphor.ttf": "icons/Phosphor.ttf", "Phosphor-Bold.ttf": "icons/Phosphor-Bold.ttf",
	"Phosphor-Fill.ttf": "icons/Phosphor-Fill.ttf"}

// readFor is the engine's read function for a panel: an icon font for a path ending in its name (the style sheet's
// @font-face), html for every other path.
func readFor(html string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if f, ok := iconFonts[path.Base(p)]; ok {
			return files.ReadFile(f)
		}
		return []byte(html), nil
	}
}

// glyphs are the icons' code points by name, from icons/codepoints.txt.
var glyphs = func() map[string]rune {
	b, err := files.ReadFile("icons/codepoints.txt")
	if err != nil {
		panic(err)
	}
	m := map[string]rune{}
	for _, l := range strings.Split(string(b), "\n") {
		var name string
		var cp rune
		if n, _ := fmt.Sscanf(l, "%s %x", &name, &cp); n == 2 {
			m[name] = cp
		}
	}
	return m
}()

// glyph is the HTML of the icon name's character; an unknown name panics, so a typo is found by the tests.
func glyph(name string) template.HTML {
	cp, ok := glyphs[name]
	if !ok {
		panic(fmt.Sprintf("ui: no icon %q", name))
	}
	return template.HTML(fmt.Sprintf("&#x%x;", cp))
}

// pointing says whether an icon is an arrow that points: it is mirrored in right to left.
func pointing(name string) bool {
	switch name {
	case "arrow-left", "caret-left", "caret-right", "sign-out":
		return true
	}
	return false
}

// An icon button, as the templates draw it; On: drawn filled; Flip: mirrored in right to left (an arrow that points).
type iconButton struct {
	ID       string
	Glyph    template.HTML
	On, Flip bool
}

var uiTemplates = template.Must(template.New("ui").Funcs(uiFuncs(nil)).ParseFS(files, "templates/*.html", "templates/*.css"))

// A button, as the templates draw it; On: ticked (for a choice).
type button struct {
	ID, Label string
	On        bool
	Count     string // shown after the label, smaller (segments)
	Dir       string // the label's own direction, "" for the page's (segments)
	Fill      bool   // drawn filled: the panel's main button
}

// A row with a minus, a value and a plus: the buttons are name-dec and name-inc.
type stepper struct {
	Name, Value string
	Dec, Inc    button
}

// uiFuncs are what the templates draw buttons with; each registers its button's id in ids.
func uiFuncs(ids *[]string) template.FuncMap {
	add := func(id string) string {
		if ids != nil {
			*ids = append(*ids, id)
		}
		return id
	}
	return template.FuncMap{
		"id":  add,
		"t":   word,
		"num": num,
		"icon": func(name string) template.HTML {
			if pointing(name) {
				return `<span class="i flip">` + glyph(name) + `</span>`
			}
			return `<span class="i">` + glyph(name) + `</span>`
		},
		"uilang": func() string { // the interface's language for a page's lang, "" for English (the default)
			if lang == "en" {
				return ""
			}
			return lang
		},
		"rtlui": RTL,
		"ibtn": func(id, name string, on bool) iconButton {
			return iconButton{ID: add(id), Glyph: glyph(name), On: on,
				Flip: pointing(name)}
		},
		"btn":  func(id, label string) button { return button{ID: add(id), Label: label} },
		"fill": func(id, label string) button { return button{ID: add(id), Label: label, Fill: true} },
		"pick": func(id, label string, on bool) button {
			return button{ID: add(id), Label: label, On: on}
		},
		"list": func(bs ...button) []button { return bs },
		"reg": func(bs []button) []button { // buttons made by the program: their ids registered
			for _, b := range bs {
				add(b.ID)
			}
			return bs
		},
		"step": func(name, id, value string) stepper {
			return stepper{name, value, button{ID: add(id + "-dec"), Label: "−"}, button{ID: add(id + "-inc"), Label: "+"}}
		},
	}
}

// render fills in the template name (a file of templates/) with data: the HTML, and the ids of the buttons it drew,
// which drawPanel looks for.
func render(name string, data any) (html string, ids []string) {
	t, err := uiTemplates.Clone()
	if err == nil {
		var b strings.Builder
		if err = t.Funcs(uiFuncs(&ids)).ExecuteTemplate(&b, name, data); err == nil {
			return b.String(), ids
		}
	}
	panic(fmt.Sprintf("ui/templates/%s: %v", name, err))
}

func union(rs []image.Rectangle) image.Rectangle {
	var u image.Rectangle
	for _, r := range rs {
		u = u.Union(r)
	}
	return u
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }
