// inktrace: from a tap to the ink, on MediaTek's e-ink driver (hwtcon: the Kobo Libra Colour, the Kindle Paperwhite 11),
// without a camera, for any app in front. Built for experiment 17 (the comparison of readers, 2026-11-21); since
// 2026-11-21 here, for devtest's time step (reader/cmd/devtest, e2e/README.md) and the benchmarks.
//
//   - every screen update from the driver's own log, /proc/hwtcon/record, switched on with "fs_record 1" in
//     /proc/hwtcon/cmd (Kobo's kernel source, drivers/misc/mediatek/hwtcon/hwtcon_debug.c; nothing else of that file's
//     commands is used: some hang a CPU or write the panel's power chip on purpose): its area, its waveform (whether it
//     flashes) and the time from its request to the ink done (total). Its id is the kernel clock (ns) when it was asked
//     for, so the request lines up with the touches. (Kobo's kernel also notes the last input event's time, event: kept
//     when there.)
//   - by hand: touches from the touch controller's interrupt count (-irq: "2-0024" on the Kindle; its
//     /sys/kernel/irq/N/per_cpu_count, kept open and read again every 10 ms: /proc/interrupts every 4 ms took 44% of a
//     core, that file opened anew every 4 ms 34%), which works while Amazon's software holds the touch screen. A touch is
//     a burst of at least 3 interrupts (-min): the Kindle's chip also fires once every ~2 s on its own.
//
// The screen's size and framebuffer come from /sys/class/graphics/fb0, so one binary serves both devices.
//
//	inktrace [-irq 2-0024] [-gap 120] > out.txt   touches by hand, until SIGINT or SIGTERM
//	inktrace tap X Y [-node /dev/input/event1] [-quiet 2500] [-max 20000]
//	                                               a tap written into the touch screen at the touch's own X Y (the
//	                                               caller turns a screen point), then its updates until none came for
//	                                               -quiet ms; timed from this tap's finger up
//	inktrace run [-quiet 2500] [-max 20000] -- CMD...
//	                                               CMD run (a launcher: it may return at once), timed from its start
//	inktrace snap FILE                             the screen as a PGM (8-bit grey), to find where to tap
//
// Each result ends in two lines a caller reads: "=> finger up to the last ink done: N ms" (every update of 2% of the
// screen or more: under that a status bar's clock or battery) and "=> to the last full-page ink done: N ms" (half the
// screen or more: what a page turn, an open or a start ends with; a clock turning a minute seconds later is not it).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// the screen's size, from fb0 (screenSize): the Kindle's 1236x1648 if it cannot be read
var width, height = 1236.0, 1648.0

type touch struct{ down, up float64 } // ms, kernel clock

type update struct {
	at, total, event float64 // the request (ms, kernel clock), request to ink done, last event to request (-1: none)
	mode             string
	area             float64 // share of the screen
	raw              string
}

func main() {
	screenSize()
	if len(os.Args) > 1 && os.Args[1] == "run" {
		run()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "tap" {
		driven()
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "snap" {
		snap(os.Args[2])
		return
	}
	irq := flag.String("irq", "2-0024", "the touch controller's name in /proc/interrupts")
	gap := flag.Float64("gap", 120, "interrupts further apart than this (ms) are two touches")
	least := flag.Int("min", 3, "the fewest interrupts in a touch")
	flag.Parse()

	must(os.WriteFile("/proc/hwtcon/cmd", []byte("fs_record 0"), 0)) // a fresh log
	must(os.WriteFile("/proc/hwtcon/cmd", []byte("fs_record 1"), 0))
	fmt.Fprintln(os.Stderr, "inktrace: recording; stop with SIGINT or SIGTERM")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	counter, err := os.Open(irqFile(*irq))
	must(err)
	var touches []touch
	var bursts []int // interrupts in each touch
	last := -1
	buf := make([]byte, 64)
loop:
	for {
		select {
		case <-stop:
			break loop
		default:
			time.Sleep(10 * time.Millisecond)
			n := count(counter, buf)
			now := mono()
			if last >= 0 && n > last {
				if len(touches) == 0 || now-touches[len(touches)-1].up > *gap {
					touches = append(touches, touch{now, now})
					bursts = append(bursts, 0)
				} else {
					touches[len(touches)-1].up = now
				}
				bursts[len(bursts)-1] += n - last
			}
			last = n
		}
	}
	rec, err := os.ReadFile("/proc/hwtcon/record")
	must(err)
	os.WriteFile("/proc/hwtcon/cmd", []byte("fs_record 0"), 0)
	var real []touch
	for i, t := range touches {
		if bursts[i] >= *least {
			real = append(real, t)
		}
	}
	fmt.Printf("(%d bursts of interrupts, %d of them touches)\n", len(touches), len(real))
	touches = real
	updates := parse(string(rec))
	report(touches, updates)
}

// irqFile: the per_cpu_count of the interrupt named name.
func irqFile(name string) string {
	dirs, _ := os.ReadDir("/sys/kernel/irq")
	for _, d := range dirs {
		b, err := os.ReadFile("/sys/kernel/irq/" + d.Name() + "/actions")
		if err == nil && strings.TrimSpace(string(b)) == name {
			return "/sys/kernel/irq/" + d.Name() + "/per_cpu_count"
		}
	}
	must(fmt.Errorf("no interrupt %q in /sys/kernel/irq", name))
	return ""
}

// count: the interrupt's total over every CPU ("26748,0"), read again from the start, nothing allocated.
func count(f *os.File, buf []byte) int {
	k, err := f.ReadAt(buf, 0)
	if k == 0 {
		must(err)
	}
	n, v := 0, 0
	for _, c := range buf[:k] {
		switch {
		case c >= '0' && c <= '9':
			v = v*10 + int(c-'0')
		default:
			n, v = n+v, 0
		}
	}
	return n + v
}

// mono: CLOCK_MONOTONIC in ms, the clock of the driver's ids (the kernel's sched_clock: both leave out suspend; checked
// against the kernel log's times on the Kindle, 2026-11-21).
func mono() float64 {
	var ts syscall.Timespec
	syscall.Syscall(syscall.SYS_CLOCK_GETTIME, 1, uintptr(unsafe.Pointer(&ts)), 0)
	return float64(ts.Sec)*1e3 + float64(ts.Nsec)/1e6
}

var (
	reTask  = regexp.MustCompile(`task:0x([0-9a-f]+)`)
	reRects = regexp.MustCompile(`\[(\d+) (\d+) (\d+) (\d+)\]`)
	reMode  = regexp.MustCompile(`->([a-z0-9_]+)`)
	reTotal = regexp.MustCompile(`total:\s*(-?\d+)`)
	reEvent = regexp.MustCompile(`event\s+(-?\d+)`)
)

// parse: one update a line. The Kindle's: "task:0xID [top left height width] ... mode:09->glkw16 ... total:366";
// Kobo's: "task:0xID [user region]->[region] ... wfm:NN->name(mode) ... event NNN submit ... total:NNN".
func parse(rec string) []update {
	var out []update
	for _, line := range strings.Split(rec, "\n") {
		m := reTask.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, _ := strconv.ParseUint(m[1], 16, 64)
		u := update{at: float64(id) / 1e6, total: -1, event: -1, raw: line}
		if rs := reRects.FindAllStringSubmatch(line, -1); len(rs) > 0 {
			r := rs[len(rs)-1]
			a, _ := strconv.Atoi(r[3])
			b, _ := strconv.Atoi(r[4])
			u.area = float64(a) * float64(b) / (width * height)
		}
		if ms := reMode.FindAllStringSubmatch(line, -1); len(ms) > 0 {
			u.mode = ms[len(ms)-1][1]
		}
		if t := reTotal.FindStringSubmatch(line); t != nil {
			u.total, _ = strconv.ParseFloat(t[1], 64)
		}
		if e := reEvent.FindStringSubmatch(line); e != nil {
			u.event, _ = strconv.ParseFloat(e[1], 64)
		}
		out = append(out, u)
	}
	return out
}

func report(touches []touch, updates []update) {
	fmt.Printf("%d touches, %d screen updates\n", len(touches), len(updates))
	j := 0
	for j < len(updates) && len(touches) > 0 && updates[j].at < touches[0].down {
		j++
	}
	if j > 0 {
		fmt.Printf("(%d updates before the first touch, left out)\n", j)
	}
	for i, t := range touches {
		next := 1e18
		if i+1 < len(touches) {
			next = touches[i+1].down
		}
		fmt.Printf("\ntouch %d: down at %.0f ms, held %.0f ms\n", i+1, t.down-touches[0].down, t.up-t.down)
		lastDone, lastFull := -1.0, -1.0
		for ; j < len(updates) && updates[j].at < next; j++ {
			u := updates[j]
			done := "dropped (merged into a later update)"
			if u.total >= 0 {
				done = fmt.Sprintf("ink done %6.0f ms", u.at-t.up+u.total)
				// under 2% of the screen: the status bar's clock or battery (a corner, every few seconds), not counted
				if d := u.at - t.up + u.total; d > lastFull && u.area >= 0.5 {
					lastFull = d
				}
				if d := u.at - t.up + u.total; d > lastDone && u.area >= 0.02 {
					lastDone = d
				} else if u.area < 0.02 {
					done += " (small: not counted)"
				}
			}
			ev := ""
			if u.event >= 0 {
				ev = fmt.Sprintf(", kernel's event to request %.0f ms", u.event)
			}
			fmt.Printf("  update: asked %6.0f ms after the finger up, %-8s %3.0f%% of the screen, %s%s\n",
				u.at-t.up, u.mode, 100*u.area, done, ev)
		}
		if lastDone >= 0 {
			fmt.Printf("  => finger up to the last ink done: %.0f ms\n", lastDone)
		}
		if lastFull >= 0 {
			fmt.Printf("  => to the last full-page ink done: %.0f ms\n", lastFull)
		}
	}
	fmt.Println("\nthe driver's log:")
	for _, u := range updates {
		fmt.Println(u.raw)
	}
}

// driven: one tap written in, and the updates after it.
func driven() {
	fs := flag.NewFlagSet("tap", flag.ExitOnError)
	quiet := fs.Float64("quiet", 2500, "no new update for this long (ms) ends the watch")
	most := fs.Float64("max", 20000, "the longest watch (ms)")
	node := fs.String("node", "/dev/input/event1", "the touch screen's input")
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "inktrace tap X Y [-node DEV] [-quiet MS] [-max MS]")
		os.Exit(2)
	}
	x, _ := strconv.Atoi(os.Args[2])
	y, _ := strconv.Atoi(os.Args[3])
	fs.Parse(os.Args[4:])
	record()
	down, up := tap(*node, x, y)
	watch(down, up, *quiet, *most)
}

// run: a command started (a launcher returns at once, its app going on), and the updates after it, from its start.
func run() {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	quiet := fs.Float64("quiet", 2500, "no new update for this long (ms) ends the watch")
	most := fs.Float64("max", 20000, "the longest watch (ms)")
	fs.Parse(os.Args[2:])
	cmd := fs.Args()
	if len(cmd) == 0 {
		fmt.Fprintln(os.Stderr, "inktrace run [-quiet MS] [-max MS] -- CMD...")
		os.Exit(2)
	}
	record()
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	t := mono()
	must(c.Start())
	go c.Wait()
	watch(t, t, *quiet, *most)
}

// record: the driver's log, fresh.
func record() {
	must(os.WriteFile("/proc/hwtcon/cmd", []byte("fs_record 0"), 0))
	must(os.WriteFile("/proc/hwtcon/cmd", []byte("fs_record 1"), 0))
}

// watch: the updates until none came for quiet ms (or most ms in all), reported from up.
func watch(down, up, quiet, most float64) {
	seen, last := 0, mono()
	for mono()-up < most && (seen == 0 || mono()-last < quiet) {
		time.Sleep(50 * time.Millisecond)
		rec, _ := os.ReadFile("/proc/hwtcon/record")
		if n := strings.Count(string(rec), "task:"); n > seen {
			seen, last = n, mono()
		}
	}
	rec, err := os.ReadFile("/proc/hwtcon/record")
	must(err)
	os.WriteFile("/proc/hwtcon/cmd", []byte("fs_record 0"), 0)
	report([]touch{{down, up}}, parse(string(rec)))
}

// tap: a finger down for 100 ms, reported every 10 ms, and lifted, as the Kindle's pt_mt reports one (slot, tracking
// id, position, touch major, pressure); its down and up times (ms, kernel clock).
func tap(node string, x, y int) (float64, float64) {
	f, err := os.OpenFile(node, os.O_WRONLY, 0)
	must(err)
	defer f.Close()
	ev := func(t, code uint16, v int32) []byte {
		b := make([]byte, 16) // the 32-bit kernel's input_event: timeval (left 0), type, code, value
		b[8], b[9], b[10], b[11] = byte(t), byte(t>>8), byte(code), byte(code>>8)
		b[12], b[13], b[14], b[15] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
		return b
	}
	const syn, abs = 0, 3
	var b []byte
	for _, e := range [][3]int32{{abs, 0x2f, 0}, {abs, 0x39, 77}, {abs, 0x35, int32(x)}, {abs, 0x36, int32(y)},
		{abs, 0x30, 8}, {abs, 0x3a, 60}, {syn, 0, 0}} {
		b = append(b, ev(uint16(e[0]), uint16(e[1]), e[2])...)
	}
	down := mono()
	_, err = f.Write(b)
	must(err)
	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Millisecond)
		_, err = f.Write(append(append(ev(abs, 0x35, int32(x)), ev(abs, 0x36, int32(y))...), ev(syn, 0, 0)...))
		must(err)
	}
	b = nil
	for _, e := range [][3]int32{{abs, 0x3a, 0}, {abs, 0x30, 0}, {abs, 0x39, -1}, {syn, 0, 0}} {
		b = append(b, ev(uint16(e[0]), uint16(e[1]), e[2])...)
	}
	_, err = f.Write(b)
	must(err)
	return down, mono()
}

// screenSize: the screen's visible size, from fb0's FBIOGET_VSCREENINFO (xres, yres: the first two of
// fb_var_screeninfo's 32-bit words; sysfs's virtual_size may count a second buffer).
func screenSize() {
	f, err := os.Open("/dev/fb0")
	if err != nil {
		return
	}
	defer f.Close()
	var v [40]uint32 // fb_var_screeninfo is 160 bytes
	const fbiogetVscreeninfo = 0x4600
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), fbiogetVscreeninfo, uintptr(unsafe.Pointer(&v[0]))); e != 0 {
		return
	}
	if v[0] > 0 && v[1] > 0 {
		width, height = float64(v[0]), float64(v[1])
	}
}

// fbInt: a number of /sys/class/graphics/fb0 (stride, bits_per_pixel), or def.
func fbInt(name string, def int) int {
	if b, err := os.ReadFile("/sys/class/graphics/fb0/" + name); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// snap: the screen as a PGM, read once: the Kindle's 8-bit framebuffer as it is, the Kobo's 32-bit one by its green byte.
func snap(file string) {
	w, h := int(width), int(height)
	bpp := fbInt("bits_per_pixel", 8) / 8
	stride := fbInt("stride", w*bpp)
	f, err := os.Open("/dev/fb0")
	must(err)
	defer f.Close()
	buf := make([]byte, stride*h)
	_, err = f.ReadAt(buf, 0)
	must(err)
	out := []byte(fmt.Sprintf("P5 %d %d 255\n", w, h))
	for y := 0; y < h; y++ {
		if bpp == 1 {
			out = append(out, buf[y*stride:y*stride+w]...)
			continue
		}
		for x := 0; x < w; x++ {
			out = append(out, buf[y*stride+bpp*x+1])
		}
	}
	must(os.WriteFile(file, out, 0o644))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "inktrace:", err)
		os.Exit(1)
	}
}
