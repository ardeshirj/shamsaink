package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The references as hashes (docs/TESTING.md, section 4): e2e/refs.sum, committed, has a line for each snapshot,
// "SHA256  TARGET/SCENARIO/NAME" (TARGET: sim or kobo, two families of pages). The pages themselves are drawn from
// books and stay out of git (e2e/out/); a local copy of a reference only draws the difference when a snapshot fails.
const refsSum = "e2e/refs.sum"

// pageHash is a snapshot's SHA-256 as refs.sum keeps it: its size and its pixels below the clock's strip, the part a
// comparison looks at.
func pageHash(w, h int, pix []byte) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "%d %d\n", w, h)
	sum.Write(pix[min(topRows*w, len(pix)):])
	return fmt.Sprintf("%x", sum.Sum(nil))
}

// readRefs reads refs.sum: each key's hash. A missing file is an empty list.
func readRefs() (map[string]string, error) {
	data, err := os.ReadFile(refsSum)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	refs := map[string]string{}
	for i, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.Fields(l)
		if len(f) != 2 || len(f[0]) != 64 {
			return nil, fmt.Errorf("%s:%d: not \"SHA256  TARGET/SCENARIO/NAME\"", refsSum, i+1)
		}
		refs[f[1]] = f[0]
	}
	return refs, nil
}

// keepRef sets key's hash in refs.sum, the other lines as they were, sorted by key.
func keepRef(key, hash string) error {
	refs, err := readRefs()
	if err != nil {
		return err
	}
	refs[key] = hash
	keys := make([]string, 0, len(refs))
	for k := range refs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# The e2e snapshots' references as SHA-256 (docs/TESTING.md, section 4; e2e/README.md): kept by SAVE=1, the\n")
	b.WriteString("# changed pages looked at before this file is committed. TARGET/SCENARIO/NAME; the clock's strip left out.\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s  %s\n", refs[k], k)
	}
	return os.WriteFile(refsSum, []byte(b.String()), 0o644)
}

// refImages holds the references of scenarios on our own books (refs: images) as pictures, committed beside their hashes
// so a changed page shows in a diff and CI's failure folder has the old page: e2e/refs/LANG/HASH.png, a folder for the
// book's language (the scenario's refs: images LANG), named by the hash refs.sum gives the snapshot. Only the
// simulator's (what CI runs): a device whose pages are the same (the Kobo's) shares its files by their hash; a device
// with another screen (the Kindle) is held by its hashes, its difference drawn from the local copy where it was kept.
const refImages = "e2e/refs"

func refImage(lang, hash string) string { return filepath.Join(refImages, lang, hash+".png") }

// pruneRefImages removes the pictures no simulator's line of refs.sum names any more (a reference kept anew).
func pruneRefImages() error {
	refs, err := readRefs()
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for k, h := range refs {
		if strings.HasPrefix(k, "sim/") {
			used[h] = true
		}
	}
	files, _ := filepath.Glob(filepath.Join(refImages, "*", "*.png"))
	for _, f := range files {
		if !used[strings.TrimSuffix(filepath.Base(f), ".png")] {
			if err := os.Remove(f); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeRefImage keeps a snapshot as a grey PNG.
func writeRefImage(path string, w, h int, pix []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	img := &image.Gray{Pix: pix, Stride: w, Rect: image.Rect(0, 0, w, h)}
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// readRefImage reads a kept picture back as the P5 file snap compares with; a picture of another size is an error.
func readRefImage(path string, w, h int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	g, ok := img.(*image.Gray)
	if !ok || g.Rect.Dx() != w || g.Rect.Dy() != h || g.Stride != w {
		return nil, fmt.Errorf("%s: not a %dx%d grey picture", path, w, h)
	}
	return pgm(w, h, g.Pix), nil
}
