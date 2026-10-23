// spanwords makes a copy of an unzipped EPUB in which every word of every paragraph and heading is in
// <span id="wNNNNN">, so each engine (ours, crengine, Chromium) can say where each word went: which line, which page.
// Writes OUT_DIR (unzipped, for Chromium), OUT_DIR.epub, OUT_DIR/words.json (spine index, file, id, in order) and
// OUT_DIR/spine.txt (the spine's files, in order).
//
//	spanwords BOOK_DIR OUT_DIR
package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	blockOpen = regexp.MustCompile(`<(p|h1|h2|h3|h4|h5|h6)(\s[^>]*)?>`)
	itemref   = regexp.MustCompile(`<itemref\s+idref="([^"]+)"`)
	manItem   = regexp.MustCompile(`<item\s[^>]*>`)
	attrRe    = regexp.MustCompile(`(\w[\w-]*)="([^"]*)"`)
	rootfile  = regexp.MustCompile(`full-path="([^"]+)"`)
)

type word struct {
	Spine int    `json:"spine"`
	File  string `json:"file"`
	ID    string `json:"id"`
}

// wrap puts each space-separated run of text in a span, leaving tags where they are.
func wrap(body string, next func() string) string {
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
	container, err := os.ReadFile(filepath.Join(src, "META-INF", "container.xml"))
	check(err)
	opfRel := rootfile.FindStringSubmatch(string(container))[1] // e.g. OEBPS/content.opf
	opf, err := os.ReadFile(filepath.Join(src, opfRel))
	check(err)
	base := strings.TrimPrefix(path.Dir(opfRel)+"/", "./")
	hrefs := map[string]string{}
	for _, it := range manItem.FindAllString(string(opf), -1) {
		a := map[string]string{}
		for _, m := range attrRe.FindAllStringSubmatch(it, -1) {
			a[m[1]] = m[2]
		}
		hrefs[a["id"]] = a["href"]
	}
	replaced := map[string]string{}
	var words []word
	var spine []string
	for si, m := range itemref.FindAllStringSubmatch(string(opf), -1) {
		file := base + hrefs[m[1]]
		spine = append(spine, file)
		raw, err := os.ReadFile(filepath.Join(src, file))
		check(err)
		x := string(raw)
		n := 0
		var b strings.Builder
		for {
			loc := blockOpen.FindStringSubmatchIndex(x)
			if loc == nil {
				break
			}
			tag := x[loc[2]:loc[3]]
			closeAt := loc[1] + strings.Index(x[loc[1]:], "</"+tag+">")
			b.WriteString(x[:loc[1]])
			b.WriteString(wrap(x[loc[1]:closeAt], func() string {
				n++
				id := fmt.Sprintf("w%05d", n)
				words = append(words, word{si, file, id})
				return id
			}))
			x = x[closeAt:]
		}
		b.WriteString(x)
		replaced[file] = b.String()
	}
	check(os.RemoveAll(out))
	check(filepath.WalkDir(src, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		dst := filepath.Join(out, rel)
		check(os.MkdirAll(filepath.Dir(dst), 0o755))
		if s, ok := replaced[filepath.ToSlash(rel)]; ok {
			return os.WriteFile(dst, []byte(s), 0o644)
		}
		data, err := os.ReadFile(p)
		check(err)
		return os.WriteFile(dst, data, 0o644)
	}))
	check(zipEPUB(out, out+".epub"))
	j, _ := json.Marshal(words)
	check(os.WriteFile(filepath.Join(out, "words.json"), j, 0o644))
	check(os.WriteFile(filepath.Join(out, "spine.txt"), []byte(strings.Join(spine, "\n")+"\n"), 0o644))
	fmt.Printf("%s: %d words\n", out, len(words))
}

// zipEPUB zips dir into an EPUB, mimetype first and stored (words.json is not part of the book).
func zipEPUB(dir, out string) error {
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
		if name == "mimetype" || name == "words.json" || name == "spine.txt" {
			return nil
		}
		w, err := z.Create(name)
		if err != nil {
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
	return z.Close()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
