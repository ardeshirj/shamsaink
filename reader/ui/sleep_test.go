//go:build blitz

package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	blitz "shamsainkengine"
)

// SLEEP: a folder to write the sleep screens into (skipped without it); FONTS as in TestPanels.
func TestSleepScreens(t *testing.T) {
	dir := os.Getenv("SLEEP")
	if dir == "" {
		t.Skip("SLEEP not set")
	}
	os.MkdirAll(dir, 0o755)
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	cases := map[string]string{
		"couplet-hafez": "verses:\n  - اگر آن تُرکِ شیرازی به‌‌ دست‌ آرَد دلِ ما را\n  - به خال هِندویَش بَخشَم سَمَرقند و بُخارا را\npoet: حافظ\ngloss: \"شاعر می‌گوید اگر آن معشوقِ شیرازی دلم را به دست آورد، سمرقند و بخارا، دو شهر بزرگ و ثروتمند، را در برابر خال سیاه او می‌بخشم. ترک: معشوق زیباروی. خال هندو: خال سیاه.\"\n",
		"couplet-long":  "verses:\n  - ای نام تو بهترین سر آغاز بی نام تو نامه کی کنم باز\n  - ای کارگشای هر چه هستند وی دست نگهدار هر چه هستند\npoet: نظامی\ngloss: شاعر خدا را می‌ستاید.\n",
		"quote-fa":      "layout: quote\ntext: \"دوش دیدم که ملایک در میخانه زدند\\nگل آدم بسرشتند و به پیمانه زدند\"\ncredit: حافظ\n",
		"quote-en":      "layout: quote\ntext: \"A reader lives a thousand lives before he dies. The man who never reads lives only one.\"\ncredit: George R. R. Martin\n",
		"title-fa":      "layout: title\ntext: رود کم‌آب\ncredit: مارا الیس\n",
		"title-en":      "layout: title\ntext: Buried Alive\ncredit: Arnold Bennett\n",
		"long-en":       "layout: quote\ntext: \"" + longText + "\"\ncredit: Somebody\n",
	}
	for name, y := range cases {
		s, err := ParseSleep([]byte(y))
		if err != nil {
			t.Fatal(err)
		}
		for _, sz := range [][2]int{{1264, 1680}, {1236, 1648}} {
			img := DrawSleep(s, fonts, sz[0], sz[1])
			f, _ := os.Create(filepath.Join(dir, name+"-"+itoa(sz[0])+".png"))
			png.Encode(f, img)
			f.Close()
		}
	}
}

func itoa(n int) string {
	return string([]byte{byte('0' + n/1000), byte('0' + n/100%10), byte('0' + n/10%10), byte('0' + n%10)})
}

const longText = "It was the best of times, it was the worst of times, it was the age of wisdom, it was the age of foolishness, it was the epoch of belief, it was the epoch of incredulity, it was the season of Light, it was the season of Darkness, it was the spring of hope, it was the winter of despair."

// SLEEP_YML: a folder of sleep screen files (one screen or a set), each screen drawn at both sizes into SLEEP (skipped without both).
func TestSleepFolder(t *testing.T) {
	in, out := os.Getenv("SLEEP_YML"), os.Getenv("SLEEP")
	if in == "" || out == "" {
		t.Skip("SLEEP_YML and SLEEP not set")
	}
	os.MkdirAll(out, 0o755)
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	names, _ := filepath.Glob(filepath.Join(in, "*.yml"))
	for _, n := range names {
		b, _ := os.ReadFile(n)
		screens, errs := ParseSleeps(b)
		for _, err := range errs {
			t.Errorf("%s: %v", n, err)
		}
		for i, s := range screens {
			name := strings.TrimSuffix(filepath.Base(n), ".yml")
			if len(screens) > 1 {
				name += "-" + itoa(i+1)
			}
			img := DrawSleep(s, fonts, 1264, 1680)
			f, _ := os.Create(filepath.Join(out, name+".png"))
			png.Encode(f, img)
			f.Close()
		}
	}
}

// A couplet is four lines, each verse on two, however long: it shrinks, and the type stays readable.
func TestCoupletFits(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	for _, y := range []string{
		"verses: [\"اگر آن ترک شیرازی به دست آرد دل ما را\", \"به خال هندویش بخشم سمرقند و بخارا را\"]\npoet: حافظ\n",
		"verses: [\"ای نام تو بهترین سر آغاز بی نام تو نامه کی کنم باز\", \"ای کارگشای هر چه هستند وی دست نگهدار هر چه هستند\"]\npoet: نظامی\n",
		"verses: [\"کوتاه\", \"کوتاه\"]\n",
	} {
		s, err := ParseSleep([]byte(y))
		if err != nil {
			t.Fatal(err)
		}
		if _, tall := drawSleepFit(s, fonts, 1264, 1680); tall {
			t.Errorf("%q: a verse is on a third line at the smallest size", s.Verses[0])
		}
	}
}

// A file with a field this version does not know still loads (a later version's: cover), and its layout is found: a
// couplet with verses (even with an image), a picture with an image alone, else a title.
func TestSleepFileFields(t *testing.T) {
	s, err := ParseSleep([]byte("verses: [a, b]\nimage: x.png\ncover: true\nfuture: 1\n"))
	if err != nil || s.Layout != "couplet" {
		t.Errorf("couplet: %v, layout %q", err, s.Layout)
	}
	if s, err = ParseSleep([]byte("image: x.png\n")); err != nil || s.Layout != "picture" || s.Image != "x.png" {
		t.Errorf("picture: %v, layout %q, image %q", err, s.Layout, s.Image)
	}
	if s, err = ParseSleep([]byte("text: Title\n")); err != nil || s.Layout != "title" {
		t.Errorf("title: %v, layout %q", err, s.Layout)
	}
	if _, err = ParseSleep([]byte("verses: [unclosed\n")); err == nil {
		t.Error("a broken file loads")
	}
}

// A file holds one screen or a set, separated by "---" lines; a broken screen is left out, the others still load.
func TestSleepSets(t *testing.T) {
	for _, c := range []struct {
		yml       string
		n, errs   int
		firstPoet string
	}{
		{"verses: [a, b]\npoet: one\n", 1, 0, "one"},
		{"---\nverses: [a, b]\npoet: one\n---\nlayout: quote\ntext: q\n---   # a comment\ntext: t\n", 3, 0, "one"},
		{"verses: [a, b]\npoet: one\n---\nverses: [unclosed\n---\ntext: t\n", 2, 1, "one"},
		{"# only comments\n---\n\n---\nverses: [a, b]\npoet: two\n---\n", 1, 0, "two"},
		{"verses: [a, b]\npoet: one\r\n---\r\ntext: t\r\n", 2, 0, "one"},
		{"text: \"a --- b\"\n", 1, 0, ""},
	} {
		screens, errs := ParseSleeps([]byte(c.yml))
		if len(screens) != c.n || len(errs) != c.errs {
			t.Errorf("%q: %d screens, %d errors (%v); want %d, %d", c.yml, len(screens), len(errs), errs, c.n, c.errs)
			continue
		}
		if screens[0].Poet != c.firstPoet {
			t.Errorf("%q: the first screen's poet %q, want %q", c.yml, screens[0].Poet, c.firstPoet)
		}
	}
	if _, errs := ParseSleeps([]byte("text: a\n---\nverses: [x\n")); len(errs) != 1 || !strings.Contains(errs[0].Error(), "line 3") {
		t.Errorf("a broken screen's error does not say where it starts: %v", errs)
	}
}
