package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Where a device's package holds the reader's files (reader/scripts/package.sh): the Kobo's from the Kobo's root, the
// Kindle's from its USB drive's (/mnt/us), which also has the library's scriptlet beside the reader's folder.
const (
	koboRoot   = "mnt/onboard/.adds/shamsaink/"
	kindleRoot = "shamsaink/"
	scriptlet  = "documents/ShamsaInk.sh" // the Kindle's, in its library: from the drive's root
)

// The launcher's library in Kobo's tree, and its name in the package's launcher/Kobo.tgz.
const (
	LauncherPath = "/usr/local/Kobo/imageformats/libshamsaink-launcher.so"
	launcherIn   = "imageformats/libshamsaink-launcher.so"
)

// Folders and files of an update, in the reader's folder (none in a package, so a USB install leaves them alone):
//
//	update/package.tgz   the download, removed once installed
//	update/new/          the package unpacked, before its files are moved in
//	update/pending       the version installed, until it reaches the library (Reached); launch.sh puts previous/
//	                     back if the reader ends while it is there
//	update/added         the files the update added (none before it), removed when it is undone
//	update/undone        the version launch.sh undid, for the notice (Undone)
//	previous/            the files the update replaced or removed, as they were; previous/launcher.so the Kobo's
//	                     launcher library, previous/scriptlet.sh the Kindle's scriptlet, when they changed
const (
	updateDir   = "update"
	previousDir = "previous"
)

// What a package must hold.
var needed = []string{"shamsaink", "launch.sh", "VERSION"}

// Folders whose files an update replaces as a whole: a file no longer in the package is moved to previous/ (as
// install.sh removes it).
var replaced = []string{"fonts", "licenses"}

// PackagePath is where the download goes, in dir.
func PackagePath(dir string) string { return filepath.Join(dir, updateDir, "package.tgz") }

// Prepare empties update/ (what an earlier try left) for a download; previous/ is kept until an install replaces it.
func Prepare(dir string) error {
	u := filepath.Join(dir, updateDir)
	for _, f := range []string{"new", "package.tgz", "package.tgz.part"} {
		if err := os.RemoveAll(filepath.Join(u, f)); err != nil {
			return err
		}
	}
	return os.MkdirAll(u, 0o755)
}

// Install puts the verified package pkg of device ("kobo" or "kindle") in place in dir, version being its version: unpacked into
// update/new, then each file moved in, the one it replaces moved into previous/ first; the launcher's library
// (launcher, "" for none: the simulator) written beside it and moved in when it differs, the old one kept as
// previous/launcher.so; the Kindle's scriptlet the same way beside dir (../documents), the old one previous/scriptlet.sh.
// update/pending and update/added are written before anything moves, so a reader that stops
// half way (the power) is undone by launch.sh like one that does not start. A failure here undoes what was moved.
func Install(pkg, dir, device, version, launcher string) error {
	stage := filepath.Join(dir, updateDir, "new")
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	os.RemoveAll(stage + ".extra")
	root, extra := koboRoot, ""
	switch device {
	case "kobo":
	case "kindle":
		root, extra, launcher = kindleRoot, scriptlet, ""
	default:
		return fmt.Errorf("no updates for the device %q", device)
	}
	files, err := unpack(pkg, stage, root, extra)
	if err != nil {
		return err
	}
	for _, f := range needed {
		if !slices.Contains(files, f) {
			return fmt.Errorf("%w: the package has no %s", ErrUnverified, f)
		}
	}
	// the files of dir an update replaces that are not in the package any more
	var gone []string
	for _, d := range replaced {
		old, _ := os.ReadDir(filepath.Join(dir, d))
		for _, e := range old {
			if f := d + "/" + e.Name(); e.Type().IsRegular() && !slices.Contains(files, f) {
				gone = append(gone, f)
			}
		}
	}
	var added []string
	for _, f := range files {
		if _, err := os.Lstat(filepath.Join(dir, f)); errors.Is(err, fs.ErrNotExist) {
			added = append(added, f)
		}
	}
	prev := filepath.Join(dir, previousDir)
	if err := os.RemoveAll(prev); err != nil {
		return err
	}
	if err := os.MkdirAll(prev, 0o755); err != nil {
		return err
	}
	// the launcher's library, if the package's differs from the one in place: written beside it, moved in last
	var newLauncher []byte
	if launcher != "" && slices.Contains(files, "launcher/Kobo.tgz") {
		if b, err := fromTgz(filepath.Join(stage, "launcher/Kobo.tgz"), launcherIn); err == nil {
			if cur, err := os.ReadFile(launcher); err != nil || !bytes.Equal(cur, b) {
				newLauncher = b
			}
		}
	}
	// the Kindle's scriptlet, if the package's differs from the one in the library
	var newScript []byte
	scriptPath := filepath.Join(filepath.Dir(dir), filepath.FromSlash(scriptlet))
	if extra != "" {
		if b, err := os.ReadFile(filepath.Join(stage+".extra", path.Base(extra))); err == nil {
			if cur, err := os.ReadFile(scriptPath); err != nil || !bytes.Equal(cur, b) {
				newScript = b
			}
		}
	}
	u := filepath.Join(dir, updateDir)
	if err := writeSynced(filepath.Join(u, "added"), []byte(strings.Join(added, "\n")+"\n")); err != nil {
		return err
	}
	if err := writeSynced(filepath.Join(u, "pending"), []byte(version+"\n")); err != nil {
		return err
	}
	type move struct{ from, to string }
	var done []move
	undo := func() {
		for i := len(done) - 1; i >= 0; i-- {
			os.Rename(done[i].to, done[i].from)
		}
		os.Remove(filepath.Join(u, "pending"))
		os.Remove(filepath.Join(u, "added"))
	}
	mv := func(from, to string) error {
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
		done = append(done, move{from, to})
		return nil
	}
	for _, f := range gone {
		if err := mv(filepath.Join(dir, f), filepath.Join(prev, f)); err != nil {
			undo()
			return err
		}
	}
	for _, f := range files {
		if !slices.Contains(added, f) {
			if err := mv(filepath.Join(dir, f), filepath.Join(prev, f)); err != nil {
				undo()
				return err
			}
		}
		if err := mv(filepath.Join(stage, f), filepath.Join(dir, f)); err != nil {
			undo()
			return err
		}
	}
	for _, d := range []string{"books", "sleep"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}
	if newLauncher != nil {
		if cur, err := os.ReadFile(launcher); err == nil {
			if err := writeSynced(filepath.Join(prev, "launcher.so"), cur); err != nil {
				undo()
				return err
			}
		}
		if err := writeSynced(launcher+".new", newLauncher); err != nil {
			undo()
			return err
		}
		os.Chmod(launcher+".new", 0o755)
		if err := os.Rename(launcher+".new", launcher); err != nil {
			os.Remove(launcher + ".new")
			undo()
			return err
		}
		fmt.Println("update: the launcher's library is new, loaded when Kobo's interface next starts")
	}
	if newScript != nil {
		if cur, err := os.ReadFile(scriptPath); err == nil {
			if err := writeSynced(filepath.Join(prev, "scriptlet.sh"), cur); err != nil {
				undo()
				return err
			}
		}
		os.MkdirAll(filepath.Dir(scriptPath), 0o755)
		if err := writeSynced(scriptPath+".new", newScript); err != nil {
			undo()
			return err
		}
		if err := os.Rename(scriptPath+".new", scriptPath); err != nil {
			os.Remove(scriptPath + ".new")
			undo()
			return err
		}
		fmt.Println("update: the library's scriptlet is new")
	}
	os.RemoveAll(stage)
	os.RemoveAll(stage + ".extra")
	os.Remove(pkg)
	// the opening and loading screens drawn by the old version (install.sh does the same)
	for _, g := range []string{"splash-*", "loading-*"} {
		m, _ := filepath.Glob(filepath.Join(dir, "cache", g))
		for _, f := range m {
			os.RemoveAll(f)
		}
	}
	syscall.Sync()
	fmt.Printf("update: %s installed, %d files (%d new), %d removed; the old ones in previous/\n", version, len(files), len(added), len(gone))
	return nil
}

// unpack writes a package's files under stage, the paths below root (its reader's folder); only plain files and
// folders inside it (a link, a device or a path leaving it is refused), and extra, a file outside it the package may
// have (the Kindle's scriptlet), written to stage.extra/ under its own name. The reader's files, sorted.
func unpack(pkg, stage, root, extra string) ([]string, error) {
	f, err := os.Open(pkg)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnverified, err)
	}
	t := tar.NewReader(z)
	var files []string
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnverified, err)
		}
		name := path.Clean(strings.TrimPrefix(h.Name, "./"))
		if extra != "" && name == extra && h.Typeflag == tar.TypeReg {
			b, err := io.ReadAll(io.LimitReader(t, 1<<20))
			if err == nil {
				err = os.MkdirAll(stage+".extra", 0o755)
			}
			if err == nil {
				err = os.WriteFile(filepath.Join(stage+".extra", path.Base(extra)), b, 0o755)
			}
			if err != nil {
				return nil, err
			}
			continue
		}
		if name+"/" == root || !strings.HasPrefix(name+"/", root) {
			if h.Typeflag == tar.TypeDir {
				continue // ./mnt, ./mnt/onboard, ./documents, ...
			}
			return nil, fmt.Errorf("%w: %s is outside the reader's folder", ErrUnverified, h.Name)
		}
		rel := strings.TrimPrefix(name, root)
		if rel == "" || strings.HasPrefix(rel, "../") || rel == ".." || strings.Contains(rel, "/../") {
			return nil, fmt.Errorf("%w: %s", ErrUnverified, h.Name)
		}
		to := filepath.Join(stage, filepath.FromSlash(rel))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(to, 0o755); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return nil, err
			}
			mode := os.FileMode(0o644)
			if h.Mode&0o111 != 0 {
				mode = 0o755
			}
			w, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(w, t)
			if err == nil {
				err = w.Sync()
			}
			if cerr := w.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return nil, err
			}
			files = append(files, rel)
		default:
			return nil, fmt.Errorf("%w: %s is not a plain file", ErrUnverified, h.Name)
		}
	}
	slices.Sort(files)
	return files, nil
}

// fromTgz is the file name inside the .tgz at p.
func fromTgz(p, name string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	t := tar.NewReader(z)
	for {
		h, err := t.Next()
		if err != nil {
			return nil, err
		}
		if path.Clean(strings.TrimPrefix(h.Name, "./")) == name {
			return io.ReadAll(io.LimitReader(t, 64<<20))
		}
	}
}

func writeSynced(p string, b []byte) error {
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Reached is called once the reader shows the library: an update waiting for it (update/pending) has worked, and
// launch.sh will not undo it. The version, "" when none was waiting.
func Reached(dir string) string {
	p := filepath.Join(dir, updateDir, "pending")
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	os.Remove(p)
	os.Remove(filepath.Join(dir, updateDir, "added"))
	syscall.Sync()
	return strings.TrimSpace(string(b))
}

// Undone is the version launch.sh undid (update/undone), once: "" when none. The notice says so.
func Undone(dir string) string {
	p := filepath.Join(dir, updateDir, "undone")
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	os.Remove(p)
	v := strings.TrimSpace(string(b))
	if v == "" {
		v = "?"
	}
	return v
}
