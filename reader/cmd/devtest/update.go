package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"shamsainkreader/update"
)

// Updates over Wi-Fi on the device (e2e/scenarios/update.txt): devtest serves the update itself, from this computer on
// the network the device reaches it by, as FarsiEPUB's latest.json and GitHub's release would.
//
//	update-serve latest|broken  the Kobo package of the latest build ($UPDATE_PKG, else the newest
//	                            out/release/shamsaink-*-kobo.tgz), announced as its version with ".e2e" after it (newer
//	                            than the same build installed) and signed with a key made for the run; broken: the same
//	                            with a reader that exits at once (the launcher must put the old one back). The device
//	                            reads it through dev/update-url and accepts the key through dev/update-key, both removed
//	                            when the scenario ends. A run with latest leaves the device on the latest build.
//	tap-dialog SCREEN CONTROL   a control of a dialog the reader placed itself (its log: "dialog ... at X,Y"), by the
//	                            screen's kept render moved there
type updateServer struct {
	sync.Mutex
	srv   *http.Server
	url   string
	key   ed25519.PrivateKey
	files map[string][]byte
}

// updateServe serves the update of kind (latest or broken) and points the device at it.
func (p *player) updateServe(kind string) (string, error) {
	d := p.d
	pkgPath := os.Getenv("UPDATE_PKG")
	if pkgPath == "" {
		m, _ := filepath.Glob("out/release/shamsaink-*-kobo.tgz")
		var newest os.FileInfo
		for _, f := range m {
			if st, err := os.Stat(f); err == nil && (newest == nil || st.ModTime().After(newest.ModTime())) {
				newest, pkgPath = st, f
			}
		}
	}
	pkg, err := os.ReadFile(pkgPath)
	if err != nil {
		return "", fmt.Errorf("no Kobo package to serve (make kobo-package, or UPDATE_PKG): %v", err)
	}
	built, err := packageVersion(pkg)
	if err != nil {
		return "", fmt.Errorf("%s: %v", pkgPath, err)
	}
	if kind == "broken" {
		if pkg, err = brokenPackage(pkg); err != nil {
			return "", err
		}
	}
	version := built + ".e2e"
	u := p.upd
	if u == nil {
		// this computer's address as the device sees it: the SSH connection's other end
		out, err := d.sh("echo $SSH_CLIENT", nil)
		host := strings.Fields(string(out))
		if err != nil || len(host) == 0 {
			return "", fmt.Errorf("this computer's address, as the device sees it (SSH_CLIENT): %v", err)
		}
		l, err := net.Listen("tcp", net.JoinHostPort(host[0], "0"))
		if err != nil {
			return "", fmt.Errorf("serving the update: %v", err)
		}
		_, key, _ := ed25519.GenerateKey(nil)
		u = &updateServer{key: key, url: "http://" + l.Addr().String()}
		u.srv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u.Lock()
			b, ok := u.files[path.Base(r.URL.Path)]
			u.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(b)
		})}
		go u.srv.Serve(l)
		p.upd = u
		pub := base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
		dir := d.a.Dir + "/dev"
		if _, err := d.sh(fmt.Sprintf("mkdir -p %s && echo %s > %s/update-url && echo '%s # e2e' > %s/update-key", dir, u.url+"/latest.json", dir, pub, dir), nil); err != nil {
			return "", fmt.Errorf("dev/update-url and dev/update-key on the device: %v", err)
		}
		p.cleanups = append(p.cleanups, func() {
			u.srv.Close()
			d.sh(fmt.Sprintf("rm -f %s/update-url %s/update-key; rmdir %s 2>/dev/null; true", dir, dir, dir), nil)
		})
	}
	sum := sha256.Sum256(pkg)
	name := fmt.Sprintf("shamsaink-%s-kobo.tgz", version)
	latest := fmt.Sprintf(`{"version":%q,"prerelease":true,"packages":{"kobo":{"url":%q,"size":%d,"sig":%q}}}`,
		version, u.url+"/"+name, len(pkg), u.url+"/"+name+".sig")
	u.Lock()
	u.files = map[string][]byte{"latest.json": []byte(latest), name: pkg, name + ".sig": update.Sign(u.key, "kobo", version, sum[:])}
	u.Unlock()
	if kind == "broken" {
		d.crashOK = true // the broken reader's exit is what is tested from here on
	}
	return fmt.Sprintf("%s (%s, %d bytes) at %s", version, kind, len(pkg), u.url), nil
}

// packageVersion is the VERSION file inside a Kobo package.
func packageVersion(pkg []byte) (string, error) {
	z, err := gzip.NewReader(bytes.NewReader(pkg))
	if err != nil {
		return "", err
	}
	t := tar.NewReader(z)
	for {
		h, err := t.Next()
		if err != nil {
			return "", fmt.Errorf("no VERSION in the package: %v", err)
		}
		if strings.HasSuffix(h.Name, ".adds/shamsaink/VERSION") {
			b, err := io.ReadAll(t)
			return strings.TrimSpace(string(b)), err
		}
	}
}

// brokenPackage is pkg with its reader replaced by a script that exits at once with an error.
func brokenPackage(pkg []byte) ([]byte, error) {
	z, err := gzip.NewReader(bytes.NewReader(pkg))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	tw := tar.NewWriter(zw)
	t := tar.NewReader(z)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(t)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(h.Name, ".adds/shamsaink/shamsaink") {
			b = []byte("#!/bin/sh\necho 'e2e: a broken reader, exiting'\nexit 1\n")
			h.Size = int64(len(b))
		}
		if err := tw.WriteHeader(h); err != nil {
			return nil, err
		}
		tw.Write(b)
	}
	tw.Close()
	zw.Close()
	return out.Bytes(), nil
}

var reDialogAt = regexp.MustCompile(`dialog .* at (\d+),(\d+)`)

// tapDialog taps a control of a dialog the reader placed itself: the screen's kept render, moved where the log's last
// "dialog ... at X,Y" says it is.
func (p *player) tapDialog(screen, id string) (string, error) {
	d := p.d
	r, name, err := p.render(screen)
	if err != nil {
		return "", err
	}
	b, ok := box(r, id)
	if !ok {
		return name, fmt.Errorf("no control %q in %s", id, name)
	}
	out, _ := d.sh(fmt.Sprintf("tail -n +%d %s | grep 'dialog .* at ' | tail -n 1", d.start0+1, d.a.Log), nil)
	m := reDialogAt.FindStringSubmatch(string(out))
	if m == nil {
		return name, fmt.Errorf("no dialog placed in the log")
	}
	var x, y int
	fmt.Sscan(m[1], &x)
	fmt.Sscan(m[2], &y)
	if err := d.settle(""); err != nil {
		return name, err
	}
	return fmt.Sprintf("%s at %d,%d", name, x, y), d.tapAt(x+(b[0]+b[2])/2, y+(b[1]+b[3])/2)
}
