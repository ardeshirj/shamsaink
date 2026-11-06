package ui

import (
	"fmt"
	"html/template"
	"image"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	blitz "shamsainkengine"

	"gopkg.in/yaml.v3"
)

// A sleep screen is a small file (YAML), drawn from templates/sleep.html at the screen's own size, so the type is the same
// everywhere and the text is fitted to its length. Layouts: "couplet" (FarsiEPUB's screensaver: verses, poet, gloss,
// signature), "title" and "quote" (the opening screen's logo, a text under it).
// Experiment (2026-11-05): not used by the reader yet.
type Sleep struct {
	Layout string   `yaml:"layout"`
	Verses []string `yaml:"verses"` // couplet: the two verses
	Poet   string   `yaml:"poet"`
	Gloss  string   `yaml:"gloss"`
	Text   string   `yaml:"text"` // title, quote
	Credit string   `yaml:"credit"`
}

// sleepSep is the line between two screens in one file: YAML's document separator, "---" alone on its line.
var sleepSep = regexp.MustCompile(`(?m)^---[ \t]*(#.*)?\r?$\n?`)

// ParseSleeps reads a sleep screens' file: one screen, or several separated by lines of "---" (a set, as FarsiEPUB serves
// them). A screen that does not load is left out and its error returned, so a mistake costs that screen, not the set.
// Parts with nothing in them (before a first "---", comments only) are not screens.
func ParseSleeps(b []byte) (screens []Sleep, errs []error) {
	text := string(b)
	at := 0
	parts := append(sleepSep.FindAllStringIndex(text, -1), []int{len(text), len(text)})
	for _, sep := range parts {
		part := text[at:sep[0]]
		line := strings.Count(text[:at], "\n") + 1
		at = sep[1]
		s, err := ParseSleep([]byte(part))
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("the screen from line %d: %w", line, err))
		case len(s.Verses) > 0 || s.Text != "" || s.Poet != "" || s.Gloss != "" || s.Credit != "":
			screens = append(screens, s)
		}
	}
	return screens, errs
}

// ParseSleep reads one sleep screen.
func ParseSleep(b []byte) (Sleep, error) {
	var s Sleep
	err := yaml.Unmarshal(b, &s)
	if s.Layout == "" {
		s.Layout = "title"
		if len(s.Verses) > 0 {
			s.Layout = "couplet"
		}
	}
	return s, err
}

type sleepData struct {
	Sleep
	Lines                                                     []template.HTML // the verses, each on two lines
	Dir                                                       string
	Size, Top, Side, H, LogoW, LogoH                          int
	Poem, Gap, PoetSize, PoetGap, RuleW, RuleH, RuleGap       int
	GlossSize, PadTop, PadSide, PadBottom, SigGap, SigW, SigH int
	TextLang, CreditLang, PoetLang, GlossLang                 template.HTMLAttr
}

// sleepSize is the type's size for a text of n letters (title, quote): large for a few words, smaller as it grows, never
// under 40 px (the reading text's size is 50).
func sleepSize(n int) int {
	switch {
	case n <= 30:
		return 64
	case n <= 90:
		return 54
	case n <= 200:
		return 46
	}
	return 40
}

// FarsiEPUB's screensaver sizes (screensavers/design.toml: poem 96 at Parastoo's scale 1.15, gloss 42, poet 40), for a
// 1236 px wide page, scaled with the width. The couplet shrinks until each verse fits on two lines, not below 70%.
func coupletFit(verses []string, w int) (poem int) {
	u := float64(w) / 1236
	full := 96 * 1.15 * u
	longest := 1
	for _, v := range verses {
		longest = max(longest, utf8.RuneCountInString(v))
	}
	// a verse on two lines: half its letters a line, about 0.62 em a letter (Parastoo Bold), in the width left by the margins
	room := float64(w) - 2*140*u
	fit := room * 2 / (0.62 * float64(longest))
	return int(max(min(full, fit), 0.5*full))
}

// twoLines is a verse on two lines, broken at the word nearest its middle (FarsiEPUB's screensaver, render.py two_lines).
func twoLines(verse string) template.HTML {
	words := strings.Split(verse, " ")
	if len(words) < 2 {
		return template.HTML(template.HTMLEscapeString(verse))
	}
	half := float64(utf8.RuneCountInString(verse)) / 2
	best, run, bestRun := 1, 0, utf8.RuneCountInString(words[0])+1
	for i, w := range words[:len(words)-1] {
		run += utf8.RuneCountInString(w) + 1
		if math.Abs(float64(run)-half) < math.Abs(float64(bestRun)-half) {
			best, bestRun = i+1, run
		}
	}
	return template.HTML(template.HTMLEscapeString(strings.Join(words[:best], " ")) + "<br>" +
		template.HTMLEscapeString(strings.Join(words[best:], " ")))
}

// DrawSleep is the sleep screen s for a screen w × h, grey.
func DrawSleep(s Sleep, fonts *blitz.Fonts, w, h int) *image.Gray {
	img, _ := drawSleepFit(s, fonts, w, h)
	return img
}

// drawSleepFit is DrawSleep, and says whether a verse is still on more than two lines at the smallest size.
func drawSleepFit(s Sleep, fonts *blitz.Fonts, w, h int) (*image.Gray, bool) {
	u := float64(w) / 1236
	px := func(v float64) int { return int(v*u + 0.5) }
	d := sleepData{Sleep: s, Size: px(float64(sleepSize(utf8.RuneCountInString(s.Text)))), Top: h * 3 / 10, Side: w / 10, H: h,
		LogoW: px(400), LogoH: px(435)}
	all := strings.Join(append([]string{s.Text, s.Credit, s.Poet, s.Gloss}, s.Verses...), " ")
	if textLang(all) != "" {
		d.Dir = "rtl"
	}
	d.TextLang, d.CreditLang, d.PoetLang, d.GlossLang = textLang(s.Text), textLang(s.Credit), textLang(s.Poet), textLang(s.Gloss)
	if s.Layout == "couplet" {
		d.TextLang = textLang(strings.Join(s.Verses, " "))
		for _, v := range s.Verses {
			d.Lines = append(d.Lines, twoLines(v))
		}
		d.Poem = coupletFit(s.Verses, w)
		d.Gap, d.PoetSize, d.PoetGap = px(70), px(40*1.15), px(40)
		d.RuleW, d.RuleH, d.RuleGap = px(120), max(px(2), 2), px(50)
		d.GlossSize = px(42 * 1.15)
		d.PadTop, d.PadSide, d.PadBottom = px(105), px(140), px(90)
		d.SigGap, d.SigH = px(30), px(90)
		d.SigW = d.SigH * 1172 / 300
		d.Size = d.GlossSize
	}
	if s.Layout != "couplet" {
		return drawSleepOnce(d, fonts, w, h)
	}
	// FarsiEPUB's fit (render.py): the couplet shrinks, 2 px at a width of 1236 a step, until each verse is on two lines at
	// most. FarsiEPUB stops at 70% and rejects the poem; a reader keeps going, to 50%, rather than show a third line.
	// The estimate above only starts it near. A verse on a third line, or more, is too big.
	floor := int(0.5 * 96 * 1.15 * u)
	for {
		img, tall := drawSleepOnce(d, fonts, w, h)
		if !tall || d.Poem <= floor {
			return img, tall
		}
		d.Poem = max(d.Poem-px(2), floor)
	}
}

// drawSleepOnce draws d, and says whether a verse is on more than two lines.
func drawSleepOnce(d sleepData, fonts *blitz.Fonts, w, h int) (img *image.Gray, tall bool) {
	html, _ := render("sleep.html", d)
	read := func(p string) ([]byte, error) {
		switch p {
		case "brand/shamsaink-stack.png", "brand/shamsaink-by.png":
			return files.ReadFile("templates/" + p)
		}
		return readFor(html)(p)
	}
	b := blitz.New([]string{"sleep.html"}, read, blitz.Options{Width: w, Height: h, FontSize: float32(d.Size)}, fonts)
	defer b.Close()
	b.Layout(0)
	for i := range d.Lines {
		pg, rs := b.Find(0, fmt.Sprintf("v%d", i))
		if pg == 0 && float64(union(rs).Dy()) > 2.1*1.4*float64(d.Poem) {
			tall = true
		}
	}
	img = image.NewGray(image.Rect(0, 0, w, h))
	b.Draw(0, img)
	return img, tall
}
