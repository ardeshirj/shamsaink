package update

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
)

// ReleaseKeys are the public keys a package's signature is checked against, base64, one a line: the release key made by
// reader/scripts/signing-key.sh (its private half only in GitHub's secret and the maintainer's backup). More than one
// may be here, so a new key can be added a release before the old one goes: a new key's public half is added here in a
// release still signed with the old one, and only then is GitHub's secret switched to it.
const ReleaseKeys = `
emaYhAs+vqO92MHvhht/wi0+0CFioFjLRyz3+B5Tw88= # 2026-11-20, make signing-key
`

// Keys are the keys a download is checked against: ReleaseKeys, and in dev mode also dev/update-key beside the reader
// (a test key's public half, make update-serve; dev/ is never in a release).
func Keys(dir string, devMode bool) []ed25519.PublicKey {
	keys := ParseKeys(ReleaseKeys)
	if devMode {
		if b, err := os.ReadFile(filepath.Join(dir, "dev", "update-key")); err == nil {
			keys = append(keys, ParseKeys(string(b))...)
		}
	}
	return keys
}

// URL is where latest.json is read: LatestURL, or in dev mode dev/update-url beside the reader when there is one.
func URL(dir string, devMode bool) string {
	if devMode {
		if b, err := os.ReadFile(filepath.Join(dir, "dev", "update-url")); err == nil {
			if u, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n"); strings.TrimSpace(u) != "" {
				return strings.TrimSpace(u)
			}
		}
	}
	return LatestURL
}
