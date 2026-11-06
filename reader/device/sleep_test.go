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
}

// A picture is drawn in colour (RGBA), so dark mode leaves it as it is (Screen.ShowRGBA) and a colour screen shows its
// colours; a transparent picture lies on white. The YAML screens stay grey, inverted in dark mode like the page.
func TestSleepPictureColour(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sleep"), 0o755)
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10)) // red on the left half, transparent on the right
	for y := range 10 {
		for x := range 10 {
			src.Set(x, y, color.NRGBA{200, 30, 30, 255})
		}
	}
	f, _ := os.Create(filepath.Join(dir, "sleep", "p.png"))
	png.Encode(f, src)
	f.Close()
	fallback := image.NewGray(image.Rect(0, 0, 200, 300))
	img, ok := sleepScreen(dir, fallback, SleepText{}).(*image.RGBA)
	if !ok {
		t.Fatal("a picture is not drawn in colour")
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
}
