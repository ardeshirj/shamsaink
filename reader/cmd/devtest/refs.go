package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
)

// The references as hashes (docs/testing.md, section 4): e2e/refs.sum, committed, has a line for each snapshot,
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
	b.WriteString("# The e2e snapshots' references as SHA-256 (docs/testing.md, section 4; e2e/README.md): kept by SAVE=1, the\n")
	b.WriteString("# changed pages looked at before this file is committed. TARGET/SCENARIO/NAME; the clock's strip left out.\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s  %s\n", refs[k], k)
	}
	return os.WriteFile(refsSum, []byte(b.String()), 0o644)
}
