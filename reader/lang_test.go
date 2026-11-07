package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The interface's language: following the book (a table for it, else English) or English only.
func TestLanguageCode(t *testing.T) {
	for _, c := range []struct {
		englishOnly bool
		book, want  string
	}{
		{false, "fa", "fa"}, {false, "en", "en"}, {false, "de", "en"}, {false, "", "en"},
		{true, "fa", "en"}, {true, "en", "en"}, {true, "de", "en"}, {true, "", "en"},
	} {
		if got := languageCode(c.englishOnly, c.book); got != c.want {
			t.Errorf("languageCode(%v, %q) = %q, want %q", c.englishOnly, c.book, got, c.want)
		}
	}
}

func TestBookLanguage(t *testing.T) {
	for _, c := range []struct {
		langs []string
		title string
		want  string
	}{
		{[]string{"fa"}, "x", "fa"}, {[]string{"fa-IR"}, "x", "fa"}, {[]string{"per"}, "x", "fa"}, {[]string{"en", "fa"}, "x", "fa"},
		{[]string{"en-GB"}, "x", "en"}, {[]string{"de"}, "x", "de"}, {nil, "x", ""}, {nil, "رود کم‌آب", "fa"},
		{[]string{"en"}, "رود کم‌آب", "en"},
	} {
		if got := bookLanguage(c.langs, c.title); got != c.want {
			t.Errorf("bookLanguage(%v, %q) = %q, want %q", c.langs, c.title, got, c.want)
		}
	}
}

// A settings file from before the language switch, or without the key, follows the book; the choice is kept.
func TestLanguageSetting(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"font_size": 42, "margin": 70}`), 0o644)
	s := loadSettings(dir)
	if s.EnglishOnly || s.FontSize != 42 {
		t.Errorf("old settings: %+v", s)
	}
	s.EnglishOnly = true
	s.save(dir)
	if s = loadSettings(dir); !s.EnglishOnly || s.FontSize != 42 {
		t.Errorf("after saving English only: %+v", s)
	}
	s.EnglishOnly = false
	s.save(dir)
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) == "" || loadSettings(dir).EnglishOnly {
		t.Errorf("following again: %s", b)
	}
}

// The sleep timeout's stepper goes through the choices and stops at both ends; 60 is the longest (no never since 2026-11-07).
func TestNextSleep(t *testing.T) {
	for _, c := range []struct {
		cur    int
		longer bool
		want   int
	}{{10, true, 15}, {10, false, 5}, {5, false, 5}, {60, true, 60}, {60, false, 30}, {0, true, 15}, {7, true, 15}} {
		if got := nextSleep(c.cur, c.longer); got != c.want {
			t.Errorf("nextSleep(%d, %v) = %d, want %d", c.cur, c.longer, got, c.want)
		}
	}
}

// The line spacing steps by 0.1 from wherever it is, so a step back undoes a step.
func TestNextLines(t *testing.T) {
	for _, c := range []struct {
		set, book float32
		wider     bool
		want      float32
	}{
		{0, 1.9, true, 2}, {2, 1.9, false, 1.9}, {0, 1.16, true, 1.26}, {1.26, 1.16, false, 1.16},
		{1.75, 1.75, true, 1.85}, {1.85, 1.85, false, 1.75}, {3, 1.9, true, 3}, {0.8, 1.9, false, 0.8}, {0, 0, true, 1.3},
	} {
		if got := nextLines(c.set, c.book, c.wider); got != c.want {
			t.Errorf("nextLines(%g, %g, %v) = %g, want %g", c.set, c.book, c.wider, got, c.want)
		}
	}
}
