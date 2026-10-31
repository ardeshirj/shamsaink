package ui

import (
	"fmt"
	"strings"
)

// The interface's words, by language and key: every word a panel shows comes from here (templates through t, Go code
// through word), whole sentences as one entry with their numbers and names as %d and %s, since the order of the parts
// differs between languages. A table for another language is one more entry of words, chosen by lang.
var words = map[string]map[string]string{
	"en": {
		"text-size":         "Text size",
		"line-spacing":      "Line spacing",
		"line-spacing-book": "Line spacing (the book's)",
		"margins":           "Margins",
		"page-turns-book":   "Page turns (this book):",
		"dir-book":          "Book's",
		"dir-rtl":           "Right to left",
		"dir-ltr":           "Left to right",
		"remove-highlight":  "Remove highlight",
		"keep":              "Keep",
		"contents":          "Contents",
		"back-to-page":      "Back to page %d",
		"page":              "page %d",
		"page-of":           "page %d of %d",
		"by":                "by",
		"opening":           "Opening the book…",
		"page-n-of":         "%s of %s",
		"percent":           "%s%%",
		"new":               "new",
		"library":           "Library",
		"leave":             "Leave",
		"reading":           "Reading",
		"new-books":         "New",
		"finished":          "Finished",
		"all":               "All",
		"sort-by":           "Sort by",
		"recent":            "Recent",
		"title":             "Title",
		"author":            "Author",
		"added":             "Added",
		"settings":          "Settings",
		"follow-book":       "Follow book language",
		"dock-page":         "%d",
		"dock-page-of":      "%d/%d",
		"pages-left":        "%d pages left in this part",
		"page-left":         "%d page left in this part",
		"last-page":         "Last page of this part",
		"text":              "Text",
		"page-tab":          "Page",
		"reset":             "Reset",
		"px":                "%s px",
		"light":             "Light",
		"warmth":            "Warmth",
		"clean-screen":      "Clean the screen",
	},
	"fa": {
		"text-size":         "اندازهٔ متن",
		"line-spacing":      "فاصلهٔ خطوط",
		"line-spacing-book": "فاصلهٔ خطوط (کتاب)",
		"margins":           "حاشیه‌ها",
		"page-turns-book":   "ورق زدن (این کتاب):",
		"dir-book":          "طبق کتاب",
		"dir-rtl":           "راست به چپ",
		"dir-ltr":           "چپ به راست",
		"remove-highlight":  "حذف نشانه",
		"keep":              "نگه‌دار",
		"contents":          "فهرست",
		"back-to-page":      "بازگشت به صفحهٔ %d",
		"page":              "صفحهٔ %d",
		"page-of":           "صفحهٔ %d از %d",
		"by":                "از",
		"opening":           "در حال باز کردن کتاب…",
		"page-n-of":         "%s از %s",
		"percent":           "%s٪",
		"new":               "جدید",
		"library":           "کتابخانه",
		"leave":             "خروج",
		"reading":           "خواندن",
		"new-books":         "جدید",
		"finished":          "تمام‌شده",
		"all":               "همه",
		"sort-by":           "مرتب‌سازی بر اساس",
		"recent":            "اخیر",
		"title":             "عنوان",
		"author":            "نویسنده",
		"added":             "تاریخ افزودن",
		"settings":          "تنظیمات",
		"follow-book":       "پیروی از زبان کتاب",
		"dock-page":         "%d",
		"dock-page-of":      "%d/%d",
		"pages-left":        "%d صفحه تا پایان این بخش",
		"page-left":         "%d صفحه تا پایان این بخش",
		"last-page":         "صفحهٔ آخر این بخش",
		"text":              "متن",
		"page-tab":          "صفحه",
		"reset":             "بازنشانی",
		"px":                "%s پیکسل",
		"light":             "نور",
		"warmth":            "گرمی",
		"clean-screen":      "پاک کردن صفحه",
	},
}

// lang is the language of the interface: a key of words.
var lang = "en"

// SetLanguage sets the interface's language to code (a language's primary subtag, "fa"); a language with no table of
// words, and "", are English.
func SetLanguage(code string) {
	if _, ok := words[code]; !ok {
		code = "en"
	}
	lang = code
}

// Languages are the codes the interface has words in.
func Languages() (codes []string) {
	for c := range words {
		codes = append(codes, c)
	}
	return codes
}

// RTL says whether the interface reads right to left (Farsi): its panels are drawn mirrored.
func RTL() bool { return lang == "fa" }

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
	if lang != "en" { // the numbers in the language's digits
		s = strings.ReplaceAll(s, "%d", "%s")
		args = append([]any(nil), args...)
		for i, a := range args {
			if n, ok := a.(int); ok {
				args[i] = num(n)
			}
		}
	}
	return fmt.Sprintf(s, args...)
}

// num is n in the interface's digits: Latin for English, Farsi (۰ to ۹) otherwise.
func num(n int) string { return digits(fmt.Sprint(n)) }

// decimal is a number written with a point ("1.16") in the interface's digits and decimal separator (Farsi: ۱٫۱۶).
func decimal(s string) string {
	if lang == "en" {
		return s
	}
	return strings.ReplaceAll(digits(s), ".", "٫")
}

// digits is s with its Latin digits in the interface's (a time, "20:48").
func digits(s string) string {
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
