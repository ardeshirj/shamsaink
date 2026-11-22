package device

import (
	"os"
	"path/filepath"
	"testing"
)

// supplies writes power supplies as the kernel shows them: name → type, online, status (empty: no such file).
func supplies(t *testing.T, s map[string][3]string) {
	t.Helper()
	old := simDir
	simDir = t.TempDir()
	t.Cleanup(func() { simDir = old })
	for name, v := range s {
		d := filepath.Join(simDir, "sys/class/power_supply", name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{"type": v[0], "online": v[1], "status": v[2]}
		if v[0] == "Battery" {
			files["capacity"] = "46"
		}
		for f, c := range files {
			if c != "" {
				os.WriteFile(filepath.Join(d, f), []byte(c+"\n"), 0o644)
			}
		}
	}
}

// What the Kobo's supplies read (2026-11-12, firmware 6.0.279225; 4.x's names before): a charger in is seen whatever
// the battery is called, and a computer's USB reads online 2.
func TestPlugged(t *testing.T) {
	for _, c := range []struct {
		name string
		s    map[string][3]string
		want bool
	}{
		{"6.0, on a computer's USB", map[string][3]string{"ac": {"Mains", "2", ""}, "battery": {"Battery", "1", "Charging"}}, true},
		{"6.0, unplugged", map[string][3]string{"ac": {"Mains", "0", ""}, "battery": {"Battery", "1", "Discharging"}}, false},
		{"6.0, full on the charger", map[string][3]string{"ac": {"Mains", "1", ""}, "battery": {"Battery", "1", "Full"}}, true},
		{"4.x, charging", map[string][3]string{"bd71827_bat": {"Battery", "", "Charging"}}, true},
		{"4.x, unplugged", map[string][3]string{"bd71827_bat": {"Battery", "", "Discharging"}}, false},
		{"no supplies", map[string][3]string{}, false},
	} {
		supplies(t, c.s)
		if got := plugged(); got != c.want {
			t.Errorf("%s: plugged() = %v, want %v", c.name, got, c.want)
		}
	}
}

// What the Kindle's supplies read on a wall charger (2026-11-21, firmware 5.19.2): the charger USB_DCP, a moisture
// sensor USB_WET, and the battery Charging until 100%, then Discharging: the charging glyph (Charging or Full) all along.
func TestKindleCharging(t *testing.T) {
	d := &Device{}
	for _, c := range []struct {
		name           string
		s              map[string][3]string
		capacity       string
		charging, full bool
	}{
		{"charging", map[string][3]string{"bd71827_ac": {"USB_DCP", "1", ""}, "bd71827_bat": {"Battery", "1", "Charging"}, "max20342_moisture": {"USB_WET", "0", ""}}, "85", true, false},
		{"full, on the charger", map[string][3]string{"bd71827_ac": {"USB_DCP", "1", ""}, "bd71827_bat": {"Battery", "1", "Discharging"}, "max20342_moisture": {"USB_WET", "0", ""}}, "100", false, true},
		{"full, unplugged", map[string][3]string{"bd71827_ac": {"USB_DCP", "0", ""}, "bd71827_bat": {"Battery", "1", "Discharging"}}, "100", false, false},
		{"the cable just in", map[string][3]string{"bd71827_ac": {"USB_DCP", "1", ""}, "bd71827_bat": {"Battery", "1", "Discharging"}}, "60", true, false},
		{"wet, unplugged", map[string][3]string{"bd71827_ac": {"USB_DCP", "0", ""}, "bd71827_bat": {"Battery", "1", "Discharging"}, "max20342_moisture": {"USB_WET", "1", ""}}, "60", false, false},
	} {
		supplies(t, c.s)
		os.WriteFile(filepath.Join(simDir, "sys/class/power_supply/bd71827_bat/capacity"), []byte(c.capacity+"\n"), 0o644)
		if got := d.Charging(); got != c.charging {
			t.Errorf("%s: Charging() = %v, want %v", c.name, got, c.charging)
		}
		if got := d.Full(); got != c.full {
			t.Errorf("%s: Full() = %v, want %v", c.name, got, c.full)
		}
		if got := plugged(); got != (c.charging || c.full) {
			t.Errorf("%s: plugged() = %v", c.name, got)
		}
	}
}
