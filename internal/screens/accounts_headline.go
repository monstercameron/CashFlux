// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// acctHeadline is the accounts page summary, built on the shared PageHeadline. Net
// worth leads, with its change this month; assets and liabilities follow as facts.
// Liabilities still link through to /debt.
func acctHeadline(netText, assetsText, liabilitiesText, tone string, delta money.Money, haveDelta bool, netTip, visual ui.Node) ui.Node {
	return uiw.PageHeadline(uiw.PageHeadlineProps{
		Visual: visual,
		Figure: uiw.PageHeadlineFigure(uiw.PageHeadlineFigureProps{Text: netText, Tone: tone, TestID: "acct-networth-value", Block: true}),
		Sentence: Fragment(
			uistate.T("dashboard.netWorth"),
			netTip,
			netWorthDeltaLine(delta, haveDelta),
		),
		Facts: []ui.Node{
			uiw.PageHeadlineFact(uiw.PageHeadlineFactProps{Label: uistate.T("accounts.assets"), Value: assetsText, TestID: "acct-assets-value"}),
			uiw.PageHeadlineFact(uiw.PageHeadlineFactProps{Label: uistate.T("dashboard.liabilities"), Value: liabilitiesText, Tone: "text-down",
				Href: uistate.RoutePath("/debt"), TestID: "acct-liabilities-value"}),
		},
		Attention: Fragment(),
	})
}
