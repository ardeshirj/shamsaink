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
