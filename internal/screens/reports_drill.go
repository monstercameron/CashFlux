// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"

	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// rptaDrillProps configures a report drill-through to the transaction list.
//
// From/To are the report's own window and are REQUIRED in practice: a drill that
// carries no dates lands on whatever filter the ledger was last left with, which
// is the defect this component exists to fix (C386). The reader clicks "view
// transactions" from a January-to-August report and reads last month's rows,
// with nothing on either screen saying they do not match.
type rptaDrillProps struct {
	// Label is the link text.
	Label string
	// TestID is the data-testid, kept stable across the conversion so existing
	// selectors keep working.
	TestID string
	// From / To bound the drill to the report window (To exclusive — it is
	// converted to the ledger's inclusive last day here, in one place).
	From, To time.Time
	// The optional narrowings. Empty fields are simply not applied, so the same
	// component serves a section-level "everything in this window" link and a
	// row-level "just this payee" one.
	Category string
	Member   string
	Text     string
	Tag      string
	// IncomeOnly / ExpenseOnly narrow by direction, for the sections that are
	// explicitly about one side of the ledger.
	IncomeOnly  bool
	ExpenseOnly bool
	// Class overrides the link class; empty uses the section-link style.
	Class string
}

// rptaDrill is a report link that carries the report's window (and any extra
// narrowing) into the transaction list before navigating (C386).
//
// It is a button rather than an anchor because it has to WRITE the filter first.
// An anchor to /transactions cannot express "these rows, in this window" — which
// is exactly why the six section links it replaces were, in effect, decoration:
// they moved the reader to a screen showing a different set of transactions from
// the one they had just clicked on.
func rptaDrill(props rptaDrillProps) ui.Node {
	nav := router.UseNavigate()
	filterAtom := uistate.UseTxFilter()

	go_ := ui.UseEvent(Prevent(func() {
		f := uistate.TxFilter{
			// The report's To is exclusive; the ledger's is inclusive. Passing the
			// exclusive boundary through unchanged would pull in the first day of
			// the following month and make every drilled total disagree with the
			// figure that was clicked.
			From:     props.From.Format(dateutil.Layout),
			To:       props.To.AddDate(0, 0, -1).Format(dateutil.Layout),
			Category: props.Category,
			Member:   props.Member,
			Text:     props.Text,
			Tag:      props.Tag,
		}
		if props.IncomeOnly {
			f.Flow = "in"
		}
		if props.ExpenseOnly {
			f.Flow = "out"
		}
		f = f.Normalize()
		filterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}))

	cls := props.Class
	if cls == "" {
		cls = "rpta-drill"
	}
	return Button(ClassStr(cls), Type("button"), Attr("data-testid", props.TestID),
		// The window is in the tooltip, not only in the filter: a reader deciding
		// whether to click deserves to know what they will land on.
		Title(uistate.T("rpta.drillTitle", props.From.Format("Jan 2006"),
			props.To.AddDate(0, 0, -1).Format("Jan 2006"))),
		OnClick(go_), props.Label)
}
