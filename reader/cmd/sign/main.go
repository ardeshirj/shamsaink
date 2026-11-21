// Command sign signs a release's packages for updates over Wi-Fi (reader/update): each PACKAGE gets PACKAGE.sig, the
// ed25519 signature of its device, version and SHA-256 (update.Message), made with the private key in
// $SHAMSAINK_SIGNING_KEY (base64, its 32-byte seed; on CI from GitHub's secret of that name). The device is read from
// the package's name (shamsaink-VERSION-DEVICE.tgz).
//
//	sign -version v0.4.0-beta out/release/shamsaink-v0.4.0-beta-kobo.tgz ...
//
// -keygen FILE makes a key pair: the private key written to FILE (base64, readable by its owner only, refused if FILE
// exists), the public key printed, the only line written. reader/scripts/signing-key.sh makes the release key with it,
// reader/scripts/update-serve.sh a test key.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"shamsainkreader/update"
)

func main() {
	version := flag.String("version", "", "the release's version, as its tag (v0.4.0-beta)")
	keygen := flag.String("keygen", "", "make a key pair: the private key into this file, the public key printed")
	flag.Parse()
	if *keygen != "" {
		pub, priv, err := ed25519.GenerateKey(nil)
		check(err)
		f, err := os.OpenFile(*keygen, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		check(err)
		_, err = fmt.Fprintln(f, base64.StdEncoding.EncodeToString(priv.Seed()))
		check(err)
		check(f.Close())
		fmt.Println(base64.StdEncoding.EncodeToString(pub))
		return
	}
	if *version == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: sign -version VERSION PACKAGE... (key in $SHAMSAINK_SIGNING_KEY) | sign -keygen FILE")
		os.Exit(2)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("SHAMSAINK_SIGNING_KEY")))
	if err != nil || len(seed) != ed25519.SeedSize {
		fmt.Fprintln(os.Stderr, "sign: $SHAMSAINK_SIGNING_KEY is not a key (base64 of a 32-byte ed25519 seed)")
		os.Exit(1)
	}
	key := ed25519.NewKeyFromSeed(seed)
	pub := key.Public().(ed25519.PublicKey)
	for _, p := range flag.Args() {
		name := strings.TrimSuffix(filepath.Base(p), ".tgz")
		dev := name[strings.LastIndex(name, "-")+1:]
		if !strings.HasPrefix(name, "shamsaink-"+*version+"-") || (dev != "kobo" && dev != "kindle") {
			fmt.Fprintf(os.Stderr, "sign: %s is not a %s package (shamsaink-%s-DEVICE.tgz)\n", p, *version, *version)
			os.Exit(1)
		}
		f, err := os.Open(p)
		check(err)
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		check(err)
		sig := update.Sign(key, dev, *version, h.Sum(nil))
		if !update.Verify([]ed25519.PublicKey{pub}, dev, *version, h.Sum(nil), sig) {
			fmt.Fprintln(os.Stderr, "sign: the signature does not verify")
			os.Exit(1)
		}
		check(os.WriteFile(p+".sig", sig, 0o644))
		fmt.Printf("%s.sig (%s %s, key %s)\n", p, dev, *version, base64.StdEncoding.EncodeToString(pub))
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "sign:", err)
		os.Exit(1)
	}
}
