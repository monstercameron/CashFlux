// SPDX-License-Identifier: MIT

package tw

import "github.com/monstercameron/GoWebComponents/v4/css"

// ColorClass maps a color-token name (the old "text-up"/"bg-warn"/… utility class
// strings that are computed dynamically into tones) to a styled class. It returns
// the token name as a stable, semantic marker class (so the tone stays selectable
// for tests/automation — up=positive, down=negative, etc.) followed by the folded
// hashed class that actually carries the color. An empty token yields "" and an
// unrecognized token passes through unchanged.
func ColorClass(token string) string {
	var rule any
	switch token {
	case "":
		return ""
	case "text-up":
		rule = TextUp
	case "text-down":
		rule = TextDown
	case "text-warn":
		rule = TextWarn
	case "text-dim":
		rule = TextDim
	case "text-faint":
		rule = TextFaint
	case "text-fg":
		rule = TextFg
	case "bg-up":
		rule = BgUp
	case "bg-down":
		rule = BgDown
	case "bg-warn":
		rule = BgWarn
	case "bg-dim":
		rule = BgDim
	case "bg-fg":
		rule = BgFg
	default:
		return token
	}
	return token + " " + Fold(rule)
}

// Fold collapses typed utility rules (css.Rule or []css.Rule, e.g. variants and
// multi-declaration tokens like Border/Truncate) into a single hashed class name,
// emitting their CSS through the gwc registry/Sink. Use it to build a class STRING
// for dynamically/conditionally composed class attributes (rail items, menus, KPI
// tiles), concatenated with any semantic class names — the typed analog of the old
// "flex items-center …" utility strings, with no Tailwind dependency.
func Fold(parts ...any) string {
	var rules []css.Rule
	for _, p := range parts {
		switch v := p.(type) {
		case css.Rule:
			rules = append(rules, v)
		case []css.Rule:
			rules = append(rules, v...)
		}
	}
	return string(css.New(rules...))
}

// Additional utilities that appear (only) inside dynamically/conditionally composed
// class strings — rail nav items, dropdown menus, KPI tiles, chips, the progress
// bar. Same exact-value contract as tw.go.
var (
	// background palette tokens
	BgDim   = css.Bg(css.Color(cDim))
	BgLine  = css.Bg(css.Color(cLine))
	BgWarn  = css.Bg(css.Color(cWarn))
	BgHover = css.Bg(css.Color(cHover))
	BgHex1c = css.Bg(css.Color("#1c1c1e")) // bg-[#1c1c1e] (active nav row)

	// tinted backgrounds / borders
	BgSky15        = css.Bg(css.Color("rgb(14 165 233 / 0.15)"))
	BgUp15         = css.Bg(css.Color("rgb(84 184 132 / 0.15)")) // faint green tint for "Free" pills (pairs with TextUp; solid BgUp made green-on-green text invisible)
	BorderSky40    = raw("border-color", "rgb(14 165 233 / 0.4)")
	BorderBlack10  = raw("border-color", "rgb(0 0 0 / 0.1)")
	HoverBgBlack03 = css.Hover(css.Bg(css.Color("rgb(0 0 0 / 0.03)")))

	// sizing
	MinH0  = raw("min-height", "0")
	MinH10 = raw("min-height", "2.5rem")
	MinW10 = raw("min-width", "2.5rem")
	W48    = css.W(css.Rem(12))
	W60    = css.W(css.Rem(15))

	// position offsets
	Left0    = raw("left", "0")
	LeftFull = raw("left", "100%")
	Right0   = raw("right", "0")
	Ml1      = raw("margin-left", "0.25rem")
	Z40      = raw("z-index", "40")

	// flex child sizing
	ShrinkO = raw("flex-shrink", "0") // shrink-0

	LineThrough = raw("text-decoration-line", "line-through") // line-through

	// effects / misc
	Opacity30         = css.Opacity(0.3)
	Opacity70         = css.Opacity(0.7)
	PointerEventsNone = raw("pointer-events", "none")

	// side borders (re-include border-style:solid — no CDN preflight)
	BorderR = []css.Rule{raw("border-right-width", "1px"), raw("border-right-style", "solid")}
	BorderL = []css.Rule{raw("border-left-width", "1px"), raw("border-left-style", "solid")}
)
