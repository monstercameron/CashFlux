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

// txnSummaryHead is the transactions summary, built on the shared PageHeadline the way
// the Budgets and Accounts headers are. The period's net leads, and the count line
// with its figures and category breakdown sits underneath as the footer.
func txnSummaryHead(props txnToolbarRenderProps) ui.Node {
	net := money.New(props.Derived.View.Net, props.Base)
	return uiw.PageHeadline(uiw.PageHeadlineProps{
		TestID:   "txn-summary",
		Figure:   uiw.PageHeadlineFigure(uiw.PageHeadlineFigureProps{Text: fmtMoney(net), TestID: "txn-summary-net"}),
		Sentence: Span(uistate.T("txnsum.sentence", txnCountPeriod(props.Filter))),
		Footer:   txnToolbarFoot(ui.CreateElement(txnToolbarCount, props)),
	})
}
