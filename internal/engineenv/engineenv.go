// SPDX-License-Identifier: MIT

// Package engineenv builds the "app engine variable surface": the named numeric
// figures a sandboxed formula can reference (net worth, income, counts, …). It is
// the single source of truth for what variables a custom KPI widget or a workflow
// condition sees, computed purely from the dataset so it unit-tests on native Go
// (no syscall/js). The wasm layer gathers the Data from app state and hands it in.
package engineenv

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Data is everything Vars needs to compute the variable surface. It is the raw
// dataset slices plus the FX rate table and the reference time (so "this month"
// totals and any time-relative figures are deterministic for a given input).
type Data struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Members      []domain.Member
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Tasks        []domain.Task
	Rates        currency.Rates
	Now          time.Time
}

// Names lists the variables Vars produces, in a stable, documented order. The
// binding editor shows these so a user knows what they can reference. Keeping the
// list here (rather than ranging a map) makes the surface explicit and ordered.
var Names = []string{
	"net_worth",
	"assets",
	"liabilities",
	"income",
	"expense",
	"accounts",
	"transactions",
	"members",
	"budgets",
	"goals",
	"tasks",
}

// Vars computes the variable surface from d. Money figures are returned in major
// units of the base currency (e.g. dollars, not cents) so formulas read naturally;
// income/expense are the totals for the calendar month containing d.Now. Counts
// (accounts excludes archived) are plain integers as float64. The result is
// deterministic for a given Data.
func Vars(d Data) map[string]float64 {
	base := d.Rates.Base
	if base == "" {
		base = "USD"
	}

	net, assets, liabilities, _ := ledger.NetWorth(d.Accounts, d.Transactions, d.Rates)
	start, end := dateutil.MonthRange(d.Now)
	income, expense, _ := ledger.PeriodTotals(d.Transactions, start, end, d.Rates)

	div := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		div *= 10
	}
	major := func(m money.Money) float64 { return float64(m.Amount) / div }

	active := 0
	for _, a := range d.Accounts {
		if !a.Archived {
			active++
		}
	}

	return map[string]float64{
		"net_worth":    major(net),
		"assets":       major(assets),
		"liabilities":  major(liabilities),
		"income":       major(income),
		"expense":      major(expense),
		"accounts":     float64(active),
		"transactions": float64(len(d.Transactions)),
		"members":      float64(len(d.Members)),
		"budgets":      float64(len(d.Budgets)),
		"goals":        float64(len(d.Goals)),
		"tasks":        float64(len(d.Tasks)),
	}
}

// SortedNames returns the variable names in alphabetical order — convenient for a
// reference list that prefers A–Z over the documented order in Names.
func SortedNames() []string {
	out := append([]string(nil), Names...)
	sort.Strings(out)
	return out
}
