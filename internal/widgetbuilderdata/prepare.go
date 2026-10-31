// SPDX-License-Identifier: MIT

// Package widgetbuilderdata prepares the native model exposed by the Visual
// Builder's dataset and net-worth string sources.
package widgetbuilderdata

import (
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/cardgraph"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nwchange"
)

// Snapshot is the immutable, typed input required to prepare Visual Builder
// datasets. At and currency settings are explicit so callers can key any memo
// on every non-entity dependency.
type Snapshot struct {
	At           time.Time
	BaseCurrency string
	FXRates      map[string]float64
	Categories   []domain.Category
	Transactions []domain.Transaction
	Accounts     []domain.Account
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Tasks        []domain.Task
}

// Prepared contains the dataset sources and the month-to-date change used by
// the string-source surface. Callers should treat its maps and collections as
// read-only.
type Prepared struct {
	Datasets       map[string]cardgraph.Collection
	NetWorthChange nwchange.Change
	// NetWorthSeriesErr and NetWorthChangeErr report unavailable derived sources
	// without blocking unrelated transaction, account, budget, goal, or task data.
	NetWorthSeriesErr error
	NetWorthChangeErr error
}

// Prepare converts one coherent typed snapshot into the Builder's published
// dataset schema. It returns calculation errors instead of disguising an
// unavailable net-worth source as an empty collection.
func Prepare(snapshot Snapshot) (Prepared, error) {
	base := strings.TrimSpace(snapshot.BaseCurrency)
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: snapshot.FXRates}
	major := func(m money.Money) float64 {
		divisor := 1.0
		for i := 0; i < currency.Decimals(m.Currency); i++ {
			divisor *= 10
		}
		return math.Abs(float64(m.Amount) / divisor)
	}
	signedMajor := func(m money.Money) float64 {
		divisor := 1.0
		for i := 0; i < currency.Decimals(m.Currency); i++ {
			divisor *= 10
		}
		return float64(m.Amount) / divisor
	}

	datasets := make(map[string]cardgraph.Collection, 7)
	catName := make(map[string]string, len(snapshot.Categories))
	for _, category := range snapshot.Categories {
		catName[category.ID] = category.Name
	}

	txCols := []cardgraph.Column{
		{Name: "category", Type: cardgraph.TypeText}, {Name: "payee", Type: cardgraph.TypeText},
		{Name: "amount", Type: cardgraph.TypeNumber}, {Name: "type", Type: cardgraph.TypeText},
		{Name: "month", Type: cardgraph.TypeText}, {Name: "date", Type: cardgraph.TypeText},
		{Name: "desc", Type: cardgraph.TypeText}, {Name: "signed", Type: cardgraph.TypeNumber},
	}
	txRows := make([]cardgraph.Row, 0, len(snapshot.Transactions))
	billRows := make([]cardgraph.Row, 0)
	for _, transaction := range snapshot.Transactions {
		kind := "transfer"
		signed := signedMajor(transaction.Amount)
		switch {
		case transaction.IsIncome():
			kind = "income"
		case transaction.IsExpense():
			kind = "expense"
			signed = -math.Abs(signed)
			billRows = append(billRows, cardgraph.Row{
				"payee": cardgraph.Text(transaction.Payee), "amount": cardgraph.Num(major(transaction.Amount)),
			})
		}
		category := catName[transaction.CategoryID]
		if strings.TrimSpace(category) == "" {
			category = "Uncategorized"
		}
		desc := strings.TrimSpace(transaction.Payee)
		if desc == "" {
			desc = category
		}
		txRows = append(txRows, cardgraph.Row{
			"category": cardgraph.Text(category), "payee": cardgraph.Text(transaction.Payee),
			"amount": cardgraph.Num(major(transaction.Amount)), "type": cardgraph.Text(kind),
			"month": cardgraph.Text(transaction.Date.Format("2006-01")),
			"date":  cardgraph.Text(transaction.Date.Format("Jan 2")), "desc": cardgraph.Text(desc),
			"signed": cardgraph.Num(signed),
		})
	}
	datasets["transactions"] = cardgraph.Collection{Cols: txCols, Rows: txRows}
	datasets["bills"] = cardgraph.Collection{
		Cols: []cardgraph.Column{{Name: "payee", Type: cardgraph.TypeText}, {Name: "amount", Type: cardgraph.TypeNumber}},
		Rows: billRows,
	}

	accountRows := make([]cardgraph.Row, 0, len(snapshot.Accounts))
	for _, account := range snapshot.Accounts {
		if account.Archived {
			continue
		}
		accountRows = append(accountRows, cardgraph.Row{
			"name": cardgraph.Text(account.Name), "type": cardgraph.Text(string(account.Type)),
		})
	}
	datasets["accounts"] = cardgraph.Collection{
		Cols: []cardgraph.Column{{Name: "name", Type: cardgraph.TypeText}, {Name: "type", Type: cardgraph.TypeText}},
		Rows: accountRows,
	}

	budgetRows := make([]cardgraph.Row, 0, len(snapshot.Budgets))
	for _, budget := range snapshot.Budgets {
		budgetRows = append(budgetRows, cardgraph.Row{
			"name": cardgraph.Text(budget.Name), "limit": cardgraph.Num(major(budget.Limit)),
		})
	}
	datasets["budgets"] = cardgraph.Collection{
		Cols: []cardgraph.Column{{Name: "name", Type: cardgraph.TypeText}, {Name: "limit", Type: cardgraph.TypeNumber}},
		Rows: budgetRows,
	}

	goalRows := make([]cardgraph.Row, 0, len(snapshot.Goals))
	for _, goal := range snapshot.Goals {
		goalRows = append(goalRows, cardgraph.Row{
			"name": cardgraph.Text(goal.Name), "target": cardgraph.Num(major(goal.TargetAmount)),
			"saved": cardgraph.Num(major(goal.CurrentAmount)),
		})
	}
	datasets["goals"] = cardgraph.Collection{
		Cols: []cardgraph.Column{{Name: "name", Type: cardgraph.TypeText}, {Name: "target", Type: cardgraph.TypeNumber}, {Name: "saved", Type: cardgraph.TypeNumber}},
		Rows: goalRows,
	}

	taskRows := make([]cardgraph.Row, 0, len(snapshot.Tasks))
	for _, task := range snapshot.Tasks {
		taskRows = append(taskRows, cardgraph.Row{
			"title": cardgraph.Text(task.Title), "status": cardgraph.Text(string(task.Status)),
		})
	}
	datasets["tasks"] = cardgraph.Collection{
		Cols: []cardgraph.Column{{Name: "title", Type: cardgraph.TypeText}, {Name: "status", Type: cardgraph.TypeText}},
		Rows: taskRows,
	}

	monthStart := dateutil.MonthStart(snapshot.At)
	cutoffs := make([]time.Time, 0, 6)
	labels := make([]string, 0, 6)
	for i := 5; i >= 0; i-- {
		end := monthStart.AddDate(0, -i, 0)
		cutoffs = append(cutoffs, end)
		labels = append(labels, end.AddDate(0, 0, -1).Format("Jan"))
	}
	netWorthSeries, seriesErr := ledger.NetWorthSeries(snapshot.Accounts, snapshot.Transactions, cutoffs, rates)
	if seriesErr != nil {
		netWorthSeries = nil
	}
	netWorthRows := make([]cardgraph.Row, 0, len(netWorthSeries))
	for i, value := range netWorthSeries {
		netWorthRows = append(netWorthRows, cardgraph.Row{
			"month": cardgraph.Text(labels[i]), "value": cardgraph.Num(signedMajor(value)),
		})
	}
	datasets["net_worth_series"] = cardgraph.Collection{
		Cols: []cardgraph.Column{{Name: "month", Type: cardgraph.TypeText}, {Name: "value", Type: cardgraph.TypeNumber}},
		Rows: netWorthRows,
	}

	change, changeErr := nwchange.MonthToDateChange(snapshot.Accounts, snapshot.Transactions, rates, snapshot.At)
	return Prepared{
		Datasets: datasets, NetWorthChange: change,
		NetWorthSeriesErr: seriesErr, NetWorthChangeErr: changeErr,
	}, nil
}
