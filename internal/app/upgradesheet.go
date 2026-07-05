// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

const upgradeSheetAtomID = "cloud:upgrade-sheet"

var (
	capturedUpgrade     state.Atom[bool]
	upgradeSheetCapture bool
)

// ShowUpgradeSheet opens the contextual Cloud upgrade sheet from anywhere (e.g. a
// Cloud-only action a free user just tapped). No-op until the sheet has rendered
// once. Non-blocking: it never gates the local action that triggered it (§7.11).
func ShowUpgradeSheet() {
	if upgradeSheetCapture {
		capturedUpgrade.Set(true)
	}
}

// UpgradeSheet is a calm, non-blocking bottom sheet that pitches CashFlux Cloud
// (sync + backup + AI) when a free user reaches a Cloud-only action: benefits +
// annual-first price + "Start free trial" (opens Cloud settings → Stripe Checkout)
// + "Maybe later". It never blocks local features. Own component so the open-atom
// + button hooks stay at a stable render position.
func UpgradeSheet() uic.Node {
	open := state.UseAtom(upgradeSheetAtomID, false)
	capturedUpgrade = open
	upgradeSheetCapture = true

	if !open.Get() {
		return Fragment()
	}

	close := uic.UseEvent(func() { open.Set(false) })
	startTrial := uic.UseEvent(func() {
		open.Set(false)
		uistate.OpenGlobalSettingsAt("cloud") // the Cloud tab hosts the trial → Stripe Checkout flow
	})
	viewPlans := uic.UseEvent(func() { open.Set(false) })

	price := uistate.T("settings.cloudPriceAnnual")
	return Div(css.Class("upsheet-backdrop"), Attr("role", "dialog"), Attr("aria-modal", "false"),
		Attr("aria-label", uistate.T("cloud.upgradeTitle")), OnClick(close),
		Div(css.Class("upsheet"), OnClick(Stop(func() {})),
			P(css.Class("upsheet-title"), uistate.T("cloud.upgradeTitle")),
			Ul(css.Class("upsheet-benefits", tw.Text13, tw.TextDim),
				Li(uistate.T("cloud.benefitSync")),
				Li(uistate.T("cloud.benefitBackup")),
				Li(uistate.T("cloud.benefitAI")),
			),
			P(css.Class("upsheet-price", tw.FontSemibold), price),
			// C303: spell out the free-vs-paid boundary + the 14-day trial in plain language,
			// right next to the price, so the pitch isn't ambiguous about what costs money.
			P(css.Class("upsheet-boundary", tw.Text12, tw.TextDim), uistate.T("cloud.upgradeBoundary", price)),
			P(css.Class("upsheet-trust", tw.Text12, tw.TextFaint), uistate.T("cloud.upgradeTrust")),
			// Name the self-host path too, so onboarding mentions both once (§3.4): the
			// managed Cloud above, or run your own server with the same app — no lock-in.
			P(css.Class("upsheet-selfhost", tw.Text12, tw.TextFaint), uistate.T("cloud.upgradeSelfHost")),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(startTrial), uistate.T("cloud.upgradeStart")),
				Button(css.Class("btn"), Type("button"), OnClick(close), uistate.T("cloud.upgradeLater")),
			),
			// R31-reengage: persistent link to the Plans comparison surface so users can
			// always return to pricing details without opening the sheet again (C301).
			A(css.Class("upsheet-plans-link", tw.Text12, tw.TextDim, tw.Underline),
				Attr("href", uistate.RoutePath("/plans")), OnClick(viewPlans),
				uistate.T("plans.backToPlans"),
			),
		),
	)
}
