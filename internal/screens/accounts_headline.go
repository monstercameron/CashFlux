// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// acctHeadlineProps carries the figures and parts of the accounts page summary.
type acctHeadlineProps struct {
	// NetText, AssetsText and LiabilitiesText are the formatted figures. NetTone is
	// the colour class of the net-worth figure.
	NetText, AssetsText, LiabilitiesText, NetTone string
	// Delta is the month-to-date change in net worth, shown only when HaveDelta is true.
	Delta     money.Money
	HaveDelta bool
	// NetTip is the explainer beside the net-worth label, Visual the trend picture
	// and Footer the quiet line under the facts.
	NetTip, Visual, Footer ui.Node
}

// acctHeadline is the accounts page summary, built on the shared PageHeadline. Net
// worth leads, with its change this month. Assets and liabilities follow as facts,
// and the liabilities figure links to /debt.
func acctHeadline(props acctHeadlineProps) ui.Node {
	return uiw.PageHeadline(uiw.PageHeadlineProps{
		Visual: props.Visual,
		Footer: props.Footer,
		Figure: uiw.PageHeadlineFigure(uiw.PageHeadlineFigureProps{Text: props.NetText, Tone: props.NetTone, TestID: "acct-networth-value", Block: true}),
		Sentence: Fragment(
			uistate.T("dashboard.netWorth"),
			props.NetTip,
			netWorthDeltaLine(props.Delta, props.HaveDelta),
		),
		Facts: []ui.Node{
			uiw.PageHeadlineFact(uiw.PageHeadlineFactProps{Label: uistate.T("accounts.assets"), Value: props.AssetsText, TestID: "acct-assets-value"}),
			uiw.PageHeadlineFact(uiw.PageHeadlineFactProps{Label: uistate.T("dashboard.liabilities"), Value: props.LiabilitiesText, Tone: tw.ColorClass("text-down"),
				Href: uistate.RoutePath("/debt"), TestID: "acct-liabilities-value"}),
		},
	})
}
