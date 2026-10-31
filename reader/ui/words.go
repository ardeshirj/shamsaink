package ui

import "fmt"

// The interface's words, by language and key: every word a panel shows comes from here (templates through t, Go code
// through word), whole sentences as one entry with their numbers and names as %d and %s, since the order of the parts
// differs between languages. A table for another language is one more entry of words, chosen by lang.
var words = map[string]map[string]string{
	"en": {
		"close-book":        "Close the book",
		"reset-settings":    "Reset settings",
		"read-aloud":        "Read aloud",
		"text-size":         "Text size",
		"line-spacing":      "Line spacing",
		"line-spacing-book": "Line spacing (the book's)",
		"margins":           "Margins",
		"clock":             "Clock",
		"page-turns-book":   "Page turns (this book):",
		"dir-book":          "Book's (%s)",
		"dir-rtl":           "Right to left",
		"dir-ltr":           "Left to right",
		"dir-rtl-short":     "RTL",
		"dir-ltr-short":     "LTR",
		"remove-highlight":  "Remove highlight",
		"keep":              "Keep",
		"contents":          "Contents",
		"back-to-book":      "Back to the book",
		"prev-chapter":      "Previous chapter",
		"next-chapter":      "Next chapter",
		"back-to-page":      "Back to page %d",
		"page":              "page %d",
		"page-of":           "page %d of %d",
		"where":             "%d of %d in this part · %s",
		"by":                "by",
	},
}

// lang is the language of the interface: a key of words.
var lang = "en"

// word is the text of key in the interface's language, with args put into its %d and %s; a key that is not in the
// table panics, so a typo is found by the tests.
func word(key string, args ...any) string {
	s, ok := words[lang][key]
	if !ok {
		panic(fmt.Sprintf("ui: no word %q in language %q", key, lang))
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// num is n in the interface's digits: Latin for English, Farsi (۰ to ۹) otherwise.
func num(n int) string {
	s := fmt.Sprint(n)
	if lang == "en" {
		return s
	}
	r := []rune(s)
	for i, c := range r {
		if c >= '0' && c <= '9' {
			r[i] = c + '۰' - '0'
		}
	}
	return string(r)
}
