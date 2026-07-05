// SPDX-License-Identifier: MIT

// Package subscriptions detects recurring charges from transaction history — the
// "what am I paying for every month" view (B25). It is a focused, pure read over
// the existing transactions (no new store): it groups identical repeated charges,
// infers a cadence from the spacing between them, and reports each subscription's
// normalized monthly and annual cost plus the next expected renewal date.
//
// Pure Go, no syscall/js; amounts are base-currency minor units (foreign charges
// are converted through the FX table), so totals across subscriptions add up.
package subscriptions

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Cadence is a detected recurrence interval.
type Cadence string

const (
	CadenceWeekly  Cadence = "weekly"
	CadenceMonthly Cadence = "monthly"
	CadenceYearly  Cadence = "yearly"
)

// Subscription is one detected recurring charge.
type Subscription struct {
	Name        string    // display name (the charge's description)
	Cadence     Cadence   // inferred recurrence
	Amount      int64     // the typical charge, base-currency minor units (positive)
	Currency    string    // base currency
	Count       int       // how many occurrences were detected
	Last        time.Time // most recent occurrence
	NextRenewal time.Time // Last advanced by one cadence interval
}

// Lapsed reports whether the pattern looks no longer active: now is more than
// one full cadence interval (plus a two-week grace) past the expected next
// renewal. A COBRA premium from a 2023 layoff must not surface as a live
// subscription whose "next renewal" is years in the past.
func (s Subscription) Lapsed(now time.Time) bool {
	var interval time.Duration
	switch s.Cadence {
	case CadenceYearly:
		interval = 366 * 24 * time.Hour
	case CadenceWeekly:
		interval = 7 * 24 * time.Hour
	default:
		interval = 31 * 24 * time.Hour
	}
	return now.After(s.NextRenewal.Add(interval + 14*24*time.Hour))
}

// MonthlyAmount normalizes the charge to a per-month figure (yearly /12, weekly
// ×52/12), so subscriptions on different cadences can be compared and summed.
func (s Subscription) MonthlyAmount() int64 {
	switch s.Cadence {
	case CadenceYearly:
		return s.Amount / 12
	case CadenceWeekly:
		return s.Amount * 52 / 12
	default: // monthly
		return s.Amount
	}
}

// AnnualAmount is the charge's yearly cost.
func (s Subscription) AnnualAmount() int64 {
	switch s.Cadence {
	case CadenceYearly:
		return s.Amount
	case CadenceWeekly:
		return s.Amount * 52
	default: // monthly
		return s.Amount * 12
	}
}

// Detect finds recurring charges in txns. It considers non-transfer expenses,
// converts each to the base currency, and groups charges that share a normalized
// description and the same (converted) amount. A group qualifies as a
// subscription when it has at least minCount occurrences whose median spacing
// matches a known cadence (weekly, monthly, or yearly). minCount is clamped to a
// floor of 2 (one gap is the minimum needed to infer a cadence). Results are
// sorted by monthly cost, largest first (ties by name).
func Detect(txns []domain.Transaction, rates currency.Rates, minCount int) ([]Subscription, error) {
	if minCount < 2 {
		minCount = 2
	}

	// C165: group by MERCHANT (normalized name) only — not name+amount. Keying on
	// the amount split a merchant whose price changed (e.g. Netflix $15.49 → $17.99)
	// into two separate "subscriptions". Now all of a merchant's charges form one
	// group; the representative amount is the most-recent charge (the current price),
	// and cadence is computed over every charge date.
	type charge struct {
		date time.Time
		amt  int64
	}
	type group struct {
		name    string
		charges []charge
	}
	groups := map[string]*group{}
	for _, t := range txns {
		if !t.IsExpense() {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, err
		}
		amt := conv.Abs().Amount
		name := strings.TrimSpace(t.Desc)
		key := strings.ToLower(name)
		g := groups[key]
		if g == nil {
			g = &group{name: name}
			groups[key] = g
		}
		g.charges = append(g.charges, charge{date: t.Date, amt: amt})
	}

	var out []Subscription
	for _, g := range groups {
		if len(g.charges) < minCount {
			continue
		}
		sort.Slice(g.charges, func(i, j int) bool { return g.charges[i].date.Before(g.charges[j].date) })
		gaps := make([]int, 0, len(g.charges)-1)
		for i := 1; i < len(g.charges); i++ {
			gaps = append(gaps, int(g.charges[i].date.Sub(g.charges[i-1].date).Hours()/24+0.5))
		}
		cad, ok := classify(medianInt(gaps))
		if !ok {
			continue
		}
		last := g.charges[len(g.charges)-1]
		out = append(out, Subscription{
			Name:        g.name,
			Cadence:     cad,
			Amount:      last.amt, // current price = most recent charge
			Currency:    rates.Base,
			Count:       len(g.charges),
			Last:        last.date,
			NextRenewal: advance(last.date, cad),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		mi, mj := out[i].MonthlyAmount(), out[j].MonthlyAmount()
		if mi != mj {
			return mi > mj
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// MonthlyTotal sums the normalized monthly cost across subscriptions — the total
// recurring burden.
func MonthlyTotal(subs []Subscription) int64 {
	var sum int64
	for _, s := range subs {
		sum += s.MonthlyAmount()
	}
	return sum
}

// classify maps a median gap (in days) to a cadence, within tolerance bands. It
// reports ok=false when the spacing doesn't look like a regular subscription.
func classify(days int) (Cadence, bool) {
	switch {
	case days >= 6 && days <= 8:
		return CadenceWeekly, true
	case days >= 26 && days <= 33:
		return CadenceMonthly, true
	case days >= 350 && days <= 380:
		return CadenceYearly, true
	default:
		return "", false
	}
}

// advance returns t moved forward by one cadence interval.
func advance(t time.Time, c Cadence) time.Time {
	switch c {
	case CadenceWeekly:
		return t.AddDate(0, 0, 7)
	case CadenceYearly:
		return t.AddDate(1, 0, 0)
	default: // monthly
		return t.AddDate(0, 1, 0)
	}
}

// medianInt returns the median of xs (which must be non-empty). For an even
// count it averages the two middle values (truncating).
func medianInt(xs []int) int {
	s := append([]int(nil), xs...)
	sort.Ints(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
