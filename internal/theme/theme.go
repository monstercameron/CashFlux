// Package theme is the pure, typed design-token model behind CashFlux's
// appearance engine. A Theme is a set of named tokens — colors, corner radius,
// fonts, a font-size scale, and density — that the UI layer turns into CSS
// custom properties on :root. It validates a theme for legibility (valid colors
// plus WCAG AA contrast), ships built-in presets, and round-trips to JSON so
// themes are shareable.
//
// Pure Go, no platform dependencies; unit-tested on native Go. The wasm layer
// applies CSSVars() to the document; nothing here imports syscall/js.
package theme

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/contrast"
)

// Density controls spacing/sizing intent. The UI maps it to concrete spacing.
type Density string

const (
	// Comfortable is the roomy default.
	Comfortable Density = "comfortable"
	// Compact tightens spacing for denser screens.
	Compact Density = "compact"
)

// Valid reports whether d is a known density.
func (d Density) Valid() bool { return d == Comfortable || d == Compact }

// Theme is a complete set of design tokens. Color fields are hex strings
// ("#rrggbb" or "#rgb"); Radius is in pixels; Scale multiplies the base font
// size (1.0 = 100%); fonts are CSS font-family names.
type Theme struct {
	Name string `json:"name"`

	// Surface and text colors.
	BgBase  string `json:"bgBase"`  // app background
	BgCard  string `json:"bgCard"`  // card/widget surface
	Border  string `json:"border"`  // hairline borders
	Text    string `json:"text"`    // primary text
	TextDim string `json:"textDim"` // secondary/muted text
	Accent  string `json:"accent"`  // interactive accent

	// Semantic colors for gains/losses.
	Up   string `json:"up"`   // positive/inflow
	Down string `json:"down"` // negative/outflow

	// Shape and type.
	Radius      int     `json:"radius"`      // corner radius, px
	FontUI      string  `json:"fontUi"`      // UI font family
	FontDisplay string  `json:"fontDisplay"` // display/heading font family
	Scale       float64 `json:"scale"`       // font-size scale multiplier
	Density     Density `json:"density"`
}

// Default returns CashFlux's built-in dark theme — the baseline every custom
// theme starts from and the target of "reset to default".
func Default() Theme {
	return Theme{
		Name:        "Default",
		BgBase:      "#0e1116",
		BgCard:      "#161b22",
		Border:      "#2a3038",
		Text:        "#e6edf3",
		TextDim:     "#9aa4af",
		Accent:      "#7c83ff",
		Up:          "#54b884",
		Down:        "#d8716f",
		Radius:      12,
		FontUI:      "Inter",
		FontDisplay: "Fraunces",
		Scale:       1.0,
		Density:     Comfortable,
	}
}

// presets are the built-in named themes, keyed by name. Midnight is added in
// init() from Default() so the two stay in sync.
var presets = map[string]Theme{
	"Paper": {
		Name:        "Paper",
		BgBase:      "#f6f5f1",
		BgCard:      "#ffffff",
		Border:      "#d9d6cd",
		Text:        "#1f2328",
		TextDim:     "#5b6068",
		Accent:      "#3f51d6",
		Up:          "#1f8a52",
		Down:        "#b3322f",
		Radius:      10,
		FontUI:      "Inter",
		FontDisplay: "Fraunces",
		Scale:       1.0,
		Density:     Comfortable,
	},
	"Forest": {
		Name:        "Forest",
		BgBase:      "#0f1714",
		BgCard:      "#16211c",
		Border:      "#27352e",
		Text:        "#e8f1ec",
		TextDim:     "#93a89d",
		Accent:      "#4fae84",
		Up:          "#5cc28d",
		Down:        "#d8826f",
		Radius:      14,
		FontUI:      "Inter",
		FontDisplay: "Fraunces",
		Scale:       1.0,
		Density:     Comfortable,
	},
}

func init() {
	midnight := Default()
	midnight.Name = "Midnight"
	presets["Midnight"] = midnight
}

// Presets returns the built-in themes, sorted by name, for a theme picker.
func Presets() []Theme {
	names := make([]string, 0, len(presets))
	for n := range presets {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Theme, 0, len(names))
	for _, n := range names {
		out = append(out, presets[n])
	}
	return out
}

// Preset returns the built-in theme with the given name and whether it exists.
func Preset(name string) (Theme, bool) {
	t, ok := presets[name]
	return t, ok
}

// Issue is one validation problem with a theme token.
type Issue struct {
	Field   string  // the token at fault (e.g. "text", "accent")
	Message string  // plain-English explanation
	Ratio   float64 // the contrast ratio when the issue is a contrast failure, else 0
}

// colorFields pairs each color token with its field name for iteration.
func (t Theme) colorFields() []struct {
	name, hex string
} {
	return []struct{ name, hex string }{
		{"bgBase", t.BgBase}, {"bgCard", t.BgCard}, {"border", t.Border},
		{"text", t.Text}, {"textDim", t.TextDim}, {"accent", t.Accent},
		{"up", t.Up}, {"down", t.Down},
	}
}

// Validate reports every problem that would make a theme unusable or illegible:
// a malformed color, an out-of-range radius/scale, an unknown density, and text
// that fails WCAG AA contrast against its background. An empty result means the
// theme is safe to apply.
func (t Theme) Validate() []Issue {
	var issues []Issue

	// 1. Every color must parse.
	for _, c := range t.colorFields() {
		if _, _, _, err := contrast.ParseHex(c.hex); err != nil {
			issues = append(issues, Issue{Field: c.name, Message: "is not a valid hex color"})
		}
	}
	// Contrast checks below need valid colors; if any failed, stop here so we
	// don't report misleading ratios.
	if len(issues) > 0 {
		issues = append(issues, t.nonColorIssues()...)
		return issues
	}

	// 2. Text must be legible on the surfaces it sits on (WCAG AA normal text).
	contrastPairs := []struct {
		field, fg, bg string
		large         bool
	}{
		{"text", t.Text, t.BgBase, false},
		{"text", t.Text, t.BgCard, false},
		{"textDim", t.TextDim, t.BgCard, false},
		{"accent", t.Accent, t.BgBase, true}, // accent is used for UI/large elements
	}
	for _, p := range contrastPairs {
		ratio, _ := contrast.Ratio(p.fg, p.bg)
		if !contrast.PassesAA(ratio, p.large) {
			issues = append(issues, Issue{
				Field:   p.field,
				Message: fmt.Sprintf("contrast against %s is too low for readable text", bgLabel(p.bg, t)),
				Ratio:   ratio,
			})
		}
	}

	issues = append(issues, t.nonColorIssues()...)
	return issues
}

// nonColorIssues validates the non-color tokens.
func (t Theme) nonColorIssues() []Issue {
	var issues []Issue
	if t.Radius < 0 || t.Radius > 48 {
		issues = append(issues, Issue{Field: "radius", Message: "must be between 0 and 48 pixels"})
	}
	if t.Scale < 0.70 || t.Scale > 2.0 {
		issues = append(issues, Issue{Field: "scale", Message: "must be between 0.70 and 2.0"})
	}
	if !t.Density.Valid() {
		issues = append(issues, Issue{Field: "density", Message: "must be comfortable or compact"})
	}
	return issues
}

func bgLabel(bg string, t Theme) string {
	if bg == t.BgCard {
		return "the card background"
	}
	return "the app background"
}

// Valid reports whether the theme has no validation issues.
func (t Theme) Valid() bool { return len(t.Validate()) == 0 }

// CSSVars returns the theme as a map of CSS custom-property names to values, for
// the UI to set on :root. Color vars are the hex strings; radius/scale carry
// their units.
func (t Theme) CSSVars() map[string]string {
	return map[string]string{
		"--bg-base":      t.BgBase,
		"--bg-card":      t.BgCard,
		"--border":       t.Border,
		"--text":         t.Text,
		"--text-dim":     t.TextDim,
		"--accent":       t.Accent,
		"--up":           t.Up,
		"--down":         t.Down,
		"--radius":       fmt.Sprintf("%dpx", t.Radius),
		"--font-ui":      t.FontUI,
		"--font-display": t.FontDisplay,
		"--ui-scale":     fmt.Sprintf("%g", t.Scale),
		"--density":      string(t.Density),
	}
}

// Merge returns a copy of t with every non-zero field of override applied, so a
// user's tweaks layer cleanly over a base theme. Empty strings, a zero radius, a
// zero scale, and an empty density are treated as "unset" and leave t's value.
func (t Theme) Merge(override Theme) Theme {
	out := t
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	set(&out.Name, override.Name)
	set(&out.BgBase, override.BgBase)
	set(&out.BgCard, override.BgCard)
	set(&out.Border, override.Border)
	set(&out.Text, override.Text)
	set(&out.TextDim, override.TextDim)
	set(&out.Accent, override.Accent)
	set(&out.Up, override.Up)
	set(&out.Down, override.Down)
	set(&out.FontUI, override.FontUI)
	set(&out.FontDisplay, override.FontDisplay)
	if override.Radius != 0 {
		out.Radius = override.Radius
	}
	if override.Scale != 0 {
		out.Scale = override.Scale
	}
	if override.Density != "" {
		out.Density = override.Density
	}
	return out
}

// ToJSON serializes the theme for export/sharing.
func (t Theme) ToJSON() ([]byte, error) {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("theme: marshal: %w", err)
	}
	return b, nil
}

// FromJSON parses an exported theme. Missing fields are filled from Default()
// so an older or partial theme file still yields a complete, usable theme.
func FromJSON(data []byte) (Theme, error) {
	t := Default()
	if err := json.Unmarshal(data, &t); err != nil {
		return Theme{}, fmt.Errorf("theme: unmarshal: %w", err)
	}
	return t, nil
}
