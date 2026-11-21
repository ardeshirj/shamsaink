package library

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A book's key does not depend on its name or modification time, and changes when its end (a ZIP's central directory)
// or its size does.
func TestHash(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	key := func(p string) string {
		k, err := Hash(p)
		if err != nil || len(k) != 12 {
			t.Fatalf("Hash(%s) = %q, %v", p, k, err)
		}
		return k
	}
	for _, size := range []int{1000, 3 * hashSample} { // whole file, then head and tail
		body := bytes.Repeat([]byte("abcdefgh"), size/8)
		a := write("a.epub", body)
		want := key(a)
		// the modification time, as the Kobo's FAT gave it 8 hours apart
		old := time.Now().Add(-8 * time.Hour)
		os.Chtimes(a, old, old)
		if got := key(a); got != want {
			t.Errorf("size %d: key changed with the modification time: %s, %s", size, want, got)
		}
		if got := key(write("renamed.epub", body)); got != want {
			t.Errorf("size %d: key changed with the name: %s, %s", size, want, got)
		}
		tail := append([]byte(nil), body...)
		tail[len(tail)-1] ^= 1
		if got := key(write("tail.epub", tail)); got == want {
			t.Errorf("size %d: a changed last byte did not change the key", size)
		}
		if got := key(write("longer.epub", append(body, 'x'))); got == want {
			t.Errorf("size %d: a changed size did not change the key", size)
		}
	}
	if _, err := Hash(filepath.Join(dir, "missing.epub")); err == nil {
		t.Error("a missing file has a key")
	}
}
