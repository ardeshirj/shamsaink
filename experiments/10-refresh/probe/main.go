// refresh-probe: experiment 10. Two ways of refreshing the Kindle's e-ink screen, timed on the same page: our own
// ioctls (MXCFB_SEND_UPDATE_MTK, then MXCFB_WAIT_FOR_UPDATE_COMPLETE; the Kindle PW5's MediaTek driver, structs as in
// KOReader's ffi/mxcfb_kindle_h.lua) and KOReader's fbink run for each refresh (09's way). The page is whatever is on
// the screen when it starts (run.sh puts one there with 09's blitz-show); page turns alternate it with itself upside
// down, so every pixel changes.
//
//	refresh-probe time [REPS]                  every waveform, both ways, whole page and a word's strip
//	refresh-probe turns WAVEFORM flash|-|both N MS   N page turns MS apart, our ioctl only, to watch
//	refresh-probe mark WAVEFORM N MS            N marks (a bar under a line, 09's underline) on and off
package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	fbioGetVscreeninfo = 0x4600
	fbioGetFscreeninfo = 0x4602
	sendUpdateMTK      = 0x4060462e // MXCFB_SEND_UPDATE_MTK, struct mxcfb_update_data_mtk (96 bytes)
	waitComplete       = 0xc008462f // MXCFB_WAIT_FOR_UPDATE_COMPLETE, struct mxcfb_update_marker_data
	tempAmbient        = 0x1000
)

// The MediaTek driver's waveform modes (MTK_WAVEFORM_MODE_*).
var waveforms = map[string]uint32{"DU": 1, "GC16": 2, "GL16": 3, "REAGL": 4, "REAGLD": 5, "A2": 6, "DU4": 7, "GCK16": 8, "GLKW16": 9}

type rect struct{ top, left, width, height uint32 }

type updateMTK struct {
	region                       rect
	waveform, mode, marker       uint32
	temp                         int32
	flags                        uint32
	ditherMode, quantBit         int32
	altPhys, altWidth, altHeight uint32
	altRegion                    rect
	swipeDirection, swipeSteps   uint32
	histBW, histGray             uint32
	tsPXP, tsEPDC                uint32
}

type fb struct {
	f              *os.File
	mem            []byte
	w, h, line, y0 int
	marker         uint32
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func open() *fb {
	f, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	check(err)
	var v [160]byte // fb_var_screeninfo
	var x [68]byte  // fb_fix_screeninfo on 32-bit ARM
	check(ioctl(f.Fd(), fbioGetVscreeninfo, unsafe.Pointer(&v)))
	check(ioctl(f.Fd(), fbioGetFscreeninfo, unsafe.Pointer(&x)))
	u32 := func(b []byte, o int) int { return int(*(*uint32)(unsafe.Pointer(&b[o]))) }
	d := &fb{f: f, w: u32(v[:], 0), h: u32(v[:], 4), y0: u32(v[:], 20), line: u32(x[:], 44)}
	mem, err := syscall.Mmap(int(f.Fd()), 0, u32(x[:], 20), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	check(err)
	d.mem = mem
	fmt.Printf("fb0: %d×%d, %d bpp, line %d bytes, shown from row %d, id %q\n", d.w, d.h, u32(v[:], 24), d.line, d.y0,
		strings.TrimRight(string(x[:16]), "\x00"))
	return d
}

// page copies the screen shown into a w×h grey page.
func (d *fb) page() []byte {
	p := make([]byte, d.w*d.h)
	for y := 0; y < d.h; y++ {
		copy(p[y*d.w:(y+1)*d.w], d.mem[(d.y0+y)*d.line:])
	}
	return p
}

func (d *fb) write(p []byte, r rect) {
	for y := int(r.top); y < int(r.top+r.height); y++ {
		o := (d.y0+y)*d.line + int(r.left)
		copy(d.mem[o:o+int(r.width)], p[y*d.w+int(r.left):])
	}
}

// refresh sends one update; the time until the ioctl returns, then until the screen is done.
func (d *fb) refresh(r rect, waveform string, flash bool) (sent, done float64) {
	t := time.Now()
	d.marker++
	u := updateMTK{region: r, waveform: waveforms[waveform], marker: d.marker, temp: tempAmbient}
	if flash {
		u.mode = 1
	}
	check(ioctl(d.f.Fd(), sendUpdateMTK, unsafe.Pointer(&u)))
	sent = ms(t)
	m := [2]uint32{d.marker, 0}
	check(ioctl(d.f.Fd(), waitComplete, unsafe.Pointer(&m)))
	return sent, ms(t)
}

var fbink = env("FBINK", "/mnt/us/koreader/fbink")

// fbinkRefresh is 09's way: fbink run once for the refresh, waiting until the screen is done.
func fbinkRefresh(r rect, waveform string, flash bool) float64 {
	t := time.Now()
	args := []string{"-q", "-w", "-W", waveform, "-s",
		fmt.Sprintf("top=%d,left=%d,width=%d,height=%d", r.top, r.left, r.width, r.height)}
	if flash {
		args = append(args, "-f")
	}
	check(exec.Command(fbink, args...).Run())
	return ms(t)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "refresh-probe time [REPS] | turns WAVEFORM flash|-|both N MS | mark WAVEFORM N MS")
		os.Exit(2)
	}
	d := open()
	a := d.page()
	b := flipped(a, d.w, d.h)
	whole := rect{0, 0, uint32(d.w), uint32(d.h)}
	// A word's strip: 09's underline, a 4 px bar in a 12 px strip, across most of a line in the middle of the page.
	strip := rect{uint32(d.h / 2), 100, uint32(d.w - 200), 12}
	barred := withBar(a, d.w, strip)
	pause := 700 * time.Millisecond

	switch os.Args[1] {
	case "time":
		reps := 5
		if len(os.Args) > 2 {
			reps = atoi(os.Args[2])
		}
		t := time.Now()
		for i := 0; i < reps; i++ {
			check(exec.Command(fbink, "-q", "-e").Run())
		}
		fmt.Printf("fbink started, nothing refreshed (-e): %.1f ms each\n", ms(t)/float64(reps))
		turn := 0
		show := func() {
			turn++
			if turn%2 == 1 {
				d.write(b, whole)
			} else {
				d.write(a, whole)
			}
		}
		for _, c := range []struct {
			wf    string
			flash bool
		}{{"GC16", true}, {"GC16", false}, {"GL16", false}, {"REAGL", false}, {"REAGLD", false}, {"DU", false}, {"A2", false}} {
			var sent, done, ink []float64
			for i := 0; i < reps; i++ {
				show()
				s, dn := d.refresh(whole, c.wf, c.flash)
				sent, done = append(sent, s), append(done, dn)
				time.Sleep(pause)
				show()
				ink = append(ink, fbinkRefresh(whole, c.wf, c.flash))
				time.Sleep(pause)
			}
			fmt.Printf("page %-6s %-5s ioctl: sent %s, done %s; fbink: done %s\n", c.wf, flashName(c.flash),
				stats(sent), stats(done), stats(ink))
		}
		// The screen back to the page as it was, cleanly.
		d.write(a, whole)
		d.refresh(whole, "GC16", true)
		time.Sleep(pause)
		for _, wf := range []string{"A2", "DU", "GL16", "REAGL"} {
			var sent, done, ink []float64
			for i := 0; i < reps; i++ {
				d.write(barred, strip)
				s, dn := d.refresh(strip, wf, false)
				sent, done = append(sent, s), append(done, dn)
				time.Sleep(pause)
				d.write(a, strip)
				ink = append(ink, fbinkRefresh(strip, wf, false))
				time.Sleep(pause)
			}
			fmt.Printf("strip %-5s ioctl: sent %s, done %s; fbink: done %s\n", wf, stats(sent), stats(done), stats(ink))
		}
	case "turns":
		wf, n, gap := os.Args[2], atoi(os.Args[4]), time.Duration(atoi(os.Args[5]))*time.Millisecond
		for i := 0; i < n; i++ {
			p := b
			if i%2 == 1 {
				p = a
			}
			d.write(p, whole)
			flash := os.Args[3] == "flash" || (os.Args[3] == "both" && i%2 == 0)
			_, dn := d.refresh(whole, wf, flash)
			fmt.Printf("turn %d: %s %s %.0f ms\n", i+1, wf, flashName(flash), dn)
			time.Sleep(gap)
		}
		if n%2 == 1 {
			d.write(a, whole)
			d.refresh(whole, "GC16", true)
		}
	case "mark":
		wf, n, gap := os.Args[2], atoi(os.Args[3]), time.Duration(atoi(os.Args[4]))*time.Millisecond
		for i := 0; i < n; i++ {
			d.write(barred, strip)
			_, on := d.refresh(strip, wf, false)
			time.Sleep(gap)
			d.write(a, strip)
			_, off := d.refresh(strip, wf, false)
			fmt.Printf("mark %d: %s on %.0f ms, off %.0f ms\n", i+1, wf, on, off)
			time.Sleep(gap)
		}
	}
}

func flipped(p []byte, w, h int) []byte {
	q := make([]byte, len(p))
	for y := 0; y < h; y++ {
		copy(q[y*w:(y+1)*w], p[(h-1-y)*w:(h-y)*w])
	}
	return q
}

func withBar(p []byte, w int, r rect) []byte {
	q := append([]byte(nil), p...)
	for y := r.top + 4; y < r.top+8; y++ {
		for x := r.left; x < r.left+r.width; x++ {
			q[int(y)*w+int(x)] = 0
		}
	}
	return q
}

func stats(v []float64) string {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if s[len(s)-1] < 10 {
		return fmt.Sprintf("%.2f–%.2f ms (median %.2f)", s[0], s[len(s)-1], s[len(s)/2])
	}
	return fmt.Sprintf("%.0f–%.0f ms (median %.0f)", s[0], s[len(s)-1], s[len(s)/2])
}

func flashName(f bool) string {
	if f {
		return "flash"
	}
	return "-"
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	check(err)
	return n
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "refresh-probe:", err)
		os.Exit(1)
	}
}
