//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Dashboard shows headline totals and recent activity from the live store.
func Dashboard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	net, _, _, _ := ledger.NetWorth(accounts, txns, rates)
	start, end := dateutil.MonthRange(time.Now())
	income, expense, _ := ledger.PeriodTotals(txns, start, end, rates)

	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}

	recent := recentTransactions(txns, 5)
	var recentBody ui.Node
	if len(recent) == 0 {
		recentBody = P(Class("empty"), "No transactions yet.")
	} else {
		rows := make([]ui.Node, 0, len(recent))
		for _, t := range recent {
			rows = append(rows, Div(Class("row"),
				Div(Class("row-main"),
					Span(Class("row-desc"), t.Desc),
					Span(Class("row-meta"), dateutil.FormatDate(t.Date)),
				),
				Span(Class(amountClass(t.Amount)), fmtMoney(t.Amount)),
			))
		}
		recentBody = Div(Class("rows"), rows)
	}

	return Div(
		Div(Class("stat-grid"),
			stat("Net worth", fmtMoney(net), accentFor(net)),
			stat("This month in", fmtMoney(income), "pos"),
			stat("This month out", fmtMoney(expense), "neg"),
			stat("Accounts", fmt.Sprintf("%d", active), ""),
		),
		Section(Class("card"),
			H2(Class("card-title"), "Recent activity"),
			recentBody,
		),
	)
}

// recentTransactions returns the n most recent transactions, newest first.
func recentTransactions(txns []domain.Transaction, n int) []domain.Transaction {
	cp := append([]domain.Transaction(nil), txns...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Date.After(cp[j].Date) })
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}
