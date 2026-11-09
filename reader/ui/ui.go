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
	"unicode"
)

// The panels' HTML is in templates/: the shared rules (ui.css), the pieces (components.html) and a template for each
// panel. brand/ has the logos, icons/ the icon fonts and the names of their glyphs.
//
//go:embed templates icons words
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
	On, Flip bool // filled (its panel open), mirrored in right to left
}

var uiTemplates = template.Must(template.New("ui").Funcs(uiFuncs(nil)).ParseFS(files, "templates/*.html", "templates/*.css"))

// A button, as the templates draw it; On: ticked (for a choice).
// dialog is the dialog component's (components.html): a question or a notice, an optional title and note, its buttons.
type dialog struct {
	ID, Text, Title, Dir, Note string
	Lang                       template.HTMLAttr
	Buttons                    []button
}

type button struct {
	ID, Label string
	On        bool
	Count     string // shown after the label, smaller (segments)
	Dir       string // the label's own direction, "" for the page's (segments)
}

// A slider: its name and value, its track (the id a tap on it gives, filled to Share per cent) between the buttons for
// less and more (id-dec and id-inc), each an icon (Less, More: their names).
type slider struct {
	Name, Value, Track, Share string
	Dec, Inc                  button
	Less, More                string
	Sized                     bool
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
		"iconfill": func(name string) template.HTML { // the icon in the fill weight: a solid shape, as when an icon is on
			return `<span class="i on">` + glyph(name) + `</span>`
		},
		"uilang": func() string { // the interface's language for a page's lang, "" for English (the default)
			if lang == "en" {
				return ""
			}
			return lang
		},
		"rtlui":    RTL,
		"textlang": textLang,
		"ibtn": func(id, name string, on bool) iconButton {
			return iconButton{ID: add(id), Glyph: glyph(name), On: on,
				Flip: pointing(name)}
		},
		"btn": func(id, label string) button { return button{ID: add(id), Label: label} },
		"dlg": func(id, text, title, dir, note string, buttons ...button) dialog { // the box's id is not a button's
			return dialog{ID: id, Text: text, Title: title, Dir: dir, Lang: textLang(title), Note: note, Buttons: buttons}
		},
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
		"slide": func(name, id, value string, share float64, less, more string, sized bool) slider {
			return slider{name, value, add(id), fmt.Sprintf("%.1f", 100*min(max(share, 0), 1)),
				button{ID: add(id + "-dec")}, button{ID: add(id + "-inc")}, less, more, sized}
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

// textLang is the lang attribute for a piece of text that is not the interface's (a book's title, a chapter's): " lang=\"fa\""
// when it is in the Arabic script, so it is marked as Farsi (its direction and line breaking; the face is the interface's,
// Vazirmatn, since 2026-11-04), else nothing.
func textLang(s string) template.HTMLAttr {
	for _, r := range s {
		if unicode.Is(unicode.Arabic, r) {
			return ` lang="fa"`
		}
	}
	return ""
}

func union(rs []image.Rectangle) image.Rectangle {
	var u image.Rectangle
	for _, r := range rs {
		u = u.Union(r)
	}
	return u
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }
