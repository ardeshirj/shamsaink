package design

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"image"
	_ "image/png"
	"io/fs"
	"sort"
	"strings"
)

// A render kept for a screen: its case and language, the file and its size.
type siteImage struct {
	Case, Lang, Path string
	W, H             int // as drawn, px
	Shown            int // width on the page, px
}

type siteScreen struct {
	*Screen
	Images  map[string][]siteImage // by case, one per language
	Cases   []string
	Buttons ButtonMap
}

type siteDevice struct {
	Device
	Screens []siteScreen
}

type siteData struct {
	*Spec
	Devices    []siteDevice
	Components []*Component
	Layers     []string
	Flow       []string
}

// Site is the design system as one page: the principles, tokens, devices, components, and each screen with its
// controls, flow, page buttons and the renders kept in golden (golden/DEVICE/*.png), the pictures linked from
// goldenURL (a path from the page to golden/).
func Site(s *Spec, golden fs.FS, goldenURL string) ([]byte, error) {
	data := siteData{Spec: s, Layers: s.Tokens.Layers}
	for _, n := range sortedKeys(s.Components) {
		data.Components = append(data.Components, s.Components[n])
	}
	names := sortedKeys(s.Screens)
	// screens in the order of the layers, then by name
	sort.SliceStable(names, func(i, j int) bool {
		return indexOf(s.Tokens.Layers, s.Screens[names[i]].Layer) < indexOf(s.Tokens.Layers, s.Screens[names[j]].Layer)
	})
	for _, d := range s.Devices {
		sd := siteDevice{Device: d}
		files, _ := fs.Glob(golden, d.Name+"/*.png")
		for _, n := range names {
			sc := s.Screens[n]
			ss := siteScreen{Screen: sc, Images: map[string][]siteImage{}, Buttons: s.Buttons[sc.Buttons]}
			for _, f := range files {
				base := strings.TrimSuffix(f[len(d.Name)+1:], ".png")
				if owner(base, s) != n {
					continue
				}
				rest := strings.TrimPrefix(base, n+"-")
				i := strings.LastIndex(rest, "-")
				c, lang := rest[:i], rest[i+1:]
				fh, err := golden.Open(f)
				if err != nil {
					return nil, err
				}
				cfg, _, err := image.DecodeConfig(fh)
				fh.Close()
				if err != nil {
					return nil, fmt.Errorf("%s: %w", f, err)
				}
				img := siteImage{Case: c, Lang: lang, Path: goldenURL + "/" + f, W: cfg.Width, H: cfg.Height, Shown: cfg.Width / 3}
				if _, seen := ss.Images[c]; !seen {
					ss.Cases = append(ss.Cases, c)
				}
				ss.Images[c] = append(ss.Images[c], img)
			}
			sd.Screens = append(sd.Screens, ss)
		}
		data.Devices = append(data.Devices, sd)
	}
	// the flow: every way from one screen to another, as one line each
	for _, n := range names {
		sc := s.Screens[n]
		for _, e := range sc.Enter {
			how := e.Gesture
			if e.Control != "" {
				how += " " + e.Control
			}
			data.Flow = append(data.Flow, fmt.Sprintf("%s → %s: %s", e.From, n, how))
		}
	}
	var b bytes.Buffer
	if err := siteTpl.Execute(&b, data); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// owner is the screen a render file belongs to: the longest screen name that starts it.
func owner(base string, s *Spec) string {
	best := ""
	for n := range s.Screens {
		if strings.HasPrefix(base, n+"-") && len(n) > len(best) {
			best = n
		}
	}
	return best
}

func indexOf(a []string, s string) int {
	for i, x := range a {
		if x == s {
			return i
		}
	}
	return len(a)
}

//go:embed site.html
var siteHTML string

var siteTpl = template.Must(template.New("site").Funcs(template.FuncMap{
	"join": strings.Join,
}).Parse(siteHTML))
