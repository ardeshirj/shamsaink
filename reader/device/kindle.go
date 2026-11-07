package device

import (
	"bufio"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// Sleep on the Kindle is powerd's: it hears the power button (only while Amazon's framework runs: with it stopped,
// powerd takes the Kindle for still booting and ignores the button, 2026-11-06), and sleeps after its own timeout. It
// says so in events: goingToScreenSaver (2 the power button, 4 the cover), then readyToSuspend and the suspend; on
// waking, outOfScreenSaver (1 the button, 6 the cover). Our launcher (reader/kindle/launch.sh) leaves the framework
// running, paused under us, as KOReader does by default, so powerd keeps doing this.
//
// The reader answers it as on the Kobo, the book kept open: goingToScreenSaver shows the sleep screen, and
// outOfScreenSaver puts the page back (kindleDoze).

// kindleSleeps sends to out when powerd goes to the screensaver, and wakes a doze when it comes out, until stop is
// called.
func (d *Device) kindleSleeps(out chan<- Event) (stop func()) {
	cmd := exec.Command("lipc-wait-event", "-m", "com.lab126.powerd", "goingToScreenSaver,outOfScreenSaver,readyToSuspend")
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return func() {} // not on a Kindle: never asleep
	}
	go func() {
		// a line an event: its name, then its source
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			f := strings.Fields(s.Text())
			if len(f) == 0 {
				continue
			}
			switch f[0] {
			case "goingToScreenSaver":
				if d.dozing.Load() {
					continue
				}
				select {
				case out <- Event{Sleep: true}:
				default:
				}
			case "readyToSuspend":
				if d.dozing.Load() {
					fmt.Printf("  powerd: ready to suspend (%s)\n", strings.Join(f[1:], " "))
				}
			case "outOfScreenSaver":
				if d.dozing.Load() {
					select {
					case d.wake <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
	return func() { cmd.Process.Kill(); cmd.Wait() }
}

// kindleDoze shows pic until powerd comes out of the screensaver, then puts back what the screen showed. powerd
// suspends the Kindle meanwhile, and takes Wi-Fi and the light down and up itself.
func (d *Device) kindleDoze(pic image.Image) {
	d.dozing.Store(true)
	defer d.dozing.Store(false)
	t := time.Now()
	slept := t.Round(0) // the wall clock: Go's own stops while the Kindle is suspended
	d.Usage("sleep", "")
	back := d.Screen.Save()
	for len(d.wake) > 0 {
		<-d.wake
	}
	switch p := pic.(type) {
	case *image.RGBA:
		d.Screen.ShowRGBA(p, image.Rect(0, 0, d.Screen.W, d.Screen.H), WfGC16, true)
	case *image.Gray:
		d.Screen.Show(p, WfGC16, true)
	}
	d.Screen.Wait()
	fmt.Printf("asleep: sleep screen shown in %.0f ms\n", ms(t))
	<-d.wake
	t = time.Now()
	d.Screen.Clean() // white first: the sleep screen, left for hours, does not stay behind the page
	d.Screen.Restore(back)
	fmt.Printf("awake: page back in %.0f ms\n", ms(t))
	d.Usage("wake", fmt.Sprintf(" asleep=%d", int(time.Now().Round(0).Sub(slept).Seconds())))
}

// kindleTouched tells powerd a touch came, at most every 30 s: the touch screen is grabbed, so powerd, which counts
// its timeout from the last input it saw, would otherwise put the Kindle to sleep in the middle of reading.
func kindleTouched(last *atomic.Int64) {
	now := time.Now().UnixNano()
	if now-last.Load() < int64(30*time.Second) {
		return
	}
	last.Store(now)
	go exec.Command("lipc-set-prop", "-i", "com.lab126.powerd", "touchScreenSaverTimeout", "1").Run()
}

// kindleInverts says whether the Kindle's screen inverts every pixel itself: its own Dark Mode sets the window
// manager's epdcMode to Y8INV (8-bit grey, inverted; Y8 without it), and the driver inverts whatever is drawn, ours
// too (2026-11-06). The launcher asks before it pauses the window manager (awesome, which answers it: asked while
// paused, lipc waits 10 s) and passes the answer as KINDLE_EPDC; it cannot change while the reader runs.
func kindleInverts() bool { return strings.Contains(os.Getenv("KINDLE_EPDC"), "INV") }
