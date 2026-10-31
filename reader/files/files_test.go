package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "f.txt")
	for _, s := range []string{"one", "two"} {
		if err := Write(p, []byte(s)); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(p); string(b) != s {
			t.Errorf("got %q; want %q", b, s)
		}
	}
	if _, err := os.Stat(p + ".new"); err == nil {
		t.Error(".new left behind")
	}
}
