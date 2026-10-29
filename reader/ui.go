package main

import (
	"embed"
	"fmt"
	"html/template"
	"strings"
)

// The panels' HTML is in ui/: the shared rules (ui.css), the pieces (components.html) and a template for each panel.
//
//go:embed ui
var uiFiles embed.FS

var uiTemplates = template.Must(template.New("ui").Funcs(uiFuncs(nil)).ParseFS(uiFiles, "ui/*.html", "ui/*.css"))

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

// render fills in the template name (a file of ui/) with data: the HTML, and the ids of the buttons it drew, which
// drawPanel looks for.
func render(name string, data any) (html string, ids []string) {
	t, err := uiTemplates.Clone()
	if err == nil {
		var b strings.Builder
		if err = t.Funcs(uiFuncs(&ids)).ExecuteTemplate(&b, name, data); err == nil {
			return b.String(), ids
		}
	}
	panic(fmt.Sprintf("ui/%s: %v", name, err))
}
