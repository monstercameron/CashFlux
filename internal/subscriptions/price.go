package subscriptions

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// PriceChange reports that a recurring charge's amount changed over time — the
// "your subscription went up" signal. Amounts are base-currency minor units.
type PriceChange struct {
	Name          string    // the charge's display name
	Currency      string    // base currency
	OldAmount     int64     // the prior charge amount (the run before the change)
	NewAmount     int64     // the current charge amount (the latest run)
	Delta         int64     // NewAmount − OldAmount (positive = a price rise)
	PercentChange int       // Delta as a percent of OldAmount, rounded
	ChangedAt     time.Time // the date of the first charge at the new amount
}

// Increased reports whether the price went up (rather than down).
func (p PriceChange) Increased() bool { return p.Delta > 0 }

// DetectPriceChanges finds recurring charges whose amount has changed. Unlike
// Detect (which groups by name AND amount, so a price change would split into two
// separate subscriptions), this groups non-transfer expenses by normalized name
// only, confirms the series looks like a regular subscription (at least minCount
// charges whose median spacing matches a cadence), then reports the most recent
// amount transition: from the charge run before the change to the current run.
// minCount is clamped to a floor of 3 — a change needs a "before" and an "after",
// so two charges aren't enough to tell a change from a one-off. Results are
// sorted most-recent-change first (ties by name).
//
// It reports the latest distinct-amount transition in each series, so a genuinely
// fluctuating (usage-based) charge can produce noise; the cadence check filters
// most such series out, but callers should present results as a heads-up.
func DetectPriceChanges(txns []domain.Transaction, rates currency.Rates, minCount int) ([]PriceChange, error) {
	if minCount < 3 {
		minCount = 3
	}

	type charge struct {
		date time.Time
		amt  int64
	}
	groups := map[string]*struct {
		name    string
		charges []charge
	}{}
	for _, t := range txns {
		if !t.IsExpense() {
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
			g = &struct {
				name    string
				charges []charge
			}{name: name}
			groups[key] = g
		}
		g.charges = append(g.charges, charge{date: t.Date, amt: conv.Abs().Amount})
	}

	var out []PriceChange
	for _, g := range groups {
		if len(g.charges) < minCount {
			continue
		}
		sort.Slice(g.charges, func(i, j int) bool { return g.charges[i].date.Before(g.charges[j].date) })

		gaps := make([]int, 0, len(g.charges)-1)
		for i := 1; i < len(g.charges); i++ {
			gaps = append(gaps, int(g.charges[i].date.Sub(g.charges[i-1].date).Hours()/24+0.5))
		}
		if _, ok := classify(medianInt(gaps)); !ok {
			continue // not a regular subscription cadence
		}

		// The current price is the latest charge; walk back to the most recent
		// charge with a different amount — that's the prior price, and the charge
		// just after it is when the new price started.
		n := len(g.charges)
		newAmt := g.charges[n-1].amt
		prevIdx := -1
		for i := n - 2; i >= 0; i-- {
			if g.charges[i].amt != newAmt {
				prevIdx = i
				break
			}
		}
		if prevIdx < 0 {
			continue // price never changed
		}
		oldAmt := g.charges[prevIdx].amt
		out = append(out, PriceChange{
			Name:          g.name,
			Currency:      rates.Base,
			OldAmount:     oldAmt,
			NewAmount:     newAmt,
			Delta:         newAmt - oldAmt,
			PercentChange: percentChange(oldAmt, newAmt),
			ChangedAt:     g.charges[prevIdx+1].date,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].ChangedAt.Equal(out[j].ChangedAt) {
			return out[i].ChangedAt.After(out[j].ChangedAt)
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// percentChange returns (cur − old) as a percent of old, rounded. A non-positive
// old amount yields 0 (no meaningful base to compare against).
func percentChange(old, cur int64) int {
	if old <= 0 {
		return 0
	}
	return int(math.Round(float64(cur-old) / float64(old) * 100))
}
