package device

import (
	"bytes"
	"cmp"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Headphones, on the Kobo (firmware 6), through BlueZ itself: bluetoothd, started at boot, on the system bus as
// org.bluez, asked with Kobo's own dbus-send (docs/devices/kobo/README.md, 2026-11-11; firmware 4.x had Kobo's mtkbtd
// with BlueZ's interfaces, the tag kobo-fw4): a paired device is /org/bluez/hci0/dev_ADDRESS (org.bluez.Device1), and
// while it carries sound it has a transport, .../fdN (org.bluez.MediaTransport1), gone once it is disconnected or
// switched off; its sound goes through PulseAudio (SoundSink). Never on the Kindle (no sound of ours there yet). On the simulator, files under DIR/sim/: headphones (connected:
// the address it holds), connect-fails (Connect fails).

// Phones is the headphones' state: whether the device has Bluetooth sound (the Kobo: the bar's headphones are there,
// Bluetooth switched on by Connect if it is off), the address of the pair connected ("" for none), and a pair of
// headphones paired ("" for none, or not known while Bluetooth is off: one to connect when none is remembered).
type Phones struct {
	On   bool
	Addr string
	Pair string
}

// Phones is the headphones' state as last seen, at once: the bar never waits on Bluetooth (a stuck chip once held a
// menu 7.9 s). Each call also asks again in the background, one at a time; when the answer differs, an Event{Sound}
// says so (Inputs' channel), and the bar is drawn again.
func (d *Device) Phones() Phones {
	var p Phones
	if q := d.phones.Load(); q != nil {
		p = *q
	}
	p.On = d.hasPhones() // known at once: the icon never waits for the first look
	if !p.On {
		p = Phones{}
	}
	if d.phonesBusy.CompareAndSwap(false, true) {
		go func() {
			defer d.phonesBusy.Store(false)
			d.notePhones(d.askPhones(500*time.Millisecond), 0)
		}()
	}
	return p
}

// Headphones is the address of the headphones connected, "" for none, asked now (up to 2 s): for the narration's volume,
// kept per pair.
func (d *Device) Headphones() string {
	p := d.askPhones(2 * time.Second)
	d.notePhones(p, 0)
	return p.Addr
}

// notePhones keeps p as the state last seen; when it changed, or a connection asked for ended (connect: 1 connected, -1
// failed), an Event{Sound} says so.
func (d *Device) notePhones(p Phones, connect int) {
	old := d.phones.Swap(&p)
	same := old == nil && p == Phones{} || old != nil && *old == p
	if same && connect == 0 {
		return
	}
	if out := d.events.Load(); out != nil {
		select {
		case *out <- Event{Sound: true, Connect: connect}:
		default: // the channel full: the next look will tell
		}
	}
}

// hasPhones says whether the device has Bluetooth sound: the Kobo, the simulated one too (a simulated Kobo Libra
// Colour: the design's walk taps the bar's headphones wherever the spec has sound, 2026-11-08).
func (d *Device) hasPhones() bool { return d.Kind == Kobo || Sim() }

// askPhones asks BlueZ for the headphones' state, waiting at most wait.
func (d *Device) askPhones(wait time.Duration) Phones {
	if Sim() { // a pair paired; connected while DIR/sim/headphones exists (its address)
		if b, err := os.ReadFile(hw("/sim/headphones")); err == nil {
			return Phones{On: true, Addr: cmp.Or(strings.TrimSpace(string(b)), simPair), Pair: simPair}
		}
		return Phones{On: true, Pair: simPair}
	}
	if d.Kind != Kobo {
		return Phones{}
	}
	if !running("bluetoothd") {
		return Phones{On: true}
	}
	out, err := bluez(wait, "/", "org.freedesktop.DBus.ObjectManager.GetManagedObjects")
	if err != nil {
		return Phones{On: true}
	}
	return Phones{On: true, Addr: soundConnected(out), Pair: pairedAudio(out)}
}

// simPair is the simulated headphones' address.
const simPair = "00:00:00:00:00:00"

// ConnectHeadphones connects the paired headphones at addr in the background, and sends an Event{Sound} when it ends:
// Connect 1 once they carry sound, -1 if they do not. BlueZ's adapter is switched on first if it is off (Bluetooth
// switched on from the reader), then the device asked to connect, and given 8 s. A call while one runs does nothing.
func (d *Device) ConnectHeadphones(addr string) {
	if !d.connecting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer d.connecting.Store(false)
		ok := d.connect(addr)
		d.notePhones(d.askPhones(2*time.Second), map[bool]int{true: 1, false: -1}[ok])
	}()
}

// DisconnectHeadphones disconnects the headphones at addr in the background (Device1.Disconnect; on 4.x gone within
// 4 s, measured 2026-11-08); the change is an Event{Sound}, as any. A call while a connection is under way does
// nothing.
func (d *Device) DisconnectHeadphones(addr string) {
	if !d.connecting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer d.connecting.Store(false)
		if Sim() {
			os.Remove(hw("/sim/headphones"))
		} else if d.Kind == Kobo && running("bluetoothd") {
			bluez(5*time.Second, "/org/bluez/hci0/dev_"+strings.ReplaceAll(addr, ":", "_"), "org.bluez.Device1.Disconnect")
			for end := time.Now().Add(6 * time.Second); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
				if d.askPhones(time.Second).Addr != addr {
					break
				}
			}
		}
		d.notePhones(d.askPhones(2*time.Second), 0)
	}()
}

// Connecting says whether ConnectHeadphones (or DisconnectHeadphones) is at work.
func (d *Device) Connecting() bool { return d.connecting.Load() }

// connect connects addr, the adapter switched on if need be (ConnectHeadphones): whether it carries sound.
func (d *Device) connect(addr string) bool {
	if Sim() {
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(hw("/sim/connect-fails")); err == nil {
			return false
		}
		return os.WriteFile(hw("/sim/headphones"), []byte(addr), 0o644) == nil
	}
	if d.Kind != Kobo {
		return false
	}
	path := "/org/bluez/hci0/dev_" + strings.ReplaceAll(addr, ":", "_")
	connected := func(within time.Duration) bool {
		for end := time.Now().Add(within); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
			if d.askPhones(time.Second).Addr == addr {
				return true
			}
		}
		return false
	}
	if !running("bluetoothd") {
		return false
	}
	// the adapter on (Bluetooth switched off in Kobo's settings), its paired devices there a moment later
	if out, err := bluez(time.Second, "/org/bluez/hci0", "org.freedesktop.DBus.Properties.Get", "string:org.bluez.Adapter1",
		"string:Powered"); err != nil || !bytes.Contains(out, []byte("true")) {
		bluez(2*time.Second, "/org/bluez/hci0", "org.freedesktop.DBus.Properties.Set", "string:org.bluez.Adapter1",
			"string:Powered", "variant:boolean:true")
		for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
			if out, err := bluez(time.Second, "/", "org.freedesktop.DBus.ObjectManager.GetManagedObjects"); err == nil &&
				bytes.Contains(out, []byte(`"`+path+`"`)) {
				break
			}
		}
	}
	bluez(8*time.Second, path, "org.bluez.Device1.Connect") // it may answer "connection timeout" and connect anyway
	return connected(8 * time.Second)
}

// bluetoothAsleep switches Bluetooth's part of the chip off for a suspend, as Nickel does before it sleeps (firmware 6:
// 0 written to /dev/wmtBluetooth, the driver's "power-disable request", hci0 unregistered), and says whether it was
// on. Left on, the kernel's suspend switched it off under Kobo's sound and speech services, whose next writes to the
// chip timed out: its firmware crashed, and Nickel found no Wi-Fi network after; BlueZ's Powered false, or rfkill,
// only take the adapter down, and the kernel still switched it off itself (2026-11-12).
func bluetoothAsleep() bool {
	if Sim() || !running("bluetoothd") {
		return false
	}
	if _, err := os.Stat("/sys/class/bluetooth/hci0"); err != nil {
		return false
	}
	return sysfs(btPower, "0") == nil
}

// bluetoothAwake switches it on again after a suspend (hci0 back in ~2 s, BlueZ's adapter powered as before); paired
// headphones reconnect by themselves or from the Sound panel.
func bluetoothAwake() {
	sysfs(btPower, "1")
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(250 * time.Millisecond) {
		if _, err := os.Stat("/sys/class/bluetooth/hci0"); err == nil {
			return
		}
	}
}

// btPower is the Bluetooth driver's power switch (MediaTek's WMT, as /dev/wmtWifi for Wi-Fi).
const btPower = "/dev/wmtBluetooth"

// bluez calls method on BlueZ at object path with args (dbus-send's), waiting at most wait.
func bluez(wait time.Duration, path, method string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wait+500*time.Millisecond)
	defer cancel()
	a := append([]string{"--system", "--print-reply", "--reply-timeout=" + strconv.Itoa(int(wait/time.Millisecond)),
		"--dest=org.bluez", path, method}, args...)
	return exec.CommandContext(ctx, "dbus-send", a...).Output()
}

// soundConnected is the address of the first connected device that plays sound (its Icon "audio-...") in dbus-send's
// print of BlueZ's objects, "" for none. Connected, not its sound transport: on firmware 6 the transport is made only
// once sound flows (a Connect: Connected and PulseAudio's sink for it after 1.0 s, the transport not in 35 s;
// 2026-11-11), so waiting for it took a connection that worked for one that failed.
func soundConnected(out []byte) string { return audioDevice(out, "Connected") }

// pairedAudio is the address of the first paired device that plays sound in dbus-send's print of BlueZ's objects, ""
// for none.
func pairedAudio(out []byte) string { return audioDevice(out, "Paired") }

// btObject is an object's path at the start of its entry in dbus-send's print of GetManagedObjects (not a property's
// value, which follows "variant"), a device's address when it is one.
var btObject = regexp.MustCompile(`^\s*object path "/org/bluez/[^"]*"$`)
var btDevice = regexp.MustCompile(`^\s*object path "/org/bluez/hci\d+/dev_([0-9A-F_]+)"$`)

// audioDevice is the address of the first device that plays sound (its Icon "audio-...") whose boolean property prop
// (Paired, Connected) is true, in dbus-send's print of BlueZ's objects; "" for none.
func audioDevice(out []byte, prop string) string {
	lines := strings.Split(string(out), "\n")
	addr, yes, audio := "", false, false
	for i, l := range lines {
		if btObject.MatchString(l) { // a new object: the one before it done
			if addr != "" && yes && audio {
				return addr
			}
			addr, yes, audio = "", false, false
			if m := btDevice.FindStringSubmatch(l); m != nil {
				addr = strings.ReplaceAll(m[1], "_", ":")
			}
			continue
		}
		if addr == "" || i+1 >= len(lines) {
			continue
		}
		switch strings.TrimSpace(l) {
		case `string "` + prop + `"`:
			yes = strings.HasSuffix(strings.TrimSpace(lines[i+1]), "true")
		case `string "Icon"`:
			audio = strings.Contains(lines[i+1], `"audio-`)
		}
	}
	if addr != "" && yes && audio {
		return addr
	}
	return ""
}

// running says whether a process of that name (its comm, as ps shows it) runs.
func running(name string) bool { return len(pids(name)) > 0 }

// pids are the processes of that name (their comm).
func pids(name string) []int {
	var ps []int
	comms, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, f := range comms {
		if b, err := os.ReadFile(f); err == nil && strings.TrimSpace(string(b)) == name {
			if p, err := strconv.Atoi(filepath.Base(filepath.Dir(f))); err == nil {
				ps = append(ps, p)
			}
		}
	}
	return ps
}
