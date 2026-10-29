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
	FontSize   float32           `json:"font_size"`            // px
	LineHeight float32           `json:"line_height"`          // as CSS's unitless line-height over the book's own; 0: the book's
	Margin     int               `json:"margin"`               // px
	LeadMs     int               `json:"lead_ms"`              // read-along: the words underlined this much after the sound starts, for the headphones' own delay
	HideClock  bool              `json:"hide_clock,omitempty"` // the clock in the top margin hidden
	Direction  map[string]string `json:"direction,omitempty"`  // by book (its key, as its cache's name): "rtl" or "ltr"
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
		s.FontSize, s.LineHeight, s.Margin, s.HideClock = d.FontSize, d.LineHeight, d.Margin, d.HideClock
		delete(s.Direction, key)
	case "clock":
		s.HideClock = !s.HideClock
		return false
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

// The menu: a panel over the page, y0 px from the top, drawn by the engine from HTML (right to left, when the menu is
// in Farsi, for free), its buttons found by their ids (in the panel's own coordinates).
type menu struct {
	img     *image.Gray
	buttons map[string]image.Rectangle
	y0      int
}

// rect is where the panel is on the screen.
func (m *menu) rect() image.Rectangle { return m.img.Rect.Add(image.Pt(0, m.y0)) }

// over draws the panel over the page in img.
func (m *menu) over(img *image.Gray) { copy(img.Pix[m.y0*img.Stride:], m.img.Pix) }

const menuHeight = 900 // px, at most

// The menu's numbers and choices, for ui/menu.html.
type menuData struct {
	Size, LinesName, Lines, Margin string
	Dir, BookDir                   string
	Narrated, HideClock            bool
}

// drawMenu draws the menu for the settings s and the book by key; lines is its line spacing as laid out now, narrated
// whether it can be read aloud, bookRTL whether the book itself reads right to left.
func drawMenu(s *settings, key string, lines float32, narrated, bookRTL bool, fonts *blitz.Fonts, w int) *menu {
	// the line spacing as it is, and whether it is the book's own
	linesName := "Line spacing (the book's)"
	if s.LineHeight > 0 {
		lines, linesName = s.LineHeight, "Line spacing"
	}
	html, ids := render("menu.html", menuData{
		Size: fmt.Sprintf("%g px", s.FontSize), LinesName: linesName, Lines: fmt.Sprintf("%.2f", lines),
		Margin: fmt.Sprintf("%d px", s.Margin), Dir: s.Direction[key], BookDir: map[bool]string{true: "RTL", false: "LTR"}[bookRTL],
		Narrated: narrated, HideClock: s.HideClock,
	})
	return drawPanel(html, ids, fonts, w, menuHeight)
}

// The panel asking whether to remove a highlight (its words are grey on the page under it).
func drawRemove(fonts *blitz.Fonts, w int) *menu {
	html, ids := render("remove.html", nil)
	return drawPanel(html, ids, fonts, w, 180)
}

// drawPanel draws a panel over the top of the page from html, as high as its buttons need (at most h px; the last
// thing on a panel is a button), a line under it, its buttons by ids.
func drawPanel(html string, ids []string, fonts *blitz.Fonts, w, h int) *menu {
	const pad = 30 // px under the last button, over the line
	// a line after everything: a page ends at its last line of text, so a box's bottom edge after its own (the
	// segments' bar) would be cut off; cropped away below
	html = strings.Replace(html, "</body>", "<p>&nbsp;</p></body>", 1)
	o := blitz.Options{Width: w, Height: h, Margin: 40, FontSize: 40}
	b := blitz.New([]string{"menu.html"}, func(string) ([]byte, error) { return []byte(html), nil }, o, fonts)
	defer b.Close()
	b.Layout(0)
	m := &menu{buttons: map[string]image.Rectangle{}}
	bottom := 0
	for _, id := range ids {
		if p, rs := b.Find(0, id); p == 0 {
			m.buttons[id] = union(rs)
			bottom = max(bottom, m.buttons[id].Max.Y)
		}
	}
	whole := image.NewGray(image.Rect(0, 0, w, h))
	b.Draw(0, whole)
	h = min(h, bottom+pad+4)
	m.img = image.NewGray(image.Rect(0, 0, w, h))
	copy(m.img.Pix, whole.Pix)
	for y := h - 4; y < h; y++ { // a line under the panel (over it, for drawNav's at the bottom)
		for x := 0; x < w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 0
		}
	}
	return m
}

// at is the button tapped at (x, y) on the screen, with some room around each for a finger: the nearest, for buttons
// side by side (a bar's segments share a border); "" if none.
func (m *menu) at(x, y int) string {
	const room = 16
	p := image.Pt(x, y-m.y0)
	id, best := "", room+1
	for b, r := range m.buttons {
		// how far p is outside r, along the farther axis (0 inside)
		d := max(r.Min.X-p.X, p.X-r.Max.X+1, r.Min.Y-p.Y, p.Y-r.Max.Y+1, 0)
		if d < best || d == best && b < id {
			id, best = b, d
		}
	}
	return id
}

// Where the reader is, for the bottom panel: the chapter's title, the page in its document, the page in the book
// (total 0 until every document is paginated) and the share of the book read; back is the page to go back to after a
// jump (0: none).
type progress struct {
	chapter     string
	at, of      int
	page, total int
	share       float64
	back        int
	rtl         bool
}

const navHeight = 700 // px, at most

// The bottom panel's words and numbers, for ui/nav.html.
type navData struct {
	Chapter, Where, Back string // Back: the way back's label, "" for none
	Dir, Fill, Width     string // the book's direction, the side the bar fills from, its filled share in per cent
}

// drawNav draws the bottom panel, opened with the menu: where the reader is, a bar to tap to go anywhere in the book,
// the table of contents, the chapters before and after, and the way back after a jump.
func drawNav(p progress, fonts *blitz.Fonts, w, h int) *menu {
	book := fmt.Sprintf("page %d", p.page)
	if p.total > 0 {
		book = fmt.Sprintf("page %d of %d", p.page, p.total)
	}
	d := navData{Chapter: p.chapter, Where: fmt.Sprintf("%d of %d in this part · %s", p.at, p.of, book),
		Dir: "ltr", Fill: "left", Width: fmt.Sprintf("%.1f", 100*min(max(p.share, 0), 1))}
	if p.rtl {
		d.Dir, d.Fill = "rtl", "right"
	}
	if p.back > 0 {
		d.Back = fmt.Sprintf("Back to page %d", p.back)
	}
	html, ids := render("nav.html", d)
	m := drawPanel(html, ids, fonts, w, navHeight)
	// the line on the panel's top edge, not under it
	nh := m.img.Rect.Dy()
	copy(m.img.Pix, m.img.Pix[(nh-4)*m.img.Stride:])
	for y := nh - 4; y < nh; y++ {
		for x := 0; x < w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 255
		}
	}
	m.y0 = h - nh
	return m
}

// share is the share of the bar at x, in the book's direction.
func (m *menu) share(x int, rtl bool) float64 {
	r := m.buttons["bar"]
	s := float64(x-r.Min.X) / float64(max(r.Dx(), 1))
	if rtl {
		s = 1 - s
	}
	return min(max(s, 0), 1)
}
