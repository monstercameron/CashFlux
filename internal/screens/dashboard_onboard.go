// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// onboardDismissKey persists the first-run onboarding card's dismissal so it stays
// gone across reloads once the user closes it (or finishes setup).
const onboardDismissKey = "cashflux:onboard-dismissed"

// onboardStep is one first-run setup step with a live done-flag and a destination.
type onboardStep struct {
	label string
	done  bool
	path  string
}

// dashOnboardCard (C329) is the dismissible first-run onboarding callout at the top
// of the dashboard. It shows a short setup checklist with live ✓/○ from the actual
// data and a "Take the tour" link to the full /help center, so a brand-new household
// has an obvious next step. It hides itself once every step is done OR the user
// dismisses it (persisted), so returning users never see it. Its own component so
// the dismissal-state hook stays at a stable render position.
func dashOnboardCard() ui.Node {
	app := appstate.Default
	if app == nil {
		return nil
	}
	// Persisted dismissal: read once into session state so a click hides it instantly
	// AND it stays hidden on reload.
	dismissed := ui.UseState(browserstore.GetString(onboardDismissKey) == "1")
	nav := router.UseNavigate()

	if dismissed.Get() {
		return nil
	}

	steps := []onboardStep{
		{uistate.T("onboard.addAccount"), len(app.Accounts()) > 0, "/accounts"},
		{uistate.T("onboard.recordTxn"), len(app.Transactions()) > 0, "/transactions"},
		{uistate.T("onboard.setBudget"), len(app.Budgets()) > 0, "/budgets"},
		{uistate.T("onboard.setGoal"), len(app.Goals()) > 0, "/goals"},
	}
	allDone := true
	for _, s := range steps {
		if !s.done {
			allDone = false
			break
		}
	}
	// Once fully set up there's nothing to onboard — don't nag.
	if allDone {
		return nil
	}

	onDismiss := ui.UseEvent(func() {
		browserstore.Set(onboardDismissKey, "1")
		dismissed.Set(true)
	})
	onTour := ui.UseEvent(func() {
		nav.Navigate(uistate.RoutePath("/help"))
	})

	rows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.Mt2)}
	for _, s := range steps {
		mark, tone := "○", "text-faint"
		if s.done {
			mark, tone = "✓", "text-up"
		}
		path := s.path
		rows = append(rows, ui.CreateElement(onboardRow, onboardRowProps{
			Mark: mark, Tone: tone, Label: s.label, Done: s.done,
			OnGo: func() { nav.Navigate(uistate.RoutePath(path)) },
		}))
	}

	return Div(
		css.Class("catchup-card"),
		Attr("role", "complementary"),
		Attr("data-testid", "onboard-card"),
		Attr("aria-label", uistate.T("onboard.title")),
		Div(css.Class("catchup-card-body", tw.Flex, tw.FlexCol),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("catchup-card-icon"), "👋"),
				Div(css.Class("catchup-card-text"),
					Strong(uistate.T("onboard.title")),
					P(uistate.T("onboard.body")),
				),
			),
			Div(rows...),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-primary btn-sm"), Type("button"),
					Attr("data-testid", "onboard-tour"), OnClick(onTour),
					uistate.T("onboard.tour")),
				Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "onboard-dismiss"), OnClick(onDismiss),
					uistate.T("onboard.dismiss")),
			),
		),
	)
}

type onboardRowProps struct {
	Mark  string
	Tone  string
	Label string
	Done  bool
	OnGo  func()
}

// onboardRow is one checklist line as its own component so its click handler hook
// stays at a stable position (the framework rule against On* options inside a loop).
// A done step is plain text; a pending step is a button that jumps to where it's done.
func onboardRow(props onboardRowProps) ui.Node {
	onGo := ui.UseEvent(func() {
		if props.OnGo != nil {
			props.OnGo()
		}
	})
	mark := Span(ClassStr("t-body "+tw.ColorClass(props.Tone)), props.Mark)
	if props.Done {
		return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			mark, Span(css.Class("t-body", tw.TextDim), props.Label))
	}
	return Button(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.TextDim, tw.HoverTextFg),
		Type("button"), OnClick(onGo),
		mark, Span(css.Class("t-body"), props.Label),
		uiw.Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4)),
	)
}
