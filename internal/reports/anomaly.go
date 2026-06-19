package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// CategoryAnomaly flags a category spending notably more this month than its
// recent monthly norm — a proactive overspend heads-up, more robust than a
// single prior-period comparison because it averages several months.
type CategoryAnomaly struct {
	CategoryID string
	Current    int64 // this month's spend, base-currency minor units
	Average    int64 // average over the prior months (its span of activity)
	OverPct    int   // how far above the average, percent (e.g. 80 = 80% over)
}

// SpendingAnomalies compares each category's spend in the current month (the
// month containing now) to its average over the prior `months` full months, and
// returns the categories whose current spend exceeds that average by at least
// overPct percent AND is at least minMinor in absolute terms (to skip noise on
// tiny categories) — biggest overage first. The trailing average divides by the
// span from the oldest month with spend through the most recent, so a young
// category isn't diluted; categories with no prior baseline are skipped.
// Transfers and income are excluded. Amounts convert to the base currency.
func SpendingAnomalies(txns []domain.Transaction, now time.Time, months, overPct int, minMinor int64, rates currency.Rates) ([]CategoryAnomaly, error) {
	if months <= 0 {
		return nil, nil
	}
	curStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	curEnd := dateutil.AddMonths(curStart, 1)

	current := map[string]int64{}
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, curStart, curEnd) {
			continue
		}
		conv, err := rates.Convert(t.Amount.Abs(), rates.Base)
		if err != nil {
			return nil, err
		}
		current[t.CategoryID] += conv.Amount
	}

	// Per-category trailing totals plus the oldest prior month (largest k) with
	// spend, so the average divides by the real span of activity.
	type span struct {
		total  int64
		oldest int
	}
	trailing := map[string]*span{}
	for k := 1; k <= months; k++ {
		ms := dateutil.AddMonths(curStart, -k)
		me := dateutil.AddMonths(curStart, -(k - 1))
		for _, t := range txns {
			if !t.IsExpense() || !dateutil.InRange(t.Date, ms, me) {
				continue
			}
			conv, err := rates.Convert(t.Amount.Abs(), rates.Base)
			if err != nil {
				return nil, err
			}
			s := trailing[t.CategoryID]
			if s == nil {
				s = &span{}
				trailing[t.CategoryID] = s
			}
			s.total += conv.Amount
			if k > s.oldest {
				s.oldest = k
			}
		}
	}

	var out []CategoryAnomaly
	for cat, cur := range current {
		if cur < minMinor {
			continue
		}
		s := trailing[cat]
		if s == nil || s.oldest == 0 {
			continue // no prior baseline to judge against
		}
		avg := s.total / int64(s.oldest)
		if avg <= 0 {
			continue
		}
		if cur < avg+avg*int64(overPct)/100 {
			continue
		}
		out = append(out, CategoryAnomaly{
			CategoryID: cat,
			Current:    cur,
			Average:    avg,
			OverPct:    int((cur - avg) * 100 / avg),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OverPct != out[j].OverPct {
			return out[i].OverPct > out[j].OverPct
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out, nil
}
