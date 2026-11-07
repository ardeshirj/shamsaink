package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
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
	// start0 is the log's length when the reader was started: a crash is looked for after it
	start0 int
	watch  bool   // a scenario's run: a crash in the log ends it
	env    string // KEY=VALUE settings the reader is started with (a scenario's env step)
	book   string // the book the reader is started on, by its path under the device's folder (open PATH); "" for the library
	// crashOK: the reader's end is expected (a scenario's freeze: the watchdog ends it), not a crash
	crashOK bool
}

// reCrash is a reader that ended badly: Go's panic, fatal error or fatal signal, or the launcher's line for an exit other than 0.
var reCrash = regexp.MustCompile(`^panic: |^fatal error: |^SIG[A-Z]+: |launch: reader exited \([1-9]\d*\)`)

// crashed says whether the log has the reader's crash since the run started: the run ends there, not ten steps later
// at a time-out. The lines from the crash on are in the error.
func (d *dev) crashed() error {
	out, _ := d.sh(fmt.Sprintf("tail -n +%d %s", d.start0+1, d.a.Log), nil)
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	for i, l := range lines {
		if reCrash.MatchString(l) {
			end := min(i+6, len(lines))
			if !strings.Contains(l, "launch: ") {
				return fmt.Errorf("the reader crashed: %q", lines[i:end])
			}
			return fmt.Errorf("the reader ended: %q, the lines before it: %q", l, lines[max(i-6, 0):i])
		}
	}
	return nil
}

// fb is the framebuffer's node on the device; fbHash its hash, taken there.
func (d *dev) fb() string {
	if d.a.Framebuffer.Node != "" {
		return d.a.Framebuffer.Node
	}
	return "/dev/fb0"
}

func (d *dev) fbHash() string {
	out, _ := d.sh("md5sum < "+d.fb()+" | cut -c1-32", nil)
	return strings.TrimSpace(string(out))
}

// settle waits for the screen to settle: the framebuffer the same over three looks 0.2 s apart (what the reader draws
// is written at once; between two of its draws there is no such pause). With from, it first waits for the screen to
// change from that hash (a tap whose answer has no log line). A wait on something, not a sleep: under a second
// when nothing moves; 15 s at most.
func (d *dev) settle(from string) error {
	script := `fb=` + d.fb() + `; from=` + from + `; p=; n=0; i=0
while [ $i -lt 75 ]; do
  h=$(md5sum < $fb | cut -c1-32); i=$((i+1))
  if [ -n "$from" ]; then [ "$h" != "$from" ] && from=; p=$h; sleep 0.2; continue; fi
  if [ "$h" = "$p" ]; then n=$((n+1)); else n=0; fi
  [ $n -ge 3 ] && { echo settled; exit 0; }
  p=$h; sleep 0.2
done
[ -n "$from" ] && echo unchanged || echo moving`
	out, err := d.sh(script, nil)
	switch s := strings.TrimSpace(string(out)); {
	case err != nil:
		return err
	case s == "unchanged":
		return fmt.Errorf("the screen did not change in 15 s")
	case s != "settled":
		return fmt.Errorf("the screen did not settle in 15 s")
	}
	return nil
}

// The reader's own files (its place in each book, highlights, settings, library): saved before a scenario and put back
// after, so a run starts known and leaves the device as it found it, whatever the scenario did or where it failed. A
// copy left by a run that never put it back (devtest killed, the device restarted) is put back first.
const (
	stateFiles = "settings.json library.json library-view.json places highlights"
	savedDir   = ".e2e-saved"
)

func (d *dev) knownStart(fixture string) (string, error) {
	said := ""
	d.start0, d.watch = d.logLines(), true
	if out, _ := d.sh("test -d "+d.a.Dir+"/"+savedDir+" && echo yes", nil); strings.TrimSpace(string(out)) == "yes" {
		if _, err := d.putBack(); err != nil {
			return "", err
		}
		said = "a copy left by an earlier run put back first; "
	}
	if _, err := d.sh("cd "+d.a.Dir+" && mkdir "+savedDir+" && for f in "+stateFiles+"; do [ ! -e $f ] || cp -Rp $f "+savedDir+"/; done", nil); err != nil {
		return "", fmt.Errorf("saving the reader's files: %v", err)
	}
	dir := filepath.Join("e2e", "fixtures", fixture)
	var laid []string
	err := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		laid = append(laid, rel)
		_, err = d.sh("cd "+d.a.Dir+" && mkdir -p $(dirname "+rel+") && cat > "+rel, data)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("laying the fixture: %v", err)
	}
	return said + "the reader's files saved, " + strings.Join(laid, ", ") + " laid from e2e/fixtures/" + fixture, nil
}

// putBack waits for the reader to be gone (it writes its place as it leaves), then puts its saved files back.
func (d *dev) putBack() (string, error) {
	for i := 0; i < 50; i++ {
		if out, _ := d.sh("pidof shamsaink || echo none", nil); strings.TrimSpace(string(out)) == "none" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	_, err := d.sh("cd "+d.a.Dir+" && test -d "+savedDir+" && ! pidof shamsaink >/dev/null && rm -rf "+stateFiles+
		" && cp -Rp "+savedDir+"/. . && rm -rf "+savedDir, nil)
	if err != nil {
		return "", fmt.Errorf("the reader still running, or no saved copy: %v", err)
	}
	return "the reader's own files put back", nil
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
	if d.a != nil && d.a.Profile != "" {
		d.name = d.a.Profile
	}
	for i := range spec.Devices {
		if spec.Devices[i].Name == d.name {
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
	if d.a.Local != "" {
		c = exec.Command("sh", "-c", cmd)
	}
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
		crash := false
		for i, l := range lines {
			if re.MatchString(l) {
				d.mark += i + 1
				return l, nil
			}
			crash = crash || reCrash.MatchString(l)
		}
		if crash && !d.crashOK && d.watch { // not waited out: the reader is gone
			return "", d.crashed()
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
	zipped, err := d.sh("gzip -1 -c "+d.fb(), nil) // 8.5 MB, mostly paper: a third of a megabyte over Wi-Fi
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(z)
	if err != nil {
		return nil, err
	}
	w, h, stride, bpp := d.prof.Screen.Width, d.prof.Screen.Height, d.a.Framebuffer.Stride, d.a.Framebuffer.Bits/8
	if len(raw) < h*stride {
		return nil, fmt.Errorf("the framebuffer gave %d bytes, want %d", len(raw), h*stride)
	}
	g := 0 // the grey: a pixel's byte on the Kindle (8 bits), its green on the Kobo (32 bits)
	if bpp == 4 {
		g = 1
	}
	s := &screen{w, h, make([]byte, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s.pix[y*w+x] = raw[y*stride+x*bpp+g]
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

// record adds a line to the recording's log when the reader is started recording (env SHAMSAINK_RECORD=DIR, for a
// video of the simulator: e2e/video/), after its time; it says whether it did.
func (d *dev) record(line string) bool {
	dir := ""
	for _, kv := range strings.Fields(d.env) {
		if v, ok := strings.CutPrefix(kv, "SHAMSAINK_RECORD="); ok {
			dir = v
		}
	}
	if dir == "" {
		return false
	}
	_, err := d.sh("mkdir -p "+dir+" && cat >> "+dir+"/log", fmt.Appendf(nil, "%d %s\n", time.Now().UnixMilli(), line))
	return err == nil
}

// tapAt writes a finger down and up at the screen's (x, y) into the touch screen.
func (d *dev) tapAt(x, y int) error {
	d.record(fmt.Sprintf("touch tap %d %d", x, y))
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

// press writes a page button's press and release into the buttons' input ("gpio-keys", event0 on the Kobo and the
// simulated Kobo, beside the touch screen's node): the lower (194) for forward, the upper (193) for back.
func (d *dev) press(forward bool) error {
	code, name := uint16(193), "upper"
	if forward {
		code, name = 194, "lower"
	}
	d.record("button " + name)
	const evKey = 1
	b := append(event(evKey, code, 1), event(evSyn, 0, 0)...)
	b = append(b, event(evKey, code, 0)...)
	b = append(b, event(evSyn, 0, 0)...)
	_, err := d.sh("cat > "+filepath.Dir(d.a.Touch.Node)+"/event0", b)
	return err
}

// power writes the power button's press and release into its input (bd71828-pwrkey: event3 on the Kobo and the
// simulated Kobo, event0 on the Kindle, where powerd hears it): the reader sleeps, or wakes.
func (d *dev) power() error {
	d.record("button power")
	const evKey, keyPower = 1, 116
	b := append(event(evKey, keyPower, 1), event(evSyn, 0, 0)...)
	b = append(b, event(evKey, keyPower, 0)...)
	b = append(b, event(evSyn, 0, 0)...)
	node := "event3"
	if d.place() == "kindle" {
		node = "event0"
	}
	_, err := d.sh("cat > "+filepath.Dir(d.a.Touch.Node)+"/"+node, b)
	return err
}

// place is where the device is, as a scenario's on: names it and refs.sum keys it: sim (the simulated Kobo), kobo or
// kindle.
func (d *dev) place() string {
	switch {
	case d.a.Local != "":
		return "sim"
	case strings.HasPrefix(d.name, "kindle"):
		return "kindle"
	}
	return "kobo"
}

// home is the launcher's line once the device's own interface is back: Kobo's Nickel started again (the simulated
// launcher says the same), or Amazon's framework resumed on the Kindle.
func (d *dev) home() string {
	if d.place() == "kindle" {
		return `framework resumed`
	}
	return `Nickel started again`
}

// holdAt writes a finger down at the screen's (x, y), held still past the reader's hold time, moved to (x2, y2) in three
// steps, and lifted.
func (d *dev) holdAt(x, y, x2, y2 int) error {
	at := func(px, py int) []byte {
		b := append(event(evAbs, posX, int32(d.turn(d.a.Touch.Turn.X, px, py))), event(evAbs, posY, int32(d.turn(d.a.Touch.Turn.Y, px, py)))...)
		return append(b, event(evSyn, 0, 0)...)
	}
	write := func(b []byte) error { _, err := d.sh("cat > "+d.a.Touch.Node, b); return err }
	d.record(fmt.Sprintf("touch hold %d %d %d %d", x, y, x2, y2)) // held 1.5 s, then moved over 0.9 s
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
	if _, err := d.sh("cd "+d.a.Dir+" && nohup env "+d.env+" sh launch.sh "+d.book+" >/dev/null 2>&1 </dev/null &", nil); err != nil {
		return nil, "", err
	}
	stop = func() {
		if out, _ := d.sh("pidof shamsaink || echo none", nil); strings.TrimSpace(string(out)) != "none" {
			fmt.Println("note the reader is still running: stopping it (launch.sh starts the device's interface again)")
			d.sh("kill $(pidof shamsaink)", nil)
		}
	}
	shown := `library: \d+ books.*shown in`
	if d.book != "" {
		shown = `opened \d+ ms` // the book itself, no library
	}
	line, err = d.expect(shown, 40*time.Second)
	if err != nil {
		return stop, "", err
	}
	return stop, line, d.settle("")
}

// leave taps the library's menu and Leave by name, and waits for the device's own interface.
func (d *dev) leave(lib, menu *boxes) error {
	before := d.fbHash()
	if err := d.tap(lib, "topbar-menu"); err != nil {
		return err
	}
	if err := d.settle(before); err != nil { // the menu has no log line: its drawing is waited for on the screen
		return err
	}
	if err := d.tap(menu, "leave"); err != nil {
		return err
	}
	if _, err := d.expect(`library: left`, 10*time.Second); err != nil {
		return err
	}
	_, err := d.expect(d.home(), 20*time.Second)
	return err
}
