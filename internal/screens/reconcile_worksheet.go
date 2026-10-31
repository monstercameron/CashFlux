// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/worksheet"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// reconcileWorksheet shows the period as the arithmetic it is (C690):
//
//	opening + money in − money out + transfers + checkpoints = closing
//
// The reconcile dialog already states a difference. What it could not say is
// WHICH term the difference is in — and "you are $8,441.80 out" is not a lead,
// it is a dead end. Every figure here exists elsewhere in the app; the worksheet
// is the only place they appear on one line, next to the total they are supposed
// to make.
//
// The period runs from the last reconciliation to the statement date being
// entered, because that is the span this statement is actually responsible for.
// Before the first reconciliation it runs from the beginning of the account.
func reconcileWorksheet(app *appstate.App, a domain.Account, statementDate string, statedMinor int64, hasStatement bool) ui.Node {
	if app == nil {
		return Fragment()
	}
	end, err := parseWorksheetDate(statementDate)
	if err {
		// No usable statement date yet — the period has no end, so there is no
		// arithmetic to show. Silence beats a worksheet of zeroes.
		return Fragment()
	}
	start := time.Time{}
	if through, ok := reconcile.Through(a.Reconciliations); ok {
		start = through.AddDate(0, 0, 1) // the day after the last statement closed
	}

	r := worksheet.Build([]domain.Account{a}, app.Transactions(), start, end)
	if len(r.Lines) == 0 {
		return Fragment()
	}
	l := r.Lines[0]
	// The statement being TYPED is not in the account's history yet — it lands
	// there only when Record is pressed — so the figure comes from the dialog
	// rather than from the ledger. Without this the residual could never appear
	// during a reconciliation, only after one (C690, found by adversarial review).
	if hasStatement {
		l = l.AgainstStatement(statedMinor)
	}
	dec := currency.Decimals(a.Currency)
	amt := func(v int64) string { return money.FormatMinor(v, dec) }

	facts := []uiw.KeyValueFact{
		{Label: uistate.T("worksheet.opening"), Value: Fragment(amt(l.Opening))},
		{Label: uistate.T("worksheet.moneyIn"), Value: Fragment(amt(l.In))},
		{Label: uistate.T("worksheet.moneyOut"), Value: Fragment(amt(-l.Out))},
	}
	// Terms that are zero are left out. A worksheet listing every term the app
	// knows about, most of them empty, buries the two that matter.
	if l.Transfers != 0 {
		facts = append(facts, uiw.KeyValueFact{Label: uistate.T("worksheet.transfers"), Value: Fragment(amt(l.Transfers))})
	}
	if l.Checkpoints != 0 {
		facts = append(facts, uiw.KeyValueFact{Label: uistate.T("worksheet.checkpoints"), Value: Fragment(amt(l.Checkpoints))})
	}
	facts = append(facts, uiw.KeyValueFact{Label: uistate.T("worksheet.computed"), Value: Fragment(amt(l.Computed)), Tone: "acct-ws-total"})

	// The gap, said in a direction rather than as a signed number. Which way it
	// runs is the first thing a person needs in order to start looking, and it is
	// the thing a minus sign in front of a currency amount conveys worst.
	var residual ui.Node = Fragment()
	if l.HasStatement && l.Residual != 0 {
		key := "worksheet.residualHigher"
		gap := l.Residual
		if gap < 0 {
			key, gap = "worksheet.residualLower", -gap
		}
		residual = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "worksheet-residual"),
			uistate.T(key, amt(gap)))
	}

	return uiw.Card(uiw.CardProps{
		TestID:     "reconcile-worksheet",
		ClassParts: []any{tw.P3, tw.FlexCol, tw.Gap1},
		Header:     P(css.Class("t-caption"), uistate.T("worksheet.heading")),
		Body:       Fragment(uiw.KeyValue(uiw.KeyValueProps{Facts: facts}), residual),
	})
}

// parseWorksheetDate turns the dialog's statement-date field into the period's
// exclusive end. It returns bad=true rather than an error because the only
// caller's response to any failure is the same: show nothing.
func parseWorksheetDate(s string) (end time.Time, bad bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, true
	}
	// The statement's closing DAY is included in the period it closes.
	return t.AddDate(0, 0, 1), false
}
