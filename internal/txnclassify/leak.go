// SPDX-License-Identifier: MIT

package txnclassify

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// # Saying how big the error is, per currency
//
// Leaked reports what the suspects are adding to income and spending as raw
// minor units, deliberately without converting: this package has no rate table,
// and inventing one would be worse than leaving the caller to convert.
//
// That contract is easy to honour and easy to forget. A caller that adds the
// figures up and stamps the household's currency on the result prints a number
// belonging to no currency at all — which is worse than the silence it replaced,
// because a leak notice that lies is a reason to distrust the next one.
//
// So the split is provided here rather than left as an exercise. A caller that
// asks for totals per currency cannot accidentally produce a cross-currency sum,
// because there is never a moment when it holds one.

// CurrencyLeak is what the suspects in ONE currency are contributing.
type CurrencyLeak struct {
	// Currency is the ISO code these figures are denominated in.
	Currency string
	// Totals is the income/spending/count for that currency alone.
	Totals LeakedTotals
}

// LeakedByCurrency splits the suspects by the currency each row is denominated
// in, so every figure a caller shows is a figure in some real currency.
//
// Sorted by code, so a notice listing several does not reorder itself between
// renders of the same data.
func LeakedByCurrency(suspects []Suspect) []CurrencyLeak {
	byCur := map[string][]Suspect{}
	for _, s := range suspects {
		cur := s.Txn.Amount.Currency
		byCur[cur] = append(byCur[cur], s)
	}
	out := make([]CurrencyLeak, 0, len(byCur))
	for cur, group := range byCur {
		out = append(out, CurrencyLeak{Currency: cur, Totals: Leaked(group)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out
}

// SuspectsIn is Suspects restricted to a date window — the rows a report or a
// budget actually computed from.
//
// A diagnostic has to describe the population beside it. Quoting a household's
// whole history next to a figure for one quarter names an error the reader
// cannot find, and sends them looking for money that is not in the number they
// are looking at.
//
// The window is half-open [start, end), matching every other range in the app.
// A zero start or end is treated as unbounded on that side.
func SuspectsIn(txns []domain.Transaction, categoryName func(id string) string, start, end time.Time) []Suspect {
	out := make([]Suspect, 0)
	for _, s := range Suspects(txns, categoryName) {
		if !start.IsZero() && s.Txn.Date.Before(start) {
			continue
		}
		if !end.IsZero() && !s.Txn.Date.Before(end) {
			continue
		}
		out = append(out, s)
	}
	return out
}
