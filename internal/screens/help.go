// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// setupSteps evaluates the first-run checklist against live data (R34-onboard /
// C23): each step's label (possibly a link) and whether it's done. Reads counts
// only — no mutation, safe to render anywhere.
type setupStep struct {
	label ui.Node
	done  bool
}

func setupSteps(app *appstate.App, openSettings func()) []setupStep {
	currencyLink := Button(
		Type("button"),
		css.Class("btn-link", tw.Underline, tw.HoverTextFg),
		OnClick(Prevent(func() { openSettings() })),
		uistate.T("help.currencyStepLabel"),
	)
	membersLink := A(
		Attr("href", uistate.RoutePath("/members")),
		css.Class(tw.Underline, tw.HoverTextFg),
		uistate.T("help.membersStepLabel"),
	)
	return []setupStep{
		{currencyLink, app.Settings().BaseCurrency != ""},
		{Span(css.Class("t-body", tw.TextDim), uistate.T("onboard.addAccount")), len(app.Accounts()) > 0},
		{Span(css.Class("t-body", tw.TextDim), uistate.T("onboard.recordTxn")), len(app.Transactions()) > 0},
		{Span(css.Class("t-body", tw.TextDim), uistate.T("onboard.setBudget")), len(app.Budgets()) > 0},
		{Span(css.Class("t-body", tw.TextDim), uistate.T("onboard.setGoal")), len(app.Goals()) > 0},
		{membersLink, len(app.Members()) >= 2},
	}
}

// setupChecklistBody renders the first-run steps with a live ✓/○ read.
func setupChecklistBody(steps []setupStep) ui.Node {
	rows := []any{css.Class(tw.Flex, tw.FlexCol)}
	for _, s := range steps {
		mark, tone := "○", "text-faint"
		if s.done {
			mark, tone = "✓", "text-up"
		}
		rows = append(rows, Div(css.Class("sys-step"),
			Span(ClassStr("sys-step-mark t-body "+tw.ColorClass(tone)), mark),
			s.label))
	}
	return Div(rows...)
}

// whatsNewBody surfaces recent highlights and a link to the full changelog
// (C293 / R34-whatsnew).
func whatsNewBody() ui.Node {
	bullets := []string{
		"Financial-health score — a 0–100 read of your overall position with next steps.",
		"Friendlier dates everywhere, and clearer light-mode contrast.",
		"Installable app (PWA) with an on-brand icon and offline support.",
		"A privacy line in the sidebar — your data stays on this device.",
	}
	body := []any{css.Class("sys-prose")}
	body = append(body, P(css.Class("t-body", tw.TextDim), uistate.T("help.aboutTagline")))
	body = append(body, P(css.Class("t-body", tw.TextDim), uistate.T("help.aboutPrivacy")))
	for _, b := range bullets {
		body = append(body, P(css.Class("t-body", tw.TextDim), "• "+b))
	}
	body = append(body, P(css.Class("t-caption", tw.Mt1),
		A(Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/CHANGELOG.md"),
			Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			css.Class(tw.Underline, tw.HoverTextFg), uistate.T("help.fullChangelog"))))
	return Div(body...)
}

// supportBody (C325) renders the "Support & feedback" section: a short invite
// line and two GitHub links (bug report + feature request) so users always have
// a reachable path to the project without leaving the app.
func supportBody() ui.Node {
	return Div(css.Class("sys-prose"),
		P(css.Class("t-body", tw.TextDim), uistate.T("help.supportInvite")),
		A(Attr("href", "https://github.com/monstercameron/CashFlux/issues/new"),
			Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			Attr("title", uistate.T("help.supportBugTitle")),
			css.Class(tw.Underline, tw.HoverTextFg),
			uistate.T("help.supportBugLabel")+" →"),
		A(Attr("href", "https://github.com/monstercameron/CashFlux/issues"),
			Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			Attr("title", uistate.T("help.supportFeatureTitle")),
			css.Class(tw.Underline, tw.HoverTextFg),
			uistate.T("help.supportFeatureLabel")+" →"),
	)
}

// helpTopicBody renders one help section's body: plain-English lines at a
// readable measure.
func helpTopicBody(lines ...string) ui.Node {
	body := []any{css.Class("sys-prose")}
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

// helpSection is one help topic: a stable id, a serif section title, and body.
type helpSection struct {
	id    string
	title string
	body  ui.Node
}

// helpSections returns the ordered help topics (the setup checklist and
// what's-new render separately beside the hero).
func helpSections() []helpSection {
	return []helpSection{
		{"help-start", "Getting started", helpTopicBody(
			"Add an account (Accounts → Add account), then record what you spend and earn from the + button or the dashboard's Add transaction.",
			"Set a budget per category in Budgets, and track savings targets in Goals — the dashboard rolls it all up.")},
		{"help-data", "Bringing in your data", helpTopicBody(
			"Import a bank CSV from Documents → import; CashFlux maps the columns and flags duplicates before anything is saved.",
			"Most banks let you export a CSV from your transactions or statements page — look for an Export or Download option.")},
		{"help-reports", "Budgets, goals & reports", helpTopicBody(
			"Budgets show what's left for the period; Goals show pace toward a target. Reports breaks down spending by category, payee, and member, with trends over time.",
			"Financial health (in Plan & analyze) scores your overall position and suggests the next step.")},
		{"help-smart", "The Smart layer", helpTopicBody(
			"Smart surfaces optional, opt-in insights and recommendations. Free insights run entirely on your device at no cost; AI features are clearly labelled and only run when you add your own key.",
			"Turn features on or off in Smart → Manage, and dial how much they surface in Appearance.")},
		{"help-shortcuts", "Keyboard shortcuts", helpTopicBody(
			"Press ? anytime to see the full shortcut list. Ctrl/⌘ K opens the command palette to jump anywhere or run an action.",
			"Alt + 1–9 jumps between the main sections; Alt + N adds a transaction.")},
		{"help-privacy", "Your privacy", helpTopicBody(
			"CashFlux is local-first: your financial data is stored on this device and is never uploaded or shared. You can export a backup at any time from Settings.",
			"An optional passcode lock (Settings) keeps the app's screens behind a code and can encrypt your data at rest.")},
		{"help-support", "Support & feedback", supportBody()},
		{"help-offline", "Works offline", helpTopicBody(uistate.T("help.worksOfflineBody"))},
	}
}

// HelpScreen is the in-app help center (/help, R34) in the Understand-surface
// language: a hero that reads your setup progress with a plain-English
// takeaway, the live checklist and what's-new beside it, then the topics as
// serif half-width sections.
func HelpScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	_ = uistate.UseDataRevision().Get() // checklist reflects live data
	openSettings := func() { uistate.OpenGlobalSettings() }

	steps := setupSteps(app, openSettings)
	done := 0
	var nextStep string
	for _, s := range steps {
		if s.done {
			done++
		}
	}
	switch {
	case done == len(steps):
		nextStep = uistate.T("help.takeAllSet")
	default:
		nextStep = uistate.T("help.takeRemaining", len(steps)-done)
	}

	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-help-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), "CashFlux "+version.Label()+" · "+uistate.T("about.chipStorageVal")),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("help.heroLabel")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), fmt.Sprintf("%d of %d", done, len(steps))),
			),
		),
		Div(css.Class("debt-chips"),
			rptChip(uistate.T("help.chipShortcut"), "?", ""),
			rptChip(uistate.T("help.chipPalette"), "Ctrl+K", ""),
			rptChip(uistate.T("help.chipOffline"), uistate.T("help.chipOfflineVal"), rptToneCls("pos")),
		),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "help-takeaway"), nextStep),
	)

	tiles := []any{css.Class("bento bento-sys")}
	tiles = append(tiles, rptTile("help-hero", "1 / span 4", rptSection("", uistate.T("help.heroTitle"), nil, heroBody)))
	tiles = append(tiles,
		rptTile("help-setup", "span 2", Div(Attr("data-testid", "help-tile"),
			rptSection("sec-help-setup", "Getting set up", nil, setupChecklistBody(steps)))),
		rptTile("help-whatsnew", "span 2", Div(Attr("data-testid", "help-tile"),
			rptSection("sec-help-whatsnew", "What's new", nil, whatsNewBody()))),
	)
	for _, s := range helpSections() {
		tiles = append(tiles, rptTile(s.id, "span 2", Div(Attr("data-testid", "help-tile"),
			rptSection("sec-"+s.id, s.title, nil, s.body))))
	}
	return Div(tiles...)
}
