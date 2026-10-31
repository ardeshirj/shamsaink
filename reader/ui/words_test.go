package ui

import (
	"strings"
	"testing"
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
	if !RTL() || num(2026) != "۲۰۲۶" || word("page-of", 3, 12) != "صفحهٔ ۳ از ۱۲" || word("battery", 82) != "۸۲٪" {
		t.Errorf("fa: %v %q %q %q", RTL(), num(2026), word("page-of", 3, 12), word("battery", 82))
	}
	if digits("20:48") != "۲۰:۴۸" {
		t.Errorf("fa: the time %q", digits("20:48"))
	}
	SetLanguage("de")
	if RTL() || num(5) != "5" || word("page-of", 3, 12) != "page 3 of 12" || word("battery", 82) != "82%" {
		t.Errorf("de: %v %q", RTL(), word("page-of", 3, 12))
	}
}
