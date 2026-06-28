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

// PayeeSummary extends PayeeTotal with a per-payee transaction count, for
// display contexts (e.g. the top-merchants insights card) that need it.
// Name is resolved from t.Payee first, falling back to t.Desc.
type PayeeSummary struct {
	Name   string
	Amount int64 // minor units, base currency
	Count  int   // number of qualifying transactions
}

// TopPayeesTrailing ranks expense payees by total spend over the trailing days
// ending at asOf, largest first (ties broken alphabetically for determinism).
// Name resolution checks t.Payee first and falls back to t.Desc when Payee is
// blank; payees are not case-folded (each distinct spelling is a separate
// entry). Transactions with no resolvable name are skipped. Transfers and
// income are excluded; amounts are converted to the Rates base currency.
// limit <= 0 returns every payee; otherwise at most the top limit.
func TopPayeesTrailing(txns []domain.Transaction, days int, asOf time.Time, rates currency.Rates, limit int) ([]PayeeSummary, error) {
	cutoff := asOf.AddDate(0, 0, -days)
	type agg struct {
		name  string
		amt   int64
		count int
	}
	groups := map[string]*agg{}
	for _, t := range txns {
		if !t.IsExpense() || t.Date.Before(cutoff) {
			continue
		}
		name := strings.TrimSpace(t.Payee)
		if name == "" {
			name = strings.TrimSpace(t.Desc)
		}
		if name == "" {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		g := groups[name]
		if g == nil {
			g = &agg{name: name}
			groups[name] = g
		}
		g.amt += conv.Abs().Amount
		g.count++
	}

	out := make([]PayeeSummary, 0, len(groups))
	for _, g := range groups {
		out = append(out, PayeeSummary{Name: g.name, Amount: g.amt, Count: g.count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount != out[j].Amount {
			return out[i].Amount > out[j].Amount
		}
		return out[i].Name < out[j].Name
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
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
