package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every scenario checks against the Kobo's kept renders: its steps known, their arguments, regular expressions and
// controls by name (make check, without a device).
func TestScenarios(t *testing.T) {
	root(t)
	d, err := load("kobo-libra-colour")
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob("e2e/scenarios/*.txt")
	more, _ := filepath.Glob("e2e/scenarios/languages/*.txt") // the other languages' scenarios, CI's languages group
	files = append(files, more...)
	if len(files) == 0 {
		t.Fatal("no scenarios in e2e/scenarios")
	}
	for _, f := range files {
		s, err := parseScenario(f)
		if err == nil {
			err = s.check(d)
		}
		if err != nil {
			t.Error(err)
		}
	}
}

// A scenario mistyped is caught, each mistake on its line.
func TestScenarioMistakes(t *testing.T) {
	root(t)
	d, err := load("kobo-libra-colour")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ text, says string }{
		{"open\ntap dock contnets", `no control "contnets"`},
		{"open\ntap dokc contents", `no kept render of the screen "dokc"`},
		{"tapp page-top", `unknown step "tapp"`},
		{"expect went to (document", "not a regular expression"},
		{"turn sideways", "turn forward|back"},
		{"tap-menu hl-mkae", `no control "hl-mkae" in the highlight's menu`},
		{"snap a\nsnap a", "a second snap named a"},
		{"freeze", "runs on the Kobo only"},
		{"hold 1 2 to 3", "hold X Y [to X2 Y2]"},
	} {
		s := parse(t, "on: sim kobo\n"+c.text)
		if err := s.check(d); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%q: got %v, want %q", c.text, err, c.says)
		}
	}
	for _, c := range []struct{ text, says string }{
		{"open", "no on: line"},
		{"on: remarkable\nopen", "not sim, kobo or kindle"},
		{"on: sim\nopen\non: kobo", "belongs in the header"},
	} {
		if _, err := parseText(t, c.text); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%q: got %v, want %q", c.text, err, c.says)
		}
	}
	s := parse(t, "on: kobo # the Kobo only\nbook: books/a.epub\nfixture: fa-basics\n# a comment\nopen books/a.epub   # by name\nclose")
	if !s.runsOn("kobo") || s.runsOn("sim") || len(s.books) != 1 || len(s.steps) != 2 || s.steps[0].n != 5 {
		t.Errorf("header or steps read wrong: %+v", s)
	}
}

func parseText(t *testing.T, text string) (*script, error) {
	f := filepath.Join(t.TempDir(), "t.txt")
	os.WriteFile(f, []byte(text), 0o644)
	return parseScenario(f)
}

func parse(t *testing.T, text string) *script {
	s, err := parseText(t, text)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// root moves to the repository's root, where devtest runs.
func root(t *testing.T) {
	t.Helper()
	wd, _ := os.Getwd()
	t.Chdir(filepath.Join(wd, "..", "..", ".."))
}
