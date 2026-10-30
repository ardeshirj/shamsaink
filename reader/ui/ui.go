// Package ui draws the reader's panels: the menu, the panels at the bottom and for removing a highlight, the table of
// contents, the clock in the top margin and the opening screen. Each is HTML (templates/) drawn by the engine; the
// reader (package main) hands this package plain data and gets pictures and the places of buttons back.
package ui

import (
	"embed"
	"fmt"
	"html/template"
	"image"
	"strings"
	"time"
)

// The panels' HTML is in templates/: the shared rules (ui.css), the pieces (components.html) and a template for each
// panel. brand/ has the logos.
//
//go:embed templates brand
var files embed.FS

var uiTemplates = template.Must(template.New("ui").Funcs(uiFuncs(nil)).ParseFS(files, "templates/*.html", "templates/*.css"))

// A button, as the templates draw it; On: ticked (for a choice).
type button struct {
	ID, Label string
	On        bool
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
		"btn": func(id, label string) button { return button{ID: add(id), Label: label} },
		"pick": func(id, label string, on bool) button {
			return button{ID: add(id), Label: label, On: on}
		},
		"list": func(bs ...button) []button { return bs },
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
