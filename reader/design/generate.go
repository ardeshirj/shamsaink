package design

import (
	"fmt"
	"regexp"
	"strings"
)

// The files the spec writes: the variables of ui.css, between its markers, and the panels' sizes as Go constants.
const (
	cssBegin = "/* generated:tokens */"
	cssEnd   = "/* end generated */"
)

var reGenerated = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(cssBegin) + `.*?` + regexp.QuoteMeta(cssEnd))

// TokensCSS is ui.css with the part between its generated markers written from the tokens.
func TokensCSS(css string, t Tokens) (string, error) {
	if !reGenerated.MatchString(css) {
		return "", fmt.Errorf("ui.css has no %s ... %s", cssBegin, cssEnd)
	}
	var b strings.Builder
	b.WriteString(cssBegin + "\n/* GENERATED from reader/design/spec/tokens.yaml by `make design`: change the values there. */\n:root {\n")
	w := 0
	for _, k := range t.Order {
		w = max(w, len(k.Name)+len(k.Value))
	}
	for _, k := range t.Order {
		line := "  " + k.Name + ": " + k.Value + ";"
		if k.Note != "" {
			line += strings.Repeat(" ", w-len(k.Name)-len(k.Value)+2) + "/* " + k.Note + " */"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("}\n" + cssEnd)
	return reGenerated.ReplaceAllLiteralString(css, b.String()), nil
}

// TokensGo is reader/ui/tokens_gen.go: the panels' sizes from the tokens.
func TokensGo(t Tokens) string {
	return fmt.Sprintf(`// Code generated from reader/design/spec/tokens.yaml by `+"`make design`"+`; DO NOT EDIT.

package ui

// The panels' base font size and the margin round their content, in px (the engine's FontSize and Margin).
const (
	panelFontPx   = %d
	panelMarginPx = %d
)
`, t.Panel.FontPx, t.Panel.MarginPx)
}
