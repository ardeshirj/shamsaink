package device

import (
	"cmp"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Headphones is the Bluetooth address of the headphones connected ("00:11:22:33:44:55"), "" for none: the bar's
// headphones icon is shown only while there are some, and the narration's volume is kept per pair. On the Kobo it asks
// Kobo's Bluetooth service, mtkbtd, which offers BlueZ's interfaces on the system bus as com.kobo.mtk.bluedroid, through
// Kobo's own dbus-send (10-20 ms): a device carrying sound has a transport, /org/bluez/hci0/dev_ADDRESS/fdN
// (org.bluez.MediaTransport1), gone once it is disconnected, from Kobo's interface or ours, or switched off; measured
// 2026-11-08, with Nickel in front and stopped (docs/devices/kobo/README.md). Asked only while mtkbtd runs: D-Bus starts
// it on demand, so asking would switch Bluetooth's daemon on in a boot where Bluetooth was never used. Never on the
// Kindle (no sound of ours there yet). On the simulator: what DIR/sim/headphones holds (an address), if it exists.
func (d *Device) Headphones() string {
	if Sim() {
		b, err := os.ReadFile(hw("/sim/headphones"))
		if err != nil {
			return ""
		}
		return cmp.Or(strings.TrimSpace(string(b)), "00:00:00:00:00:00")
	}
	if d.Kind != Kobo || !running("mtkbtd") {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "dbus-send", "--system", "--print-reply", "--reply-timeout=1500",
		"--dest=com.kobo.mtk.bluedroid", "/", "org.freedesktop.DBus.ObjectManager.GetManagedObjects").Output()
	if err != nil {
		return ""
	}
	return soundConnected(out)
}

// soundTransport is a sound transport's object path in dbus-send's print of GetManagedObjects, its device's address.
var soundTransport = regexp.MustCompile(`object path "/org/bluez/hci\d+/dev_([0-9A-F_]+)/fd\d+"`)

// soundConnected is the address of the first device with a sound transport in dbus-send's print of the Bluetooth
// service's objects, "" for none.
func soundConnected(out []byte) string {
	m := soundTransport.FindSubmatch(out)
	if m == nil {
		return ""
	}
	return strings.ReplaceAll(string(m[1]), "_", ":")
}

// running says whether a process of that name (its comm, as ps shows it) runs.
func running(name string) bool {
	comms, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, f := range comms {
		if b, err := os.ReadFile(f); err == nil && strings.TrimSpace(string(b)) == name {
			return true
		}
	}
	return false
}
