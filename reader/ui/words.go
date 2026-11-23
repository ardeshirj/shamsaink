package ui

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// The interface's words, by language and key, from words/LANG.json (en.json, fa.json): every word a panel shows comes
// from there (templates through t, Go code through word), whole sentences as one entry with their numbers and names as
// %d and %s, since the order of the parts differs between languages. Another language is one more file.
var words = func() map[string]map[string]string {
	all := map[string]map[string]string{}
	names, _ := fs.Glob(files, "words/*.json")
	for _, name := range names {
		b, err := files.ReadFile(name)
		table := map[string]string{}
		if err == nil {
			err = json.Unmarshal(b, &table)
		}
		if err != nil {
			panic(fmt.Sprintf("ui/%s: %v", name, err))
		}
		all[strings.TrimSuffix(path.Base(name), ".json")] = table
	}
	return all
}()

// lang is the language of the interface: a key of words.
var lang = "en"

// SetLanguage sets the interface's language to code (a language's primary subtag, "fa"); a language with no table of
// words, and "", are English. The interface takes that language's direction: English is always left to right, over a
// right-to-left book too, whose own parts (its pages, the dock's progress row, its contents) take the book's direction
// (2026-11-22, the maintainer: English mirrored, from 2026-11-12, read oddly, for nothing gained in English).
func SetLanguage(code string) {
	if _, ok := words[code]; !ok {
		code = "en"
	}
	lang = code
}

// Interface names the interface's language, for the log: "en" or "fa" (the design system's renders are kept under the
// same names).
func Interface() string { return lang }

// Language is the interface's language now: a code of Languages.
func Language() string { return lang }

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
func word(key string, args ...any) string { return words1(key, args...) }

func words1(key string, args ...any) string {
	s, ok := words[lang][key]
	if !ok && lang != "en" { // a word of an English-only screen (the library's): English
		s, ok = words["en"][key]
	}
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

// count is how many of something (a catalogue's books) in the interface's digits, its thousands grouped from four
// digits ("3,278", as GOV.UK writes numbers) with the interface's separator (Farsi: ٬); not for a year or a page.
func count(n int) string {
	s := fmt.Sprint(n)
	if n >= 1000 {
		sep := ","
		if lang != "en" {
			sep = "٬"
		}
		var b strings.Builder
		for i, r := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				b.WriteString(sep)
			}
			b.WriteRune(r)
		}
		s = b.String()
	}
	return digits(s)
}

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
