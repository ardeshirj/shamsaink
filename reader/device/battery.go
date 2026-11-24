package device

// The battery and the chargers, read from the kernel's power supplies (/sys/class/power_supply), the same on the Kindle
// and the Kobo; power.go has the kernel's events when they change.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// plugged says whether a charger is in: the battery not discharging, or a charger supply online. Found by what the
// supplies are, not their names: firmware 6 calls the battery "battery" (4.x: bd71827_bat), and its charger supply
// reads online 2 on a computer's USB (2026-11-12: a charging Kobo was not seen, and tried to suspend again and again).
func plugged() bool {
	return chargerOnline() || !batteryIs("Discharging") && batteryKnown()
}

// batteryDetail is " mV=3694 mA=-159" (the voltage and the current, negative while discharging) from the power supply
// that has them, so a drop in the charge can be told from the gauge's own correcting itself; "" if none says.
func batteryDetail() string {
	out := ""
	if v, ok := batteryNum("voltage_now"); ok { // µV
		out += fmt.Sprintf(" mV=%d", v/1000)
	}
	if c, ok := batteryNum("current_now"); ok {
		out += fmt.Sprintf(" mA=%d", c/1000)
	}
	return out
}

// Charging says whether the battery is charging, or a charger is plugged in and the battery not yet full; Full whether it
// is on the charger and full. A charger counts at once: the Kindle's battery says Charging only ~5 s after the cable
// goes in (its chip settling first), while its charger supply (bd71827_ac: Mains on 2026-11-06, USB_DCP on a wall charger 2026-11-21) is online at once, as Amazon's own
// interface shows it (2026-11-06). The Kobo's battery says Charging at once; its Mains supply stays offline.
func (d *Device) Charging() bool {
	return batteryIs("Charging") || chargerOnline() && !d.Full()
}

// chargerOnline says whether a power supply that charges is online: anything but 0 (the Kobo on firmware 6 reads 2 on
// a computer's USB). Every type of charger the kernel names counts (chargerTypes), not the battery itself, which reads
// online all the time, nor the Kindle's moisture sensor (max20342_moisture, USB_WET).
func chargerOnline() bool {
	files, _ := filepath.Glob(hw("/sys/class/power_supply/*/online"))
	for _, f := range files {
		dir := filepath.Dir(f)
		t, _ := os.ReadFile(filepath.Join(dir, "type"))
		if slices.Contains(chargerTypes, strings.TrimSpace(string(t))) {
			if b, err := os.ReadFile(f); err == nil && strings.TrimSpace(string(b)) != "0" && strings.TrimSpace(string(b)) != "" {
				return true
			}
		}
	}
	return false
}

// chargerTypes are the kernel's types of a supply that charges (power_supply's sysfs ABI). The Kindle's charger reads
// USB_DCP on a wall charger (2026-11-21; Mains on 2026-11-06): only Mains, USB and Wireless counted until then, so a
// full battery on it, which reads Discharging, showed no charging glyph.
var chargerTypes = []string{"Mains", "USB", "USB_DCP", "USB_CDP", "USB_ACA", "USB_C", "USB_PD", "USB_PD_DRP", "BrickID", "Wireless"}

// Full says whether the battery is full on the charger: it says Full, or a charger is online and the battery at 100%
// is not charging (the Kindle's then says Discharging, 2026-11-21).
func (d *Device) Full() bool {
	return batteryIs("Full") || chargerOnline() && !batteryIs("Charging") && d.Battery() >= 100
}

// batteryKnown says whether a battery reports its status at all.
func batteryKnown() bool {
	_, ok := batteryText("status")
	return ok
}

// batteryIs says whether the battery's status is status.
func batteryIs(status string) bool {
	s, ok := batteryText("status")
	return ok && s == status
}

// Battery is the battery's charge in per cent, from the kernel's power supplies (the first that has one); -1 if none
// says.
func (d *Device) Battery() int {
	if n, ok := batteryNum("capacity"); ok {
		return n
	}
	return -1
}

// batteryText is the file name of the battery's power supply (one with a capacity: the battery's own, not the
// charger's), trimmed, from the first that has it; false if none does.
func batteryText(name string) (string, bool) {
	caps, _ := filepath.Glob(hw("/sys/class/power_supply/*/capacity"))
	for _, c := range caps {
		if b, err := os.ReadFile(filepath.Join(filepath.Dir(c), name)); err == nil {
			return strings.TrimSpace(string(b)), true
		}
	}
	return "", false
}

// batteryNum is batteryText as a number, from the first supply whose file is one.
func batteryNum(name string) (int, bool) {
	caps, _ := filepath.Glob(hw("/sys/class/power_supply/*/capacity"))
	for _, c := range caps {
		if b, err := os.ReadFile(filepath.Join(filepath.Dir(c), name)); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}
