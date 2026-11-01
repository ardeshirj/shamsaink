package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// A usageEvent is one `usage:` line of the reader's log (device.Usage): start, stop, sleep or wake, when (UTC), the
// battery's per cent, whether it was on a charger, and on a wake the seconds asleep.
type usageEvent struct {
	kind    string
	at      time.Time
	battery int
	plugged bool
}

// A stretch is the time from one event to the next of its kind: awake (start or wake to sleep or stop) or asleep (sleep to wake).
type stretch struct {
	what     string
	from, to usageEvent
}

func (s stretch) hours() float64 { return s.to.at.Sub(s.from.at).Hours() }
func (s stretch) used() int      { return s.from.battery - s.to.battery }
func (s stretch) onBattery() bool {
	return !s.from.plugged && !s.to.plugged && s.from.battery >= 0 && s.to.battery >= 0
}

// parseUsage reads the usage lines of a log.
func parseUsage(log string) (evs []usageEvent) {
	for _, l := range strings.Split(log, "\n") {
		if !strings.HasPrefix(l, "usage:") {
			continue
		}
		f := strings.Fields(strings.TrimPrefix(l, "usage:"))
		if len(f) == 0 {
			continue
		}
		e := usageEvent{kind: f[0], battery: -1}
		for _, kv := range f[1:] {
			k, v, _ := strings.Cut(kv, "=")
			switch k {
			case "at":
				e.at, _ = time.Parse(time.RFC3339, v)
			case "battery":
				e.battery, _ = strconv.Atoi(v)
			case "plugged":
				e.plugged = v == "true"
			}
		}
		evs = append(evs, e)
	}
	return evs
}

// stretches pairs the events: awake from a start or a wake to the next sleep or stop, asleep from a sleep to its wake.
func stretches(evs []usageEvent) (ss []stretch) {
	var open *usageEvent
	for i, e := range evs {
		switch e.kind {
		case "start", "wake":
			open = &evs[i]
		case "sleep", "stop":
			if open != nil {
				ss = append(ss, stretch{"awake", *open, e})
				open = nil
			}
			if e.kind == "sleep" && i+1 < len(evs) && evs[i+1].kind == "wake" {
				ss = append(ss, stretch{"asleep", e, evs[i+1]})
			}
		}
	}
	return ss
}

// usage prints the stretches awake and asleep from the device's log, the battery each used, and the rate per hour of
// each kind (only stretches on the battery at both ends). A battery reads in whole per cents: an hour is rough, a week
// is not.
func usage(name string) bool {
	b, err := os.ReadFile("e2e/devices.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, err, "(run from the repository's root)")
		return false
	}
	var all map[string]*Access
	if err := yaml.Unmarshal(b, &all); err != nil || all[name] == nil {
		fmt.Fprintf(os.Stderr, "no device %q in e2e/devices.yaml\n", name)
		return false
	}
	a := all[name]
	out, err := ssh(a, "grep -h '^usage:' "+a.Log)
	if err != nil && out == "" {
		fmt.Println("no usage lines yet (or the device did not answer): install this build with make kobo-reader-install and use it")
		return true
	}
	ss := stretches(parseUsage(out))
	fmt.Printf("%-19s %-7s %8s %9s %5s\n", "from (UTC)", "", "length", "battery", "used")
	var hours, used [2]float64
	for _, s := range ss {
		note := ""
		if !s.onBattery() {
			note = "  (plugged in, or the battery not read)"
		}
		fmt.Printf("%-19s %-7s %8s %9s %4d%%%s\n", s.from.at.Format("2006-01-02 15:04:05"), s.what, dur(s.hours()),
			fmt.Sprintf("%d→%d%%", s.from.battery, s.to.battery), s.used(), note)
		if s.onBattery() && s.hours() > 0 {
			k := map[string]int{"awake": 0, "asleep": 1}[s.what]
			hours[k] += s.hours()
			used[k] += float64(s.used())
		}
	}
	for k, what := range []string{"awake", "asleep"} {
		if hours[k] > 0 {
			fmt.Printf("%-7s on the battery: %s, %.0f%% used, %.2f%% an hour\n", what, dur(hours[k]), used[k], used[k]/hours[k])
		}
	}
	return true
}

func dur(h float64) string {
	m := int(h*60 + 0.5)
	return fmt.Sprintf("%dh%02dm", m/60, m%60)
}
