// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// txnSummaryHead is the transactions summary, built on the shared PageHeadline the way
// the Budgets and Accounts headers are. The period's net leads, and the count line sits
// underneath as the footer. The two actions are pinned to the top-right corner of the
// bubble, so the figure starts at the top-left with no empty row above it.
func txnSummaryHead(props txnToolbarRenderProps) ui.Node {
	net := money.New(props.Derived.View.Net, props.Base)
	return Div(css.Class("txn-summary-frame"),
		txnTopActions(props),
		uiw.PageHeadline(uiw.PageHeadlineProps{
			TestID:   "txn-summary",
			Figure:   uiw.PageHeadlineFigure(uiw.PageHeadlineFigureProps{Text: fmtMoney(net), TestID: "txn-summary-net"}),
			Sentence: Span(uistate.T("txnsum.sentence", txnCountPeriod(props.Filter))),
			Footer:   txnToolbarFoot(ui.CreateElement(txnToolbarCount, props)),
		}),
	)
}
