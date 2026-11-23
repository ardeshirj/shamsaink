package main

import (
	"slices"
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"

	"shamsainkreader/library"
	"shamsainkreader/ui"
)

// The interface's language (menus, panels): the setting "Follow book language", on unless settings.english_only is
// set. On: the language of the book open, when the interface has words in it (ui.Languages: "en", "fa"), else English.
// The library is always in English (the maintainer, 2026-10-31: the language is for reading a book). Off: English everywhere.

// languageCode is the language code for ui.SetLanguage: bookCode (a book's language, "" for none) when the interface
// follows the book and has a table for it, else "en".
func languageCode(englishOnly bool, bookCode string) string {
	if englishOnly || !slices.Contains(ui.Languages(), bookCode) {
		return "en"
	}
	return bookCode
}

// bookLanguage is the language of a book from its metadata (languages in BCP 47, as the metadata gives them) and its
// title: Farsi if any of the languages is, else the first, else Farsi for a title in Arabic script, else "".
func bookLanguage(langs []string, title string) string {
	if persian(langs, false) {
		return "fa"
	}
	first := ""
	if len(langs) > 0 {
		first = langs[0]
	}
	return library.Lang(library.Book{Lang: first, Title: title})
}

// rtlLanguages are the languages written right to left, by primary subtag: Readium's rule for a book with no stated
// direction knows only Arabic, Farsi and Hebrew (readium-css, "missing page-progression-direction"), so an Urdu or
// Yiddish book would turn its pages the wrong way.
var rtlLanguages = []string{"ar", "fa", "per", "fas", "he", "iw", "yi", "ur", "ps", "sd", "ckb", "ug", "dv", "syr"}

// writtenRTL says whether a language (BCP 47, or a primary subtag as library.Lang gives it) is written right to left:
// a book's rows and panels in the library and the catalogues are laid out from the right for it (before 2026-11-22,
// Farsi only: an Urdu, Arabic or Hebrew book's row was set as an English one's).
func writtenRTL(lang string) bool {
	return slices.Contains(rtlLanguages, strings.ToLower(strings.SplitN(strings.TrimSpace(lang), "-", 2)[0]))
}

// readsRTL says whether a book's pages turn right to left: its stated direction, else Readium's rule (one language
// given, and written right to left) with rtlLanguages.
func readsRTL(m manifest.Metadata) bool {
	if m.ReadingProgression != manifest.None {
		return m.ReadingProgression == manifest.RTL
	}
	if m.EffectiveReadingProgression() == manifest.RTL {
		return true
	}
	if len(m.Languages) != 1 {
		return false
	}
	return slices.Contains(rtlLanguages, strings.ToLower(strings.SplitN(m.Languages[0], "-", 2)[0]))
}
