// linecmp compares where lines and pages start in three engines, from the positions of every word of the spanned
// book: ours (boxes.json from cmd/pages), crengine (experiment 02's wordboxes; "-" to leave it out) and Chromium
// (chrome/run.sh: per spine document, word positions on one long screen page, and the printed PDF's text). Pages are
// checked against Chromium by the lines of text on each (ours from pages.json).
//
//	linecmp WORDS.json OURS_DIR CRE.json CHROME_DIR [NAME]
//
// NAME labels the third engine's boxes (default crengine), so the same comparison serves another engine.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path"
	"strings"
)

type word struct {
	Spine int    `json:"spine"`
	File  string `json:"file"`
	ID    string `json:"id"`
}

type box struct {
	Spine int               `json:"spine"`
	ID    string            `json:"id"`
	Page  int               `json:"page"`
	Rects []image.Rectangle `json:"rects"`
}

type pos struct{ page, top int }

func load(p string, v any) {
	raw, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		panic(fmt.Sprint(p, ": ", err))
	}
}

func key(spine int, id string) string { return fmt.Sprint(spine, "/", id) }

func boxes(p string) map[string]pos {
	var bs []box
	load(p, &bs)
	m := map[string]pos{}
	for _, b := range bs {
		if b.Page >= 0 && len(b.Rects) > 0 {
			m[key(b.Spine, b.ID)] = pos{b.Page, b.Rects[0].Min.Y}
		}
	}
	return m
}

// starts: the words (indexes into words) that begin a line, and those that begin a page.
func starts(words []word, at map[string]pos, near int) (lines, pages map[int]bool, missing int) {
	lines, pages = map[int]bool{}, map[int]bool{}
	var prev pos
	for i, w := range words {
		p, ok := at[key(w.Spine, w.ID)]
		if !ok {
			missing++
			continue
		}
		if i == 0 || words[i-1].Spine != w.Spine || p.page != prev.page || abs(p.top-prev.top) > near {
			lines[i] = true
		}
		if i == 0 || words[i-1].Spine != w.Spine || p.page != prev.page {
			pages[i] = true
		}
		prev = p
	}
	return
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func agree(a, b map[int]bool) (both, onlyA, onlyB int) {
	for i := range a {
		if b[i] {
			both++
		} else {
			onlyA++
		}
	}
	for i := range b {
		if !a[i] {
			onlyB++
		}
	}
	return
}

func main() {
	third := "crengine"
	if len(os.Args) > 5 {
		third = os.Args[5]
	}
	var words []word
	load(os.Args[1], &words)
	ours := boxes(path.Join(os.Args[2], "boxes.json"))
	cre := map[string]pos{}
	if os.Args[3] != "-" {
		cre = boxes(os.Args[3])
	}
	chrome := map[string]pos{}
	files := map[int]string{}
	for _, w := range words {
		files[w.Spine] = w.File
	}
	raw, err := os.ReadFile(path.Join(path.Dir(os.Args[1]), "spine.txt"))
	if err != nil {
		panic(err)
	}
	spineFiles := strings.Fields(string(raw))
	for spine, f := range files {
		name := strings.TrimSuffix(path.Base(f), ".xhtml")
		var rows [][]any
		load(path.Join(os.Args[4], name+".json"), &rows)
		for _, r := range rows {
			chrome[key(spine, r[0].(string))] = pos{0, int(r[1].(float64))}
		}
	}
	engines := []struct {
		name string
		at   map[string]pos
	}{{"ours", ours}, {"Chromium", chrome}}
	if len(cre) > 0 {
		engines = append(engines, struct {
			name string
			at   map[string]pos
		}{third, cre})
	}
	L := map[string]map[int]bool{}
	P := map[string]map[int]bool{}
	for _, e := range engines {
		l, p, miss := starts(words, e.at, 8)
		L[e.name], P[e.name] = l, p
		fmt.Printf("%-9s %6d lines, %4d page starts, %d of %d words not found\n", e.name, len(l), len(p), miss, len(words))
	}
	fmt.Println("\nLine starts in common (same word begins a line in both):")
	pairs := [][2]string{{"ours", "Chromium"}}
	if len(cre) > 0 {
		pairs = append(pairs, [2]string{third, "Chromium"}, [2]string{"ours", third})
	}
	for _, pair := range pairs {
		both, a, b := agree(L[pair[0]], L[pair[1]])
		fmt.Printf("  %-8s vs %-8s %6d in common, %4d only in %s, %4d only in %s (%.1f%% the same)\n",
			pair[0], pair[1], both, a, pair[0], b, pair[1], 100*float64(both)/float64(both+a+b))
	}
	if len(cre) > 0 {
		both, a, b := agree(P["ours"], P[third])
		fmt.Printf("\nPage starts, ours vs %s: %d in common, %d only ours, %d only %s\n", third, both, a, b, third)
	}

	// pages: the lines of text on each, ours against Chromium's printed PDF, per spine document
	var infos []struct{ Spine, Lines int }
	load(path.Join(os.Args[2], "pages.json"), &infos)
	samePages, chromePages := 0, 0
	fmt.Println("\nPages per spine item (ours / Chromium) and lines of text on each:")
	for spine := 0; spine < len(files) || spine < len(spineFiles); spine++ {
		var mine []int
		for _, pi := range infos {
			if pi.Spine == spine {
				mine = append(mine, pi.Lines)
			}
		}
		name := strings.TrimSuffix(path.Base(spineFiles[spine]), ".xhtml")
		raw, err := os.ReadFile(path.Join(os.Args[4], name+".txt"))
		if err != nil {
			panic(err)
		}
		var theirs []int
		for _, pg := range strings.Split(strings.TrimRight(string(raw), "\f\n"), "\f") {
			n := 0
			for _, l := range strings.Split(pg, "\n") {
				if strings.TrimSpace(l) != "" {
					n++
				}
			}
			theirs = append(theirs, n)
		}
		chromePages += len(theirs)
		same := 0
		for i := range min(len(mine), len(theirs)) {
			if mine[i] == theirs[i] {
				same++
			}
		}
		samePages += same
		msg := "same"
		if same != len(theirs) || len(mine) != len(theirs) {
			msg = fmt.Sprintf("differ:\n      ours     %v\n      Chromium %v", mine, theirs)
		}
		fmt.Printf("  %d %-26s %3d / %3d  %s\n", spine, name, len(mine), len(theirs), msg)
	}
	fmt.Printf("Pages holding the same lines as Chromium's: %d of %d (ours %d pages)\n", samePages, chromePages, len(infos))

	// per chapter: lines in each engine, and the first line where ours and Chromium part
	fmt.Println("\nPer spine item: lines (ours / crengine / Chromium), first word where ours and Chromium break differently")
	for spine := 0; spine < len(spineFiles); spine++ {
		if _, ok := files[spine]; !ok {
			continue
		}
		var n [3]int
		first := -1
		for i, w := range words {
			if w.Spine != spine {
				continue
			}
			for k, e := range []string{"ours", third, "Chromium"} {
				if L[e][i] {
					n[k]++
				}
			}
			if first < 0 && L["ours"][i] != L["Chromium"][i] {
				first = i
			}
		}
		msg := "same breaks throughout"
		if first >= 0 {
			msg = fmt.Sprintf("first difference at %s (ours %v, Chromium %v)", words[first].ID, L["ours"][first], L["Chromium"][first])
		}
		fmt.Printf("  %d %-28s %4d / %4d / %4d  %s\n", spine, path.Base(files[spine]), n[0], n[1], n[2], msg)
	}
}
