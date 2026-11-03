package main

import (
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
	s := defaults()
	if s.cleanEvery() != 0 {
		t.Errorf("light mode starts at %d, not off", s.cleanEvery())
	}
	s.Dark = true
	if s.cleanEvery() != 5 {
		t.Errorf("dark mode starts at %d, not 5", s.cleanEvery())
	}
	s.stepClean("clean-dark-inc")
	s.stepClean("clean-dark-inc")
	if s.cleanEvery() != 20 || s.CleanLight != 0 {
		t.Errorf("two steps up in dark: dark %d (want 20), light %d (want 0, untouched)", s.cleanEvery(), s.CleanLight)
	}
	s.stepClean("clean-dark-inc") // the end stays
	if s.cleanEvery() != 20 {
		t.Errorf("past the end: %d", s.cleanEvery())
	}
	s.Dark = false
	s.stepClean("clean-light-inc")
	s.stepClean("clean-light-inc")
	if s.CleanLight != 5 || s.CleanDark != 20 {
		t.Errorf("two steps up in light: light %d (want 5), dark %d (want 20, untouched)", s.CleanLight, s.CleanDark)
	}
	for i := 0; i < 9; i++ {
		s.stepClean("clean-light-dec")
	}
	if s.cleanEvery() != 0 {
		t.Errorf("down to off: %d", s.cleanEvery())
	}
	if s.stepClean("light-inc") || s.stepClean("clean-light-x") || s.stepClean("clean-every-inc") {
		t.Error("another button stepped the count")
	}
}
