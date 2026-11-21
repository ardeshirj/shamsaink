package design

import (
	"bytes"
	_ "embed"
	"encoding/json"
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
	Thumb   *siteImage // its first case, in English when there is one: the closed row's picture
}

type siteDevice struct {
	Device
	Screens []siteScreen
}

// How a device other than the primary differs: the renders where it draws other buttons (one a screen: which ones it
// lacks), the screens it cannot reach (a control it lacks), and what only it shows (its profile's only_here).
type siteOther struct {
	Device
	Differ   []siteDiffer
	Missing  []string
	OnlyHere []siteImage
}

type siteGroup struct {
	Name    string
	Screens []siteScreen
}

type siteDiffer struct {
	Screen string
	Image  siteImage
	Why    string
}

type siteData struct {
	*Spec
	Devices    []Device    // every profile, for the table
	Primary    siteDevice  // the screens, as the primary device draws them
	Groups     []siteGroup // the same, in their groups (groups.yaml)
	Others     []siteOther // how each other device differs
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
	data.Devices = append([]Device(nil), s.Devices...)
	sort.SliceStable(data.Devices, func(i, j int) bool { return data.Devices[i].Primary && !data.Devices[j].Primary }) // the primary first
	if len(data.Devices) == 0 {
		return nil, fmt.Errorf("no device profiles")
	}
	var all []siteDevice
	for _, d := range data.Devices {
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
			sort.SliceStable(ss.Cases, func(i, j int) bool { return indexOf(sc.Order, ss.Cases[i]) < indexOf(sc.Order, ss.Cases[j]) })
			if len(ss.Cases) > 0 {
				imgs := ss.Images[ss.Cases[0]]
				ss.Thumb = &imgs[0]
				for i := range imgs {
					if imgs[i].Lang == "en" {
						ss.Thumb = &imgs[i]
						break
					}
				}
			}
			sd.Screens = append(sd.Screens, ss)
		}
		all = append(all, sd)
	}
	data.Primary = all[0]
	for _, g := range s.Groups {
		sg := siteGroup{Name: g.Name}
		for _, n := range g.Screens {
			for _, ss := range data.Primary.Screens {
				if ss.Name == n {
					sg.Screens = append(sg.Screens, ss)
				}
			}
		}
		data.Groups = append(data.Groups, sg)
	}
	for _, sd := range all[1:] {
		o, err := other(s, golden, data.Primary, sd)
		if err != nil {
			return nil, err
		}
		data.Others = append(data.Others, o)
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

// other is how device d differs from the primary p: for each screen, the first render (English first) whose buttons
// are not the primary's (from the boxes kept beside each picture), the screens d cannot reach (every way in through a
// control that requires what it has not), and its only_here renders.
func other(s *Spec, golden fs.FS, p, d siteDevice) (siteOther, error) {
	o := siteOther{Device: d.Device}
	ids := func(path string) (map[string]bool, error) {
		b, err := fs.ReadFile(golden, strings.TrimSuffix(path, ".png")+".json")
		if err != nil {
			return nil, err
		}
		var k struct{ Boxes map[string]any }
		if err := json.Unmarshal(b, &k); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		m := map[string]bool{}
		for id := range k.Boxes {
			m[id] = true
		}
		return m, nil
	}
	local := func(img siteImage) string { // the file under golden/ of a picture's page path
		return img.Path[strings.Index(img.Path, d.Name+"/"):]
	}
	for i, ss := range d.Screens {
		ps := p.Screens[i]
	screen:
		for _, en := range []bool{true, false} {
			for _, c := range ss.Cases {
				for _, img := range ss.Images[c] {
					if (img.Lang == "en") != en {
						continue
					}
					var pimg *siteImage
					for _, q := range ps.Images[c] {
						if q.Lang == img.Lang {
							pimg = &q
							break
						}
					}
					if pimg == nil {
						continue
					}
					a, err := ids(local(img))
					if err != nil {
						continue // a render with no boxes kept: nothing to compare
					}
					b, err := ids(pimg.Path[strings.Index(pimg.Path, p.Name+"/"):])
					if err != nil {
						continue
					}
					var lacks, adds []string
					for id := range b {
						if !a[id] {
							lacks = append(lacks, id)
						}
					}
					for id := range a {
						if !b[id] {
							adds = append(adds, id)
						}
					}
					if len(lacks)+len(adds) == 0 {
						continue
					}
					sort.Strings(lacks)
					sort.Strings(adds)
					why := ""
					if len(lacks) > 0 {
						why = "without " + strings.Join(lacks, ", ")
					}
					if len(adds) > 0 {
						why = strings.TrimPrefix(why+"; with "+strings.Join(adds, ", "), "; ")
					}
					o.Differ = append(o.Differ, siteDiffer{Screen: ss.Name, Image: img, Why: why})
					break screen
				}
			}
		}
	}
	for _, n := range sortedKeys(s.Screens) {
		ways, lacking := 0, 0
		for _, e := range s.Screens[n].Enter {
			ways++
			if e.Control == "" || s.Screens[e.From] == nil {
				continue
			}
			for _, c := range s.Screens[e.From].Controls {
				// dev mode is no device's: the Developer panel is on each, in dev mode
				if c.ID == e.Control && !hasAll(append(d.Has, "development"), c.Requires) {
					lacking++
				}
			}
		}
		if ways > 0 && lacking == ways {
			o.Missing = append(o.Missing, n)
		}
	}
	for _, sc := range d.OnlyHere {
		n, c, _ := strings.Cut(sc, "/")
		found := false
		for _, ss := range d.Screens {
			if ss.Name == n {
				o.OnlyHere = append(o.OnlyHere, ss.Images[c]...)
				found = len(ss.Images[c]) > 0
			}
		}
		if !found {
			return o, fmt.Errorf("%s: only_here %s: no such render", d.Name, sc)
		}
	}
	return o, nil
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
	"has": func(a []string, s string) bool {
		for _, x := range a {
			if x == s {
				return true
			}
		}
		return false
	},
}).Parse(siteHTML))
