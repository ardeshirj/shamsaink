// probe: what experiment 13's step 0 needs to know about a Kobo that a shell cannot read (PLAN.md): the framebuffer
// as its driver reports it, the input devices with their names and touch ranges, and what a touch sends. Reads only.
// A static ARMv7 binary, no cgo (build.sh); probe.sh runs it with the rest.
//
//	probe fb                  the framebuffer's variable and fixed screen information
//	probe input               every /dev/input/event*: its name, and its touch axes' ranges
//	probe touch DEV SECONDS   every event DEV sends for SECONDS, named, with its time
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

const (
	fbioGetVscreeninfo = 0x4600
	fbioGetFscreeninfo = 0x4602
)

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func u32(b []byte, o int) uint32 { return binary.LittleEndian.Uint32(b[o:]) }

// fb prints fb_var_screeninfo and fb_fix_screeninfo (32-bit ARM layouts), the fields a page's way into the
// framebuffer depends on.
func fb() error {
	f, err := os.Open("/dev/fb0")
	if err != nil {
		return err
	}
	defer f.Close()
	var v [160]byte
	var x [68]byte
	if err := ioctl(f.Fd(), fbioGetVscreeninfo, unsafe.Pointer(&v)); err != nil {
		return fmt.Errorf("FBIOGET_VSCREENINFO: %w", err)
	}
	if err := ioctl(f.Fd(), fbioGetFscreeninfo, unsafe.Pointer(&x)); err != nil {
		return fmt.Errorf("FBIOGET_FSCREENINFO: %w", err)
	}
	field := func(o int) string { return fmt.Sprintf("offset %d length %d", u32(v[:], o), u32(v[:], o+4)) }
	id := x[:16]
	for i, c := range id {
		if c == 0 {
			id = id[:i]
			break
		}
	}
	fmt.Printf("id %q\n", id)
	fmt.Printf("visible %d x %d, virtual %d x %d, offset %d, %d\n", u32(v[:], 0), u32(v[:], 4), u32(v[:], 8),
		u32(v[:], 12), u32(v[:], 16), u32(v[:], 20))
	fmt.Printf("bits per pixel %d, grayscale %d\n", u32(v[:], 24), u32(v[:], 28))
	fmt.Printf("red %s, green %s, blue %s, transparency %s\n", field(32), field(44), field(56), field(68))
	fmt.Printf("rotate %d (0 upright, 1 clockwise, 2 upside down, 3 counter-clockwise)\n", u32(v[:], 136))
	fmt.Printf("size %d x %d mm\n", u32(v[:], 92), u32(v[:], 88))
	fmt.Printf("line length %d bytes, memory %d bytes, visual %d\n", u32(x[:], 44), u32(x[:], 20), u32(x[:], 32))
	return nil
}

// eviocgname and eviocgabs are EVIOCGNAME(len) and EVIOCGABS(axis) (struct input_absinfo: 6 ints).
func eviocgname(n int) uintptr   { return 2<<30 | uintptr(n)<<16 | 'E'<<8 | 0x06 }
func eviocgabs(axis int) uintptr { return 2<<30 | 24<<16 | 'E'<<8 | uintptr(0x40+axis) }

var axes = map[int]string{0x00: "ABS_X", 0x01: "ABS_Y", 0x18: "ABS_PRESSURE", 0x2f: "ABS_MT_SLOT",
	0x30: "ABS_MT_TOUCH_MAJOR", 0x35: "ABS_MT_POSITION_X", 0x36: "ABS_MT_POSITION_Y", 0x39: "ABS_MT_TRACKING_ID",
	0x3a: "ABS_MT_PRESSURE"}

// input prints each event device's name and the ranges of the touch axes it has.
func input() error {
	paths, _ := filepath.Glob("/dev/input/event*")
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			fmt.Printf("%s: %v\n", p, err)
			continue
		}
		var name [128]byte
		ioctl(f.Fd(), eviocgname(len(name)), unsafe.Pointer(&name))
		n := 0
		for n < len(name) && name[n] != 0 {
			n++
		}
		fmt.Printf("%s: %q\n", p, name[:n])
		for _, a := range []int{0x00, 0x01, 0x18, 0x2f, 0x30, 0x35, 0x36, 0x39, 0x3a} {
			var info [6]int32 // value, minimum, maximum, fuzz, flat, resolution
			if ioctl(f.Fd(), eviocgabs(a), unsafe.Pointer(&info)) == nil && info[2] != info[1] {
				fmt.Printf("  %s: %d to %d\n", axes[a], info[1], info[2])
			}
		}
		f.Close()
	}
	return nil
}

var types = map[uint16]string{0: "EV_SYN", 1: "EV_KEY", 3: "EV_ABS"}
var keys = map[uint16]string{0x14a: "BTN_TOUCH", 0x145: "BTN_TOOL_FINGER", 0x74: "KEY_POWER"}

// touch prints every event dev sends for d (struct input_event on 32-bit ARM: timeval, type, code, value).
func touch(dev string, d time.Duration) error {
	f, err := os.Open(dev)
	if err != nil {
		return err
	}
	time.AfterFunc(d, func() { f.Close() })
	var ev [16]byte
	var t0 float64
	for {
		if _, err := io.ReadFull(f, ev[:]); err != nil {
			return nil
		}
		t := float64(u32(ev[:], 0)) + float64(u32(ev[:], 4))/1e6
		if t0 == 0 {
			t0 = t
		}
		typ, code := binary.LittleEndian.Uint16(ev[8:]), binary.LittleEndian.Uint16(ev[10:])
		val := int32(u32(ev[:], 12))
		name := fmt.Sprintf("0x%x", code)
		switch typ {
		case 0:
			name = map[uint16]string{0: "SYN_REPORT", 2: "SYN_MT_REPORT"}[code]
		case 1:
			if k, ok := keys[code]; ok {
				name = k
			}
		case 3:
			if a, ok := axes[int(code)]; ok {
				name = a
			}
		}
		tn, ok := types[typ]
		if !ok {
			tn = strconv.Itoa(int(typ))
		}
		fmt.Printf("%8.3f %s %s %d\n", t-t0, tn, name, val)
	}
}

func main() {
	var err error
	switch {
	case len(os.Args) == 2 && os.Args[1] == "fb":
		err = fb()
	case len(os.Args) == 2 && os.Args[1] == "input":
		err = input()
	case len(os.Args) == 4 && os.Args[1] == "touch":
		s, e := strconv.Atoi(os.Args[3])
		if e != nil {
			err = e
			break
		}
		err = touch(os.Args[2], time.Duration(s)*time.Second)
	default:
		fmt.Fprintln(os.Stderr, "probe fb | probe input | probe touch DEV SECONDS")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		os.Exit(1)
	}
}
