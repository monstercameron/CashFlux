// SPDX-License-Identifier: MIT

// Package coverplan prepares the Cover-all read model and explicit cover requests.
// It leaves ledger evaluation and balanced writes to budgeting.PlanCover.
package coverplan

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// NextSource is the picker value for borrowing from the destination's next period.
const NextSource = "__next__"

// Over is an evaluated overage and its viewed-period write boundaries.
type Over struct {
	ID, Name, Currency string
	Rollover           bool
	Shortfall          money.Money
	Error              error
	ThisStart          time.Time
	NextStart          time.Time
}

// Source is available donor room after committed bills, with monthly ranking room.
type Source struct {
	ID, Name, Currency string
	Avail              money.Money
	FreeRoom           money.Money
	Monthly            money.Money
	ThisStart          time.Time
}

// Data derives donors and overages from the same statuses as the budget grid.
// The period is explicit so callers can preserve the viewed window independently
// of the ledger input's own period. Unavailable donors are never offered.
func Data(statuses []budgeting.Status, in budgeting.ViewInput, period budgeting.ViewPeriod) (overs []Over, sources []Source) {
	for _, st := range statuses {
		b := st.Budget
		if b.IsSaving() {
			continue
		}
		start, end := budgeting.PeriodRangeAnchored(b.Period, period.Anchor, period.WeekStart, period.PayCycleAnchor)
		switch {
		case st.State == budgeting.StateOver:
			// Use checked negation so an unrepresentable overage is unavailable.
			shortfall := money.Zero(st.Remaining.Currency)
			var err error
			if st.Remaining.IsNegative() {
				shortfall, err = st.Remaining.NegChecked()
			}
			overs = append(overs, Over{ID: b.ID, Name: b.Name, Rollover: b.Rollover, Currency: b.Limit.Currency,
				Shortfall: shortfall, Error: err, ThisStart: start, NextStart: end})
		case st.State == budgeting.StateOK && st.Remaining.Amount > 0:
			funds, err := budgeting.CoverSourceFunds(b, in)
			if err != nil || !funds.CanGive() {
				continue
			}
			monthly, err := MonthlyEquivalent(funds.Free, b.Period)
			if err != nil || monthly.Amount < monthlyDonorMinimum(monthly.Currency) {
				continue
			}
			sources = append(sources, Source{ID: b.ID, Name: b.Name, Currency: funds.Free.Currency,
				Avail: funds.Free, FreeRoom: monthly, Monthly: monthly, ThisStart: start})
		}
	}
	sort.Slice(overs, func(i, j int) bool { return overs[i].Name < overs[j].Name })
	sort.SliceStable(sources, func(i, j int) bool { return SourceLess(sources[i], sources[j]) })
	return overs, sources
}

// SourceLess ranks donors by descending monthly room, then ascending name.
func SourceLess(a, b Source) bool {
	if a.FreeRoom.Amount != b.FreeRoom.Amount {
		return a.FreeRoom.Amount > b.FreeRoom.Amount
	}
	return a.Name < b.Name
}

func monthlyDonorMinimum(code string) int64 {
	minimum := int64(1)
	for range currency.Decimals(code) {
		minimum *= 10
	}
	return minimum
}

// MonthlyRoom caps the monthly equivalent at the actual movable period balance.
func MonthlyRoom(remaining money.Money, cadence domain.Period) (money.Money, error) {
	monthly, err := MonthlyEquivalent(remaining, cadence)
	if err != nil {
		return money.Money{}, err
	}
	if monthly.Amount > remaining.Amount {
		return remaining, nil
	}
	return monthly, nil
}

// MonthlyEquivalent normalizes room for ranking and donor labels, without FX.
func MonthlyEquivalent(amount money.Money, cadence domain.Period) (money.Money, error) {
	return budgeting.PeriodEquivalent(amount, cadence, domain.PeriodMonthly)
}

// CoveredAt returns the start of the covered period, including past windows.
func CoveredAt(over Over) time.Time { return over.ThisStart }

// DefaultSource returns the first ranked donor in the overage's currency.
func DefaultSource(over Over, sources []Source) string {
	for _, source := range sources {
		if source.Currency == over.Currency {
			return source.ID
		}
	}
	return ""
}

// SourcesFor preserves donor order while selecting the overage's currency.
func SourcesFor(over Over, sources []Source) []Source {
	matching := make([]Source, 0, len(sources))
	for _, source := range sources {
		if source.Currency == over.Currency {
			matching = append(matching, source)
		}
	}
	return matching
}
