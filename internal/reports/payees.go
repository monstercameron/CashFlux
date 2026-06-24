// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// PayeeTotal is one payee's total spend over the reporting period, in base-
// currency minor units. Name is the payee as written (the transaction
// description); payees are grouped case-insensitively.
type PayeeTotal struct {
	Name   string
	Amount int64
}

// TopPayees ranks expense payees by total spend over [start, end), largest
// first (ties broken by name for determinism). Payees are grouped by their
// description, case-insensitively, keeping the first spelling seen for display.
// Transfers and income are excluded and amounts are converted to the base
// currency. n <= 0 returns every payee; otherwise at most the top n. Expenses
// with a blank description are grouped under one "(no description)"-empty key
// and surfaced like any other.
func TopPayees(txns []domain.Transaction, start, end time.Time, rates currency.Rates, n int) ([]PayeeTotal, error) {
	type agg struct {
		name string
		amt  int64
	}
	groups := map[string]*agg{}
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(t.Desc)
		key := strings.ToLower(name)
		g := groups[key]
		if g == nil {
			g = &agg{name: name}
			groups[key] = g
		}
		g.amt += conv.Abs().Amount
	}

	out := make([]PayeeTotal, 0, len(groups))
	for _, g := range groups {
		out = append(out, PayeeTotal{Name: g.name, Amount: g.amt})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].Name < out[j].Name
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}
