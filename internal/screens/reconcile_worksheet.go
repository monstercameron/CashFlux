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
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/worksheet"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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

	rows := []ui.Node{
		worksheetRow(uistate.T("worksheet.opening"), amt(l.Opening), false),
		worksheetRow(uistate.T("worksheet.moneyIn"), amt(l.In), false),
		worksheetRow(uistate.T("worksheet.moneyOut"), amt(-l.Out), false),
	}
	// Terms that are zero are left out. A worksheet listing every term the app
	// knows about, most of them empty, buries the two that matter.
	if l.Transfers != 0 {
		rows = append(rows, worksheetRow(uistate.T("worksheet.transfers"), amt(l.Transfers), false))
	}
	if l.Checkpoints != 0 {
		rows = append(rows, worksheetRow(uistate.T("worksheet.checkpoints"), amt(l.Checkpoints), false))
	}
	rows = append(rows, worksheetRow(uistate.T("worksheet.computed"), amt(l.Computed), true))

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

	return Div(css.Class("card", tw.P3, tw.FlexCol, tw.Gap1), Attr("data-testid", "reconcile-worksheet"),
		P(css.Class("t-caption"), uistate.T("worksheet.heading")),
		Div(css.Class(tw.FlexCol, tw.Gap05), rows),
		residual,
	)
}

// worksheetRow is one term: its name on the left, its amount on the right.
func worksheetRow(label, amount string, total bool) ui.Node {
	cls := "t-caption"
	if total {
		cls = "t-body"
	}
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
		Span(ClassStr(cls), css.Class(tw.Flex1), label),
		Span(ClassStr(cls+" fig"), amount),
	)
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
