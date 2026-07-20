// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgetSmartFeaturesLink renders the budgets list header's Smart affordance as a
// clear NAVIGATION control (C397): the reviewer found the bare "Smart" button too
// vague and was surprised it navigated away. It now reads "Smart features →" with a
// tooltip naming the destination, so the jump to the /smart page is expected. It
// preserves the same density gate the shared section-action used, so a user who has
// dialed smart affordances down still sees nothing.
func budgetSmartFeaturesLink(settings smart.Settings) ui.Node {
	if !settings.DensityOrDefault().Shows(smart.AffordanceSectionAction) {
		return Fragment()
	}
	return ui.CreateElement(budgetSmartLinkBtn, struct{}{})
}

// budgetSmartLinkBtn owns the navigate hook — its own component so UseEvent sits at a
// stable render position.
func budgetSmartLinkBtn(_ struct{}) ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/smart")) })
	return Button(css.Class("btn btn-sm btn-ghost budget-smart-link"), Type("button"),
		Attr("data-testid", "budget-smart-features-link"),
		Attr("aria-label", uistate.T("budgets.smartNavAria")),
		Title(uistate.T("budgets.smartNavTooltip")),
		OnClick(open),
		smartGlyph(false, tw.Fold(tw.W4, tw.H4)),
		Span(ClassStr(tw.Fold(tw.Ml1)), uistate.T("budgets.smartNavLabel")),
	)
}
