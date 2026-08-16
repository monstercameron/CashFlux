// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnCountLineProps carries the numbers the line states and the scope that
// produced them.
type txnCountLineProps struct {
	Shown int // rows after every filter and the member lens
	Total int // rows in the whole ledger
	Net   money.Money
	Lens  string // the lensed member's name, "" for everyone
}

// txnCountLine is the ledger's one-sentence answer to "what am I looking at?".
//
// C575: the page showed a review backlog ("Review inbox (249)"), quick-filter
// counts, and a pager range ("1–25 of 3,227") side by side, each measured against
// a different population, and the only statement that tied a number to its scope
// was SCREEN-READER ONLY — the sighted user got the numbers and none of the
// context. A count without its denominator is not information; "3,227" and "122
// of 3,227" are different facts and the page rendered them the same way.
//
// So the summary is visible, it always names the denominator, and it says whose
// money it is counting. It is also the live region, replacing the hidden one:
// announcing the same sentence twice — once visibly, once invisibly — is how a
// screen-reader user ends up hearing every filter change in duplicate.
func txnCountLine(props txnCountLineProps) ui.Node {
	var text string
	switch {
	case props.Total == 0:
		// The empty state below already explains an empty ledger, and a count line
		// reading "0 of 0" over it is noise on the emptiest possible screen.
		return Fragment()
	case props.Shown == 0:
		text = uistate.T("transactions.noMatch")
	case props.Shown < props.Total:
		text = uistate.T("transactions.scopeNarrowed", props.Shown, plural(props.Total, "transaction"))
	default:
		text = uistate.T("transactions.scopeAll", plural(props.Total, "transaction"))
	}
	parts := []ui.Node{Span(css.Class("txn-count-main"), text)}
	if props.Shown > 0 {
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-net"), uistate.T("transactions.scopeNet", fmtMoney(props.Net))))
	}
	if props.Lens != "" {
		// Stated here as well as on the chip: the chip is a control the eye skips, and
		// this is the sentence that explains the number sitting next to it.
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-lens"), uistate.T("transactions.scopeLens", props.Lens)))
	}
	return Span(css.Class("txn-countline", tw.MinW0),
		Attr("data-testid", "txn-count-line"),
		Attr("role", "status"), Attr("aria-live", "polite"), Attr("aria-atomic", "true"),
		Attr("aria-label", uistate.T("transactions.scopeAria")),
		parts)
}
