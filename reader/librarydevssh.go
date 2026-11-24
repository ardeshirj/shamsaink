package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"shamsainkreader/ui"
)

// Dev mode's Developer panel (2026-11-11; dev mode: dev/ beside the reader, main.go): the development SSH's switch, the
// device's dev/ssh.sh (installed by make kobo-dev-ssh, never in a release; the Kobo's is reader/kobo/dev/ssh.sh). The script is the
// device's own, so nothing here asks which device it is on: `ssh.sh status` prints the state, `ssh.sh` switches it,
// each in one line ("SSH on: ADDRESS, port 2222", "SSH off"). The server it starts outlives the reader, so it serves
// for Kobo's own interface too once the reader is closed.

// developerView is what the Developer panel shows for the reader in dir.
func developerView(dir string) ui.Developer {
	if s := devSSHScript(dir); s != "" {
		return ui.Developer{SSH: devSSH(s, "status")}
	}
	return ui.Developer{}
}

// devSSHScript is the device's dev/ssh.sh under dir, or "" when there is none.
func devSSHScript(dir string) string {
	p := filepath.Join(dir, "dev", "ssh.sh")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// devSSH runs the script (status, or the switch with no argument), 3 s at most, and gives its last line.
func devSSH(script string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sh", append([]string{script}, args...)...).Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return last
	}
	if err != nil {
		return "SSH: " + err.Error()
	}
	return "SSH: no answer"
}
