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
