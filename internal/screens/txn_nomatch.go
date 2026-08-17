// SPDX-License-Identifier: MIT

//go:build js && wasm

// The ledger's no-match state (C628).
//
// "No transactions match your filters." was true and inert: the only way back was
// an unlabelled ✕ in the toolbar the user had to go find, and every other control
// on the page still described a view with nothing in it. The escape belongs where
// the disappointment is.
//
// It clears the SEARCH only. The period, member and category filters are not what
// the user just typed, and a "clear" that silently widened the view to a year
// they were not looking at would be a worse surprise than the empty list.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/debounce"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnNoMatchProps carries the query that found nothing. Empty means the view was
// narrowed by something other than a search, and there is no search to clear.
type txnNoMatchProps struct {
	Search string
}

// txnNoMatch is the empty state for a filtered ledger with no rows. Its own
// component so the clear button's click hook sits at a stable position — it is
// rendered from a switch arm that only sometimes runs.
func txnNoMatch(props txnNoMatchProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	pending := uistate.UseTxnSearchPending()
	clear := ui.UseEvent(func() {
		// Cancel the in-flight debounce FIRST. Without this the pending callback
		// fires ~400ms later and writes the query straight back — the clear would
		// work and then undo itself, which is a worse bug than the one it fixes.
		debounce.Flush("txn-filter:text")
		pending.Set(false)
		setTxFilterOn(filterAtom, func(x *uistate.TxFilter) { x.Text = "" })
	})

	if props.Search == "" {
		return P(css.Class("empty"), Attr("data-testid", "txn-nomatch"),
			Attr("role", "status"), uistate.T("transactions.noMatch"))
	}
	return Div(css.Class("empty", tw.Flex, tw.FlexCol, tw.ItemsCenter, tw.Gap2),
		Attr("data-testid", "txn-nomatch"), Attr("role", "status"),
		P(uistate.T("transactions.noMatchSearch", props.Search)),
		Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "txn-nomatch-clear"),
			OnClick(clear), uistate.T("action.clearSearch")),
		P(css.Class("t-caption", tw.TextDim), uistate.T("transactions.noMatchKeepsFilters")),
	)
}
