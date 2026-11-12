package device

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Sleep on the Kobo, where Kobo's interface is stopped while a book is open (13's NOTES.md), so the power button and
// sleep are the reader's: a press shows the sleep screen and suspends the Kobo, the next press wakes it and puts back
// what the screen showed. As KOReader and Plato suspend a Kobo (their code read, not copied): Wi-Fi down first (a
// suspend with it up fails or locks the kernel), the light off, 1 written to /sys/power/state-extended (the drivers
// told) and 2 s let pass, then "mem" to /sys/power/state, a write that returns once the Kobo wakes. On MediaTek Kobos
// a suspend while plugged in hangs the kernel, so then the Kobo stays awake behind the sleep screen until unplugged
// or pressed again. A wake that is not a press (the charger, the battery's chip) goes back to sleep after 15 s, at
// most 20 times in a row. On the Kindle sleep is powerd's (kindle.go).
const (
	powerDev = "/dev/input/event3" // bd71828-pwrkey
	evKey    = 1
	keyPower = 116
	// the white light, 0 to 100 (KOReader's): set in brightness, which always reads 0; its level in actual_brightness
	lightFile     = "/sys/class/backlight/mxc_msp430.0/brightness"
	lightLevel    = "/sys/class/backlight/mxc_msp430.0/actual_brightness"
	batteryStatus = "/sys/class/power_supply/bd71827_bat/status"
	// this Kobo's Wi-Fi (wlan_drv_gen4m), as the launcher takes it down for Kobo's interface
	wifiOff = "killall -q -TERM dhcpcd udhcpc; wpa_cli -i wlan0 terminate; ifconfig wlan0 down; echo 0 > /dev/wmtWifi"
	// and up again on waking, its drivers still loaded: the Wi-Fi function cycled through MediaTek's wmt_dbg (as
	// Nickel's own log shows it, and KOReader's enable-wifi.sh), powered, then wpa_supplicant and dhcpcd as Nickel
	// runs them (its known networks in /etc/wpa_supplicant)
	wifiOn = "echo 0xDB9DB9 > /proc/driver/wmt_dbg; echo '7 9 0' > /proc/driver/wmt_dbg; sleep 1; " +
		"echo 0xDB9DB9 > /proc/driver/wmt_dbg; echo '7 9 1' > /proc/driver/wmt_dbg; echo 1 > /dev/wmtWifi; sleep 1; " +
		"ifconfig wlan0 up && wpa_supplicant -D nl80211 -s -i wlan0 -c /etc/wpa_supplicant/wpa_supplicant.conf " +
		"-C /var/run/wpa_supplicant -B && dhcpcd -d -t 30 -w wlan0"
	wifiState = "/sys/class/net/wlan0/operstate"
)

// presses sends each press of the power button to out, or to the doze while the Kobo dozes (the press that wakes it),
// until the file is closed (not opened: nothing sent).
func (d *Device) presses(out chan<- Event) io.Closer {
	f, err := openInput(hw(powerDev))
	if err != nil {
		return io.NopCloser(nil)
	}
	go func() {
		var ev [16]byte // struct input_event on 32-bit ARM: timeval (8), type, code (2 each), value (4)
		for {
			if _, err := io.ReadFull(f, ev[:]); err != nil {
				return
			}
			typ, code := binary.LittleEndian.Uint16(ev[8:]), binary.LittleEndian.Uint16(ev[10:])
			if typ == evKey && code == keyPower && binary.LittleEndian.Uint32(ev[12:]) == 1 {
				if d.dozing.Load() {
					select {
					case d.wake <- struct{}{}:
					default:
					}
					continue
				}
				select {
				case out <- Event{Sleep: true}:
				default:
				}
			}
		}
	}()
	return f
}

// The page buttons are keys of the "gpio-keys" input: 193 the upper, 194 the lower (the Kobo Libra's side buttons), a press
// 1 and its release 0, and 2 repeated while it is held (a hold is not used yet). Found by name, not by number.
const (
	keyUpper = 193
	keyLower = 194
)

// buttonsDev is the input node of the page buttons, "" if there is none.
func buttonsDev() string {
	b, err := os.ReadFile(hw("/proc/bus/input/devices"))
	if err != nil {
		return ""
	}
	if n := buttonsNodeIn(string(b)); n != "" {
		return hw(n)
	}
	return ""
}

// buttonsNodeIn is buttonsDev for the text of /proc/bus/input/devices.
func buttonsNodeIn(list string) string {
	for _, block := range strings.Split(list, "\n\n") {
		if !strings.Contains(block, `Name="gpio-keys"`) {
			continue
		}
		for _, f := range strings.Fields(block) { // "H: Handlers=event0 perfmgr"
			if _, node, ok := strings.Cut(f, "Handlers="); ok && strings.HasPrefix(node, "event") {
				return "/dev/input/" + node
			}
			if strings.HasPrefix(f, "event") {
				return "/dev/input/" + f
			}
		}
	}
	return ""
}

// buttons sends a Button event for each press of a page button (the lower forward, the upper back; swapped when the
// setting reverses them) until the file is closed; none sent if the Kobo has no such keys. A press counts as the
// reader being used (the idle sleep), and does nothing while the Kobo dozes.
func (d *Device) buttons(out chan<- Event) io.Closer {
	path := buttonsDev()
	if path == "" {
		return io.NopCloser(nil)
	}
	f, err := openInput(path)
	if err != nil {
		return io.NopCloser(nil)
	}
	go func() {
		var ev [16]byte
		held := false // the held button's hold already sent: its release is not a press
		for {
			if _, err := io.ReadFull(f, ev[:]); err != nil {
				return
			}
			typ, code, val := binary.LittleEndian.Uint16(ev[8:]), binary.LittleEndian.Uint16(ev[10:]), binary.LittleEndian.Uint32(ev[12:])
			if typ != evKey || (code != keyUpper && code != keyLower) || d.dozing.Load() {
				continue
			}
			d.active.Store(time.Now().UnixNano())
			b := 1
			if (code == keyLower) == d.reverse.Load() {
				b = -1
			}
			send := func(e Event) {
				select {
				case out <- e:
				default:
				}
			}
			switch val {
			case 1: // down: nothing yet, a press is what is let go
				held = false
			case 2: // the device repeats a held key from about 0.4 s: that is a hold, once
				if !held {
					held = true
					send(Event{Button: b, Held: true})
				}
			case 0: // up: a press, unless it was a hold
				if !held {
					send(Event{Button: b})
				}
				held = false
			}
		}
	}()
	return f
}

// idles sends a Sleep event to out once SleepAfter has passed since the last touch or press (not while it dozes), until done is closed or the returned function is called. It looks every few seconds.
func (d *Device) idles(out chan<- Event, done <-chan struct{}) (stop func()) {
	quit := make(chan struct{})
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				idle := time.Since(time.Unix(0, d.active.Load()))
				if limit := time.Duration(d.sleepAfter.Load()); limit > 0 && idle >= limit && !d.dozing.Load() {
					select {
					case out <- Event{Sleep: true}:
						d.active.Store(time.Now().UnixNano()) // asked: counts again if it is not taken at once
					default:
					}
				}
			case <-quit:
				return
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(quit) }) }
}

// sysfs writes v to a kernel file, as one write.
func sysfs(path, v string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(v)
	return err
}

// setLight sets the white light's level (0 to 100). The Kobo reads it back from actual_brightness; the simulated
// device, whose files are plain ones, has it written there too.
func setLight(level int) {
	sysfs(hw(lightFile), strconv.Itoa(level))
	if Sim() {
		sysfs(hw(lightLevel), strconv.Itoa(level))
	}
}

// lightDelay is how long the light stays on after the sleep screen is shown, so the screen settles before the light
// goes out.
const lightDelay = 2 * time.Second

// plugged says whether the Kobo is on a charger (charging or full: anything but discharging, as KOReader reads it).
func plugged() bool {
	b, err := os.ReadFile(hw(batteryStatus))
	return err == nil && strings.TrimSpace(string(b)) != "Discharging"
}

// doze shows pic (grey, or a picture in colour: sleepScreen), takes Wi-Fi down and the light off, and suspends the Kobo
// until the power button is pressed again, then puts the light and the screen back as they were, and Wi-Fi if it was up
// (in the background: a few seconds to connect).
func (d *Device) doze(pic image.Image) {
	power := d.wake
	d.dozing.Store(true)
	defer d.dozing.Store(false)
	t := time.Now()
	slept := t.Round(0) // the wall clock: Go's own stops while the Kobo is suspended
	d.Usage("sleep", "")
	back := d.Screen.Save()
	light, _ := os.ReadFile(hw(lightLevel))
	lit, _ := strconv.Atoi(strings.TrimSpace(string(light)))
	switch p := pic.(type) {
	case *image.RGBA:
		d.Screen.ShowRGBA(p, image.Rect(0, 0, d.Screen.W, d.Screen.H), WfGC16, true)
	case *image.Gray:
		d.Screen.Show(p, WfGC16, true)
	}
	d.Screen.Wait()
	time.Sleep(lightDelay)
	setLight(0)
	wifi, _ := os.ReadFile(hw(wifiState))
	defer func() {
		if strings.TrimSpace(string(wifi)) != "up" {
			return
		}
		go func() {
			s := time.Now()
			out, err := exec.Command("sh", "-c", wifiOn).CombinedOutput()
			if err != nil {
				fmt.Printf("  Wi-Fi not back: %v: %s\n", err, strings.TrimSpace(string(out)))
				return
			}
			fmt.Printf("  Wi-Fi back in %.1f s\n", time.Since(s).Seconds())
		}()
	}()
	if !Sim() {
		exec.Command("sh", "-c", wifiOff).Run()
	}
	fmt.Printf("asleep: sleep screen shown, light and Wi-Fi off in %.0f ms\n", ms(t))
	for len(power) > 0 {
		<-power
	}
	for wakes, refused := 0, 0; ; {
		if Sim() { // no suspend: asleep until the power key's pipe has a press
			fmt.Println("  suspended (simulated): until the power key")
			<-power
			break
		}
		if plugged() {
			// awake behind the sleep screen: a press wakes, an unplugged charger lets it suspend
			select {
			case <-power:
			case <-time.After(5 * time.Second):
				continue
			}
			break
		}
		if wakes >= 20 {
			fmt.Println("  woken 20 times without a press: awake until one")
			<-power
			break
		}
		if err := sysfs("/sys/power/state-extended", "1"); err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: sleep:", err)
		}
		// a press while the drivers get ready cancels
		cancel := false
		select {
		case <-power:
			cancel = true
		case <-time.After(2 * time.Second):
		}
		if cancel {
			sysfs("/sys/power/state-extended", "0")
			break
		}
		syscall.Sync()
		s := time.Now().Round(0) // the wall clock: Go's own stops while the Kobo is suspended
		err := sysfs("/sys/power/state", "mem")
		sysfs("/sys/power/state-extended", "0")
		time.Sleep(100 * time.Millisecond)
		if err != nil && refused < 5 {
			// a driver not ready: the screen's, while the panel stays powered a moment after the last refresh
			// ("hwtcon ... xon off timer pending", 2026-10-30); tried again soon, a press still waking
			refused++
			fmt.Printf("  suspend refused (%v), again in 3 s\n", err)
			select {
			case <-power:
			case <-time.After(3 * time.Second):
				continue
			}
			break
		}
		refused = 0
		fmt.Printf("  suspended for %.0f s\n", time.Now().Round(0).Sub(s).Seconds())
		select {
		case <-power:
		case <-time.After(15 * time.Second):
			wakes++
			continue
		}
		break
	}
	t = time.Now()
	d.Screen.Clean() // white first: the sleep screen, left for hours, does not stay behind the page
	d.Screen.Restore(back)
	setLight(lit)
	fmt.Printf("awake: page back in %.0f ms\n", ms(t))
	d.Usage("wake", fmt.Sprintf(" asleep=%d", int(time.Now().Round(0).Sub(slept).Seconds())))
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

// SoundSink is the GStreamer element the narration's player ends in (reader/sink.go), and what its environment needs.
// On the Kobo (firmware 6) PulseAudio's, in system mode: its socket named, since the reader is no PulseAudio user
// (2026-11-11: a test tone and the player's own line heard through Bluetooth headphones); the headphones' sink is the
// default once they connect (Kobo's module-switch-on-connect). 4.x's was MediaTek's mtkbtmwrpcaudiosink.
func (d *Device) SoundSink() (element string, env []string) {
	return "pulsesink", []string{"PULSE_SERVER=unix:/var/run/pulse/native"}
}
