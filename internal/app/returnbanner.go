// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// returnBannerProps carries the route the shell is currently showing, so the
// breadcrumb can tell "I am on the page this trip was headed for" from "the user
// has moved on".
type returnBannerProps struct {
	ActivePath string
}

// ReturnBanner renders the one-hop "back to what you were doing" crumb (C581).
//
// It appears only on the route a cross-page action navigated TO, names the task it
// returns to, and clears itself the moment the user goes anywhere else — so it is
// a thread back out of a side trip, never a stale bar following you around the app.
//
// The nav rail highlights the destination while you are on it, which is correct
// and is also exactly why this is needed: without it, stepping from the ledger to
// Rules to write one rule reads as having left the transaction task, and the way
// back is the browser's Back button and the hope that the filter survived.
//
// All hooks run before the visibility guard, so hook positions stay stable whether
// or not the crumb is showing.
func ReturnBanner(props returnBannerProps) uic.Node {
	crumbAtom := uistate.UseReturnTo()
	nav := router.UseNavigate()
	crumb := crumbAtom.Get()

	goBack := uic.UseEvent(Prevent(func() {
		c := crumbAtom.Get()
		crumbAtom.Set(uistate.ReturnTo{})
		if c.From != "" {
			nav.Navigate(uistate.RoutePath(c.From))
		}
	}))
	dismiss := uic.UseEvent(Prevent(func() { crumbAtom.Set(uistate.ReturnTo{}) }))

	// Hiding the crumb off-route is not enough — it has to be FORGOTTEN. Hiding
	// alone leaves the trip recorded, so: Transactions → Rules (crumb) → Budgets
	// (hidden) → Rules again for some unrelated reason, and a crumb offering to
	// return to a correction the user finished or abandoned an hour ago reappears.
	// Once they are on neither end of the trip, the trip is over. Cleared in an
	// effect, keyed on the route, because writing state during render re-enters it.
	uic.UseEffect(func() func() {
		c := crumbAtom.Get()
		if c.From != "" && props.ActivePath != c.To && props.ActivePath != c.From {
			crumbAtom.Set(uistate.ReturnTo{})
		}
		return nil
	}, "return-crumb-scope:"+props.ActivePath)

	// Quiet unless there is a trip to return from AND we are on the page it was for.
	// If the user is back at the origin the crumb has served its purpose; if they are
	// somewhere else entirely the side trip is over and a crumb pointing at it would
	// misdescribe what they are doing.
	//
	// The quiet state is a HIDDEN PLACEHOLDER, not an empty Fragment. The reconciler
	// has no element anchor for a fiber whose previous output was empty, so a
	// component that starts empty and fills in later is appended to the end of its
	// container — this crumb rendered 2,500px down the Rules page, below all its
	// content, until it kept a persistent element in its slot.
	if crumb.From == "" || crumb.Label == "" || props.ActivePath != crumb.To {
		return Div(css.Class("return-crumb-slot"), Attr("aria-hidden", "true"),
			Style(map[string]string{"display": "none"}))
	}

	return Div(css.Class("return-crumb"), Attr("data-testid", "return-crumb"),
		Attr("role", "navigation"), Attr("aria-label", uistate.T("returnTo.aria", crumb.Label)),
		Button(css.Class("btn btn-sm return-crumb-back"), Type("button"),
			Attr("data-testid", "return-crumb-back"),
			Attr("aria-label", uistate.T("returnTo.aria", crumb.Label)),
			OnClick(goBack),
			ui.Icon(icon.ChevronLeft, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("returnTo.back", crumb.Label))),
		Span(css.Class("return-crumb-note", tw.TextDim), uistate.T("returnTo.note")),
		Button(css.Class("btn-link return-crumb-x"), Type("button"),
			Attr("data-testid", "return-crumb-dismiss"),
			Attr("aria-label", uistate.T("returnTo.dismiss")),
			Attr("title", uistate.T("returnTo.dismiss")),
			OnClick(dismiss),
			ui.Icon(icon.Close, css.Class(tw.W3, tw.H3))),
	)
}
