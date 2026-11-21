// Package update is updates over Wi-Fi (roadmap step 2, docs/DESIGN.md "Updates over Wi-Fi"): the newest release read
// from FarsiEPUB's latest.json, its package downloaded and verified against the ed25519 keys the reader carries, then
// installed by the same whole-file moves as reader/kobo/install.sh, the old files kept in previous/ (install.go). The
// launcher (reader/kobo/launch.sh) starts the new version and puts previous/ back if it does not reach the library.
// Nothing here turns Wi-Fi on: the caller asks first (never in the background).
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	// Go's own copy of the usual CA roots, used when the device has none it can find (the Kobo's are not where Go
	// looks): HTTPS to FarsiEPUB and GitHub without depending on the firmware
	_ "golang.org/x/crypto/x509roots/fallback"
)

// RestartCode is the reader's exit code once an update is installed: the launcher starts the new version (launch.sh).
const RestartCode = 85

// The URL of latest.json (FarsiEPUB's endpoint, from GitHub's releases of this repository); in dev mode, dev/update-url
// beside the reader can point elsewhere (the Mac's, make update-serve).
const LatestURL = "https://farsiepub.org/shamsaink/latest.json"

// Latest is latest.json: the newest release, pre-releases included.
type Latest struct {
	Version    string             `json:"version"`
	Prerelease bool               `json:"prerelease"`
	Published  string             `json:"published"`
	Notes      string             `json:"notes"`
	Packages   map[string]Package `json:"packages"` // by device: "kobo", "kindle"
}

// Package is one device's package in latest.json: its URL and size, and the URLs of its checksum and signature (each
// omitted when the release has none).
type Package struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	Sig    string `json:"sig,omitempty"`
}

// What went wrong, each its own notice (notice-update-*).
var (
	ErrUnreachable = errors.New("the update server was not reached")   // update-unreachable
	ErrCut         = errors.New("the download stopped before its end") // update-cut
	ErrUnverified  = errors.New("the download could not be verified")  // update-unverified
	ErrCancelled   = errors.New("the download was cancelled")          // no notice: back to Settings
	ErrNotFor      = errors.New("the release has no package for this") // update-none, as nothing newer for it
)

// Client is the HTTP client for every request: a request that has not answered in 20 s fails (the download's body
// itself is bounded by its context only).
var Client = &http.Client{Transport: &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: 20 * time.Second}).DialContext,
	TLSHandshakeTimeout:   20 * time.Second,
	ResponseHeaderTimeout: 20 * time.Second,
	IdleConnTimeout:       30 * time.Second,
}}

// Check reads latest.json at url. Every failure is ErrUnreachable (wrapped, with its cause for the log).
func Check(ctx context.Context, url string) (Latest, error) {
	var l Latest
	b, err := get(ctx, url, 1<<16)
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(b, &l); err != nil || l.Version == "" {
		return l, fmt.Errorf("%w: latest.json: %v", ErrUnreachable, err)
	}
	return l, nil
}

// get reads a small file (at most max bytes).
func get(ctx context.Context, url string, max int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	res, err := Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%w: %s: %s", ErrUnreachable, url, res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, max))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	return b, nil
}

// Newer tells whether version a is newer than b (vX.Y.Z with an optional suffix, -beta, -beta.2: semantic versioning's
// order, a suffix before the release without one). A version that is not one (a build named "dev") is older than
// every version.
func Newer(a, b string) bool { return compare(a, b) > 0 }

func compare(a, b string) int {
	ac, ap, aok := parse(a)
	bc, bp, bok := parse(b)
	switch {
	case !aok && !bok:
		return 0
	case !aok:
		return -1
	case !bok:
		return 1
	}
	for i := range ac {
		if ac[i] != bc[i] {
			return cmpInt(ac[i], bc[i])
		}
	}
	switch {
	case ap == bp:
		return 0
	case ap == "":
		return 1
	case bp == "":
		return -1
	}
	as, bs := strings.Split(ap, "."), strings.Split(bp, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			return cmpInt(an, bn)
		case aerr == nil: // numbers before words
			return -1
		case berr == nil:
			return 1
		}
		return strings.Compare(as[i], bs[i])
	}
	return cmpInt(len(as), len(bs))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// parse splits v0.3.6-beta.2 into 0, 3, 6 and "beta.2"; ok false when it is not a version.
func parse(v string) (core [3]int, pre string, ok bool) {
	v, _ = strings.CutPrefix(v, "v")
	v, pre, _ = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return core, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return core, "", false
		}
		core[i] = n
	}
	return core, pre, true
}

// Message is what a package's signature signs: the device, the version and the package's SHA-256, one line. The
// version and device in it keep an old or another device's package, signed too, from passing for the one announced.
func Message(device, version string, sum []byte) []byte {
	return fmt.Appendf(nil, "shamsaink %s %s sha256:%x\n", device, version, sum)
}

// Sign is a package's .sig: the signature of Message, base64, one line (cmd/sign, on CI).
func Sign(key ed25519.PrivateKey, device, version string, sum []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, Message(device, version, sum))) + "\n")
}

// Verify tells whether sig (a .sig's content) is a signature of Message by one of keys.
func Verify(keys []ed25519.PublicKey, device, version string, sum, sig []byte) bool {
	s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil || len(s) != ed25519.SignatureSize {
		return false
	}
	m := Message(device, version, sum)
	for _, k := range keys {
		if ed25519.Verify(k, m, s) {
			return true
		}
	}
	return false
}

// ParseKeys reads public keys, base64, one a line ('#' starts a comment); lines that are not one are skipped.
func ParseKeys(text string) []ed25519.PublicKey {
	var keys []ed25519.PublicKey
	for _, line := range strings.Split(text, "\n") {
		line, _, _ = strings.Cut(line, "#")
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(line))
		if err == nil && len(b) == ed25519.PublicKeySize {
			keys = append(keys, ed25519.PublicKey(b))
		}
	}
	return keys
}

// Download fetches p, the package of version for device, into dst and verifies it: its size as announced, its
// signature by one of keys. progress (may be nil) is called as it comes, with the bytes so far and the whole. A package
// without a signature is refused before anything is downloaded. On any failure dst is not there.
func Download(ctx context.Context, p Package, device, version string, keys []ed25519.PublicKey, dst string, progress func(done, total int64)) error {
	if p.Sig == "" || len(keys) == 0 {
		return fmt.Errorf("%w: no signature (or no key to check it with)", ErrUnverified)
	}
	if p.URL == "" || p.Size <= 0 {
		return fmt.Errorf("%w: no package URL or size", ErrUnreachable)
	}
	sig, err := get(ctx, p.Sig, 1024)
	if err != nil {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	res, err := Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("%w: %s: %s", ErrUnreachable, p.URL, res.Status)
	}
	part := dst + ".part"
	f, err := os.Create(part)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(part)
		}
	}()
	h := sha256.New()
	w := io.MultiWriter(f, h)
	var done int64
	buf := make([]byte, 64<<10)
	body := io.LimitReader(res.Body, p.Size+1) // one byte more than announced is enough to refuse it
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return err // the storage full: not the network's
			}
			done += int64(n)
			if progress != nil {
				progress(min(done, p.Size), p.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return ErrCancelled
			}
			return fmt.Errorf("%w: %v after %d of %d bytes", ErrCut, rerr, done, p.Size)
		}
	}
	switch {
	case done < p.Size:
		return fmt.Errorf("%w: %d of %d bytes", ErrCut, done, p.Size)
	case done > p.Size:
		return fmt.Errorf("%w: larger than the %d bytes announced", ErrUnverified, p.Size)
	}
	if !Verify(keys, device, version, h.Sum(nil), sig) {
		return fmt.Errorf("%w: the signature does not match (%s %s)", ErrUnverified, device, version)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(part, dst); err != nil {
		return err
	}
	ok = true
	return nil
}
