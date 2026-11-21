package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/ui"
	"shamsainkreader/update"
)

// Updates over Wi-Fi from Settings' Updates row (roadmap step 2; the package update): Check asks before turning Wi-Fi
// on when it is off, reads latest.json, and offers a newer version; Update downloads it, verifies it, installs it and
// ends the reader with update.RestartCode, so the launcher starts the new version (and undoes it if it does not reach
// the library). Wi-Fi turned on for it is turned off again after. Only when asked: nothing here runs by itself.

// errRestart ends the library once an update is installed: main exits with update.RestartCode.
var errRestart = errors.New("an update was installed: restarting")

// What an update's check or download sent back to the library's loop.
type updMsg struct {
	kind   string        // "checked", "progress", "installed" or "failed"
	latest update.Latest // checked: the newer release (Version "" when there is none)
	err    error         // checked or failed: what went wrong
	share  int           // progress: per cent downloaded
}

// updater is the update under way in the library: at most one check or download.
type updater struct {
	out     chan updMsg
	cancel  context.CancelFunc
	latest  update.Latest // the newer release offered
	dialog  string        // its dialog on the screen: "ask", "available", "progress", "notice" ("" none)
	wifi    bool          // waiting for Wi-Fi asked for by Check
	shown   int           // the share last drawn, and when
	shownAt time.Time
}

func newUpdater() *updater { return &updater{out: make(chan updMsg, 1)} }

// updateDevice is the package's device name for this device ("" when it has no updates over Wi-Fi yet).
func (rd *reader) updateDevice() string {
	if rd.dev.Kind == device.Kobo {
		return "kobo"
	}
	return ""
}

// check reads latest.json in the background; a release with no package for this device is no newer one.
func (rd *reader) updateCheck(u *updater) {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	url := update.URL(rd.dir, ui.DevMode)
	go func() {
		t := time.Now()
		l, err := update.Check(ctx, url)
		m := updMsg{kind: "checked", err: err}
		switch {
		case err != nil:
			fmt.Printf("update: %s: %v\n", url, err)
		case !update.Newer(l.Version, version):
			fmt.Printf("update: %s is the latest (%s says %s, in %.0f ms)\n", version, url, l.Version, ms(t))
		case l.Packages[rd.updateDevice()].URL == "":
			fmt.Printf("update: %s has no package for %s\n", l.Version, rd.updateDevice())
		default:
			fmt.Printf("update: %s is newer than %s (in %.0f ms)\n", l.Version, version, ms(t))
			m.latest = l
		}
		u.out <- m
	}()
}

// download fetches, verifies and installs u.latest in the background, its share sent as it comes (a share is dropped
// when the loop has not taken the last one: only the newest matters).
func (rd *reader) updateDownload(u *updater) {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	l, dev := u.latest, rd.updateDevice()
	keys := update.Keys(rd.dir, ui.DevMode)
	launcher := update.LauncherPath
	if device.Sim() {
		launcher = ""
	}
	go func() {
		t := time.Now()
		fail := func(err error) { u.out <- updMsg{kind: "failed", err: err} }
		if err := update.Prepare(rd.dir); err != nil {
			fail(err)
			return
		}
		pkg := update.PackagePath(rd.dir)
		last := -1
		err := update.Download(ctx, l.Packages[dev], dev, l.Version, keys, pkg, func(done, total int64) {
			if s := int(done * 100 / total); s != last {
				last = s
				select {
				case u.out <- updMsg{kind: "progress", share: s}:
				default:
				}
			}
		})
		if err != nil {
			fmt.Printf("update: %s: %v\n", l.Version, err)
			fail(err)
			return
		}
		fmt.Printf("update: %s downloaded and verified in %.1f s (%d bytes)\n", l.Version, time.Since(t).Seconds(), l.Packages[dev].Size)
		if err := update.Install(pkg, rd.dir, l.Version, launcher); err != nil {
			fmt.Printf("update: %s not installed: %v\n", l.Version, err)
			fail(err)
			return
		}
		u.out <- updMsg{kind: "installed"}
	}()
}

// updateNotice is the notice for what went wrong: its words, icon and note.
func updateNotice(err error) (key, icon string) {
	switch {
	case errors.Is(err, update.ErrCut):
		return "update-cut", "warning-circle"
	case errors.Is(err, update.ErrUnverified):
		return "update-unverified", "warning-circle"
	case errors.Is(err, update.ErrUnreachable):
		return "update-unreachable", "warning-circle"
	}
	return "update-cut", "warning-circle" // the storage (full, failing): nothing was changed either
}
