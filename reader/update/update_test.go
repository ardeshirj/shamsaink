package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b  string
		newer bool
	}{
		{"v0.3.6-beta", "v0.3.5-beta", true},
		{"v0.3.5-beta", "v0.3.6-beta", false},
		{"v0.4.0", "v0.4.0-beta", true},
		{"v0.4.0-beta", "v0.4.0", false},
		{"v0.4.0-beta.2", "v0.4.0-beta", true},
		{"v0.4.0-beta.10", "v0.4.0-beta.2", true},
		{"v0.10.0", "v0.9.9", true},
		{"v0.3.6-beta", "v0.3.6-beta", false},
		{"v0.3.6-beta", "dev", true},
		{"dev", "v0.3.6-beta", false},
		// a build between releases (git describe): newer than its tag, older than the next one
		{"v0.3.6-beta", "v0.3.5-beta-12-g1234567", true},
		{"v0.3.5-beta-12-g1234567", "v0.3.5-beta", true},
	} {
		if got := Newer(c.a, c.b); got != c.newer {
			t.Errorf("Newer(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

func TestSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	other, _, _ := ed25519.GenerateKey(nil)
	sum := sha256.Sum256([]byte("package"))
	sig := Sign(priv, "kobo", "v0.4.0-beta", sum[:])
	keys := ParseKeys("# the release key\n" + base64.StdEncoding.EncodeToString(other) + "\n" + base64.StdEncoding.EncodeToString(pub) + " # dev\n")
	if len(keys) != 2 {
		t.Fatalf("%d keys", len(keys))
	}
	if !Verify(keys, "kobo", "v0.4.0-beta", sum[:], sig) {
		t.Error("a good signature refused")
	}
	other2 := sha256.Sum256([]byte("other"))
	for _, c := range []struct{ dev, v string }{{"kindle", "v0.4.0-beta"}, {"kobo", "v0.3.9-beta"}} {
		if Verify(keys, c.dev, c.v, sum[:], sig) {
			t.Errorf("the signature passed for %s %s", c.dev, c.v)
		}
	}
	if Verify(keys, "kobo", "v0.4.0-beta", other2[:], sig) || Verify(keys[:1], "kobo", "v0.4.0-beta", sum[:], sig) || Verify(keys, "kobo", "v0.4.0-beta", sum[:], []byte("bad")) {
		t.Error("a bad signature passed")
	}
}

// a Kobo package as package.sh makes it, with these files under the reader's folder
func makePackage(t *testing.T, files map[string]string) []byte {
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	w := tar.NewWriter(z)
	for _, d := range []string{"./mnt/", "./mnt/onboard/", "./mnt/onboard/.adds/", "./mnt/onboard/.adds/shamsaink/", "./mnt/onboard/.adds/shamsaink/books/"} {
		w.WriteHeader(&tar.Header{Name: d, Typeflag: tar.TypeDir, Mode: 0o755})
	}
	for name, body := range files {
		mode := int64(0o644)
		if name == "shamsaink" || name == "launch.sh" {
			mode = 0o755
		}
		if err := w.WriteHeader(&tar.Header{Name: "./" + koboRoot + name, Typeflag: tar.TypeReg, Mode: mode, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	w.Close()
	z.Close()
	return b.Bytes()
}

func TestDownload(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	keys := []ed25519.PublicKey{pub}
	pkg := makePackage(t, map[string]string{"shamsaink": "new", "launch.sh": "#!/bin/sh", "VERSION": "v0.4.0-beta\n"})
	sum := sha256.Sum256(pkg)
	sig := Sign(priv, "kobo", "v0.4.0-beta", sum[:])
	cut := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/p.tgz":
			if cut {
				w.Header().Set("Content-Length", "999999")
				w.Write(pkg[:100])
				return
			}
			w.Write(pkg)
		case "/p.tgz.sig":
			w.Write(sig)
		case "/latest.json":
			w.Write([]byte(`{"version":"v0.4.0-beta","prerelease":true,"packages":{"kobo":{"url":"` + "http://" + r.Host + `/p.tgz","size":` + strconv.Itoa(len(pkg)) + `,"sig":"http://` + r.Host + `/p.tgz.sig"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	l, err := Check(ctx, srv.URL+"/latest.json")
	if err != nil || l.Version != "v0.4.0-beta" || l.Packages["kobo"].Size != int64(len(pkg)) {
		t.Fatalf("Check: %+v, %v", l, err)
	}
	if _, err := Check(ctx, srv.URL+"/nothing.json"); !errors.Is(err, ErrUnreachable) {
		t.Errorf("a missing latest.json: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "package.tgz")
	p := l.Packages["kobo"]
	var last int64
	if err := Download(ctx, p, "kobo", l.Version, keys, dst, func(done, total int64) { last = done }); err != nil {
		t.Fatal(err)
	}
	if last != int64(len(pkg)) {
		t.Errorf("progress ended at %d of %d", last, len(pkg))
	}
	os.Remove(dst)
	// the same package announced as another version: refused
	if err := Download(ctx, p, "kobo", "v0.5.0-beta", keys, dst, nil); !errors.Is(err, ErrUnverified) {
		t.Errorf("another version: %v", err)
	}
	// no signature: refused before downloading
	ns := p
	ns.Sig = ""
	if err := Download(ctx, ns, "kobo", l.Version, keys, dst, nil); !errors.Is(err, ErrUnverified) {
		t.Errorf("no signature: %v", err)
	}
	// a size that is not the package's
	big := p
	big.Size--
	if err := Download(ctx, big, "kobo", l.Version, keys, dst, nil); !errors.Is(err, ErrUnverified) {
		t.Errorf("larger than announced: %v", err)
	}
	cut = true
	if err := Download(ctx, p, "kobo", l.Version, keys, dst, nil); !errors.Is(err, ErrCut) {
		t.Errorf("cut: %v", err)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Error("a failed download left its file")
	}
	cut = false
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := Download(cctx, p, "kobo", l.Version, keys, dst, nil); !errors.Is(err, ErrCancelled) {
		t.Errorf("cancelled: %v", err)
	}
}

func write(t *testing.T, p, s string) {
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "<none>"
	}
	return string(b)
}

func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shamsaink")
	launcher := filepath.Join(t.TempDir(), "libshamsaink-launcher.so")
	// the version in place, with the user's own files beside it
	for f, s := range map[string]string{"shamsaink": "old", "launch.sh": "old launch", "VERSION": "v0.3.6-beta\n",
		"fonts/a.ttf": "a1", "fonts/gone.ttf": "gone", "licenses/LICENSE": "l1", "settings.json": "{mine}",
		"books/b.epub": "book", "cache/splash-x.png": "x", "cache/book.cache": "c"} {
		write(t, filepath.Join(dir, f), s)
	}
	write(t, launcher, "old so")
	var lb bytes.Buffer
	{
		z := gzip.NewWriter(&lb)
		w := tar.NewWriter(z)
		w.WriteHeader(&tar.Header{Name: "imageformats/libshamsaink-launcher.so", Typeflag: tar.TypeReg, Mode: 0o755, Size: 6})
		w.Write([]byte("new so"))
		w.Close()
		z.Close()
	}
	pkg := makePackage(t, map[string]string{"shamsaink": "new", "launch.sh": "new launch", "VERSION": "v0.4.0-beta\n",
		"fonts/a.ttf": "a2", "fonts/b.ttf": "b", "licenses/LICENSE": "l2", "launcher/Kobo.tgz": lb.String()})
	p := filepath.Join(dir, "update", "package.tgz")
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	write(t, p, string(pkg))
	if err := Install(p, dir, "v0.4.0-beta", launcher); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]string{"shamsaink": "new", "VERSION": "v0.4.0-beta\n", "fonts/a.ttf": "a2",
		"fonts/b.ttf": "b", "fonts/gone.ttf": "<none>", "settings.json": "{mine}", "books/b.epub": "book",
		"cache/book.cache": "c", "cache/splash-x.png": "<none>", "update/pending": "v0.4.0-beta\n",
		"update/added": "fonts/b.ttf\nlauncher/Kobo.tgz\n", "update/package.tgz": "<none>",
		"previous/shamsaink": "old", "previous/launch.sh": "old launch", "previous/fonts/a.ttf": "a1",
		"previous/fonts/gone.ttf": "gone", "previous/launcher.so": "old so", "previous/settings.json": "<none>"} {
		if got := read(t, filepath.Join(dir, f)); got != want {
			t.Errorf("%s: %q, want %q", f, got, want)
		}
	}
	if got := read(t, launcher); got != "new so" {
		t.Errorf("launcher: %q", got)
	}
	if st, _ := os.Stat(filepath.Join(dir, "shamsaink")); st.Mode()&0o111 == 0 {
		t.Error("the reader is not executable")
	}
	if v := Reached(dir); v != "v0.4.0-beta" || read(t, filepath.Join(dir, "update/pending")) != "<none>" {
		t.Errorf("Reached: %q", v)
	}
	if v := Reached(dir); v != "" {
		t.Errorf("Reached twice: %q", v)
	}
}

func TestInstallRefuses(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "shamsaink"), "old")
	p := filepath.Join(dir, "p.tgz")
	// no launch.sh
	write(t, p, string(makePackage(t, map[string]string{"shamsaink": "new", "VERSION": "v1.0.0\n"})))
	if err := Install(p, dir, "v1.0.0", ""); !errors.Is(err, ErrUnverified) {
		t.Errorf("a package without launch.sh: %v", err)
	}
	// a path out of the reader's folder
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	w := tar.NewWriter(z)
	w.WriteHeader(&tar.Header{Name: "./" + koboRoot + "../../evil", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1})
	w.Write([]byte("x"))
	w.Close()
	z.Close()
	write(t, p, b.String())
	if err := Install(p, dir, "v1.0.0", ""); !errors.Is(err, ErrUnverified) {
		t.Errorf("a path leaving the folder: %v", err)
	}
	if read(t, filepath.Join(dir, "shamsaink")) != "old" || read(t, filepath.Join(dir, "update/pending")) != "<none>" {
		t.Error("a refused package changed something")
	}
}

// the release key is one: a typo in keys.go would refuse every update
func TestReleaseKeys(t *testing.T) {
	if n := len(ParseKeys(ReleaseKeys)); n != 1 {
		t.Errorf("%d release keys read from keys.go", n)
	}
}
