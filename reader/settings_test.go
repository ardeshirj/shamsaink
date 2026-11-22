package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A settings file from when the clock could be hidden still loads: the old key is ignored, the rest kept.
func TestOldSettingsLoad(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"font_size": 42, "margin": 70, "hide_clock": true}`), 0o644)
	if s := loadSettings(dir); s.FontSize != 42 || s.Margin != 70 {
		t.Errorf("settings %+v", s)
	}
}

// The cleaning count: a Reading settings stepper for each mode, stepping through cleanSteps; light starts at off, dark at 5 pages.
func TestCleanEvery(t *testing.T) {
	s, dark := defaults(), false
	if s.cleanEvery(dark) != 0 {
		t.Errorf("light mode starts at %d, not off", s.cleanEvery(dark))
	}
	dark = true
	if s.cleanEvery(dark) != 5 {
		t.Errorf("dark mode starts at %d, not 5", s.cleanEvery(dark))
	}
	s.stepClean("clean-dark-inc")
	s.stepClean("clean-dark-inc")
	if s.cleanEvery(dark) != 20 || s.CleanLight != 0 {
		t.Errorf("two steps up in dark: dark %d (want 20), light %d (want 0, untouched)", s.cleanEvery(dark), s.CleanLight)
	}
	s.stepClean("clean-dark-inc") // the end stays
	if s.cleanEvery(dark) != 20 {
		t.Errorf("past the end: %d", s.cleanEvery(dark))
	}
	dark = false
	s.stepClean("clean-light-inc")
	s.stepClean("clean-light-inc")
	if s.CleanLight != 5 || s.CleanDark != 20 {
		t.Errorf("two steps up in light: light %d (want 5), dark %d (want 20, untouched)", s.CleanLight, s.CleanDark)
	}
	for i := 0; i < 9; i++ {
		s.stepClean("clean-light-dec")
	}
	if s.cleanEvery(dark) != 0 {
		t.Errorf("down to off: %d", s.cleanEvery(dark))
	}
	if s.stepClean("light-inc") || s.stepClean("clean-light-x") || s.stepClean("clean-every-inc") {
		t.Error("another button stepped the count")
	}
}

// Dark mode at a start: the device's own until chosen in the Light panel, then the choice; a file from before (with
// "dark": true) keeps its dark, one without the key follows the device.
func TestStartDark(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		choice       *bool
		device, want bool
	}{{nil, false, false}, {nil, true, true}, {&yes, false, true}, {&no, true, false}} {
		if got := startDark(c.choice, c.device); got != c.want {
			t.Errorf("chosen %v, device dark %v: dark %v, want %v", c.choice, c.device, got, c.want)
		}
	}
	for file, want := range map[string]*bool{`{"dark": true}`: &yes, `{"dark": false}`: &no, `{}`: nil} {
		var s settings
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if (s.DarkChoice == nil) != (want == nil) || s.DarkChoice != nil && *s.DarkChoice != *want {
			t.Errorf("%s: chosen %v, want %v", file, s.DarkChoice, want)
		}
	}
}
