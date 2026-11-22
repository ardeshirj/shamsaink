package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Timing (2026-11-21, from experiment 17's comparison of readers): how long a step takes, from the tap to the ink, read
// from the e-ink driver's own log by inktrace (reader/cmd/inktrace, built by make inktrace into reader/out/inktrace,
// copied into the device's RAM folder, devices.yaml's tmp, for the scenario and removed after). On the devices only: the
// simulator has no driver.
//
// perf/README.md says how the benchmarks use them.
//
//	time NAME STEP...   STEP played with its first tap timed (tap, turn) or, for library and open, the reader's start
//	                    timed from its launcher's command: NAME's result is the last full-page update's ink done (half
//	                    the screen or more), also the last of any size and the waveforms; e2e/out/[DEVICE/]SCENARIO/
//	                    times.tsv (perf/out/DEVICE/SCENARIO/ for perf's) gets a line, time/NAME.txt inktrace's whole report
//	mem NAME            the reader's resident memory and the device's available memory, into times.tsv
//	cold PATH           the book's saved layout (cache/HASH.cache) and place removed: its next open is a first open
//	place PATH DOC PAGE the book's place set: it opens on that document's page (0: its first)
//
// Each line of times.tsv: the run (the scenario's start, local time), NAME, what (time or mem), then for time the full
// page's ms, the last update's ms, its waveforms, whether one flashed and the reader's last "shown in" line; for mem the reader's RSS and the available
// memory, in kB.

// timer is a scenario's timing: inktrace's place on the device, the run's name and the results' file.
type timer struct {
	ink, out, run string
	tsv           string
}

var (
	reFull    = regexp.MustCompile(`=> to the last full-page ink done: (\d+) ms`)
	reLast    = regexp.MustCompile(`=> finger up to the last ink done: (\d+) ms`)
	reUpdate  = regexp.MustCompile(`update: asked\s+(-?\d+) ms after the finger up, (\S+)\s+(\d+)% of the screen`)
	reFlashes = regexp.MustCompile(`^(gc16|gcc16|gck16)$`)
	reShown   = regexp.MustCompile(`shown in \d+ ms`)
)

// timing gets inktrace onto the device once a scenario, and its removal after.
func (p *player) timing() (*timer, error) {
	if p.tm != nil {
		return p.tm, nil
	}
	d := p.d
	if d.a.Local != "" || d.a.Tmp == "" {
		return nil, fmt.Errorf("time and mem run on a device with the e-ink driver's log and a tmp folder in devices.yaml, not the simulator")
	}
	bin, err := os.ReadFile(filepath.Join("reader", "out", "inktrace"))
	if err != nil {
		return nil, fmt.Errorf("no reader/out/inktrace: make inktrace (%v)", err)
	}
	ink := d.a.Tmp + "/devtest-inktrace"
	if _, err := d.sh("cat > "+ink+" && chmod +x "+ink, bin); err != nil {
		return nil, err
	}
	p.cleanups = append(p.cleanups, func() { d.sh("rm -f "+ink+" "+d.a.Tmp+"/devtest-time.txt", nil) })
	os.MkdirAll(filepath.Join(p.out, "time"), 0o755)
	p.tm = &timer{ink: ink, out: filepath.Join(p.out, "time"), run: time.Now().Format("2006-01-02T15:04:05"),
		tsv: filepath.Join(p.out, "times.tsv")}
	return p.tm, nil
}

// timeStep plays a step with its first tap, or the reader's start, timed by inktrace.
func (p *player) timeStep(a []string) (string, error) {
	if len(a) < 3 {
		return "", fmt.Errorf("time NAME STEP...")
	}
	t, err := p.timing()
	if err != nil {
		return "", err
	}
	d, name, inner := p.d, a[1], a[2:]
	res := d.a.Tmp + "/devtest-time.txt"
	from := d.logLines()
	d.sh("rm -f "+res, nil)
	switch inner[0] {
	case "library", "open":
		d.launchWrap = func(cmd string) string {
			return fmt.Sprintf("%s run -quiet 6000 -max 40000 -- sh -c '%s' > %s 2>&1", t.ink, cmd, res)
		}
	default:
		d.tapHook = func(x, y int) error {
			d.record(fmt.Sprintf("touch tap %d %d (timed)", x, y))
			tx, ty := d.turn(d.a.Touch.Turn.X, x, y), d.turn(d.a.Touch.Turn.Y, x, y)
			_, err := d.sh(fmt.Sprintf("%s tap %d %d -node %s -quiet 3000 -max 30000 > %s 2>&1", t.ink, tx, ty, d.a.Touch.Node, res), nil)
			return err
		}
	}
	said, err := p.step(inner)
	d.tapHook, d.launchWrap = nil, nil
	if err != nil {
		return said, err
	}
	out, err := d.sh("cat "+res, nil)
	if err != nil || len(out) == 0 {
		return said, fmt.Errorf("no timing: inktrace said nothing (did the step tap or start the reader?)")
	}
	os.WriteFile(filepath.Join(t.out, name+".txt"), out, 0o644)
	full, last, modes, flash := readTrace(string(out))
	if full < 0 {
		return said, fmt.Errorf("no update of half the screen or more after the step (time/%s.txt)", name)
	}
	// the reader's own word on what the step showed (a book: its document, page and whether its layout was kept), for
	// the results: the step's own wait may have taken the line already, so it is read back here
	shown := ""
	if log, err := d.sh(fmt.Sprintf("tail -n +%d %s", from+1, d.a.Log), nil); err == nil {
		for _, l := range strings.Split(string(log), "\n") {
			if reShown.MatchString(l) {
				shown = strings.TrimSpace(l)
			}
		}
	}
	appendTSV(t.tsv, t.run, name, "time", strconv.Itoa(full), strconv.Itoa(last), strings.Join(modes, ","), strconv.FormatBool(flash), shown)
	f := ""
	if flash {
		f = ", flashed"
	}
	return fmt.Sprintf("%d ms (%s%s) %s", full, strings.Join(modes, ","), f, shown), nil
}

// readTrace: inktrace's report read: the full page's and the last update's ms (-1: none), the full-page updates'
// waveforms in order, and whether one of them flashed.
func readTrace(s string) (full, last int, modes []string, flash bool) {
	full, last = -1, -1
	if m := reFull.FindStringSubmatch(s); m != nil {
		full, _ = strconv.Atoi(m[1])
	}
	if m := reLast.FindStringSubmatch(s); m != nil {
		last, _ = strconv.Atoi(m[1])
	}
	seen := map[string]bool{}
	for _, m := range reUpdate.FindAllStringSubmatch(s, -1) {
		if pct, _ := strconv.Atoi(m[3]); pct >= 50 {
			if !seen[m[2]] {
				seen[m[2]] = true
				modes = append(modes, m[2])
			}
			flash = flash || reFlashes.MatchString(m[2])
		}
	}
	return
}

// memStep notes the reader's resident memory (every shamsaink process) and the device's available memory.
func (p *player) memStep(name string) (string, error) {
	t, err := p.timing()
	if err != nil {
		return "", err
	}
	out, err := p.d.sh(`r=0; for q in $(pidof shamsaink); do v=$(awk '/VmRSS/ {print $2}' /proc/$q/status); r=$((r + v)); done; `+
		`echo $r $(awk '/MemAvailable/ {print $2}' /proc/meminfo)`, nil)
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	if len(f) != 2 || f[0] == "0" {
		return "", fmt.Errorf("no reader running to measure (%q)", strings.TrimSpace(string(out)))
	}
	appendTSV(t.tsv, t.run, name, "mem", f[0], f[1], "", "")
	rss, _ := strconv.Atoi(f[0])
	avail, _ := strconv.Atoi(f[1])
	return fmt.Sprintf("reader %d MB, available %d MB", rss/1024, avail/1024), nil
}

// bookKey is the shell recipe for a book's key (library.Key: the SHA-256 of its size, a newline, its first and last
// 64 KB, or the whole file at 128 KB or less), in $k, for the file in $f. dd, not head -c: the Kobo's BusyBox has none.
const bookKey = `s=$(stat -c %s "$f") && if [ $s -le 131072 ]; then k=$({ echo $s; cat "$f"; } | sha256sum | cut -c1-12); ` +
	`else k=$({ echo $s; dd if="$f" bs=65536 count=1 2>/dev/null; tail -c 65536 "$f"; } | sha256sum | cut -c1-12); fi`

// coldStep removes a book's layout and place (by its key, which is both for a book first seen as it is).
func (p *player) coldStep(path string) (string, error) {
	out, err := p.d.sh("cd "+p.d.a.Dir+" && f="+path+" && "+bookKey+" && rm -f cache/$k.cache places/$k.place places/$k.mark && sync && echo $k", nil)
	if err != nil {
		return "", err
	}
	return "layout and place of " + strings.TrimSpace(string(out)) + " removed", nil
}

// placeStep sets a book's place: DOC PAGE, kept for this file.
func (p *player) placeStep(path, doc, page string) (string, error) {
	out, err := p.d.sh("cd "+p.d.a.Dir+" && f="+path+" && "+bookKey+" && mkdir -p places && "+
		"printf '%s %s 0.0000\\n%s\\n%s\\n' "+doc+" "+page+" $k "+path+" > places/$k.place && sync && echo $k", nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s placed at document %s page %s", strings.TrimSpace(string(out)), doc, page), nil
}

func appendTSV(path string, fields ...string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, strings.Join(fields, "\t"))
}
