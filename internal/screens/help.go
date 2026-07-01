// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// helpSection is one widgetized help card: a stable id, a title, a standard
// column width (ColSpan on the 4-column bento; height follows content), and its
// body. It mirrors, at a small scale, the unified WidgetSpec+Placement model from
// docs/UNIFIED_WIDGET_API.md — the /help page is the first surface to render its
// sections as locked tiles on the dashboard's bento grid (no drag/gear: a
// non-custom surface, §7.4) so layout, standard sizes, and reflow are shared.
type helpSection struct {
	id    string
	title string
	col   int // ColSpan (standard width); height is content-driven (§7.5)
	body  ui.Node
}

// setupChecklistBody (R34-onboard / C23) shows the first-run steps with a live
// ✓/○ from the actual data, so a new household sees what's left to set up and a
// returning one sees it's all done. Reads counts only — no mutation, safe to
// render anywhere. The first step surfaces base currency + week-start so new
// users can no longer miss these settings (C23 [MAJOR F3]).
func setupChecklistBody() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	type step struct {
		label ui.Node // may be plain text or a link
		done  bool
	}

	currencySet := app.Settings().BaseCurrency != ""
	currencyLink := A(
		Attr("href", uistate.RoutePath("/appearance")),
		css.Class(tw.Underline, tw.HoverTextFg),
		uistate.T("help.currencyStepLabel"),
	)
	membersLink := A(
		Attr("href", uistate.RoutePath("/members")),
		css.Class(tw.Underline, tw.HoverTextFg),
		uistate.T("help.membersStepLabel"),
	)

	steps := []step{
		{currencyLink, currencySet},
		{Span(css.Class("t-body", tw.TextDim), "Add an account"), len(app.Accounts()) > 0},
		{Span(css.Class("t-body", tw.TextDim), "Record a transaction"), len(app.Transactions()) > 0},
		{Span(css.Class("t-body", tw.TextDim), "Set a budget"), len(app.Budgets()) > 0},
		{Span(css.Class("t-body", tw.TextDim), "Set a savings goal"), len(app.Goals()) > 0},
		{membersLink, len(app.Members()) >= 2},
	}
	rows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2)}
	allDone := true
	for _, s := range steps {
		mark, tone := "○", "text-faint"
		if s.done {
			mark, tone = "✓", "text-up"
		} else {
			allDone = false
		}
		rows = append(rows, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(ClassStr("t-body "+tw.ColorClass(tone)), mark),
			s.label))
	}
	lead := "A few steps to get the most out of CashFlux:"
	if allDone {
		lead = "You're all set up — nice work. ✓"
	}
	body := append([]any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
		P(css.Class("t-body", tw.TextDim), lead)}, Div(rows...))
	return Div(body...)
}

// whatsNewBody surfaces a plain-English description of what CashFlux is, the
// local-first privacy commitment, the current version, recent highlights, and a
// link to the full changelog (C293 / R34-whatsnew).
func whatsNewBody() ui.Node {
	bullets := []string{
		"Financial-health score — a 0–100 read of your overall position with next steps.",
		"Friendlier dates everywhere, and clearer light-mode contrast.",
		"Installable app (PWA) with an on-brand icon and offline support.",
		"A privacy line in the sidebar — your data stays on this device.",
	}
	body := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2)}
	// Version pill sits at the top, directly under the tile title, where it reads as
	// a label for the section rather than floating mid-paragraph.
	body = append(body, Div(
		Span(Attr("style", "display:inline-block;padding:.12rem .5rem;border:1px solid var(--border);border-radius:999px;font-size:.78rem;color:var(--text-dim)"),
			version.Label())))
	body = append(body, P(css.Class("t-body", tw.TextDim), uistate.T("help.aboutTagline")))
	body = append(body, P(css.Class("t-body", tw.TextDim), uistate.T("help.aboutPrivacy")))
	for _, b := range bullets {
		body = append(body, P(css.Class("t-body", tw.TextDim), "• "+b))
	}
	body = append(body, P(css.Class("t-caption", tw.Mt1),
		A(Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/CHANGELOG.md"),
			Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			css.Class(tw.Underline, tw.HoverTextFg), "See the full changelog →")))
	return Div(body...)
}

// supportBody (C325) renders the "Support & feedback" section: a short invite
// line and two GitHub links (bug report + feature request) so users always have
// a reachable path to the project without leaving the app.
func supportBody() ui.Node {
	body := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2)}
	body = append(body, P(css.Class("t-body", tw.TextDim), uistate.T("help.supportInvite")))
	body = append(body,
		A(Attr("href", "https://github.com/monstercameron/CashFlux/issues/new"),
			Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			Attr("title", uistate.T("help.supportBugTitle")),
			css.Class(tw.Underline, tw.HoverTextFg),
			uistate.T("help.supportBugLabel")+" →"))
	body = append(body,
		A(Attr("href", "https://github.com/monstercameron/CashFlux/issues"),
			Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			Attr("title", uistate.T("help.supportFeatureTitle")),
			css.Class(tw.Underline, tw.HoverTextFg),
			uistate.T("help.supportFeatureLabel")+" →"))
	return Div(body...)
}

// helpTopicBody renders one help card's body: one or more plain-English lines.
func helpTopicBody(lines ...string) ui.Node {
	body := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2)}
	for _, l := range lines {
		body = append(body, P(css.Class("t-body", tw.TextDim), l))
	}
	return Div(body...)
}

// About is the dedicated /about route (C290 / C293). It delegates to
// AboutScreen (internal/screens/about.go).
func About() ui.Node {
	return AboutScreen()
}

// helpSections returns the ordered, widgetized help content with standard bento
// sizes. Every tile is 2 columns wide (two per row) so the grid stays a balanced
// pair-per-row layout; heights follow content.
func helpSections() []helpSection {
	return []helpSection{
		{"help-setup", "Getting set up", 2, setupChecklistBody()},
		{"help-whatsnew", "What's new", 2, whatsNewBody()},
		{"help-start", "Getting started", 2, helpTopicBody(
			"Add an account (Accounts → Add account), then record what you spend and earn from the + button or the dashboard's Add transaction.",
			"Set a budget per category in Budgets, and track savings targets in Goals — the dashboard rolls it all up.")},
		{"help-data", "Bringing in your data", 2, helpTopicBody(
			"Import a bank CSV from Documents → import; CashFlux maps the columns and flags duplicates before anything is saved.",
			"Most banks let you export a CSV from your transactions or statements page — look for an Export or Download option.")},
		{"help-reports", "Budgets, goals & reports", 2, helpTopicBody(
			"Budgets show what's left for the period; Goals show pace toward a target. Reports breaks down spending by category, payee, and member, with trends over time.",
			"Financial health (in Plan & analyze) scores your overall position and suggests the next step.")},
		{"help-smart", "The Smart layer", 2, helpTopicBody(
			"Smart surfaces optional, opt-in insights and recommendations. Free insights run entirely on your device at no cost; AI features are clearly labelled and only run when you add your own key.",
			"Turn features on or off in Smart → Manage, and dial how much they surface in Appearance.")},
		{"help-shortcuts", "Keyboard shortcuts", 2, helpTopicBody(
			"Press ? anytime to see the full shortcut list. Ctrl/⌘ K opens the command palette to jump anywhere or run an action.",
			"Alt + 1–9 jumps between the main sections; Alt + N adds a transaction.")},
		{"help-privacy", "Your privacy", 2, helpTopicBody(
			"CashFlux is local-first: your financial data is stored on this device and is never uploaded or shared. You can export a backup at any time from Settings.",
			"An optional passcode lock (Settings) keeps the app's screens behind a code and can encrypt your data at rest.")},
		{"help-support", "Support & feedback", 2, supportBody()},
		{"help-offline", "Works offline", 2, P(css.Class("t-body", tw.TextDim),
			"Everything here works offline — CashFlux runs entirely in your browser.")},
	}
}

// helpTile renders one section as a locked bento tile: the same .w/.wh/.wbody
// chrome as a dashboard widget, minus the drag grip and settings gear (this is a
// non-custom, movement-locked surface — unified spec §7.4). Tiles span standard
// column widths but size to their CONTENT height (the help grid uses auto rows),
// so there's no dead space below short copy — the §7.5 lesson that text has
// intrinsic height. Background uses the theme card token and a subtle shadow
// (§7.7) so every tile reads as a distinct card, consistently — no one-off accent.
func helpTile(s helpSection) ui.Node {
	style := "grid-column:span " + strconv.Itoa(s.col) +
		";background:var(--bg-card);box-shadow:0 1px 3px rgba(0,0,0,.35)"
	return Div(css.Class("w"),
		Attr("data-widget", s.id),
		Attr("data-testid", "help-tile"),
		Attr("style", style),
		// Left-aligned title (override the bento's centered .wh h2) at a larger size
		// for clear title↔body hierarchy — this is informational prose, not a KPI.
		Div(css.Class("wh"), H2(Attr("style", "text-align:left;flex:1;font-size:1.15rem"), s.title)),
		// Cap the reading measure (~62ch) so long help lines stay legible.
		Div(ClassStr("wbody"), Attr("style", "max-width:62ch"), s.body),
	)
}

// HelpScreen is the in-app help center (/help, R34), widgetized onto the app's
// bento grid: each section is a locked tile at a standard column width, sized to
// its content height. The page header (title) comes from the screen registry via
// the shell, so the page format matches every other screen. The grid overrides
// the dashboard's fixed row tracks with auto rows so text tiles fit their copy.
func HelpScreen() ui.Node {
	sections := helpSections()
	tiles := make([]any, 0, len(sections)+2)
	tiles = append(tiles, css.Class("bento no-touch-chrome"))
	// Content-height rows: drop the fixed 152px row tracks the dashboard uses;
	// each text tile takes exactly the height its copy needs (paired tiles in a
	// row equalize via grid stretch), eliminating the half-empty-box look.
	tiles = append(tiles, Attr("style", "grid-template-rows:none;grid-auto-rows:auto"))
	for _, s := range sections {
		tiles = append(tiles, helpTile(s))
	}
	return Div(tiles...)
}
