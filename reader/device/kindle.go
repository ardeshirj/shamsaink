package device

import (
	"bufio"
	"os/exec"
	"strings"
)

// Sleep is the Kindle's powerd's (it suspends the device after its timeout, or when the power button is pressed), and
// KOReader draws the screensaver when powerd says it is going there. While a book is open KOReader waits for us and
// draws nothing, so the reader leaves the book then: KOReader, its turn again, finds powerd's message and shows its
// screensaver, and the plugin opens the book again when the Kindle wakes.

// kindleSleeps sends when powerd goes to the screensaver, until stop is called.
func kindleSleeps() (<-chan struct{}, func()) {
	out := make(chan struct{}, 4)
	cmd := exec.Command("lipc-wait-event", "-m", "com.lab126.powerd", "goingToScreenSaver")
	stdout, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return out, func() {} // not on a Kindle: never asleep
	}
	go func() {
		// a line an event: its name, then its source (2 the power button, 4 the cover)
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			if f := strings.Fields(s.Text()); len(f) > 0 && f[0] == "goingToScreenSaver" {
				out <- struct{}{}
			}
		}
	}()
	return out, func() { cmd.Process.Kill(); cmd.Wait() }
}
