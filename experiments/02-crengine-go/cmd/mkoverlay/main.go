// mkoverlay makes a read-along copy of the test book: in text/sec002.xhtml the words of the first paragraphs get
// <span id="w0001"> etc., and text/sec002.smil times them (word level, inside one <seq> per paragraph: sentence and
// word granularity in one SMIL, as EDRLab offered to test). The timings are made up (a steady pace, a pause after each
// paragraph) and the audio file is not included: this trial needs the timings, not the sound.
//
//	mkoverlay BOOK_DIR OUT.epub [PARAGRAPHS]
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	wordSec  = 0.42 // seconds per word
	pauseSec = 0.8  // after each paragraph
	chapter  = "text/sec002.xhtml"
	smilHref = "text/sec002.smil"
	audio    = "../audio/sec002.mp3"
)

type word struct {
	id         string
	begin, end float64
}

type para struct {
	id    string
	words []word
}

var pOpen = regexp.MustCompile(`<p id="([^"]+)"[^>]*>`)

// wrapWords puts each space-separated word of the paragraph's text in a span, leaving tags (page-break markers)
// where they are. ZWNJ half-spaces stay inside their word.
func wrapWords(body string, next func() string) string {
	var b strings.Builder
	for len(body) > 0 {
		if body[0] == '<' {
			end := strings.IndexByte(body, '>') + 1
			b.WriteString(body[:end])
			body = body[end:]
			continue
		}
		end := strings.IndexByte(body, '<')
		if end < 0 {
			end = len(body)
		}
		text := body[:end]
		body = body[end:]
		for i, w := range strings.Split(text, " ") {
			if i > 0 {
				b.WriteByte(' ')
			}
			if strings.TrimSpace(w) == "" {
				b.WriteString(w)
				continue
			}
			fmt.Fprintf(&b, `<span id="%s">%s</span>`, next(), w)
		}
	}
	return b.String()
}

func main() {
	src, out := os.Args[1], os.Args[2]
	n := 5
	if len(os.Args) > 3 {
		n, _ = strconv.Atoi(os.Args[3])
	}
	oebps := filepath.Join(src, "OEBPS")

	raw, err := os.ReadFile(filepath.Join(oebps, chapter))
	check(err)
	x := string(raw)
	var paras []para
	t := 0.0
	var b strings.Builder
	for i := 0; i < n; i++ {
		loc := pOpen.FindStringSubmatchIndex(x)
		if loc == nil {
			break
		}
		closeAt := loc[1] + strings.Index(x[loc[1]:], "</p>")
		p := para{id: x[loc[2]:loc[3]]}
		wrapped := wrapWords(x[loc[1]:closeAt], func() string {
			id := fmt.Sprintf("w%04d", len(paras)*1000+len(p.words)+1)
			p.words = append(p.words, word{id, t, t + wordSec})
			t += wordSec
			return id
		})
		t += pauseSec
		b.WriteString(x[:loc[1]])
		b.WriteString(wrapped)
		x = x[closeAt:]
		paras = append(paras, p)
	}
	b.WriteString(x)
	chapterOut := b.String()

	var smil strings.Builder
	smil.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" xmlns:epub="http://www.idpf.org/2007/ops" version="3.0">
<body>
`)
	words := 0
	for _, p := range paras {
		fmt.Fprintf(&smil, "<seq epub:textref=\"sec002.xhtml#%s\" epub:type=\"paragraph\">\n", p.id)
		for _, w := range p.words {
			fmt.Fprintf(&smil, "<par id=\"par-%s\"><text src=\"sec002.xhtml#%s\"/><audio src=\"%s\" clipBegin=\"%.3fs\" clipEnd=\"%.3fs\"/></par>\n",
				w.id, w.id, audio, w.begin, w.end)
			words++
		}
		smil.WriteString("</seq>\n")
	}
	smil.WriteString("</body>\n</smil>\n")

	opf, err := os.ReadFile(filepath.Join(oebps, "content.opf"))
	check(err)
	o := string(opf)
	dur := fmt.Sprintf("%.3fs", t)
	o = replaceOnce(o, `<item id="sec002" href="text/sec002.xhtml" media-type="application/xhtml+xml"/>`,
		`<item id="sec002" href="text/sec002.xhtml" media-type="application/xhtml+xml" media-overlay="sec002-mo"/>
<item id="sec002-mo" href="`+smilHref+`" media-type="application/smil+xml"/>
<item id="sec002-audio" href="audio/sec002.mp3" media-type="audio/mpeg"/>`)
	o = replaceOnce(o, "</metadata>", `<meta property="media:duration" refines="#sec002-mo">`+dur+`</meta>
<meta property="media:duration">`+dur+`</meta>
<meta property="media:active-class">-epub-media-overlay-active</meta>
</metadata>`)
	o = replaceOnce(o, `prefix="`, `prefix="media: http://www.idpf.org/epub/vocab/overlays/# `)

	replaced := map[string]string{"OEBPS/" + chapter: chapterOut, "OEBPS/" + smilHref: smil.String(), "OEBPS/content.opf": o}
	check(writeEPUB(src, out, replaced))
	fmt.Printf("%s: %d paragraphs, %d words timed, %s\n", out, len(paras), words, dur)
}

func replaceOnce(s, old, new string) string {
	if strings.Count(s, old) != 1 {
		panic("not found once: " + old)
	}
	return strings.Replace(s, old, new, 1)
}

// writeEPUB zips dir into an EPUB (mimetype first, stored), with some files' contents replaced or added.
func writeEPUB(dir, out string, replaced map[string]string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	w, _ := z.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	io.WriteString(w, "application/epub+zip")
	err = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		name, _ := filepath.Rel(dir, p)
		name = filepath.ToSlash(name)
		if name == "mimetype" {
			return nil
		}
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		if s, ok := replaced[name]; ok {
			delete(replaced, name)
			_, err = io.WriteString(w, s)
			return err
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		return err
	})
	if err != nil {
		return err
	}
	for name, s := range replaced { // new files
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		io.WriteString(w, s)
	}
	return z.Close()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
