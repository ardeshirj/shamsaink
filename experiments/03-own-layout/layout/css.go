package layout

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// A small CSS reader: enough for the markup and stylesheets rtlbook makes (type, class and id selectors,
// descendant and child combinators, @font-face), not a general one. Unknown rules and properties are skipped.

type decl struct{ prop, value string }

type rule struct {
	sel   []compound // rightmost last
	comb  []byte     // comb[i] joins sel[i] and sel[i+1]: ' ' descendant, '>' child
	spec  int
	order int
	decls []decl
}

type compound struct {
	tag     string // "" or "*": any
	id      string
	classes []string
	attrs   [][2]string // [name, value]; value "" means present
}

type fontFace struct {
	family string
	bold   bool
	italic bool
	src    string // url, as written
}

type Sheet struct {
	rules []rule
	faces []fontFace
	base  string // the stylesheet's own path, for its urls
}

func stripComments(s string) string {
	for {
		a := strings.Index(s, "/*")
		if a < 0 {
			return s
		}
		b := strings.Index(s[a+2:], "*/")
		if b < 0 {
			return s[:a]
		}
		s = s[:a] + " " + s[a+2+b+2:]
	}
}

// block returns the text between the brace at s[i] and its match, and the index after it.
func block(s string, i int) (string, int) {
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[i+1 : j], j + 1
			}
		}
	}
	return s[i+1:], len(s)
}

func parseDecls(s string) []decl {
	var ds []decl
	for _, part := range strings.Split(s, ";") {
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		ds = append(ds, decl{strings.ToLower(strings.TrimSpace(k)), v})
	}
	return ds
}

// ParseSheet reads a stylesheet; order continues the numbering of earlier sheets so later rules win ties.
func ParseSheet(src, base string, order int) *Sheet {
	sh := &Sheet{base: base}
	s := stripComments(src)
	for i := 0; i < len(s); {
		b := strings.IndexByte(s[i:], '{')
		semi := strings.IndexByte(s[i:], ';')
		if b < 0 {
			break
		}
		head := strings.TrimSpace(s[i : i+b])
		if strings.HasPrefix(head, "@") && semi >= 0 && semi < b { // @import, @charset, @namespace
			i += semi + 1
			continue
		}
		body, next := block(s, i+b)
		i = next
		switch {
		case strings.HasPrefix(head, "@font-face"):
			ff := fontFace{}
			for _, d := range parseDecls(body) {
				switch d.prop {
				case "font-family":
					ff.family = unquote(d.value)
				case "font-weight":
					ff.bold = isBold(d.value, false)
				case "font-style":
					ff.italic = d.value == "italic" || d.value == "oblique"
				case "src":
					if a := strings.Index(d.value, "url("); a >= 0 {
						if z := strings.IndexByte(d.value[a:], ')'); z > 0 {
							ff.src = unquote(d.value[a+4 : a+z])
						}
					}
				}
			}
			sh.faces = append(sh.faces, ff)
		case strings.HasPrefix(head, "@"): // @media, @page, @supports: not read
		default:
			ds := parseDecls(body)
			for _, sel := range strings.Split(head, ",") {
				if r, ok := parseSelector(strings.TrimSpace(sel)); ok {
					r.decls, r.order = ds, order
					order++
					sh.rules = append(sh.rules, r)
				}
			}
		}
	}
	return sh
}

func unquote(s string) string { return strings.Trim(strings.TrimSpace(s), `"'`) }

func parseSelector(s string) (rule, bool) {
	var r rule
	s = strings.ReplaceAll(s, ">", " > ")
	pendingChild := false
	for _, tok := range strings.Fields(s) {
		if tok == ">" {
			pendingChild = true
			continue
		}
		c, ok := parseCompound(tok)
		if !ok {
			return r, false
		}
		if len(r.sel) > 0 {
			if pendingChild {
				r.comb = append(r.comb, '>')
			} else {
				r.comb = append(r.comb, ' ')
			}
		}
		pendingChild = false
		r.sel = append(r.sel, c)
		r.spec += c.specificity()
	}
	return r, len(r.sel) > 0
}

func parseCompound(t string) (compound, bool) {
	var c compound
	i := 0
	for i < len(t) && t[i] != '.' && t[i] != '#' && t[i] != '[' && t[i] != ':' {
		i++
	}
	c.tag = strings.ToLower(t[:i])
	for i < len(t) {
		switch t[i] {
		case '.', '#':
			j := i + 1
			for j < len(t) && t[j] != '.' && t[j] != '#' && t[j] != '[' && t[j] != ':' {
				j++
			}
			if t[i] == '.' {
				c.classes = append(c.classes, t[i+1:j])
			} else {
				c.id = t[i+1 : j]
			}
			i = j
		case '[':
			j := strings.IndexByte(t[i:], ']')
			if j < 0 {
				return c, false
			}
			k, v, _ := strings.Cut(t[i+1:i+j], "=")
			c.attrs = append(c.attrs, [2]string{strings.TrimSpace(k), unquote(v)})
			i += j + 1
		default: // pseudo-classes and elements: not supported, so the rule never matches
			return c, false
		}
	}
	return c, true
}

func (c compound) specificity() int {
	s := len(c.classes)*100 + len(c.attrs)*100
	if c.id != "" {
		s += 10000
	}
	if c.tag != "" && c.tag != "*" {
		s++
	}
	return s
}

func attr(n *html.Node, name string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == name || (a.Namespace == "" && strings.HasSuffix(a.Key, ":"+name)) {
			return a.Val, true
		}
	}
	return "", false
}

func (c compound) matches(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	if c.tag != "" && c.tag != "*" && c.tag != n.Data {
		return false
	}
	if c.id != "" {
		if v, _ := attr(n, "id"); v != c.id {
			return false
		}
	}
	if len(c.classes) > 0 {
		v, _ := attr(n, "class")
		have := strings.Fields(v)
		for _, want := range c.classes {
			found := false
			for _, h := range have {
				found = found || h == want
			}
			if !found {
				return false
			}
		}
	}
	for _, a := range c.attrs {
		v, ok := attr(n, a[0])
		if !ok || (a[1] != "" && v != a[1]) {
			return false
		}
	}
	return true
}

func (r *rule) matches(n *html.Node) bool { return r.matchAt(n, len(r.sel)-1) }

func (r *rule) matchAt(n *html.Node, i int) bool {
	if !r.sel[i].matches(n) {
		return false
	}
	if i == 0 {
		return true
	}
	if r.comb[i-1] == '>' {
		return r.matchAt(n.Parent, i-1)
	}
	for p := n.Parent; p != nil; p = p.Parent {
		if r.matchAt(p, i-1) {
			return true
		}
	}
	return false
}

// uaCSS is the part of a browser's default stylesheet that matters for these books (after Chromium's html.css),
// plus two e-reader conventions crengine also has: no page break right after a heading or inside it.
const uaCSS = `
html, body, div, p, h1, h2, h3, h4, h5, h6, nav, section, article, aside, header, footer, main, blockquote, figure,
figcaption, ol, ul, li, dl, dt, dd, address, hr, table, tr, center, pre { display: block }
head, style, script, title, meta, link { display: none }
body { margin: 8px }
p { margin: 1em 0 }
h1 { font-size: 2em; margin: 0.67em 0; font-weight: bold }
h2 { font-size: 1.5em; margin: 0.83em 0; font-weight: bold }
h3 { font-size: 1.17em; margin: 1em 0; font-weight: bold }
h4 { margin: 1.33em 0; font-weight: bold }
h5 { font-size: 0.83em; margin: 1.67em 0; font-weight: bold }
h6 { font-size: 0.67em; margin: 2.33em 0; font-weight: bold }
h1, h2, h3, h4, h5, h6 { page-break-after: avoid; page-break-inside: avoid }
blockquote, figure { margin: 1em 40px }
ol, ul { margin: 1em 0; padding-right: 40px }
b, strong { font-weight: bold }
i, em, cite, var { font-style: italic }
center { text-align: center }
`

// Style is an element's computed style, lengths in px.
type Style struct {
	Display      string // block, inline, none
	Family       []string
	Bold, Italic bool
	Size         float64
	LineHeight   float64 // a multiple of Size when lineFactor, else px
	lineFactor   bool
	Align        string // start, end, left, right, center, justify
	Indent       float64
	RTL          bool
	Lang         string
	Margin       [4]length // top right bottom left
	Padding      [4]length
	BreakBefore  string // auto, always, avoid
	BreakAfter   string
	BreakInside  string
}

// A length that may be a percentage of the containing block's width, resolved at layout.
type length struct {
	v   float64
	pct bool
}

func (l length) px(cbWidth float64) float64 {
	if l.pct {
		return l.v * cbWidth / 100
	}
	return l.v
}

// LineHeightPx is the line box height for this style.
func (s *Style) LineHeightPx() float64 {
	if s.lineFactor {
		return s.LineHeight * s.Size
	}
	return s.LineHeight
}

func rootStyle(sizePx float64) *Style {
	return &Style{Display: "block", Family: []string{"serif"}, Size: sizePx, LineHeight: 1.2, lineFactor: true,
		Align: "start", Lang: "", BreakBefore: "auto", BreakAfter: "auto", BreakInside: "auto"}
}

func isBold(v string, parent bool) bool {
	switch v {
	case "bold", "bolder":
		return true
	case "normal", "lighter":
		return false
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n >= 600
	}
	return parent
}

// parseLength reads px, em, rem, %, pt and 0; ok is false for anything else (auto, calc() ...).
func parseLength(v string, em, rem float64) (length, bool) {
	v = strings.TrimSpace(v)
	f := func(suffix string) (float64, bool) {
		n, err := strconv.ParseFloat(strings.TrimSuffix(v, suffix), 64)
		return n, err == nil
	}
	switch {
	case v == "0":
		return length{}, true
	case strings.HasSuffix(v, "rem"):
		n, ok := f("rem")
		return length{v: n * rem}, ok
	case strings.HasSuffix(v, "em"):
		n, ok := f("em")
		return length{v: n * em}, ok
	case strings.HasSuffix(v, "px"):
		n, ok := f("px")
		return length{v: n}, ok
	case strings.HasSuffix(v, "pt"):
		n, ok := f("pt")
		return length{v: n * 96 / 72}, ok
	case strings.HasSuffix(v, "%"):
		n, ok := f("%")
		return length{v: n, pct: true}, ok
	}
	return length{}, false
}

// box sets a margin or padding shorthand (1 to 4 values) or one of its sides.
func box(dst *[4]length, prop, v string, em, rem float64) {
	sides := map[string]int{"top": 0, "right": 1, "bottom": 2, "left": 3}
	if _, side, ok := strings.Cut(prop, "-"); ok {
		if i, ok := sides[side]; ok {
			if l, ok := parseLength(v, em, rem); ok {
				dst[i] = l
			}
		}
		return
	}
	var ls []length
	for _, f := range strings.Fields(v) {
		l, _ := parseLength(f, em, rem) // auto counts as 0
		ls = append(ls, l)
	}
	switch len(ls) {
	case 1:
		*dst = [4]length{ls[0], ls[0], ls[0], ls[0]}
	case 2:
		*dst = [4]length{ls[0], ls[1], ls[0], ls[1]}
	case 3:
		*dst = [4]length{ls[0], ls[1], ls[2], ls[1]}
	case 4:
		*dst = [4]length{ls[0], ls[1], ls[2], ls[3]}
	}
}

func breakValue(v string) string {
	switch v {
	case "always", "page", "left", "right", "recto", "verso":
		return "always"
	case "avoid", "avoid-page":
		return "avoid"
	}
	return "auto"
}

// computeStyle cascades the matching rules of the sheets over n's inherited style.
func computeStyle(n *html.Node, parent *Style, sheets []*Sheet, rootSize float64) *Style {
	s := *parent
	s.Display, s.Margin, s.Padding = "inline", [4]length{}, [4]length{}
	s.BreakBefore, s.BreakAfter, s.BreakInside = "auto", "auto", "auto"
	if v, ok := attr(n, "lang"); ok {
		s.Lang = v
	}
	if v, ok := attr(n, "dir"); ok {
		s.RTL = strings.EqualFold(v, "rtl")
	}
	type hit struct {
		r     *rule
		sheet int
	}
	var hits []hit
	for si, sh := range sheets {
		for i := range sh.rules {
			if sh.rules[i].matches(n) {
				hits = append(hits, hit{&sh.rules[i], si})
			}
		}
	}
	// sheets in order (UA first), then specificity, then source order
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0; j-- {
			a, b := hits[j-1], hits[j]
			if a.sheet < b.sheet || (a.sheet == b.sheet && (a.r.spec < b.r.spec || (a.r.spec == b.r.spec && a.r.order <= b.r.order))) {
				break
			}
			hits[j-1], hits[j] = b, a
		}
	}
	var ds []decl
	for _, h := range hits {
		ds = append(ds, h.r.decls...)
	}
	if v, ok := attr(n, "style"); ok {
		ds = append(ds, parseDecls(v)...)
	}
	// font-size first: em lengths in the other properties depend on it
	for _, d := range ds {
		if d.prop == "font-size" {
			if l, ok := parseLength(d.value, parent.Size, rootSize); ok {
				if l.pct {
					s.Size = l.v * parent.Size / 100
				} else {
					s.Size = l.v
				}
			}
		}
	}
	for _, d := range ds {
		v := strings.ToLower(d.value)
		switch d.prop {
		case "display":
			s.Display = v
			if v == "list-item" || v == "table" || v == "table-row" || v == "flex" || v == "grid" {
				s.Display = "block"
			}
		case "font-family":
			s.Family = nil
			for _, f := range strings.Split(d.value, ",") {
				s.Family = append(s.Family, unquote(f))
			}
		case "font-weight":
			s.Bold = isBold(v, parent.Bold)
		case "font-style":
			s.Italic = v == "italic" || v == "oblique"
		case "line-height":
			if v == "normal" {
				s.LineHeight, s.lineFactor = 1.2, true
			} else if n, err := strconv.ParseFloat(v, 64); err == nil {
				s.LineHeight, s.lineFactor = n, true
			} else if l, ok := parseLength(v, s.Size, rootSize); ok {
				s.LineHeight, s.lineFactor = l.v, false
				if l.pct {
					s.LineHeight = l.v * s.Size / 100
				}
			}
		case "text-align":
			s.Align = v
		case "text-indent":
			if l, ok := parseLength(v, s.Size, rootSize); ok {
				s.Indent = l.v // percentages not supported
			}
		case "direction":
			s.RTL = v == "rtl"
		case "margin", "margin-top", "margin-right", "margin-bottom", "margin-left":
			box(&s.Margin, d.prop, v, s.Size, rootSize)
		case "padding", "padding-top", "padding-right", "padding-bottom", "padding-left":
			box(&s.Padding, d.prop, v, s.Size, rootSize)
		case "page-break-before", "break-before":
			s.BreakBefore = breakValue(v)
		case "page-break-after", "break-after":
			s.BreakAfter = breakValue(v)
		case "page-break-inside", "break-inside":
			s.BreakInside = breakValue(v)
		}
	}
	return &s
}
