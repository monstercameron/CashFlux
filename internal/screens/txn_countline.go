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
	// Unconverted is how many of the Shown rows could not be folded into the base
	// currency (no FX rate), and so are missing from Net. The count and the net are
	// printed side by side, so a net that quietly covers fewer rows than the count
	// is two different facts wearing one sentence — it is stated instead.
	Unconverted int
	// C681: Net covers CASH FLOW only. Transfer legs are money you still own, so
	// summing them into a figure labelled "net" describes moving money as earning
	// or spending it — and a filter showing one leg of a pair makes that reading
	// unavoidable. Moved is their total magnitude, MovedRows how many there are,
	// and UnpairedRows how many have no counterpart IN THIS VIEW (a claim about
	// the view, not about the data: the far side may sit outside the filter).
	Moved        money.Money
	MovedRows    int
	UnpairedRows int
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
		//
		// A hidden PLACEHOLDER, not an empty Fragment: the reconciler has no element
		// anchor for a fiber whose previous output was empty, so a component that
		// starts empty and fills in later is appended to the end of its container.
		// A brand-new ledger starts at zero and gains its first row minutes later —
		// exactly the sequence that would strand this line at the bottom of the
		// toolbar tile. (The return crumb hit this for real; this is the same trap
		// one component over.)
		return Span(css.Class("txn-countline-slot"), Attr("aria-hidden", "true"),
			Style(map[string]string{"display": "none"}))
	case props.Shown == 0:
		text = uistate.T("transactions.noMatch")
	case props.Shown < props.Total:
		text = uistate.T("transactions.scopeNarrowed", props.Shown, plural(props.Total, "transaction"))
	default:
		text = uistate.T("transactions.scopeAll", plural(props.Total, "transaction"))
	}
	parts := []ui.Node{Span(css.Class("txn-count-main"), text)}
	if props.Shown > 0 {
		// C681: only say "net" about rows that actually are cash flow. When the view
		// holds nothing BUT transfer legs, a net of $0.00 is true and useless — it
		// invites the reader to conclude the money vanished — so the figure is
		// dropped entirely and the moved-money sentence carries the view instead.
		if props.MovedRows == 0 || props.Shown > props.MovedRows {
			parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
				Span(css.Class("txn-count-net"), uistate.T("transactions.scopeNet", fmtMoney(props.Net))))
		}
	}
	if props.MovedRows > 0 {
		// Money that changed accounts without entering or leaving the household.
		// Said separately from the net rather than folded into it: a transfer leg
		// summed into a cash-flow figure describes moving money as spending it.
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-moved"), Attr("data-testid", "txn-count-moved"),
				uistate.T("transactions.scopeMoved", fmtMoney(props.Moved))))
		if props.UnpairedRows > 0 {
			// The far side is not in view. It may exist outside the filter, so this
			// says the view is one-sided rather than claiming the data is broken.
			parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
				Span(css.Class("txn-count-unpaired"), Attr("data-testid", "txn-count-unpaired"),
					uistate.T("transactions.scopeUnpaired", props.UnpairedRows)))
		}
	}
	if props.Unconverted > 0 {
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-unconverted"),
				uistate.T("transactions.scopeUnconverted", props.Unconverted)))
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
