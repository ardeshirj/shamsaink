package device

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// The heartbeat (the watchdog, plan stage 5): the reader's event loops answer a tick every few seconds by writing a
// counter to a file in RAM, and the launcher (kobo/launch.sh) kills the reader when the counter stands still for a
// minute: Kobo's own watchdog does the same for Nickel (its `sickel`, which Nickel pings). A loop blocked on a sound, a
// lock or a screen that never answers stops answering. Asleep the loop is meant to wait, so the file says "sleep" and
// the launcher leaves it alone.

// BeatFile is where the counter is written, and what the launcher reads (HEARTBEAT in the environment changes it).
var BeatFile = env("HEARTBEAT", "/tmp/shamsaink-alive")

// BeatEvery is how often a loop is asked to answer: 5 s, or HEARTBEAT_EVERY (a Go duration, "1s") for a test on a faster
// clock (the watcher's WATCH_EVERY goes with it: launch.sh).
var BeatEvery = func() time.Duration {
	if d, err := time.ParseDuration(env("HEARTBEAT_EVERY", "5s")); err == nil && d > 0 {
		return d
	}
	return 5 * time.Second
}()

var (
	beatOnce sync.Once
	beatTick *time.Ticker
	beatN    int
	beatMu   sync.Mutex
)

// Beats is the ticks every loop that waits for events listens to, to call Beat when it gets one. All the loops share
// the channel: one waiting at a time.
func (d *Device) Beats() <-chan time.Time {
	beatOnce.Do(func() { beatTick = time.NewTicker(BeatEvery) })
	return beatTick.C
}

// Beat says the loop is alive: the counter in BeatFile goes up by one.
func (d *Device) Beat() {
	beatMu.Lock()
	defer beatMu.Unlock()
	beatN++
	os.WriteFile(BeatFile, []byte(strconv.Itoa(beatN)), 0o644)
}

// beatSleeping says the loop is to wait a long time (the sleep screen), so no one is to wonder.
func beatSleeping() {
	beatMu.Lock()
	defer beatMu.Unlock()
	os.WriteFile(BeatFile, []byte("sleep"), 0o644)
}
