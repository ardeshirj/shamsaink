package device

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every screen counts once in the pick, whether alone in its file or one of a set: a file of 3 and a file of 1 give 4
// screens, each shown about a quarter of the time.
func TestSleepScreenSets(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sleep"), 0o755)
	os.WriteFile(filepath.Join(dir, "sleep", "set.yml"), []byte("a\n---\nb\n---\nc\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "sleep", "one.YAML"), []byte("d\n"), 0o644)
	// macOS's metadata beside a file copied from a Mac, and a file hidden on purpose: never read (2026-11-08: ._ logged
	// a YAML error on the Kobo); a screen from either would show as "x" or "y"
	os.WriteFile(filepath.Join(dir, "sleep", "._set.yml"), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "sleep", ".hidden.yml"), []byte("y\n"), 0o644)
	text := SleepText{
		Count: func(yml []byte) (int, []error) { return len(strings.Split(string(yml), "---")), nil },
		Draw: func(yml []byte, i int) (*image.Gray, error) {
			img := image.NewGray(image.Rect(0, 0, 1, 1))
			img.SetGray(0, 0, color.Gray{strings.TrimSpace(strings.Split(string(yml), "---")[i])[0]})
			return img, nil
		},
	}
	fallback := image.NewGray(image.Rect(0, 0, 1, 1))
	seen := map[byte]int{}
	for range 4000 {
		seen[sleepScreen(dir, fallback, text).(*image.Gray).GrayAt(0, 0).Y]++
	}
	for _, c := range "abcd" {
		if n := seen[byte(c)]; n < 800 || n > 1200 {
			t.Errorf("screen %c shown %d times in 4000, want about 1000", c, n)
		}
	}
	if seen[0] > 0 {
		t.Errorf("the fallback shown %d times", seen[0])
	}
	if seen['x'] > 0 || seen['y'] > 0 {
		t.Errorf("a hidden file's screen shown: ._set.yml %d times, .hidden.yml %d", seen['x'], seen['y'])
	}
}

// A picture is shown when a YAML screen names it (layout picture), never alone in the folder. In colour it is drawn in
// colour (RGBA), so dark mode leaves it as it is (Screen.ShowRGBA) and a colour screen shows its colours; a transparent
// picture lies on white. A grey picture is drawn grey, so it follows dark mode like the page.
func TestSleepPicture(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sleep"), 0o755)
	save := func(name string, img image.Image) {
		f, _ := os.Create(filepath.Join(dir, "sleep", name))
		png.Encode(f, img)
		f.Close()
	}
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10)) // red on the left half, transparent on the right
	for y := range 10 {
		for x := range 10 {
			src.Set(x, y, color.NRGBA{200, 30, 30, 255})
		}
	}
	save("p.png", src)
	save("g.png", image.NewGray(image.Rect(0, 0, 20, 10)))
	os.MkdirAll(filepath.Join(dir, "sleep", "pictures"), 0o755)
	save("pictures/q.png", src)
	fallback := image.NewGray(image.Rect(0, 0, 200, 300))
	names := func(pic string) SleepText {
		return SleepText{
			Count:   func([]byte) (int, []error) { return 1, nil },
			Draw:    func([]byte, int) (*image.Gray, error) { return nil, nil },
			Picture: func([]byte, int) string { return pic },
		}
	}
	// alone in the folder: not shown
	if sleepScreen(dir, fallback, names("p.png")) != image.Image(fallback) {
		t.Error("a picture with no YAML file is shown")
	}
	os.WriteFile(filepath.Join(dir, "sleep", "s.yml"), []byte("image: p.png\n"), 0o644)
	img, ok := sleepScreen(dir, fallback, names("p.png")).(*image.RGBA)
	if !ok {
		t.Fatal("a picture in colour is not drawn in colour")
	}
	if img.Rect != fallback.Rect {
		t.Errorf("drawn at %v, the screen is %v", img.Rect, fallback.Rect)
	}
	// fitted to the width (200 × 100, centred: rows 100 to 200), red on the left, white on the right and above
	if c := img.RGBAAt(50, 150); c.R < 180 || c.G > 60 {
		t.Errorf("the red half is %v", c)
	}
	for _, p := range []image.Point{{150, 150}, {100, 20}} {
		if c := img.RGBAAt(p.X, p.Y); c != (color.RGBA{255, 255, 255, 255}) {
			t.Errorf("at %v %v, want white", p, c)
		}
	}
	if _, ok := sleepScreen(dir, fallback, names("g.png")).(*image.Gray); !ok {
		t.Error("a grey picture is not drawn grey")
	}
	if _, ok := sleepScreen(dir, fallback, names("pictures/q.png")).(*image.RGBA); !ok {
		t.Error("a picture in a folder beside the file is not shown")
	}
	// a picture that is not there, or a path out of the file's folder (even to a picture that is there): the fallback
	for _, pic := range []string{"missing.png", "../sleep/p.png", "/p.png"} {
		if sleepScreen(dir, fallback, names(pic)) != image.Image(fallback) {
			t.Errorf("%s: not the fallback", pic)
		}
	}
}
