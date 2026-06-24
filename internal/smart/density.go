// SPDX-License-Identifier: MIT

package smart

// Density is the user's global control over HOW MUCH the SMART layer weaves
// itself into the app. It layers ON TOP of the per-feature opt-in: an affordance
// shows only when (a) its feature is enabled and not muted, AND (b) the current
// density is high enough to permit that KIND of affordance. So "riddle the app
// with smart features" is the user's dial, never forced.
//
//   - Off        — no smart affordances anywhere (the layer is invisible).
//   - Minimal    — only the quietest: severity badges, the per-page strip, and
//     help in empty states.
//   - Standard   — adds explainer tooltips, in-form field assists, section
//     "Ask/Run" actions, and smart dashboard widgets. (Default.)
//   - Everywhere — adds the deep entity overlays / coach panels on top.
type Density string

const (
	DensityOff        Density = "off"
	DensityMinimal    Density = "minimal"
	DensityStandard   Density = "standard"
	DensityEverywhere Density = "everywhere"
)

// Affordance is a kind of inline smart surface, ranked by how prominent/invasive
// it is so a single density dial governs them all.
type Affordance string

const (
	AffordanceStrip         Affordance = "strip"          // the per-page insight strip
	AffordanceBadge         Affordance = "badge"          // a quiet severity dot on a row/figure
	AffordanceEmptyState    Affordance = "empty_state"    // "set this up with help" in an empty section
	AffordanceTooltip       Affordance = "tooltip"        // an opt-in explainer on a figure/control
	AffordanceFieldAssist   Affordance = "field_assist"   // a suggest-this-value affordance in a form field
	AffordanceSectionAction Affordance = "section_action" // a sparkle Ask/Run button in a toolbar/section
	AffordanceWidget        Affordance = "widget"         // a placeable smart dashboard widget
	AffordanceOverlay       Affordance = "overlay"        // a deep entity coach/insights overlay
)

// densityRank orders densities from invisible (0) to fullest (3).
func densityRank(d Density) int {
	switch d {
	case DensityMinimal:
		return 1
	case DensityStandard:
		return 2
	case DensityEverywhere:
		return 3
	default: // DensityOff or unknown
		return 0
	}
}

// affordanceMinRank is the minimum density rank at which an affordance appears.
func affordanceMinRank(a Affordance) int {
	switch a {
	case AffordanceStrip, AffordanceBadge, AffordanceEmptyState:
		return 1 // Minimal+
	case AffordanceTooltip, AffordanceFieldAssist, AffordanceSectionAction, AffordanceWidget:
		return 2 // Standard+
	case AffordanceOverlay:
		return 3 // Everywhere
	default:
		return 2
	}
}

// Valid reports whether d is a known density.
func (d Density) Valid() bool {
	switch d {
	case DensityOff, DensityMinimal, DensityStandard, DensityEverywhere:
		return true
	}
	return false
}

// Label returns a short human label for the density.
func (d Density) Label() string {
	switch d {
	case DensityOff:
		return "Off"
	case DensityMinimal:
		return "Minimal"
	case DensityStandard:
		return "Standard"
	case DensityEverywhere:
		return "Everywhere"
	default:
		return string(d)
	}
}

// AllDensities returns every density in increasing-prominence order, for the picker.
func AllDensities() []Density {
	return []Density{DensityOff, DensityMinimal, DensityStandard, DensityEverywhere}
}

// Shows reports whether this density permits the given affordance kind. It is the
// density half of the gate; callers AND it with the per-feature enabled/mute check.
func (d Density) Shows(a Affordance) bool {
	r := densityRank(d)
	return r > 0 && r >= affordanceMinRank(a)
}
