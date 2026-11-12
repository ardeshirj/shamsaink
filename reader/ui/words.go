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

// mirrored: the interface drawn right to left in a language that reads left to right (SetMirrored).
var mirrored bool

// SetLanguage sets the interface's language to code (a language's primary subtag, "fa"); a language with no table of
// words, and "", are English. The interface takes that language's direction (SetMirrored after it to mirror English).
func SetLanguage(code string) {
	if _, ok := words[code]; !ok {
		code = "en"
	}
	lang = code
	mirrored = false
}

// SetMirrored draws the interface right to left whatever its language: a right-to-left book (Hebrew, Urdu, ...) read
// with the interface in English, for want of words in its own language, is laid out as a Farsi one is (the maintainer,
// 2026-11-12), its words and digits English.
func SetMirrored(on bool) { mirrored = on }

// Interface names the interface's language and direction, for the log: "en", "fa", or "en-rtl" for English mirrored
// (the design system's renders are kept under the same names).
func Interface() string {
	if mirrored && lang != "fa" {
		return lang + "-rtl"
	}
	return lang
}

// Language is the interface's language now: a code of Languages.
func Language() string { return lang }

// Languages are the codes the interface has words in.
func Languages() (codes []string) {
	for c := range words {
		codes = append(codes, c)
	}
	return codes
}

// RTL says whether the interface reads right to left (Farsi, or mirrored for a right-to-left book): its panels are
// drawn mirrored.
func RTL() bool { return lang == "fa" || mirrored }

// word is the text of key in the interface's language, with args put into its %d and %s; a key that is not in the
// table panics, so a typo is found by the tests.
func word(key string, args ...any) string {
	return isolated(words1(key, args...))
}

// isolated is s kept left to right inside a mirrored interface: in English laid out right to left, a sentence's own
// direction stays its own (a full stop at its end, not its start), in Unicode's isolate marks (LRI ... PDI), which
// are not drawn.
func isolated(s string) string {
	if mirrored && lang != "fa" && s != "" {
		return "\u2066" + s + "\u2069"
	}
	return s
}

func words1(key string, args ...any) string {
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
