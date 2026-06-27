// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// setupChecklist (R34-onboard) shows the first-run steps with a live ✓/○ from the
// actual data, so a new household sees what's left to set up and a returning one
// sees it's all done. Reads counts only — no mutation, safe to render anywhere.
func setupChecklist() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	type step struct {
		label string
		done  bool
	}
	steps := []step{
		{"Add an account", len(app.Accounts()) > 0},
		{"Record a transaction", len(app.Transactions()) > 0},
		{"Set a budget", len(app.Budgets()) > 0},
		{"Set a savings goal", len(app.Goals()) > 0},
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
			Span(css.Class("t-body", tw.TextDim), s.label)))
	}
	lead := "A few steps to get the most out of CashFlux:"
	if allDone {
		lead = "You're all set up — nice work. ✓"
	}
	body := append([]any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
		P(css.Class("t-body", tw.TextDim), lead)}, Div(rows...))
	return uiw.Card(uiw.CardProps{Title: "Getting set up", Body: Div(body...)})
}

// whatsNewCard surfaces a plain-English description of what CashFlux is, the
// local-first privacy commitment, the current version, recent highlights, and
// a link to the full changelog (C293 / R34-whatsnew). A discoverable surface
// rather than an auto-popping sheet — calmer, and works offline.
func whatsNewCard() ui.Node {
	bullets := []string{
		"Financial-health score — a 0–100 read of your overall position with next steps.",
		"Friendlier dates everywhere, and clearer light-mode contrast.",
		"Installable app (PWA) with an on-brand icon and offline support.",
		"A privacy line in the sidebar — your data stays on this device.",
	}
	body := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2)}
	// C293: plain-English identity + privacy commitment so the card explains what
	// CashFlux is and why it is safe to use — not just what changed.
	body = append(body, P(css.Class("t-body", tw.TextDim), uistate.T("help.aboutTagline")))
	body = append(body, P(css.Class("t-body", tw.TextDim), uistate.T("help.aboutPrivacy")))
	body = append(body, P(css.Class("t-caption", tw.TextFaint), "Version "+version.Label()))
	for _, b := range bullets {
		body = append(body, P(css.Class("t-body", tw.TextDim), "• "+b))
	}
	body = append(body, P(css.Class("t-caption", tw.Mt1),
		A(Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/CHANGELOG.md"),
			Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			css.Class(tw.Underline, tw.HoverTextFg), "See the full changelog →")))
	return uiw.Card(uiw.CardProps{Title: "What's new", Body: Div(body...)})
}

// supportCard (C325) renders the "Support & feedback" section: a short invite
// line and two GitHub links (bug report + feature request) so users always have
// a reachable path to the project without leaving the app.
func supportCard() ui.Node {
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
	return uiw.Card(uiw.CardProps{Title: uistate.T("help.supportTitle"), Body: Div(body...)})
}

// helpTopic renders one help card: a title and one or more plain-English lines.
func helpTopic(title string, lines ...string) ui.Node {
	body := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2)}
	for _, l := range lines {
		body = append(body, P(css.Class("t-body", tw.TextDim), l))
	}
	return uiw.Card(uiw.CardProps{Title: title, Body: Div(body...)})
}

// About is the dedicated /about route (C290): renders the full Help/About
// screen — same content as HelpScreen — and scrolls the browser to the
// "about" anchor so the What's-new / privacy cards are immediately visible.
// Using the existing HelpScreen avoids duplication: one source of truth for
// the content, two routable entry points (/help and /about).
func About() ui.Node {
	return HelpScreen()
}

// HelpScreen is the in-app help center (/help, R34): short plain-English topics
// covering the everyday flows, the optional Smart layer, the privacy model, and
// the keyboard shortcuts — all on-device, nothing fetched.
func HelpScreen() ui.Node {
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5),
		setupChecklist(),
		whatsNewCard(),
		supportCard(),
		helpTopic("Getting started",
			"Add an account (Accounts → Add account), then record what you spend and earn from the + button or the dashboard's Add transaction.",
			"Set a budget per category in Budgets, and track savings targets in Goals — the dashboard rolls it all up."),
		helpTopic("Bringing in your data",
			"Import a bank CSV from Documents → import; CashFlux maps the columns and flags duplicates before anything is saved.",
			"Most banks let you export a CSV from your transactions or statements page — look for an Export or Download option."),
		helpTopic("Budgets, goals & reports",
			"Budgets show what's left for the period; Goals show pace toward a target. Reports breaks down spending by category, payee, and member, with trends over time.",
			"Financial health (in Plan & analyze) scores your overall position and suggests the next step."),
		helpTopic("The Smart layer",
			"Smart surfaces optional, opt-in insights and recommendations. Free insights run entirely on your device at no cost; AI features are clearly labelled and only run when you add your own key.",
			"Turn features on or off in Smart → Manage, and dial how much they surface in Appearance."),
		helpTopic("Keyboard shortcuts",
			"Press ? anytime to see the full shortcut list. Ctrl/⌘ K opens the command palette to jump anywhere or run an action.",
			"Alt + 1–9 jumps between the main sections; Alt + N adds a transaction."),
		helpTopic("Your privacy",
			"CashFlux is local-first: your financial data is stored on this device and is never uploaded or shared. You can export a backup at any time from Settings.",
			"An optional passcode lock (Settings) keeps the app's screens behind a code and can encrypt your data at rest."),
		P(css.Class("t-caption", tw.TextFaint),
			"Everything here works offline — CashFlux runs entirely in your browser."),
	)
}
