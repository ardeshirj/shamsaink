package main

import (
	"slices"

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
	return bookLang(libBook{Lang: first, Title: title})
}
