package main

import (
	"fmt"
	"strconv"
	"strings"
)

// koboCheck holds a Kobo to what ShamsaInk needs beside Kobo's own interface on firmware 5.x/6.x (2026-11-11,
// docs/devices/kobo/README.md): the firmware, the launcher in place, Kobo's library kept out of ours, nothing of the
// development SSH held open on the storage (or Kobo cannot lend it over USB). Read only.
func koboCheck(a *Access, report func(good bool, what, detail string)) {
	k := a.Kobo
	fw, _ := ssh(a, "cut -d, -f3 /mnt/onboard/.kobo/version")
	major, err := strconv.Atoi(strings.SplitN(fw, ".", 2)[0])
	report(err == nil && major >= k.MinFirmware, "firmware "+fw, fmt.Sprintf("launch.sh needs %d or newer", k.MinFirmware))

	out, _ := ssh(a, "test -f "+k.Launcher+" && echo in; test -f /usr/local/Kobo/libshamsaink-launcher.so.parked && echo parked")
	report(strings.Contains(out, "in") && !strings.Contains(out, "parked"), "launcher "+k.Launcher,
		map[bool]string{true: "", false: "missing or parked (" + strings.ReplaceAll(out, "\n", ", ") +
			"): make kobo-launcher-install, then restart the Kobo"}[strings.Contains(out, "in") && !strings.Contains(out, "parked")])

	line, _ := ssh(a, "grep '^ExcludeSyncFolders=' '"+k.Conf+"'")
	want := "ExcludeSyncFolders=" + k.ExcludeSyncFolders
	report(line == want, "Kobo's library kept out of "+a.Dir+" (ExcludeSyncFolders)",
		map[bool]string{true: "", false: fmt.Sprintf("is %q; add %q under [FeatureSettings] with Nickel stopped, then connect USB once", line, want)}[line == want])

	n, _ := ssh(a, "sqlite3 -readonly '"+k.Database+"' \"select count(*) from content where ContentType=6 and ContentID like '%/.adds/%';\"")
	report(n == "0", "no books from .adds/ in Kobo's library", map[bool]string{true: "", false: n + " (gone after a USB connection, once ExcludeSyncFolders is set)"}[n == "0"])

	conv, _ := ssh(a, "find "+a.Dir+" -name '*.txt.epub' | wc -l")
	report(conv == "0", "no .txt files converted by Kobo in "+a.Dir, map[bool]string{true: "", false: conv + " .txt.epub (a .txt file in the package?)"}[conv == "0"])

	held, _ := ssh(a, "for p in $(pidof shamsaink-dropbear dropbear); do ls -l /proc/$p/exe /proc/$p/fd 2>/dev/null; done | grep -c /mnt/onboard")
	report(held == "0", "development SSH holds nothing open on the storage", map[bool]string{true: "", false: held + " files (USB Connect fails: switch SSH off and on, dev/ssh.sh runs it from RAM)"}[held == "0"])
}
