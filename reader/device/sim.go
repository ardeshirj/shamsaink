package device

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// The simulated device (docs/TESTING.md, section 2): with SHAMSAINK_SIM=DIR the reader runs as a plain Linux program
// and every path of the hardware is looked up under DIR: the screen DIR/dev/fb0 (a plain file in the Kobo's framebuffer
// layout), the touch screen and the keys DIR/dev/input/eventN (named pipes, in the Kobo's input-event format), the
// battery and the light DIR/sys/class/... (plain files), the Kobo's version file. What a file cannot do is done here:
// no ioctls (a refresh is logged instead), no grab, no suspend (asleep until the power key's pipe has a press), no Wi-Fi.
// End-to-end scenarios drive it as they drive the Kobo (reader/cmd/devtest, e2e/sim/).
var simDir = os.Getenv("SHAMSAINK_SIM")

// Sim says whether the reader runs on the simulated device.
func Sim() bool { return simDir != "" }

// hw is path, under the simulator's folder on the simulated device.
func hw(path string) string { return simDir + path }

// The simulated screen: the Kobo Libra Colour's (reader/design/spec/devices/kobo-libra-colour.yaml), four bytes a pixel.
const (
	simW, simH, simBytes = 1264, 1680, 4
	simLine              = 5056 // bytes a row, as the Kobo's fb0 has it (e2e/devices.yaml)
)

// openSimScreen maps DIR/dev/fb0, made the screen's size if it is not.
func openSimScreen() (*Screen, error) {
	f, err := os.OpenFile(hw("/dev/fb0"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	size := simLine * simH
	if err := f.Truncate(int64(size)); err != nil {
		return nil, err
	}
	s := &Screen{f: f, W: simW, H: simH, bytes: simBytes, line: simLine}
	s.mem, err = syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	return s, err
}

// waveformNames are the waveforms' names, for the simulator's log of refreshes.
var waveformNames = map[uint32]string{WfGC16: "GC16", WfGL16: "GL16", WfREAGL: "REAGL", WfA2: "A2"}

// simRefresh logs a refresh the simulated screen would have sent: its region, waveform and flash. The scenarios read it
// to check that only what changed was refreshed. When recording, the region is kept too.
func simRefresh(s *Screen, r image.Rectangle, waveform uint32, flash bool) {
	name := waveformNames[waveform]
	if name == "" {
		name = fmt.Sprint(waveform)
	}
	fmt.Printf("sim refresh: %d,%d %dx%d %s flash=%t\n", r.Min.X, r.Min.Y, r.Dx(), r.Dy(), name, flash)
	if recDir != "" {
		recordRefresh(s, r, name, flash)
	}
}

// Recording, for a video of the simulator (e2e/video/): with SHAMSAINK_RECORD=DIR each refresh's region is saved as it
// was sent (DIR/NNNNNN.ppm, the framebuffer's colours) and listed in DIR/log with its time, and the sound the
// simulated sink plays is kept (DIR/sound-*.raw, Record). Times are Unix milliseconds, as devtest's captions have them.
var (
	recDir  = os.Getenv("SHAMSAINK_RECORD")
	recMu   sync.Mutex
	recN    int
	recOnce sync.Once
)

// RecordNote adds a line to the recording's log, after its time; nothing when not recording.
func RecordNote(format string, a ...any) {
	if recDir == "" {
		return
	}
	recMu.Lock()
	defer recMu.Unlock()
	recOnce.Do(func() { os.MkdirAll(recDir, 0o755) })
	f, err := os.OpenFile(filepath.Join(recDir, "log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "%d %s\n", time.Now().UnixMilli(), fmt.Sprintf(format, a...))
	f.Close()
}

// Record creates DIR/name for the recording, or gives nil when not recording.
func Record(name string) *os.File {
	if recDir == "" {
		return nil
	}
	recOnce.Do(func() { os.MkdirAll(recDir, 0o755) })
	f, err := os.Create(filepath.Join(recDir, name))
	if err != nil {
		return nil
	}
	return f
}

func recordRefresh(s *Screen, r image.Rectangle, waveform string, flash bool) {
	recMu.Lock()
	recN++
	n := recN
	recMu.Unlock()
	name := fmt.Sprintf("%06d.ppm", n)
	f := Record(name)
	if f == nil {
		return
	}
	buf := make([]byte, 0, r.Dx()*r.Dy()*3+32)
	buf = fmt.Appendf(buf, "P6\n%d %d\n255\n", r.Dx(), r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		at := (s.y0+y)*s.line + r.Min.X*s.bytes
		for x := 0; x < r.Dx(); x++ {
			buf = append(buf, s.mem[at+x*4:at+x*4+3]...) // red, green, blue
		}
	}
	f.Write(buf)
	f.Close()
	RecordNote("refresh %d %d %d %d %s %t %s", r.Min.X, r.Min.Y, r.Dx(), r.Dy(), waveform, flash, name)
}

// openInput opens an input node: on the simulated device a named pipe, opened for reading and writing so the open does
// not wait for a writer and the pipe is not closed between two writers (devtest writes each gesture on its own).
func openInput(path string) (*os.File, error) {
	if Sim() {
		return os.OpenFile(path, os.O_RDWR, 0)
	}
	return os.Open(path)
}
