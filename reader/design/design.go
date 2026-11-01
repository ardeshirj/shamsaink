// Package design is the reader's design system as data: tokens, device profiles, components and screens with their
// flow, in spec/*.yaml, and the checks that keep the spec, the templates in reader/ui and a device in step
// (validate.go). docs/design-system.md says how it is used.
package design

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed spec
var specFS embed.FS

// Token is one CSS variable of the look: its name, value and the note that goes with it.
type Token struct{ Name, Value, Note string }

// Tokens are the look as values (spec/tokens.yaml); CSS keeps the order the file has them in.
type Tokens struct {
	CSS   map[string]string
	Order []Token
	Panel struct {
		FontPx   int `yaml:"font_px"`
		MarginPx int `yaml:"margin_px"`
	}
	Layers []string
}

// UnmarshalYAML reads the tokens, keeping the order of the css variables and the comment after each.
func (t *Tokens) UnmarshalYAML(n *yaml.Node) error {
	var aux struct {
		Panel struct {
			FontPx   int `yaml:"font_px"`
			MarginPx int `yaml:"margin_px"`
		} `yaml:"panel"`
		Layers []string `yaml:"layers"`
	}
	t.CSS = map[string]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "css":
			for j := 0; j+1 < len(v.Content); j += 2 {
				name, val := v.Content[j], v.Content[j+1]
				t.CSS[name.Value] = val.Value
				t.Order = append(t.Order, Token{name.Value, val.Value, strings.TrimSpace(strings.TrimPrefix(val.LineComment, "#"))})
			}
		case "panel", "layers":
		default:
			return fmt.Errorf("tokens.yaml: unknown key %q", k.Value)
		}
	}
	if err := n.Decode(&aux); err != nil {
		return err
	}
	t.Panel, t.Layers = aux.Panel, aux.Layers
	return nil
}

// ButtonMap is what the four page-button gestures do on a screen.
type ButtonMap struct {
	Forward     string `yaml:"forward"`
	Back        string `yaml:"back"`
	HoldForward string `yaml:"hold_forward"`
	HoldBack    string `yaml:"hold_back"`
}

// Capabilities are what a device may have and a control may require: a front light, one whose colour can be set (warmth), a
// colour screen, a way to play sound (the Kobo has none of its own: its headphones, over Bluetooth), and page-turn buttons.
var Capabilities = []string{"light", "warmth", "colour", "sound", "buttons"}

// Device is what the design is judged against (spec/devices/*.yaml).
type Device struct {
	Name   string `yaml:"name"`
	Screen struct {
		Width, Height, PPI int
	} `yaml:"screen"`
	PageButtons bool     `yaml:"page_buttons"` // the device has page-turn buttons
	Waveforms   []string `yaml:"waveforms"`
	Has         []string `yaml:"has"`             // what the device has, of Capabilities: a control that requires something else is not on it
	SleepAfter  int      `yaml:"sleep_after_min"` // minutes untouched before the reader puts the device to sleep (reader/device.SleepAfter); 0: the device does it
	Tap         struct {
		MinMM   float64 `yaml:"min_mm"`
		SlackPx int     `yaml:"slack_px"`
	} `yaml:"tap"`
	Fonts map[string]string `yaml:"fonts"`
}

// Refresh is how a component's change reaches the screen.
type Refresh struct {
	Waveform string `yaml:"waveform"`
	Region   string `yaml:"region"`
}

// Component is one piece the screens are made of (spec/components/*.yaml).
type Component struct {
	Name     string   `yaml:"name"`
	Purpose  string   `yaml:"purpose"`
	Template string   `yaml:"template"` // its {{define}} in components.html, "" for one drawn in a screen's own template
	Helper   string   `yaml:"helper"`
	Role     string   `yaml:"role"` // what a control of it is to a user or a test: button, slider, tab, ...
	IDs      []string `yaml:"ids"`  // what it registers from a base id; none: the id alone
	States   []string `yaml:"states"`
	RTL      string   `yaml:"rtl"`
	CSS      *struct {
		Selector string `yaml:"selector"`
		Width    string `yaml:"width"`
		Height   string `yaml:"height"`
	} `yaml:"css"`
	Refresh Refresh `yaml:"refresh"`
}

// Control is one thing on a screen that answers a tap.
type Control struct {
	ID        string   `yaml:"id"`
	Component string   `yaml:"component"`
	Action    string   `yaml:"action"` // goto (to another screen) or act (does something, stays)
	To        string   `yaml:"to"`
	Does      string   `yaml:"does"`
	When      string   `yaml:"when"`     // present only sometimes
	Requires  []string `yaml:"requires"` // what the device must have (Capabilities), or the control is not on it
	Dynamic   bool     `yaml:"dynamic"`  // made per item (a list's rows): its id is a pattern the template does not spell out
	Values    []string `yaml:"values"`   // for a dynamic control, the names after its id (filter-reading, ...)
}

// Enter is how a screen is reached: a gesture on another screen, or a tap on one of its controls.
type Enter struct {
	Gesture string `yaml:"gesture"`
	Control string `yaml:"control"`
	From    string `yaml:"from"`
}

// Screen is one thing drawn over the page, and the flow out of and into it (spec/screens/*.yaml).
type Screen struct {
	Name      string    `yaml:"name"`
	Layer     string    `yaml:"layer"`
	Templates []string  `yaml:"templates"` // its files in ui/templates
	Status    string    `yaml:"status"`    // built or planned
	Purpose   string    `yaml:"purpose"`
	Page      bool      `yaml:"page"` // the screen every way out lands on (the book's page)
	Entry     bool      `yaml:"entry"`
	Enter     []Enter   `yaml:"enter"`
	Controls  []Control `yaml:"controls"`
	Internal  []string  `yaml:"internal"` // ids the template registers that are not controls
	Dismiss   string    `yaml:"dismiss"`  // how it is left for what is under it
	Buttons   string    `yaml:"buttons"`  // its map of the page buttons (spec/buttons.yaml)
	Words     []string  `yaml:"words"`    // the strings it uses, in both language tables
}

// Principle is something the design stands on, and what keeps it so.
type Principle struct {
	Name  string `yaml:"name"`
	Text  string `yaml:"text"`
	Check string `yaml:"check"`
}

// Spec is the whole design.
type Spec struct {
	Tokens     Tokens
	Buttons    map[string]ButtonMap
	Principles []Principle
	Rules      []Principle
	Devices    []Device
	Components map[string]*Component
	Screens    map[string]*Screen
}

func decode(b []byte, v any) error {
	d := yaml.NewDecoder(strings.NewReader(string(b)))
	d.KnownFields(true) // a misspelt key is an error, not silence
	return d.Decode(v)
}

func readAll[T any](fsys fs.FS, dir string) ([]T, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var out []T
	for _, n := range names {
		b, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, err
		}
		var v T
		if err := decode(b, &v); err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// Load reads the spec embedded in the program.
func Load() (*Spec, error) {
	fsys, err := fs.Sub(specFS, "spec")
	if err != nil {
		return nil, err
	}
	return LoadFS(fsys)
}

// LoadFS reads a spec from a folder (spec/: tokens.yaml, devices/, components/, screens/).
func LoadFS(fsys fs.FS) (*Spec, error) {
	s := &Spec{Components: map[string]*Component{}, Screens: map[string]*Screen{}}
	b, err := fs.ReadFile(fsys, "tokens.yaml")
	if err != nil {
		return nil, err
	}
	if err := decode(b, &s.Tokens); err != nil {
		return nil, fmt.Errorf("tokens.yaml: %w", err)
	}
	if b, err = fs.ReadFile(fsys, "buttons.yaml"); err != nil {
		return nil, err
	}
	var bm struct {
		Maps map[string]ButtonMap `yaml:"maps"`
	}
	if err := decode(b, &bm); err != nil {
		return nil, fmt.Errorf("buttons.yaml: %w", err)
	}
	s.Buttons = bm.Maps
	if b, err = fs.ReadFile(fsys, "principles.yaml"); err != nil {
		return nil, err
	}
	var pr struct {
		Principles []Principle `yaml:"principles"`
		Rules      []Principle `yaml:"rules"`
	}
	if err := decode(b, &pr); err != nil {
		return nil, fmt.Errorf("principles.yaml: %w", err)
	}
	s.Principles, s.Rules = pr.Principles, pr.Rules
	if s.Devices, err = readAll[Device](fsys, "devices"); err != nil {
		return nil, err
	}
	cs, err := readAll[Component](fsys, "components")
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		if s.Components[c.Name] != nil {
			return nil, fmt.Errorf("component %s twice", c.Name)
		}
		c := c
		if len(c.IDs) == 0 {
			c.IDs = []string{""}
		}
		s.Components[c.Name] = &c
	}
	ss, err := readAll[Screen](fsys, "screens")
	if err != nil {
		return nil, err
	}
	for _, sc := range ss {
		if s.Screens[sc.Name] != nil {
			return nil, fmt.Errorf("screen %s twice", sc.Name)
		}
		sc := sc
		s.Screens[sc.Name] = &sc
	}
	return s, nil
}
