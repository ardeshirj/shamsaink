package device

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Wi-Fi on demand (2026-11-20; the trial in docs/devices/kobo/README.md): the reader has no Wi-Fi switch. Wi-Fi is
// left as the Kobo has it; an action that needs the network asks first when it is off (the reader's dialog), and then
// ConnectWiFi brings it up the way Kobo's interface does: the chip on through its driver, wlan0 up, wpa_supplicant
// with the networks Kobo saved (/mnt/onboard/.kobo/wpa_supplicant.conf) and dhcpcd, each started only if not running.
// ReleaseWiFi undoes only what ConnectWiFi started, so Kobo's interface finds Wi-Fi as it left it. Kobo's "Wi-Fi off"
// stops both programs and switches the chip off (its driver's module stays loaded); off to an address took 2.8 s in
// the trial. Never in the background: nothing turns Wi-Fi on without the person's yes.
//
// The Kindle (2026-11-20): Amazon's own Wi-Fi service (cmd and wifid) keeps running while the reader has the framework
// paused, and does the work itself: com.lab126.cmd's wirelessEnable, set to 1, joins a network saved in the Kindle's
// settings (2.2 s to an address in the trial), 0 disconnects; it starts and stops wpa_supplicant and udhcpc itself. So
// ConnectWiFi sets it to 1 and ReleaseWiFi back to 0 if it was 0; the rest is as on the Kobo.

// WiFi is Wi-Fi's state, as the reader asks it before an action that needs the network.
type WiFi int

const (
	WiFiNone WiFi = iota // no Wi-Fi of ours on this device (the Kindle until 2026-11-20)
	WiFiOff              // off: an action asks before turning it on
	WiFiOn               // on (Kobo's switch on), maybe still joining: an action just runs
)

// The end of a connection asked for (Event.WiFi).
const (
	WiFiConnected = 1  // an address: WiFiAddress
	WiFiNoNetwork = -1 // no network saved: one must be joined in the device's settings first (nothing was changed)
	WiFiNoAddress = -2 // no address in wifiWait (what was turned on is off again)
)

const (
	wpaConf  = "/mnt/onboard/.kobo/wpa_supplicant.conf" // the networks Kobo's interface saved
	wifiChip = "/dev/wmtWifi"                           // the chip's Wi-Fi, on (1) and off (0), as Kobo switches it
	wifiWait = 20 * time.Second                         // for an address
)

// What ConnectWiFi started, for ReleaseWiFi; and whether a connection is under way.
type wifiOurs struct {
	sync.Mutex
	busy             bool
	chip, wpa, dhcpd bool
	addr             string
}

// WiFiState is Wi-Fi's state now: on when wpa_supplicant runs and wlan0 is there (Kobo's switch on). The simulated
// device's is the file sim/wifi ("on", or "off", "off no-network", "off no-address" for the failures; none: off).
func (d *Device) WiFiState() WiFi {
	switch {
	case Sim():
		if strings.HasPrefix(simWiFi(), "on") {
			return WiFiOn
		}
		return WiFiOff
	case d.Kind == Kindle:
		if kindleWiFiEnabled() {
			return WiFiOn
		}
		return WiFiOff
	case d.Kind != Kobo:
		return WiFiNone
	case running("wpa_supplicant") && exists("/sys/class/net/wlan0"):
		return WiFiOn
	}
	return WiFiOff
}

// WiFiAddress is Wi-Fi's address now ("" without one; the simulated device's, the last connection's).
func (d *Device) WiFiAddress() string {
	if Sim() {
		d.wifi.Lock()
		defer d.wifi.Unlock()
		if d.WiFiState() != WiFiOn {
			return ""
		}
		return d.wifi.addr
	}
	return address()
}

// TurnOffWiFi turns Wi-Fi off whoever turned it on, as Kobo's own switch does: dhcpcd and wpa_supplicant stopped,
// wlan0 down, the chip off. For the Developer panel's switch only (the person's own off); an action that turned Wi-Fi
// on for itself uses ReleaseWiFi.
func (d *Device) TurnOffWiFi() {
	d.wifi.Lock()
	defer d.wifi.Unlock()
	switch {
	case Sim():
		os.WriteFile(hw("/sim/wifi"), []byte("off\n"), 0o644)
	case d.Kind == Kobo:
		exec.Command("dhcpcd", "-x").Run()
		exec.Command("wpa_cli", "-i", "wlan0", "terminate").Run()
		wifiChipOff()
	case d.Kind == Kindle:
		kindleWiFi(false)
	default:
		return
	}
	d.wifi.chip, d.wifi.wpa, d.wifi.dhcpd = false, false, false
	fmt.Println("wi-fi: turned off")
}

// WiFiBusy says whether a connection is under way.
func (d *Device) WiFiBusy() bool {
	d.wifi.Lock()
	defer d.wifi.Unlock()
	return d.wifi.busy
}

// ConnectWiFi brings Wi-Fi up in the background if it is off (asked first: the caller's dialog), or waits for an address
// if it is on; its end is an Event{WiFi} (WiFiConnected, WiFiNoNetwork, WiFiNoAddress). Logs each step and its time.
// A call while one is under way does nothing.
func (d *Device) ConnectWiFi() {
	d.wifi.Lock()
	if d.wifi.busy {
		d.wifi.Unlock()
		return
	}
	d.wifi.busy = true
	d.wifi.Unlock()
	go func() {
		r := d.connectWiFi()
		d.wifi.Lock()
		d.wifi.busy = false
		d.wifi.Unlock()
		if out := d.events.Load(); out != nil {
			select {
			case *out <- Event{WiFi: r}:
			case <-time.After(5 * time.Second): // nobody reading: the reader has left
			}
		}
	}()
}

func (d *Device) connectWiFi() int {
	t := time.Now()
	if Sim() {
		time.Sleep(300 * time.Millisecond)
		switch s := simWiFi(); {
		case strings.HasPrefix(s, "on"):
		case strings.Contains(s, "no-network"):
			fmt.Println("wi-fi: no network saved")
			return WiFiNoNetwork
		case strings.Contains(s, "no-address"):
			fmt.Println("wi-fi: no address in 20 s (simulated): turned off again")
			return WiFiNoAddress
		default:
			d.wifi.Lock()
			d.wifi.chip = true
			d.wifi.Unlock()
			os.WriteFile(hw("/sim/wifi"), []byte("on\n"), 0o644)
		}
		d.wifi.Lock()
		d.wifi.addr = "192.0.2.10" // the documentation range
		d.wifi.Unlock()
		fmt.Printf("wi-fi: address 192.0.2.10 in %.1f s (simulated)\n", time.Since(t).Seconds())
		return WiFiConnected
	}
	if d.Kind == Kindle && d.WiFiState() == WiFiOff {
		if kindleSavedNetworks() == 0 {
			fmt.Println("wi-fi: no network saved")
			return WiFiNoNetwork
		}
		d.wifi.Lock()
		d.wifi.chip = true
		d.wifi.Unlock()
		kindleWiFi(true)
		fmt.Printf("wi-fi: turned on in %.1f s (Amazon's wirelessEnable)\n", time.Since(t).Seconds())
	}
	if d.Kind == Kobo && d.WiFiState() == WiFiOff {
		if savedNetworks() == 0 {
			fmt.Println("wi-fi: no network saved")
			return WiFiNoNetwork
		}
		d.wifi.Lock()
		ours := &d.wifi
		ours.chip = true
		if !exists("/sys/class/net/wlan0") {
			wifiChipOn()
		}
		exec.Command("ifconfig", "wlan0", "up").Run()
		if !running("wpa_supplicant") {
			// as Kobo's interface starts it (seen in its process list, firmware 6)
			ours.wpa = exec.Command("wpa_supplicant", "-D", "nl80211", "-s", "-i", "wlan0", "-c", wpaConf, "-B").Run() == nil
		}
		if !running("dhcpcd") {
			ours.dhcpd = true
			// in the background: it returns once it has an address (or gives up after wifiWait)
			ctx, cancel := context.WithTimeout(context.Background(), wifiWait+2*time.Second)
			go func() { defer cancel(); exec.CommandContext(ctx, "dhcpcd", "-q", "-t", "20", "wlan0").Run() }()
		}
		d.wifi.Unlock()
		fmt.Printf("wi-fi: turned on in %.1f s (chip, wlan0, wpa_supplicant %v, dhcpcd %v)\n", time.Since(t).Seconds(), ours.wpa, ours.dhcpd)
	}
	for time.Since(t) < wifiWait {
		if a := address(); a != "" {
			d.wifi.Lock()
			d.wifi.addr = a
			d.wifi.Unlock()
			fmt.Printf("wi-fi: address %s in %.1f s\n", a, time.Since(t).Seconds())
			return WiFiConnected
		}
		time.Sleep(250 * time.Millisecond)
	}
	fmt.Println("wi-fi: no address in 20 s")
	d.ReleaseWiFi()
	return WiFiNoAddress
}

// ReleaseWiFi turns off what ConnectWiFi turned on (nothing when Wi-Fi was on already): dhcpcd and wpa_supplicant
// stopped if it started them, then wlan0 down and the chip off, as Kobo's own "off" leaves it. It says whether it
// turned anything off.
func (d *Device) ReleaseWiFi() bool {
	d.wifi.Lock()
	defer d.wifi.Unlock()
	o := &d.wifi
	if !o.chip {
		return false
	}
	if Sim() {
		os.WriteFile(hw("/sim/wifi"), []byte("off\n"), 0o644)
	} else if d.Kind == Kindle {
		kindleWiFi(false)
	} else {
		if o.dhcpd {
			exec.Command("dhcpcd", "-x").Run()
		}
		if o.wpa {
			exec.Command("wpa_cli", "-i", "wlan0", "terminate").Run()
		}
		wifiChipOff()
	}
	o.chip, o.wpa, o.dhcpd = false, false, false
	fmt.Println("wi-fi: turned off again (as it was)")
	return true
}

// wifiChipOn switches the chip's Wi-Fi on and waits up to 10 s for wlan0 to be there.
func wifiChipOn() error {
	if err := sysfs(wifiChip, "1"); err != nil {
		return err
	}
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(250 * time.Millisecond) {
		if exists("/sys/class/net/wlan0") {
			return nil
		}
	}
	return fmt.Errorf("no wlan0 10 s after the chip was switched on")
}

// wifiChipOff takes wlan0 down and switches the chip's Wi-Fi off.
func wifiChipOff() {
	exec.Command("ifconfig", "wlan0", "down").Run()
	sysfs(wifiChip, "0")
}

// savedNetworks counts the networks Kobo's interface saved (their network= blocks; nothing else is read).
func savedNetworks() int {
	f, err := os.Open(hw(wpaConf))
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	for s := bufio.NewScanner(f); s.Scan(); {
		if strings.HasPrefix(strings.TrimSpace(s.Text()), "network=") {
			n++
		}
	}
	return n
}

// simWiFi is the simulated device's Wi-Fi (sim/wifi), "" when there is none.
func simWiFi() string {
	b, _ := os.ReadFile(hw("/sim/wifi"))
	return strings.TrimSpace(string(b))
}

func exists(path string) bool {
	_, err := os.Stat(hw(path))
	return err == nil
}

// kindleWiFiEnabled says whether the Kindle's Wi-Fi is on (Amazon's wirelessEnable, ~40 ms to ask).
func kindleWiFiEnabled() bool {
	out, err := exec.Command("lipc-get-prop", "com.lab126.cmd", "wirelessEnable").Output()
	return err == nil && strings.TrimSpace(string(out)) == "1"
}

// kindleWiFi turns the Kindle's Wi-Fi on or off through Amazon's service, which joins a saved network itself.
func kindleWiFi(on bool) {
	v := "0"
	if on {
		v = "1"
	}
	if err := exec.Command("lipc-set-prop", "com.lab126.cmd", "wirelessEnable", v).Run(); err != nil {
		fmt.Println("wi-fi: wirelessEnable", v, "failed:", err)
	}
}

// kindleSavedNetworks counts the networks saved in the Kindle's settings, from wifid's diagnostic text (its property
// 711, section "3- Networks": a line "3.N" each; nothing else of it is read). Its shape is Amazon's: when the section
// is not found, one is assumed, so a connection is tried and gives up after wifiWait as anywhere.
func kindleSavedNetworks() int {
	out, err := exec.Command("lipc-get-prop", "com.lab126.wifid", "711").Output()
	if err != nil || !strings.Contains(string(out), "- Networks") {
		return 1
	}
	return kindleNetworks(string(out))
}

// kindleNetworks counts the "3.N" lines of wifid's diagnostic text.
func kindleNetworks(text string) int {
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); len(l) > 2 && strings.HasPrefix(l, "3.") && l[2] >= '0' && l[2] <= '9' {
			n++
		}
	}
	return n
}

// online says whether wlan0 is up with an IPv4 address.
func online() bool { return address() != "" }

// address is wlan0's IPv4 address while it is up, else "".
func address() string {
	state, _ := os.ReadFile(hw(wifiState))
	if strings.TrimSpace(string(state)) != "up" {
		return ""
	}
	i, err := net.InterfaceByName("wlan0")
	if err != nil {
		return ""
	}
	addrs, _ := i.Addrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
			return n.IP.String()
		}
	}
	return ""
}
