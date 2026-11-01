// Command devtest holds a device against the design spec's profile of it, over SSH (docs/design-system.md).
//
//	devtest check DEVICE     read-only: the device's screen, touch, fonts and reader against the profile
//
// DEVICE is a profile of reader/design/spec/devices and a key of e2e/devices.yaml. Run from the repository's root. It uses
// the system's ssh (the host named in devices.yaml), changes nothing on the device, and reads no key.
package main

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"shamsainkreader/design"
)

// Access is how to reach and drive a device (e2e/devices.yaml).
type Access struct {
	SSH   string `yaml:"ssh"`
	Dir   string `yaml:"dir"`
	Log   string `yaml:"log"`
	Touch struct {
		Node string `yaml:"node"`
		Name string `yaml:"name"`
	} `yaml:"touch"`
	Framebuffer struct {
		Stride int `yaml:"stride"`
		Bits   int `yaml:"bits"`
	} `yaml:"framebuffer"`
	FontsLocal  string `yaml:"fonts_local"`
	ReaderLocal string `yaml:"reader_local"`
}

func main() {
	if len(os.Args) != 3 || os.Args[1] != "check" {
		fmt.Fprintln(os.Stderr, "usage: devtest check DEVICE")
		os.Exit(2)
	}
	if !check(os.Args[2]) {
		os.Exit(1)
	}
}

// ssh runs a command on the device and gives what it printed.
func ssh(a *Access, cmd string) (string, error) {
	out, err := exec.Command("ssh", "-o", "ConnectTimeout=10", "-o", "BatchMode=yes", a.SSH, cmd).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func check(name string) bool {
	spec, err := design.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "spec:", err)
		return false
	}
	var profile *design.Device
	for i := range spec.Devices {
		if spec.Devices[i].Name == name {
			profile = &spec.Devices[i]
		}
	}
	b, err := os.ReadFile("e2e/devices.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, err, "(run from the repository's root)")
		return false
	}
	var all map[string]*Access
	if err := yaml.Unmarshal(b, &all); err != nil {
		fmt.Fprintln(os.Stderr, "e2e/devices.yaml:", err)
		return false
	}
	a := all[name]
	if profile == nil || a == nil {
		fmt.Fprintf(os.Stderr, "no device %q in the spec's profiles and e2e/devices.yaml\n", name)
		return false
	}
	ok := true
	report := func(good bool, what, detail string) {
		mark := "ok  "
		if !good {
			mark, ok = "FAIL", false
		}
		fmt.Printf("%s %s", mark, what)
		if detail != "" {
			fmt.Printf(": %s", detail)
		}
		fmt.Println()
	}

	if out, err := ssh(a, "echo up"); err != nil || out != "up" {
		report(false, "reachable over ssh "+a.SSH, fmt.Sprint(out, err)+" (is it awake? make kobo-wait)")
		return false
	}
	report(true, "reachable over ssh "+a.SSH, "")

	// the screen: size and stride, against the profile and the access facts
	out, _ := ssh(a, "cat /sys/class/graphics/fb0/virtual_size /sys/class/graphics/fb0/stride /sys/class/graphics/fb0/bits_per_pixel")
	var w, h, stride, bits int
	fmt.Sscanf(strings.NewReplacer(",", " ", "\n", " ").Replace(out), "%d %d %d %d", &w, &h, &stride, &bits)
	report(w == profile.Screen.Width && h == profile.Screen.Height, "screen size",
		fmt.Sprintf("device %d × %d, profile %d × %d", w, h, profile.Screen.Width, profile.Screen.Height))
	report(stride == a.Framebuffer.Stride && bits == a.Framebuffer.Bits, "framebuffer stride and depth",
		fmt.Sprintf("device %d bytes, %d bits; expected %d, %d", stride, bits, a.Framebuffer.Stride, a.Framebuffer.Bits))
	if stride > 0 && w > 0 {
		report(stride == w*bits/8, "stride is the width times the pixel", fmt.Sprintf("%d = %d × %d", stride, w, bits/8))
	}

	// the touch screen: the node is the one that has the name
	node := filepath.Base(a.Touch.Node)
	touchName, _ := ssh(a, "cat /sys/class/input/"+node+"/device/name")
	report(touchName == a.Touch.Name, "touch screen "+a.Touch.Node, fmt.Sprintf("is %q, expected %q", touchName, a.Touch.Name))

	// the fonts: the device's are the files the renders are drawn with, by hash
	local, err := filepath.Glob(filepath.Join(a.FontsLocal, "*.ttf"))
	if err != nil || len(local) == 0 {
		report(false, "local fonts "+a.FontsLocal, "none found (make reader builds them)")
	} else {
		sort.Strings(local)
		remote, _ := ssh(a, "cd "+a.Dir+"/fonts && md5sum *.ttf")
		have := map[string]string{}
		for _, l := range strings.Split(remote, "\n") {
			if f := strings.Fields(l); len(f) == 2 {
				have[f[1]] = f[0]
			}
		}
		for _, p := range local {
			data, err := os.ReadFile(p)
			if err != nil {
				report(false, "font "+filepath.Base(p), err.Error())
				continue
			}
			sum := fmt.Sprintf("%x", md5.Sum(data))
			got, present := have[filepath.Base(p)]
			switch {
			case !present:
				report(false, "font "+filepath.Base(p), "not on the device")
			default:
				report(got == sum, "font "+filepath.Base(p), map[bool]string{true: "", false: "differs from the renders' file"}[got == sum])
			}
		}
		for f := range have {
			found := false
			for _, p := range local {
				found = found || filepath.Base(p) == f
			}
			if !found {
				report(false, "font "+f, "on the device, not in the renders' folder")
			}
		}
	}

	// the reader: installed, and the same file as the latest local build (a note, not a failure: an old one is a choice)
	sum, _ := ssh(a, "md5sum "+a.Dir+"/shamsaink | cut -d' ' -f1")
	running, _ := ssh(a, "pidof shamsaink || echo none")
	report(len(sum) == 32, "reader installed", "running: "+running)
	if built, err := os.ReadFile(a.ReaderLocal); err == nil && len(sum) == 32 {
		if got := fmt.Sprintf("%x", md5.Sum(built)); got == sum {
			fmt.Println("ok   installed reader is the latest local build")
		} else {
			fmt.Println("note installed reader differs from " + a.ReaderLocal + " (make kobo-reader-install puts the new one)")
		}
	}
	return ok
}
