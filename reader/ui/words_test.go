package ui

import (
	"strings"
	"testing"
	"time"
)

// Every language has the same words as English, with the same slots (%d, %s).
func TestWordTablesAgree(t *testing.T) {
	slots := func(s string) string {
		var out []string
		for i := 0; i+1 < len(s); i++ {
			if s[i] == '%' && s[i+1] != '%' { // (a percent sign may be written as ٪)
				out = append(out, s[i:i+2])
				i++
			}
		}
		return strings.Join(out, " ")
	}
	for code, table := range words {
		for key, en := range words["en"] {
			v, ok := table[key]
			if !ok || v == "" {
				t.Errorf("%s: no word %q", code, key)
			} else if code != "en" && slots(v) != slots(en) {
				t.Errorf("%s: %q has slots %q, English %q", code, key, slots(v), slots(en))
			}
		}
		for key := range table {
			if _, ok := words["en"][key]; !ok {
				t.Errorf("%s: %q is not in English", code, key)
			}
		}
	}
}

// SetLanguage falls back to English for a language with no table; numbers are in the language's digits.
func TestSetLanguage(t *testing.T) {
	defer SetLanguage("en")
	for code, want := range map[string]string{"fa": "fa", "en": "en", "de": "en", "": "en", "ar": "en"} {
		SetLanguage(code)
		if lang != want {
			t.Errorf("SetLanguage(%q): %q, want %q", code, lang, want)
		}
	}
	SetLanguage("fa")
	if !RTL() || num(2026) != "۲۰۲۶" || word("page-of", 3, 12) != "صفحهٔ ۳ از ۱۲" || word("percent", num(82)) != "۸۲٪" {
		t.Errorf("fa: %v %q %q %q", RTL(), num(2026), word("page-of", 3, 12), word("percent", num(82)))
	}
	if decimal("1.16") != "۱٫۱۶" || word("px", num(60)) != "۶۰ پیکسل" {
		t.Errorf("fa: %q %q", decimal("1.16"), word("px", num(60)))
	}
	if digits("20:48") != "۲۰:۴۸" {
		t.Errorf("fa: the time %q", digits("20:48"))
	}
	SetLanguage("de")
	if RTL() || num(5) != "5" || word("page-of", 3, 12) != "page 3 of 12" || word("percent", num(82)) != "82%" {
		t.Errorf("de: %v %q", RTL(), word("page-of", 3, 12))
	}
}

// When a book was added, as its row says it.
func TestAddedWhen(t *testing.T) {
	now := time.Date(2026, 10, 31, 0, 30, 0, 0, time.Local)
	for _, c := range []struct {
		ago  time.Duration
		want string
	}{{0, "Today"}, {time.Hour, "Yesterday"}, {49 * time.Hour, "3 Days"}, {6 * 24 * time.Hour, "6 Days"},
		{8 * 24 * time.Hour, "23 Oct"}, {300 * 24 * time.Hour, "4 Jan"}, {400 * 24 * time.Hour, "26 Sep 2025"}} {
		if got := AddedWhen(now.Add(-c.ago), now); got != c.want {
			t.Errorf("%v ago: %q, want %q", c.ago, got, c.want)
		}
	}
}
