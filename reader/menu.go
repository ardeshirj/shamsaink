package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	blitz "indiumengine"
)

// The settings, in DIR/settings.json: text size, line spacing and margins for every book, the page direction by book.
type settings struct {
	FontSize   float32           `json:"font_size"`   // px
	LineHeight float32           `json:"line_height"` // as CSS's unitless line-height over the book's own; 0: the book's
	Margin     int               `json:"margin"`      // px
	LeadMs     int               `json:"lead_ms"`     // read-along: the words underlined this much after the sound starts, for the headphones' own delay
	Direction  map[string]string `json:"direction,omitempty"` // by book (its key, as its cache's name): "rtl" or "ltr"
}

// defaults are the settings before any is set (the environment's, else the reader's own).
func defaults() *settings {
	f := func(s string) float32 { v, _ := strconv.ParseFloat(s, 32); return float32(v) }
	return &settings{FontSize: f(fontSize), LineHeight: f(lineHeight), Margin: atoi(margin), LeadMs: atoi(leadMs)}
}

func loadSettings(dir string) *settings {
	s := defaults()
	if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		json.Unmarshal(b, s)
	}
	return s
}

func (s *settings) save(dir string) {
	b, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(dir, "settings.json"), append(b, '\n'), 0o644)
}

func (s *settings) options(w, h int) blitz.Options {
	return blitz.Options{Width: w, Height: h, Margin: s.Margin, FontSize: s.FontSize, LineHeight: s.LineHeight}
}

// change applies a menu button's change (lines: the line spacing as laid out now); layout says whether the book must
// be laid out again.
func (s *settings) change(id, key string, lines float32) (layout bool) {
	step := func(v, by, lo, hi float32) float32 { return min(max(v+by, lo), hi) }
	switch id {
	case "size-dec", "size-inc":
		s.FontSize = step(s.FontSize, map[bool]float32{true: 2, false: -2}[id == "size-inc"], 30, 90)
	case "margin-dec", "margin-inc":
		s.Margin = int(step(float32(s.Margin), map[bool]float32{true: 10, false: -10}[id == "margin-inc"], 0, 200))
	case "lines-dec", "lines-inc":
		// from the spacing as it is (the book's own, measured, until one is set), by 0.05
		v := cmp.Or(s.LineHeight, lines, 1.2)
		v = step(v, map[bool]float32{true: 0.05, false: -0.05}[id == "lines-inc"], 0.8, 3)
		s.LineHeight = float32(math.Round(float64(v)*20) / 20)
	case "reset":
		// every setting back to its default (the book's line spacing), the direction too for this book
		d := defaults()
		s.FontSize, s.LineHeight, s.Margin = d.FontSize, d.LineHeight, d.Margin
		delete(s.Direction, key)
	case "dir-", "dir-rtl", "dir-ltr":
		if s.Direction == nil {
			s.Direction = map[string]string{}
		}
		if d := strings.TrimPrefix(id, "dir-"); d != "" {
			s.Direction[key] = d
		} else {
			delete(s.Direction, key)
		}
		return false
	default:
		return false
	}
	return true
}

// The menu: a panel over the top of the page, drawn by the engine from HTML (right to left, when the menu is in
// Farsi, for free), its buttons found by their ids.
type menu struct {
	img     *image.Gray
	buttons map[string]image.Rectangle
}

const menuHeight = 700 // px, the panel's

var menuButtons = []string{"leave", "reset", "size-dec", "size-inc", "lines-dec", "lines-inc", "margin-dec", "margin-inc",
	"dir-", "dir-rtl", "dir-ltr"}

// drawMenu draws the menu for the settings s and the book by key; lines is its line spacing as laid out now, narrated
// whether it can be read aloud.
func drawMenu(s *settings, key string, lines float32, narrated bool, fonts *blitz.Fonts, w int) *menu {
	// the line spacing as it is, and whether it is the book's own
	linesName := "Line spacing (the book's)"
	if s.LineHeight > 0 {
		lines, linesName = s.LineHeight, "Line spacing"
	}
	dir := s.Direction[key]
	aloud, ids := "", menuButtons
	if narrated {
		aloud, ids = ` <span class="b" id="read-aloud">Read aloud</span>`, append(ids, "read-aloud")
	}
	on := func(d string) string {
		if d == dir {
			return "✓ "
		}
		return ""
	}
	row := func(name, id, value string) string {
		return fmt.Sprintf(`<tr><td>%s</td><td class="n"><span class="b" id="%s-dec">−</span></td>`+
			`<td class="v">%s</td><td class="n"><span class="b" id="%s-inc">+</span></td></tr>`, name, id, value, id)
	}
	html := `<html><head><style>
html { font-family: sans-serif; }
body { margin: 0; }
table { width: 100%; border-collapse: collapse; }
td { padding: 0.3em 0; }
td.n { width: 2.4em; text-align: center; }
td.v { width: 6em; text-align: center; }
span.b { display: inline-block; border: 3px solid black; border-radius: 0.4em; padding: 0.05em 0.6em; }
p { margin: 0.3em 0; }
p span.b { margin-right: 0.5em; }
</style></head><body>
<p><span class="b" id="leave">Close the book</span> <span class="b" id="reset">Reset settings</span>` + aloud +
		`</p><table>` +
		row("Text size", "size", fmt.Sprintf("%g px", s.FontSize)) +
		row(linesName, "lines", fmt.Sprintf("%.2f", lines)) +
		row("Margins", "margin", fmt.Sprintf("%d px", s.Margin)) +
		`</table><p>Pages turn (this book):</p><p>` +
		`<span class="b" id="dir-">` + on("") + `As the book says</span> ` +
		`<span class="b" id="dir-rtl">` + on("rtl") + `Right to left</span> ` +
		`<span class="b" id="dir-ltr">` + on("ltr") + `Left to right</span></p></body></html>`

	return drawPanel(html, ids, fonts, w, menuHeight)
}

// The panel asking whether to remove a highlight, its text's start shown.
func drawRemove(text string, fonts *blitz.Fonts, w int) *menu {
	if r := []rune(text); len(r) > 80 {
		text = string(r[:80]) + "…"
	}
	html := `<html><head><style>
html { font-family: sans-serif; }
body { margin: 0; }
p { margin: 0.3em 0; }
p.t { unicode-bidi: plaintext; }
span.b { display: inline-block; border: 3px solid black; border-radius: 0.4em; padding: 0.05em 0.6em; margin-right: 0.5em; }
</style></head><body><p class="t">` + escape(text) + `</p>
<p><span class="b" id="remove-highlight">Remove highlight</span> <span class="b" id="keep-highlight">Keep</span></p></body></html>`
	return drawPanel(html, []string{"remove-highlight", "keep-highlight"}, fonts, w, 300)
}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// drawPanel draws a panel h px high over the top of the page from html, a line under it, its buttons by ids.
func drawPanel(html string, ids []string, fonts *blitz.Fonts, w, h int) *menu {
	o := blitz.Options{Width: w, Height: h, Margin: 40, FontSize: 40}
	b := blitz.New([]string{"menu.html"}, func(string) ([]byte, error) { return []byte(html), nil }, o, fonts)
	defer b.Close()
	b.Layout(0)
	m := &menu{img: image.NewGray(image.Rect(0, 0, w, h)), buttons: map[string]image.Rectangle{}}
	b.Draw(0, m.img)
	for y := h - 4; y < h; y++ { // a line under the panel
		for x := 0; x < w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 0
		}
	}
	for _, id := range ids {
		if p, rs := b.Find(0, id); p == 0 {
			var r image.Rectangle
			for _, q := range rs {
				r = r.Union(q)
			}
			m.buttons[id] = r
		}
	}
	return m
}

// at is the button tapped at (x, y), with some room around each for a finger; "" if none.
func (m *menu) at(x, y int) string {
	const room = 16
	for id, r := range m.buttons {
		if image.Pt(x, y).In(r.Inset(-room)) {
			return id
		}
	}
	return ""
}
