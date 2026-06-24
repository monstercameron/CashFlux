// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendsummary"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// spendSummaryCardProps carries the data the SpendSummaryCard needs.
type spendSummaryCardProps struct {
	Rows         []extract.Row
	Accounts     []domain.Account
	ImportAcctID string
	BaseCurrency string
}

// SpendSummaryCard renders the monthly-spend preview for the rows awaiting
// import: out vs in vs net per month, so the user can see what a statement
// says they spent before committing any rows.
// Returns nil when there are no rows.
func SpendSummaryCard(props spendSummaryCardProps) ui.Node {
	rows := props.Rows
	if len(rows) == 0 {
		return nil
	}

	cur := props.BaseCurrency
	if cur == "" {
		cur = "USD"
	}
	if acc, ok := domain.AccountByID(props.Accounts, props.ImportAcctID); ok && acc.Currency != "" {
		cur = acc.Currency
	}

	months := spendsummary.Summarize(rows, currency.Decimals(cur))
	sumRows := make([]ui.Node, 0, len(months))
	for _, m := range months {
		label := m.Month
		if label == "" {
			label = uistate.T("documents.summaryUndated")
		}
		sumRows = append(sumRows, Div(css.Class("row"),
			Span(css.Class("row-desc"), label),
			Span(css.Class("muted"), plural(m.Count, "row")),
			Span(css.Class("amount fig"), uistate.T("documents.summaryOutIn",
				fmtMoney(money.New(m.Out, cur)),
				fmtMoney(money.New(m.In, cur)),
				fmtMoney(money.New(m.Net(), cur)),
			)),
		))
	}

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("documents.summaryTitle"),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("documents.summaryDesc")),
			Div(css.Class("rows"), sumRows),
		),
	})
}
