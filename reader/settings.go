package main

import (
	"cmp"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	blitz "shamsainkengine"
)

// The settings, in DIR/settings.json: text size, line spacing and margins for every book, the page direction by book.
type settings struct {
	FontSize   float32           `json:"font_size"`           // px
	LineHeight float32           `json:"line_height"`         // as CSS's unitless line-height over the book's own; 0: the book's
	Margin     int               `json:"margin"`              // px
	LeadMs     int               `json:"lead_ms"`             // read-along: the words underlined this much after the sound starts, for the headphones' own delay
	Direction  map[string]string `json:"direction,omitempty"` // by book (its key, as its cache's name): "rtl" or "ltr"
	// The interface's language (lang.go): the menus follow the book's language unless this is set ("Follow book language"
	// off: English everywhere). Stored as the opposite so that a file from before it, without the key, follows the book.
	EnglishOnly bool `json:"english_only,omitempty"`
	// The front light (light.go), as last set in the reader's Light panel: its level (0 to 100) and warmth (0 to 10);
	// none until it is set there, and then the light is left as Kobo's interface had it.
	Light  *int `json:"light,omitempty"`
	Warmth *int `json:"warmth,omitempty"`
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
	writeFile(filepath.Join(dir, "settings.json"), append(b, '\n'))
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
