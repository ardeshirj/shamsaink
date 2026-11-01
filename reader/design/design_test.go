package design

import (
	"bytes"
	"os"
	"shamsainkreader/device"
	"shamsainkreader/library"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func load(t *testing.T) *Spec {
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The spec agrees with the templates, the stylesheet, the words and the icons of reader/ui.
func TestSpecInStepWithUI(t *testing.T) {
	for _, p := range Validate(load(t), Sources{UI: os.DirFS("../ui")}) {
		t.Error(p)
	}
}

// The checks catch what they are for: each of these changes to a copy of the spec makes Validate say so.
func TestValidateCatches(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Spec)
		want   string
	}{
		{"token", func(s *Spec) { s.Tokens.CSS["--line"] = "5px" }, "token --line"},
		{"unknown token", func(s *Spec) { delete(s.Tokens.CSS, "--gap") }, "tokens.yaml lacks"},
		{"slack", func(s *Spec) { s.Devices[0].Tap.SlackPx = 4 }, "tap slack"},
		{"small target", func(s *Spec) { s.Devices[0].Tap.MinMM = 20 }, "a target"},
		{"waveform", func(s *Spec) { s.Components["slider"].Refresh.Waveform = "XX" }, "not on"},
		{"css size", func(s *Spec) { s.Components["icon-button"].CSS.Width = "3em" }, "width 3em"},
		{"dangling goto", func(s *Spec) { s.Screens["dock"].Controls[4].To = "nowhere" }, "no screen"},
		{"no way out", func(s *Spec) { s.Screens["light"].Dismiss = "" }, "no way out"},
		{"unreachable", func(s *Spec) {
			s.Screens["light"].Enter = []Enter{{Gesture: "tap", From: "text"}}
			s.Screens["text"].Enter = []Enter{{Gesture: "tap", From: "light"}}
		}, "cannot be reached"},
		{"enter by a control that goes elsewhere", func(s *Spec) { s.Screens["text"].Enter[0].Control = "contents" }, "does not go to it"},
		{"no button map", func(s *Spec) { s.Screens["dock"].Buttons = "" }, "no page-button map"},
		{"missing map", func(s *Spec) { s.Screens["dock"].Buttons = "nope" }, "does not exist"},
		{"idle gesture", func(s *Spec) { s.Buttons["ring"] = ButtonMap{Forward: "x"} }, "does nothing"},
		{"bad dismiss", func(s *Spec) { s.Screens["contents"].Dismiss = "no-such-control" }, "neither tap-outside"},
		{"panel from panel", func(s *Spec) {
			s.Screens["light"].Controls = append(s.Screens["light"].Controls, Control{ID: "x", Component: "button", Action: "goto", To: "light"})
		}, "a panel from a panel"},
		{"replace across layers", func(s *Spec) { s.Screens["library-book-menu"].Controls[3].To = "library-sort" }, "another layer"},
		{"replace into nothing", func(s *Spec) { s.Screens["menu"].Controls[0].To = "nowhere" }, "no screen"},
		{"nothing enters", func(s *Spec) { s.Screens["library-book-menu"].Enter = nil }, "nothing enters it"},
		{"word", func(s *Spec) { s.Screens["light"].Words = append(s.Screens["light"].Words, "no-such-word") }, "no-such-word"},
	}
	for _, c := range cases {
		s := load(t)
		c.mutate(s)
		found := false
		for _, p := range Validate(s, Sources{UI: os.DirFS("../ui")}) {
			found = found || strings.Contains(p, c.want)
		}
		if !found {
			t.Errorf("%s: Validate did not say %q", c.name, c.want)
		}
	}
}

// A misspelt key in the spec is an error.
func TestUnknownKey(t *testing.T) {
	_, err := LoadFS(fstest.MapFS{"tokens.yaml": {Data: []byte("css: {}\ncsss: {}\n")}})
	if err == nil {
		t.Error("a misspelt key was accepted")
	}
}

// The filters and sorts the spec lists are the library's.
func TestValuesAreTheLibrarys(t *testing.T) {
	s := load(t)
	for _, c := range []struct {
		screen, id string
		want       []string
	}{{"library", "filter-", library.Filters}, {"library-sort", "sort-", library.Sorts}} {
		found := false
		for _, ctl := range s.Screens[c.screen].Controls {
			if ctl.ID == c.id {
				found = true
				if !slices.Equal(ctl.Values, c.want) {
					t.Errorf("%s %s: spec %v, library %v", c.screen, c.id, ctl.Values, c.want)
				}
			}
		}
		if !found {
			t.Errorf("%s has no %s", c.screen, c.id)
		}
	}
}

// docs/design/system.html is the page the spec and the kept renders make now (`make design` writes it).
func TestSiteUpToDate(t *testing.T) {
	want, err := Site(load(t), os.DirFS("golden"), "../../reader/design/golden")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../docs/design/system.html")
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("docs/design/system.html is not what the spec makes: run `make design` (%v)", err)
	}
}

// ui.css's variables and the panels' sizes are what tokens.yaml makes (`make design` writes them).
func TestGeneratedInStep(t *testing.T) {
	s := load(t)
	css, err := os.ReadFile("../ui/templates/ui.css")
	if err != nil {
		t.Fatal(err)
	}
	want, err := TokensCSS(string(css), s.Tokens)
	if err != nil {
		t.Fatal(err)
	}
	if want != string(css) {
		t.Error("the variables of ui.css are not what tokens.yaml makes: run `make design`")
	}
	if got, _ := os.ReadFile("../ui/tokens_gen.go"); string(got) != TokensGo(s.Tokens) {
		t.Error("ui/tokens_gen.go is not what tokens.yaml makes: run `make design`")
	}
}

// CheckControls says what is missing or extra between the spec's controls and the buttons a screen drew.
func TestCheckControls(t *testing.T) {
	s := load(t)
	text := []string{"tab-text", "tab-page", "size-dec", "size-inc", "lines-dec", "lines-inc", "margin-dec", "margin-inc",
		"dir-", "dir-rtl", "dir-ltr", "reset"}
	if bad := CheckControls(s, "text", text); len(bad) != 0 {
		t.Errorf("the text sheet's buttons: %v", bad)
	}
	for _, c := range []struct {
		name   string
		found  []string
		screen string
		want   string
	}{
		{"missing", text[1:], "text", `declares "tab-text"`},
		{"extra", append([]string{"ghost"}, text...), "text", `drew "ghost"`},
		{"no stepper minus", text[:2], "text", `"size-dec"`},
		{"a list's rows", []string{"contents-close"}, "contents", "e..."},
		{"unknown screen", nil, "nowhere", "no screen"},
	} {
		bad := strings.Join(CheckControls(s, c.screen, c.found), "\n")
		if !strings.Contains(bad, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, bad, c.want)
		}
	}
	if bad := CheckControls(s, "contents", []string{"contents-close", "e0", "e1"}); len(bad) != 0 {
		t.Errorf("contents with rows: %v", bad)
	}
}

// A component's role is one a user or a test can name.
func TestRoles(t *testing.T) {
	s := load(t)
	s.Components["slider"].Role = "gizmo"
	found := false
	for _, p := range Validate(s, Sources{UI: os.DirFS("../ui")}) {
		found = found || strings.Contains(p, "role")
	}
	if !found {
		t.Error("a role that is none of the known ones was accepted")
	}
}


// The profile's sleep_after_min is the device's.
func TestSleepAfter(t *testing.T) {
	s := load(t)
	for _, d := range s.Devices {
		if time.Duration(d.SleepAfter)*time.Minute != device.DefaultSleepAfter {
			t.Errorf("%s: sleep_after_min %d, device.DefaultSleepAfter %v", d.Name, d.SleepAfter, device.DefaultSleepAfter)
		}
	}
}

// The sleep setting's choices are the device's.
func TestSleepChoices(t *testing.T) {
	s := load(t)
	var want []string
	for _, m := range device.SleepMinutes {
		want = append(want, strconv.Itoa(m))
	}
	for _, ctl := range s.Screens["library-language"].Controls {
		if ctl.ID == "sleep" {
			if !slices.Equal(ctl.Values, want) {
				t.Errorf("sleep: spec %v, device %v", ctl.Values, want)
			}
			return
		}
	}
	t.Error("library-language has no sleep")
}
