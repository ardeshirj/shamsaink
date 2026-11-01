package design

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
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
		{"missing control", func(s *Spec) { s.Screens["dock"].Controls = s.Screens["dock"].Controls[1:] }, `makes "chapter-prev"`},
		{"extra control", func(s *Spec) {
			s.Screens["dock"].Controls = append(s.Screens["dock"].Controls, Control{ID: "ghost", Component: "button", Action: "act", Does: "x"})
		}, `declares "ghost"`},
		{"dangling goto", func(s *Spec) { s.Screens["dock"].Controls[4].To = "nowhere" }, "no screen"},
		{"no way out", func(s *Spec) { s.Screens["light"].Dismiss = "" }, "no way out"},
		{"unreachable", func(s *Spec) {
			s.Screens["light"].Enter = []Enter{{Gesture: "tap", From: "text"}}
			s.Screens["text"].Enter = []Enter{{Gesture: "tap", From: "light"}}
		}, "cannot be reached"},
		{"stepper ids", func(s *Spec) { s.Screens["text"].Controls = s.Screens["text"].Controls[:2] }, `makes "size-dec"`},
		{"contents close", func(s *Spec) { s.Screens["contents"].Controls = s.Screens["contents"].Controls[1:] }, `makes "contents-close"`},
		{"enter by a control that goes elsewhere", func(s *Spec) { s.Screens["text"].Enter[0].Control = "contents" }, "does not go to it"},
		{"no button map", func(s *Spec) { s.Screens["dock"].Buttons = "" }, "no page-button map"},
		{"missing map", func(s *Spec) { s.Screens["dock"].Buttons = "nope" }, "does not exist"},
		{"idle gesture", func(s *Spec) { s.Buttons["ring"] = ButtonMap{Forward: "x"} }, "does nothing"},
		{"bad dismiss", func(s *Spec) { s.Screens["contents"].Dismiss = "no-such-control" }, "neither tap-outside"},
		{"panel from panel", func(s *Spec) {
			s.Screens["light"].Controls = append(s.Screens["light"].Controls, Control{ID: "x", Component: "button", Action: "goto", To: "light"})
		}, "a panel from a panel"},
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
