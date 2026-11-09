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
	"syscall"
	"time"
)

// Headphones, on the Kobo, through Kobo's Bluetooth service, mtkbtd, which offers BlueZ's interfaces on the system bus
// as com.kobo.mtk.bluedroid, asked with Kobo's own dbus-send (10-20 ms; docs/devices/kobo/README.md, 2026-11-08): a
// paired device is /org/bluez/hci0/dev_ADDRESS (org.bluez.Device1), and while it carries sound it has a transport,
// .../fdN (org.bluez.MediaTransport1), gone once it is disconnected or switched off. Asked only while mtkbtd runs:
// D-Bus starts it on demand, so asking would switch Bluetooth's daemon on in a boot where Bluetooth was never used.
// Never on the Kindle (no sound of ours there yet). On the simulator, files under DIR/sim/: headphones (connected:
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

// askPhones asks Kobo's Bluetooth service for the headphones' state, waiting at most wait.
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
	if !running("mtkbtd") { // Bluetooth off in this boot: nothing connected; not asked (D-Bus would start it)
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
// Connect 1 once they carry sound, -1 if they do not. Kobo's service is asked to connect them (0.3-2 s, measured); if
// they are not connected 8 s later, the service is taken to be stuck (as after the chip reset of 2026-11-08: it then
// does nothing, docs/devices/kobo/README.md), restarted, its adapter switched on, and asked once more. With Bluetooth
// off (the service not running) it is started the same way: Bluetooth switched on from the reader. A call while one
// runs does nothing.
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

// DisconnectHeadphones disconnects the headphones at addr in the background (Kobo's service, Device1.Disconnect: gone
// within 4 s, measured 2026-11-08); the change is an Event{Sound}, as any. A call while a connection is under way does
// nothing.
func (d *Device) DisconnectHeadphones(addr string) {
	if !d.connecting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer d.connecting.Store(false)
		if Sim() {
			os.Remove(hw("/sim/headphones"))
		} else if d.Kind == Kobo && running("mtkbtd") {
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

// connect connects addr, the stuck service restarted if need be (ConnectHeadphones): whether it carries sound.
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
	if running("mtkbtd") {
		bluez(8*time.Second, path, "org.bluez.Device1.Connect") // it may answer "connection timeout" and connect anyway
		if connected(8 * time.Second) {
			return true
		}
	}
	restartBluetooth()
	// the fresh service starts with its adapter off: switched on (its own On does nothing); D-Bus starts it on this call
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
		bluez(2*time.Second, "/org/bluez/hci0", "org.freedesktop.DBus.Properties.Set", "string:org.bluez.Adapter1",
			"string:Powered", "variant:boolean:true")
		if out, err := bluez(time.Second, "/org/bluez/hci0", "org.freedesktop.DBus.Properties.Get",
			"string:org.bluez.Adapter1", "string:Powered"); err == nil && bytes.Contains(out, []byte("true")) {
			break
		}
	}
	// its paired devices come a moment later
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
		if out, err := bluez(time.Second, "/", "org.freedesktop.DBus.ObjectManager.GetManagedObjects"); err == nil &&
			bytes.Contains(out, []byte(`"`+path+`"`)) {
			break
		}
	}
	bluez(8*time.Second, path, "org.bluez.Device1.Connect")
	return connected(8 * time.Second)
}

// restartBluetooth stops Kobo's Bluetooth service, mtkbtd (its launcher script first, unless init took it) and its
// btservice, as reader/kobo/launch.sh does before starting Nickel again; the next D-Bus call starts a fresh one.
func restartBluetooth() {
	for _, p := range pids("mtkbtd") {
		if b, err := os.ReadFile("/proc/" + strconv.Itoa(p) + "/stat"); err == nil {
			if f := strings.Fields(string(b)); len(f) > 3 {
				if pp, _ := strconv.Atoi(f[3]); pp > 1 {
					syscall.Kill(pp, syscall.SIGTERM)
				}
			}
		}
		syscall.Kill(p, syscall.SIGTERM)
	}
	for _, p := range pids("btservice") {
		syscall.Kill(p, syscall.SIGTERM)
	}
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end) && (running("mtkbtd") || running("btservice")); {
		time.Sleep(250 * time.Millisecond)
	}
}

// bluez calls method on Kobo's Bluetooth service at object path with args (dbus-send's), waiting at most wait.
func bluez(wait time.Duration, path, method string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), wait+500*time.Millisecond)
	defer cancel()
	a := append([]string{"--system", "--print-reply", "--reply-timeout=" + strconv.Itoa(int(wait/time.Millisecond)),
		"--dest=com.kobo.mtk.bluedroid", path, method}, args...)
	return exec.CommandContext(ctx, "dbus-send", a...).Output()
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

// btObject is an object's path at the start of its entry in dbus-send's print of GetManagedObjects (not a property's
// value, which follows "variant"), a device's address when it is one.
var btObject = regexp.MustCompile(`^\s*object path "/org/bluez/[^"]*"$`)
var btDevice = regexp.MustCompile(`^\s*object path "/org/bluez/hci\d+/dev_([0-9A-F_]+)"$`)

// pairedAudio is the address of the first paired device that plays sound (its Icon "audio-...") in dbus-send's print of
// the Bluetooth service's objects, "" for none.
func pairedAudio(out []byte) string {
	lines := strings.Split(string(out), "\n")
	addr, paired, audio := "", false, false
	for i, l := range lines {
		if btObject.MatchString(l) { // a new object: the one before it done
			if addr != "" && paired && audio {
				return addr
			}
			addr, paired, audio = "", false, false
			if m := btDevice.FindStringSubmatch(l); m != nil {
				addr = strings.ReplaceAll(m[1], "_", ":")
			}
			continue
		}
		if addr == "" || i+1 >= len(lines) {
			continue
		}
		switch strings.TrimSpace(l) {
		case `string "Paired"`:
			paired = strings.HasSuffix(strings.TrimSpace(lines[i+1]), "true")
		case `string "Icon"`:
			audio = strings.Contains(lines[i+1], `"audio-`)
		}
	}
	if addr != "" && paired && audio {
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
