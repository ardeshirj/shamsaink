package main

import (
	"cmp"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/files"
	"shamsainkreader/ui"

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
	// Minutes untouched before the device is put to sleep (device.SleepMinutes; one not among them, such as the 0 that
	// was never until 2026-11-07, read as the default).
	SleepMin int `json:"sleep_min"`
	// Dark mode: the screen inverted (device.Screen.SetInvert), set from the Light panel.
	Dark bool `json:"dark,omitempty"`
	// The page buttons swapped (the lower goes back): off by default, the lower button forward.
	ReverseButtons bool `json:"reverse_buttons,omitempty"`
	// Clean the screen (a full refresh with its flash) every this many page turns, one count for light mode (none at first)
	// and one for dark, where ghosting shows most (0: never). A choice of cleanSteps, set in the Reading settings.
	CleanLight int `json:"clean_light"`
	CleanDark  int `json:"clean_dark"`
	// The front light (light.go), as last set in the reader's Light panel: its level (0 to 100) and warmth (0 to 10);
	// none until it is set there, and then the light is left as Kobo's interface had it.
	Light  *int `json:"light,omitempty"`
	Warmth *int `json:"warmth,omitempty"`
}

// defaults are the settings before any is set (the environment's, else the reader's own).
func defaults() *settings {
	f := func(s string) float32 { v, _ := strconv.ParseFloat(s, 32); return float32(v) }
	return &settings{FontSize: f(fontSize), LineHeight: f(lineHeight), Margin: atoi(margin), LeadMs: atoi(leadMs), SleepMin: int(device.DefaultSleepAfter / time.Minute), CleanDark: 5}
}

func loadSettings(dir string) *settings {
	s := defaults()
	if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		json.Unmarshal(b, s)
	}
	if !slices.Contains(device.SleepMinutes, s.SleepMin) {
		s.SleepMin = int(device.DefaultSleepAfter / time.Minute)
	}
	return s
}

func (s *settings) save(dir string) {
	b, _ := json.MarshalIndent(s, "", "  ")
	files.Write(filepath.Join(dir, "settings.json"), append(b, '\n'))
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
		// from the spacing as it is (the book's own, measured, until one is set), by 0.1, on tenths
		v := cmp.Or(s.LineHeight, lines, 1.2)
		v = step(v, map[bool]float32{true: 0.1, false: -0.1}[id == "lines-inc"], 0.8, 3)
		s.LineHeight = float32(math.Round(float64(v)*10) / 10)
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

// nextSleep is the sleep timeout after cur minutes, a step longer or shorter in device.SleepMinutes (it stops at both
// ends).
func nextSleep(cur int, longer bool) int {
	i := slices.Index(device.SleepMinutes, cur)
	if i < 0 {
		i = slices.Index(device.SleepMinutes, int(device.DefaultSleepAfter/time.Minute))
	}
	if longer {
		i++
	} else {
		i--
	}
	return device.SleepMinutes[max(0, min(i, len(device.SleepMinutes)-1))]
}

// cleanSteps are the page counts "Clean every" steps through (0: off).
var cleanSteps = []int{0, 3, 5, 10, 20}

// cleanEvery is how many page turns come between cleanings now, by the screen's mode (0: never).
func (s *settings) cleanEvery() int {
	if s.Dark {
		return s.CleanDark
	}
	return s.CleanLight
}

// stepClean applies a Reading settings stepper's button (clean-light-dec, clean-light-inc, clean-dark-dec, clean-dark-inc):
// that mode's count to the next or the previous of cleanSteps; false for any other id.
func (s *settings) stepClean(id string) bool {
	var p *int
	switch {
	case strings.HasPrefix(id, "clean-light-"):
		p = &s.CleanLight
	case strings.HasPrefix(id, "clean-dark-"):
		p = &s.CleanDark
	default:
		return false
	}
	if !strings.HasSuffix(id, "-dec") && !strings.HasSuffix(id, "-inc") {
		return false
	}
	i := slices.Index(cleanSteps, *p)
	if i < 0 {
		i = 0
	}
	if strings.HasSuffix(id, "-inc") {
		i++
	} else {
		i--
	}
	*p = cleanSteps[max(0, min(i, len(cleanSteps)-1))]
	return true
}

// settingsView is what the settings panel shows for set on dev.
func settingsView(set *settings, dev *device.Device) ui.LibrarySettings {
	return ui.LibrarySettings{Follow: !set.EnglishOnly, SleepMin: set.SleepMin, HasButtons: dev.HasButtons(), ReverseButtons: set.ReverseButtons, Version: version}
}
