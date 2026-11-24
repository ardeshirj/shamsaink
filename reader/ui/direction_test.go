package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"text/template/parse"
)

// Text in either direction (principles: The interface's frame, the content's direction; 2026-11-24): every value a
// template writes as text is either the interface's own words, which follow the interface (the page's dir), or content
// (a book's title, a feed's summary, a catalogue's name...), whose element or one round it says its direction: a bdi or
// dir="auto" (the engine's rule, from its own letters), the book's (dir="{{.Dir}}") or ours (textdir) where the engine's falls
// short. A value written in no element with a dir must be listed in interfaceWords with why it is the interface's: so a
// new field of content cannot slip in without its direction (as a Farsi book's English summary once read right to left).

// interfaceWords are the values written with no dir of their own, by "template element value", and why.
var interfaceWords = map[string]string{
	"battery.html div.level {{.Level}}":             "the battery's level, a number",
	"battery.html span.state {{.State}}":            "the battery's state, our words",
	"button span.b {{.Label}}":                      "a button's label, our words",
	"catalogue-book.html dt {{index . 0}}":          "a fact's name, our words",
	"catalogue-book.html span.b {{.Read}}":          "Read, our word",
	"catalogue-filters.html span.b {{.Show}}":       "Show n books, our words",
	"catalogue-filters.html span.clr {{.Clear}}":    "Clear all, our words",
	"catalogue-filters.html span.ld {{.}}":          "Loading…, our word",
	"catalogue-filters.html span.n {{.}}":           "a choice's count, a number",
	"catalogue-filters.html span.ttl {{.Title}}":    "Filter or Browse, our words",
	"catalogue-head.html span.c {{.}}":              "the filters' count, a number",
	"catalogue-head.html span.loading {{.Loading}}": "Loading…, our word",
	"chip span.n {{.}}":                             "a chip's count, a number",
	"choice span.b {{.Label}}":                      "a choice's label, our words",
	"dialog p {{.Text}}":                            "a dialog's sentence, our words",
	"dialog p.n {{.Note}}":                          "a dialog's note, our words",
	"dialog span {{.Text}}":                         "a dialog's sentence after its icon, our words",
	"dock.html span.b {{.Back}}":                    "Back, our word",
	"dock.html span.n {{.Page}}":                    "the page's number, a number",
	"dock.html span.t {{.Left}}":                    "the time left in the chapter, our words",
	"dropdown span.lgt {{.Label}}":                  "a dropdown's name, our words",
	"iconbutton span.i {{.Glyph}}":                  "an icon's drawing",
	"library-book-details.html dt {{index . 0}}":    "a fact's name, our words",
	"library-tiles.html span.act {{.Read}}":         "Read, our word",
	"library.html h2 {{.Title}}":                    "the empty library's heading, our words",
	"library.html p {{.After}}":                     "the empty library's hint, our words",
	"library.html p {{.Hint}}":                      "the empty library's hint, our words",
	"menuitem li.tick {{.Label}}":                   "a menu item's label (one with an icon), our words",
	"segments span.n {{.Count}}":                    "a segment's count, a number",
	"segments span.on {{.Label}}":                   "a segment's label, our words",
	"shelf-name.html b {{.Name}}":                   "the library's side, our words",
	"shelf-name.html span.about {{.About}}":         "what the side holds, our words",
	"slider span {{.Name}}":                         "a slider's name, our words",
	"slider span {{.Value}}":                        "a slider's value, a number",
	"stepper span.name {{.Name}}":                   "a stepper's name, our words",
	"stepper span.sub {{.}}":                        "a stepper's note, our words",
	"stepper span.v {{.Value}}":                     "a stepper's value, our words or a number",
	"switch span.name {{.Label}}":                   "a switch's name, our words",
	"switch span.sub {{.}}":                         "a switch's note, our words",
	"tabs span.on {{.Label}}":                       "a tab's label, our words",
	"topbar.html span.time {{.Time}}":               "the clock, a number",
}

// pageInItsWords are the pages laid out in the direction of what they show (head .Dir), and why; others whose
// head takes a .Dir are the interface's (the top bar, the pager: "ltr" or the interface's), their content checked.
var pageInItsWords = map[string]string{
	"sleep.html":    "a sleep screen in its words' language (sleep.go)",
	"contents.html": "a book's contents, in its direction (contents.go); each row its own (listrow, textdir)",
}

// textValues lists every value a template writes as text with no dir on its element or round it.
func textValues() []string {
	var out []string
	for _, t := range uiTemplates.Templates() {
		if t.Tree == nil || t.Tree.Root == nil {
			continue
		}
		var src strings.Builder
		var vals []string
		page := false
		emit(t.Tree.Root, &src, &vals, &page)
		s := src.String()
		// a page's style, opened in its head (components.html) and closed in the page: values in it are CSS
		if end, open := strings.Index(s, "</style>"), strings.Index(s, "<style"); end >= 0 && (open < 0 || end < open) {
			s = s[end+len("</style>"):]
		}
		if page && pageInItsWords[t.Name()] != "" {
			s = `<page dir="data">` + s
		}
		for _, v := range undirected(s, vals) {
			out = append(out, t.Name()+" "+v)
		}
	}
	sort.Strings(out)
	return out
}

// emit writes the template's HTML with each value written as text replaced by a mark (\x01n\x01, n its index in vals);
// a branch's both arms in turn (they open and close the same elements).
func emit(n parse.Node, src *strings.Builder, vals *[]string, page *bool) {
	switch n := n.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, c := range n.Nodes {
				emit(c, src, vals, page)
			}
		}
	case *parse.TextNode:
		src.Write(n.Text)
	case *parse.ActionNode:
		if len(n.Pipe.Decl) == 0 && isValue(n.Pipe) {
			fmt.Fprintf(src, "\x01%d\x01", len(*vals))
			*vals = append(*vals, n.String())
		}
	case *parse.TemplateNode:
		if n.Name == "head" && n.Pipe != nil {
			if f, ok := n.Pipe.Cmds[0].Args[0].(*parse.FieldNode); ok && f.String() == ".Dir" {
				*page = true // which pages' Dir is their content's is pageInItsWords' to say
			}
		}
	case *parse.IfNode:
		emit(n.List, src, vals, page)
		emit(n.ElseList, src, vals, page)
	case *parse.RangeNode:
		emit(n.List, src, vals, page)
		emit(n.ElseList, src, vals, page)
	case *parse.WithNode:
		emit(n.List, src, vals, page)
		emit(n.ElseList, src, vals, page)
	}
}

// isValue: the action writes a value of the data (a field, ., a variable, an element of a list), not the program's
// markup (icon, t's words, id...).
func isValue(p *parse.PipeNode) bool {
	c := p.Cmds[len(p.Cmds)-1]
	switch a := c.Args[0].(type) {
	case *parse.FieldNode, *parse.ChainNode, *parse.DotNode, *parse.VariableNode:
		return true
	case *parse.IdentifierNode:
		return a.Ident == "index"
	}
	return false
}

var (
	tagRe  = regexp.MustCompile(`^<(/?)([a-zA-Z0-9]+)([^>]*)>`)
	markRe = regexp.MustCompile("\x01([0-9]+)\x01")
	voids  = map[string]bool{"br": true, "img": true, "hr": true, "meta": true, "input": true, "link": true}
)

type openTag struct {
	name, class string
	dir         bool
}

// undirected walks src's elements and gives each value written as text in no element with a dir, as "element value".
func undirected(src string, vals []string) []string {
	var out []string
	var stack []openTag
	for i := 0; i < len(src); {
		if strings.HasPrefix(src[i:], "<!--") {
			if j := strings.Index(src[i:], "-->"); j >= 0 {
				i += j + 3
				continue
			}
		}
		if m := tagRe.FindStringSubmatch(src[i:]); m != nil {
			name := strings.ToLower(m[2])
			if m[1] == "/" {
				for k := len(stack) - 1; k >= 0; k-- {
					if stack[k].name == name {
						stack = stack[:k]
						break
					}
				}
			} else if !voids[name] && !strings.HasSuffix(m[3], "/") {
				class := ""
				if c := regexp.MustCompile(`class="([^"\x01]*)`).FindStringSubmatch(m[3]); c != nil {
					class = "." + strings.Fields(c[1] + " x")[0]
				}
				dir := name == "bdi" || strings.Contains(m[3], " dir=") || strings.Contains(m[3], "textdir") // a bdi is dir="auto" by default (HTML)
				stack = append(stack, openTag{name, class, dir})
			}
			i += len(m[0])
			continue
		}
		if m := markRe.FindStringSubmatch(src[i:]); m != nil && strings.HasPrefix(src[i:], m[0]) {
			var n int
			fmt.Sscan(m[1], &n)
			dir, in, el := false, false, "(top)"
			for k, t := range stack {
				dir = dir || t.dir
				in = in || t.name == "style" || t.name == "script"
				if k == len(stack)-1 {
					el = t.name + t.class
				}
			}
			if !dir && !in {
				out = append(out, el+" "+vals[n])
			}
			i += len(m[0])
			continue
		}
		i++
	}
	return out
}

// Every value written as text says its direction, or is listed as the interface's words with why.
func TestTextSaysItsDirection(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range textValues() {
		seen[v] = true
		if _, ok := interfaceWords[v]; !ok {
			t.Errorf("%s: content needs dir=\"auto\" (or the book's dir) on its element; the interface's words go in interfaceWords with why", v)
		}
	}
	for v := range interfaceWords {
		if !seen[v] {
			t.Errorf("interfaceWords lists %q, which no template writes any more", v)
		}
	}
}
