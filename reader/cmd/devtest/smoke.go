package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"shamsainkreader/design"
)

// A device being driven: how to reach it, the profile's screen, and the log's lines before the run.
type dev struct {
	a    *Access
	prof *design.Device
	name string
	mark int
	env  string // KEY=VALUE settings the reader is started with (a scenario's env step)
}

// load reads the spec's profile and the access facts of a device.
func load(name string) (*dev, error) {
	spec, err := design.Load()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile("e2e/devices.yaml")
	if err != nil {
		return nil, fmt.Errorf("%v (run from the repository's root)", err)
	}
	var all map[string]*Access
	if err := yaml.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	d := &dev{a: all[name], name: name}
	for i := range spec.Devices {
		if spec.Devices[i].Name == name {
			d.prof = &spec.Devices[i]
		}
	}
	if d.a == nil || d.prof == nil {
		return nil, fmt.Errorf("no device %q in the spec's profiles and e2e/devices.yaml", name)
	}
	return d, nil
}

func (d *dev) sh(cmd string, in []byte) ([]byte, error) {
	c := exec.Command("ssh", "-o", "ConnectTimeout=10", "-o", "BatchMode=yes", d.a.SSH, cmd)
	if in != nil {
		c.Stdin = strings.NewReader(string(in))
	}
	return c.Output()
}

func (d *dev) logLines() int {
	out, _ := d.sh("wc -l < "+d.a.Log, nil)
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

// expect waits for a line of the reader's log after the last one expected, and gives it.
func (d *dev) expect(pattern string, within time.Duration) (string, error) {
	re := regexp.MustCompile(pattern)
	end := time.Now().Add(within)
	for {
		out, _ := d.sh(fmt.Sprintf("tail -n +%d %s", d.mark+1, d.a.Log), nil)
		lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		for i, l := range lines {
			if re.MatchString(l) {
				d.mark += i + 1
				return l, nil
			}
		}
		if time.Now().After(end) {
			tail := lines
			if len(tail) > 4 {
				tail = tail[len(tail)-4:]
			}
			return "", fmt.Errorf("not in the log after %v: %q; the last lines: %q", within, pattern, tail)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// screen is the framebuffer as grey, w × h (a pixel's green channel: the screen is grey).
type screen struct {
	w, h int
	pix  []byte
}

func (d *dev) screen() (*screen, error) {
	raw, err := d.sh("cat /dev/fb0", nil)
	if err != nil {
		return nil, err
	}
	w, h, stride, bytes := d.prof.Screen.Width, d.prof.Screen.Height, d.a.Framebuffer.Stride, d.a.Framebuffer.Bits/8
	if len(raw) < h*stride {
		return nil, fmt.Errorf("the framebuffer gave %d bytes, want %d", len(raw), h*stride)
	}
	s := &screen{w, h, make([]byte, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s.pix[y*w+x] = raw[y*stride+x*bytes+1]
		}
	}
	return s, nil
}

// dark counts the dark pixels in the box [x0, y0, x1, y1].
func (s *screen) dark(b [4]int) int {
	n := 0
	for y := max(b[1], 0); y < min(b[3], s.h); y++ {
		for x := max(b[0], 0); x < min(b[2], s.w); x++ {
			if s.pix[y*s.w+x] < 128 {
				n++
			}
		}
	}
	return n
}

// boxes are the kept boxes of a render's buttons: golden/DEVICE/SCREEN-CASE-LANG.json.
type boxes struct {
	Placement string           `json:"placement"`
	Boxes     map[string][]int `json:"boxes"`
}

func (d *dev) boxes(render string) (*boxes, error) {
	b, err := os.ReadFile(filepath.Join("reader", "design", "golden", d.name, render+".json"))
	if err != nil {
		return nil, err
	}
	var r boxes
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func box(r *boxes, id string) ([4]int, bool) {
	b, ok := r.Boxes[id]
	if !ok || len(b) != 4 {
		return [4]int{}, false
	}
	return [4]int{b[0], b[1], b[2], b[3]}, true
}

// turn evaluates one of the touch's coordinate formulas (devices.yaml): x, y, w-x, h-y and the like.
func (d *dev) turn(f string, x, y int) int {
	w, h := d.prof.Screen.Width, d.prof.Screen.Height
	switch f {
	case "x":
		return x
	case "y":
		return y
	case "w-x":
		return w - x
	case "h-y":
		return h - y
	}
	panic("devices.yaml: touch turn " + f)
}

// input events as the 32-bit ARM kernel's struct input_event: a timeval (left 0), type, code, value.
func event(t, code uint16, value int32) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[8:], t)
	binary.LittleEndian.PutUint16(b[10:], code)
	binary.LittleEndian.PutUint32(b[12:], uint32(value))
	return b
}

const (
	evSyn, evAbs                        = 0, 3
	slot, posX, posY, trackingID uint16 = 0x2F, 0x35, 0x36, 0x39
)

// tapAt writes a finger down and up at the screen's (x, y) into the touch screen.
func (d *dev) tapAt(x, y int) error {
	at := func() []byte {
		b := append(event(evAbs, posX, int32(d.turn(d.a.Touch.Turn.X, x, y))), event(evAbs, posY, int32(d.turn(d.a.Touch.Turn.Y, x, y)))...)
		return append(b, event(evSyn, 0, 0)...)
	}
	b := append(event(evAbs, slot, 0), event(evAbs, trackingID, 7)...)
	b = append(b, at()...)
	b = append(b, event(evAbs, trackingID, -1)...)
	b = append(b, event(evSyn, 0, 0)...)
	_, err := d.sh("cat > "+d.a.Touch.Node, b)
	return err
}

// holdAt writes a finger down at the screen's (x, y), held still past the reader's hold time, moved to (x2, y2) in three
// steps, and lifted.
func (d *dev) holdAt(x, y, x2, y2 int) error {
	at := func(px, py int) []byte {
		b := append(event(evAbs, posX, int32(d.turn(d.a.Touch.Turn.X, px, py))), event(evAbs, posY, int32(d.turn(d.a.Touch.Turn.Y, px, py)))...)
		return append(b, event(evSyn, 0, 0)...)
	}
	write := func(b []byte) error { _, err := d.sh("cat > "+d.a.Touch.Node, b); return err }
	down := append(event(evAbs, slot, 0), event(evAbs, trackingID, 7)...)
	if err := write(append(down, at(x, y)...)); err != nil {
		return err
	}
	time.Sleep(1500 * time.Millisecond) // past the reader's 1000 ms (device/touch.go, holdTime)
	for i := 1; i <= 3; i++ {
		if err := write(at(x+(x2-x)*i/3, y+(y2-y)*i/3)); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return write(append(event(evAbs, trackingID, -1), event(evSyn, 0, 0)...))
}

// tap taps the centre of the control id of the kept render.
func (d *dev) tap(r *boxes, id string) error {
	b, ok := box(r, id)
	if !ok {
		return fmt.Errorf("no control %q in the kept render", id)
	}
	if r.Placement != "screen" {
		return fmt.Errorf("control %q is on a panel hung by something (%s): its place is not known without it", id, r.Placement)
	}
	return d.tapAt((b[0]+b[2])/2, (b[1]+b[3])/2)
}

// start launches the reader on the device (as NickelMenu launches it: Nickel stopped, the reader started, detached) and
// waits for its library; stop puts right a reader left running by a failed step (launch.sh then starts Nickel again).
func (d *dev) start() (stop func(), line string, err error) {
	if out, _ := d.sh("pidof shamsaink || echo none", nil); strings.TrimSpace(string(out)) != "none" {
		return nil, "", fmt.Errorf("the reader is already running on the device: leave it first")
	}
	d.mark = d.logLines()
	if _, err := d.sh("cd "+d.a.Dir+" && nohup env "+d.env+" sh launch.sh >/dev/null 2>&1 </dev/null &", nil); err != nil {
		return nil, "", err
	}
	stop = func() {
		if out, _ := d.sh("pidof shamsaink || echo none", nil); strings.TrimSpace(string(out)) != "none" {
			fmt.Println("note the reader is still running: stopping it (launch.sh starts the device's interface again)")
			d.sh("kill $(pidof shamsaink)", nil)
		}
	}
	line, err = d.expect(`library: \d+ books.*shown in`, 40*time.Second)
	if err != nil {
		return stop, "", err
	}
	time.Sleep(2 * time.Second) // the screen's refresh
	return stop, line, nil
}

// leave taps the library's menu and Leave by name, and waits for the device's own interface.
func (d *dev) leave(lib, menu *boxes) error {
	if err := d.tap(lib, "topbar-menu"); err != nil {
		return err
	}
	time.Sleep(2500 * time.Millisecond)
	if err := d.tap(menu, "leave"); err != nil {
		return err
	}
	if _, err := d.expect(`library: left`, 10*time.Second); err != nil {
		return err
	}
	_, err := d.expect(`Nickel started again`, 20*time.Second)
	return err
}

// smoke launches the reader on the device, holds its library's screen against the kept render, taps the menu and Leave by
// name, and waits for the device's own interface to come back.
func smoke(name string) bool {
	d, err := load(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	ok := true
	step := func(good bool, what string, detail ...any) {
		mark := "ok  "
		if !good {
			mark, ok = "FAIL", false
		}
		fmt.Printf("%s %s", mark, what)
		if len(detail) > 0 {
			fmt.Print(": ", fmt.Sprint(detail...))
		}
		fmt.Println()
	}
	fail := func(what string, err error) bool { step(false, what, err); return false }

	lib, err := d.boxes("library-reading-en")
	if err != nil {
		return fail("the kept boxes of the library", err)
	}
	menu, err := d.boxes("menu-library-en")
	if err != nil {
		return fail("the kept boxes of the menu", err)
	}
	stop, line, err := d.start()
	if stop != nil {
		defer stop()
	}
	if err != nil {
		return fail("the library drawn", err)
	}
	step(true, "the library drawn", line)

	scr, err := d.screen()
	if err != nil {
		return fail("the screen read", err)
	}
	// every button the render keeps has ink where it is; those that depend on the device (headphones, the pager when the
	// books fit one page) may be absent
	optional := map[string]bool{"topbar-phones": true, "list-prev": true, "list-next": true}
	for _, id := range []string{"topbar-start", "topbar-light", "topbar-battery", "topbar-menu", "filter-reading", "filter-new",
		"filter-finished", "filter-all", "list-sort", "topbar-phones", "list-prev", "list-next"} {
		b, _ := box(lib, id)
		n := scr.dark(b)
		switch {
		case n >= 20:
			step(true, "ink at "+id, fmt.Sprintf("%d dark pixels at %v", n, b))
		case optional[id]:
			fmt.Printf("note %s has no ink at %v (it depends on the device and the books)\n", id, b)
		default:
			step(false, "ink at "+id, fmt.Sprintf("%d dark pixels at %v", n, b))
		}
	}

	// the menu, by name; then its entries have ink where the render keeps them; Leave, by name
	if err := d.tap(lib, "topbar-menu"); err != nil {
		return fail("tap topbar-menu", err)
	}
	time.Sleep(2500 * time.Millisecond)
	if scr, err = d.screen(); err != nil {
		return fail("the screen read", err)
	}
	for _, id := range []string{"settings", "leave"} {
		b, _ := box(menu, id)
		n := scr.dark(b)
		step(n >= 20, "ink at the menu's "+id, fmt.Sprintf("%d dark pixels at %v", n, b))
	}
	if err := d.tap(menu, "leave"); err != nil {
		return fail("tap leave", err)
	}
	if _, err := d.expect(`library: left`, 10*time.Second); err != nil {
		return fail("the reader left", err)
	}
	step(true, "tap Leave: the reader left", "")
	if _, err := d.expect(`Nickel started again`, 20*time.Second); err != nil {
		return fail("the device's own interface back", err)
	}
	step(true, "the device's own interface back", "")
	return ok
}
