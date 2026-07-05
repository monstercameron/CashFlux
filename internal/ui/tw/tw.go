// SPDX-License-Identifier: MIT

// Package tw is CashFlux's typed, Tailwind-compatible utility vocabulary, built on
// the GoWebComponents css engine (Layer-1 css + the css/u utilities). It exists to
// retire the runtime Tailwind CDN (C91): every utility class the app used as a
// string is recreated here as a typed Go symbol that emits the *exact same* CSS
// the Tailwind v3 default config produced, via the css registry/Sink (so the CSS
// is injected at runtime into <style id="gwc-css"> with no external dependency and
// works offline).
//
// Usage at a call site mirrors the old string form, mixing semantic app classes
// (kept as literal strings — they live in the hand-written design-system stylesheet)
// with typed utilities:
//
//	// before: ClassStr("btn btn-primary inline-flex items-center gap-1.5")
//	css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15)
//
// Values match Tailwind's defaults exactly (spacing = n×0.25rem, the type scale's
// font-size+line-height pairs, etc.) so the migration is visually lossless. Custom
// palette tokens (base/tile/line/hover/fg/dim/faint/up/down/warn) use the exact
// hex from the former inline tailwind.config; switching them to theme CSS vars is a
// deliberate follow-up (theming reconciliation), kept separate to keep this change
// parity-safe. Border utilities re-include border-style:solid because dropping the
// CDN also drops Tailwind's preflight (which set border-style globally).
package tw

import "github.com/monstercameron/GoWebComponents/v4/css"

// --- the app's custom palette (exact hex from the former tailwind.config) ------
const (
	cBase  = "#0e0e0f"
	cTile  = "#121214"
	cLine  = "#232325"
	cHover = "#161617"
	cFg    = "#f4f4f5"
	cDim   = "#ababb3"
	cFaint = "#7d7d85"
	cUp    = "#54b884"
	cDown  = "#d8716f"
	cWarn  = "#cfa14e"
)

// raw is a brief alias for css.Raw (an exact property:value declaration).
func raw(prop, val string) css.Rule { return css.Raw(prop, val) }

// --- display & layout ---------------------------------------------------------
var (
	Block       = css.Display.Block
	InlineBlock = css.Display.InlineBlock
	Flex        = css.Display.Flex
	InlineFlex  = css.Display.InlineFlex
	Grid        = css.Display.Grid

	FlexCol  = css.FlexDir.Col
	FlexWrap = raw("flex-wrap", "wrap")
	Flex1    = raw("flex", "1 1 0%")

	GridCols2 = raw("grid-template-columns", "repeat(2, minmax(0, 1fr))")
	GridCols3 = raw("grid-template-columns", "repeat(3, minmax(0, 1fr))")

	ItemsStart       = css.Items.Start
	ItemsCenter      = css.Items.Center
	ItemsEnd         = css.Items.End
	JustifyStart     = css.Justify.Start
	JustifyCenter    = css.Justify.Center
	JustifyBetween   = css.Justify.Between
	JustifyEnd       = raw("justify-content", "flex-end")
	ContentStart     = raw("align-content", "flex-start")
	PlaceItemsCenter = raw("place-items", "center")
	SelfStart        = raw("align-self", "flex-start")

	Relative = css.Position.Relative
	Absolute = css.Position.Absolute
	Sticky   = css.Position.Sticky
	Top0     = raw("top", "0")
	TopFull  = raw("top", "100%")
	Right1   = raw("right", "0.25rem")
	Z20      = raw("z-index", "20")
	Z30      = raw("z-index", "30")
)

// --- flex/grid gap (n×0.25rem) ------------------------------------------------
var (
	Gap05 = css.Gap(css.Rem(0.125))
	Gap1  = css.Gap(css.Rem(0.25))
	Gap15 = css.Gap(css.Rem(0.375))
	Gap2  = css.Gap(css.Rem(0.5))
	Gap25 = css.Gap(css.Rem(0.625))
	Gap3  = css.Gap(css.Rem(0.75))
	Gap4  = css.Gap(css.Rem(1))
	Gap5  = css.Gap(css.Rem(1.25))

	GapX4 = raw("column-gap", "1rem")
	GapX7 = raw("column-gap", "1.75rem")
	GapY1 = raw("row-gap", "0.25rem")
)

// --- space-y (margin-top on every non-first child) ----------------------------
var (
	SpaceY15 = css.Child(css.Sel("* + *"), raw("margin-top", "0.375rem"))
	SpaceY25 = css.Child(css.Sel("* + *"), raw("margin-top", "0.625rem"))
	SpaceY4  = css.Child(css.Sel("* + *"), raw("margin-top", "1rem"))
)

// --- margin -------------------------------------------------------------------
var (
	MlN1   = raw("margin-left", "-0.25rem")
	M3     = css.Margin(css.Rem(0.75))
	MAuto  = css.Margin(css.Auto)
	Mb1    = raw("margin-bottom", "0.25rem")
	Mb2    = raw("margin-bottom", "0.5rem")
	Mb3    = raw("margin-bottom", "0.75rem")
	MlAuto = raw("margin-left", "auto")
	Mr2    = raw("margin-right", "0.5rem")
	Mt05   = raw("margin-top", "0.125rem")
	Mt1    = raw("margin-top", "0.25rem")
	Mt15   = raw("margin-top", "0.375rem")
	Mt2    = raw("margin-top", "0.5rem")
	Mt3    = raw("margin-top", "0.75rem")
	Mt5    = raw("margin-top", "1.25rem")
	Mt045  = raw("margin-top", "0.45rem")
	MtAuto = raw("margin-top", "auto")
	Mx3    = css.MarginX(css.Rem(0.75))
	MxAuto = css.MarginX(css.Auto)
	My2    = css.MarginY(css.Rem(0.5))
)

// --- padding ------------------------------------------------------------------
var (
	P1    = css.Padding(css.Rem(0.25))
	P3    = css.Padding(css.Rem(0.75))
	P10px = css.Padding(css.Px(10))
	Pb1   = raw("padding-bottom", "0.25rem")
	Pb2   = raw("padding-bottom", "0.5rem")
	Pr1   = raw("padding-right", "0.25rem")
	Pr3   = raw("padding-right", "0.75rem")
	Pt15  = raw("padding-top", "0.375rem")
	Pt2   = raw("padding-top", "0.5rem")
	Pt3   = raw("padding-top", "0.75rem")
	Pt4   = raw("padding-top", "1rem")
	Px1   = css.PaddingX(css.Rem(0.25))
	Px15  = css.PaddingX(css.Rem(0.375))
	Px2   = css.PaddingX(css.Rem(0.5))
	Px3   = css.PaddingX(css.Rem(0.75))
	Px35  = css.PaddingX(css.Rem(0.875))
	Px5   = css.PaddingX(css.Rem(1.25))
	Px6   = css.PaddingX(css.Rem(1.5))
	Py05  = css.PaddingY(css.Rem(0.125))
	Py1   = css.PaddingY(css.Rem(0.25))
	Py15  = css.PaddingY(css.Rem(0.375))
	Py2   = css.PaddingY(css.Rem(0.5))
	Py25  = css.PaddingY(css.Rem(0.625))
)

// --- sizing (w/h n×0.25rem; named fractions/keywords as Tailwind) -------------
var (
	W2    = css.W(css.Rem(0.5))
	W25   = css.W(css.Rem(0.625))
	W3    = css.W(css.Rem(0.75))
	W35   = css.W(css.Rem(0.875))
	W4    = css.W(css.Rem(1))
	W5    = css.W(css.Rem(1.25))
	W7    = css.W(css.Rem(1.75))
	W8    = css.W(css.Rem(2))
	W9    = css.W(css.Rem(2.25))
	W10   = css.W(css.Rem(2.5))
	W16   = css.W(css.Rem(4))
	W24   = css.W(css.Rem(6))
	W18px = css.W(css.Px(18))
	WFull = css.W(css.Full)

	H2      = css.H(css.Rem(0.5))
	H25     = css.H(css.Rem(0.625))
	H3      = css.H(css.Rem(0.75))
	H35     = css.H(css.Rem(0.875))
	H4      = css.H(css.Rem(1))
	H5      = css.H(css.Rem(1.25))
	H7      = css.H(css.Rem(1.75))
	H8      = css.H(css.Rem(2))
	H9      = css.H(css.Rem(2.25))
	H10     = css.H(css.Rem(2.5))
	H14     = css.H(css.Rem(3.5))
	H18px   = css.H(css.Px(18))
	HFull   = css.H(css.Full)
	HScreen = raw("height", "100vh")

	MinW0    = raw("min-width", "0")
	MinW150  = raw("min-width", "150px")
	MaxH55vh = raw("max-height", "55vh")
	MaxHFull = raw("max-height", "100%")
	MaxW160  = raw("max-width", "160px")
	MaxW85   = raw("max-width", "85%")
	MaxWFull = raw("max-width", "100%")
)

// --- typography ---------------------------------------------------------------
// Type-scale tokens carry Tailwind's font-size + line-height pair; arbitrary
// text-[..px] sets font-size only (Tailwind's arbitrary behavior).
var (
	TextXs   = []css.Rule{css.FontSize(css.Rem(0.75)), raw("line-height", "1rem")}
	TextBase = []css.Rule{css.FontSize(css.Rem(1)), raw("line-height", "1.5rem")}
	TextLg   = []css.Rule{css.FontSize(css.Rem(1.125)), raw("line-height", "1.75rem")}

	Text11  = css.FontSize(css.Px(11))
	Text12  = css.FontSize(css.Px(12))
	Text13  = css.FontSize(css.Px(13))
	Text135 = raw("font-size", "13.5px")
	Text14  = css.FontSize(css.Px(14))
	Text15  = css.FontSize(css.Px(15))
	Text18  = css.FontSize(css.Px(18))
	Text28  = css.FontSize(css.Px(28))

	TextCenter = raw("text-align", "center")
	TextLeft   = raw("text-align", "left")
	TextRight  = raw("text-align", "right")

	FontSans     = raw("font-family", "var(--font-ui), Inter, ui-sans-serif, system-ui, sans-serif")
	FontDisplay  = raw("font-family", "var(--font-display), Fraunces, Georgia, serif")
	FontMedium   = css.FontWeight.Medium
	FontSemibold = css.FontWeight.Semibold

	LeadingNone   = raw("line-height", "1")
	LeadingTight  = raw("line-height", "1.25")
	TrackingTight = raw("letter-spacing", "-0.025em")
	Tracking008   = raw("letter-spacing", "0.08em")
	Uppercase     = raw("text-transform", "uppercase")
	Underline     = raw("text-decoration-line", "underline")

	Truncate          = []css.Rule{raw("overflow", "hidden"), raw("text-overflow", "ellipsis"), raw("white-space", "nowrap")}
	WhitespacePreWrap = raw("white-space", "pre-wrap")
	// LineClamp2 truncates multi-line text to two lines with a clean ellipsis at a
	// word boundary (vs Truncate, which clips a single line mid-word).
	LineClamp2 = []css.Rule{raw("display", "-webkit-box"), raw("-webkit-line-clamp", "2"), raw("-webkit-box-orient", "vertical"), raw("overflow", "hidden")}
)

// --- color (exact palette hex; theme-var migration is a follow-up) ------------
var (
	BgBase    = css.Bg(css.Color(cBase))
	BgFg      = css.Bg(css.Color(cFg))
	BgAccent  = css.Bg(css.Color("var(--accent, #2e8b57)")) // brand-accent fill (e.g. the rail logo mark)
	BgDown    = css.Bg(css.Color(cDown))
	BgUp      = css.Bg(css.Color(cUp))
	// Foreground text tokens follow the live theme: `var(--text…)` so they flip with
	// data-theme (the hex fallback equals the dark default, so dark mode is unchanged).
	// Hardcoding the dark hex made these vanish on white in light mode — e.g. the
	// Insights "New chat"/"Edit prompt" pills (TextFg, contrast 1.04) and the breadcrumb
	// parent crumb (TextDim, contrast 1.4). NB: only TEXT color follows the theme; BgFg
	// (an intentional inverted surface) keeps the literal hex.
	TextFg    = css.TextColor(css.Color("var(--text, " + cFg + ")"))
	TextDim   = css.TextColor(css.Color("var(--text-dim, " + cDim + ")"))
	TextFaint = css.TextColor(css.Color("var(--text-faint, " + cFaint + ")"))
	// Semantic up/down TEXT follows the live theme so amounts stay legible in light
	// mode: the literal dark-mode reds/greens (#d8716f/#54b884) measure ~1.8:1 on a
	// white card (a negative "−$1,718.00" is barely readable). The theme engine emits
	// readable light values (--down #b3322f, --up #1f8a52) and the dark values equal
	// these literals exactly, so dark mode is byte-identical. BgDown/BgUp keep the
	// literal hex (intentional fills, like BgFg).
	TextDown  = css.TextColor(css.Color("var(--down, " + cDown + ")"))
	TextUp    = css.TextColor(css.Color("var(--up, " + cUp + ")"))
	TextWarn  = css.TextColor(css.Color(cWarn))

	// tinted backgrounds / borders (Tailwind color/opacity)
	BgAmber10     = css.Bg(css.Color("rgb(251 191 36 / 0.1)"))
	BorderAmber50 = raw("border-color", "rgb(251 191 36 / 0.5)")
	BgSky10       = css.Bg(css.Color("rgb(14 165 233 / 0.1)"))
	BgBlack04     = css.Bg(css.Color("rgb(0 0 0 / 0.04)"))

	HoverBgHover = css.Hover(css.Bg(css.Color(cHover)))
	HoverTextFg  = css.Hover(css.TextColor(css.Color("var(--text, " + cFg + ")")))
)

// --- border & radius (border re-includes border-style:solid — no CDN preflight)
var (
	Border  = []css.Rule{raw("border-width", "1px"), raw("border-style", "solid")}
	BorderB = []css.Rule{raw("border-bottom-width", "1px"), raw("border-bottom-style", "solid")}
	BorderT = []css.Rule{raw("border-top-width", "1px"), raw("border-top-style", "solid")}

	BorderLine   = raw("border-color", cLine)
	BorderLine70 = raw("border-color", "rgb(35 35 37 / 0.7)")

	Rounded     = css.Rounded(css.Rem(0.25))
	RoundedXl   = css.Rounded(css.Rem(0.75))
	Rounded2xl  = css.Rounded(css.Rem(1))
	RoundedFull = css.Rounded(css.Px(9999))
	Rounded4    = css.Rounded(css.Px(4))
)

// --- effects ------------------------------------------------------------------
var (
	ShadowLg  = raw("box-shadow", "0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)")
	Opacity0  = css.Opacity(0)
	Opacity60 = css.Opacity(0.6)

	HoverOpacity100             = css.Hover(css.Opacity(1))
	GroupHoverOpacity100        = css.DefineVariant(".group:hover &")(css.Opacity(1))
	GroupFocusWithinOpacity100  = css.DefineVariant(".group:focus-within &")(css.Opacity(1))
	MotionSafeTransitionOpacity = css.Media(css.RawMedia("(prefers-reduced-motion: no-preference)"), raw("transition-property", "opacity"), raw("transition-duration", "150ms"))
)

// --- misc ---------------------------------------------------------------------
var (
	CursorGrab     = raw("cursor", "grab")
	CursorPointer  = raw("cursor", "pointer")
	OverflowHidden = raw("overflow", "hidden")
	OverflowYAuto  = raw("overflow-y", "auto")
	ObjectContain  = raw("object-fit", "contain")
	ObjectCover    = raw("object-fit", "cover")

	// sr-only: visually hidden but screen-reader accessible.
	SrOnly = []css.Rule{
		raw("position", "absolute"), raw("width", "1px"), raw("height", "1px"),
		raw("padding", "0"), raw("margin", "-1px"), raw("overflow", "hidden"),
		raw("clip", "rect(0, 0, 0, 0)"), raw("white-space", "nowrap"), raw("border-width", "0"),
	}
)
