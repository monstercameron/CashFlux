// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestEnvelopeAvailable(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	curRef := mustDate("2026-06-15") // current period = June
	monthly := domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Scope: domain.ScopeShared, Limit: usd(10000)}

	tests := []struct {
		name string
		txns []domain.Transaction
		want int64 // available, minor units
	}{
		{
			name: "no spend funds one period",
			txns: nil,
			want: 10000,
		},
		{
			name: "current period only",
			txns: []domain.Transaction{expense(3000, "USD", "food", "", "2026-06-10")},
			want: 7000, // 10000 - 3000
		},
		{
			name: "carries unspent forward",
			txns: []domain.Transaction{
				expense(5000, "USD", "food", "", "2026-05-10"), // May: +5000 leftover
				expense(3000, "USD", "food", "", "2026-06-10"), // June: +7000 leftover
			},
			want: 12000, // 5000 + 7000 carried into June
		},
		{
			name: "overdraw nets against the carryover",
			txns: []domain.Transaction{
				expense(8000, "USD", "food", "", "2026-05-10"),  // May: +2000
				expense(12000, "USD", "food", "", "2026-06-10"), // June: -2000 (over)
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnvelopeAvailableAnchored(monthly, tt.txns, curRef, time.Sunday, time.Time{}, rates, nil)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if got.Amount != tt.want {
				t.Errorf("EnvelopeAvailable = %d, want %d", got.Amount, tt.want)
			}
		})
	}
}

func TestEnvelopeAvailableUsesPriorEffectiveCapChain(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	b := domain.Budget{CategoryID: "groceries", Period: domain.PeriodMonthly, Scope: domain.ScopeShared, Limit: usd(45000)}
	sep := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	oct := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	b = b.WithPeriodBoost(sep, -24480)
	b = b.WithPeriodBoost(oct, 18564)
	txns := []domain.Transaction{
		expense(63564, "USD", "groceries", "", "2026-09-10"), // effective Sep cap $205.20, leaving $430.44 debt.
		expense(24250, "USD", "groceries", "", "2026-10-10"), // effective Oct cap $205.20, leaving $37.30 debt.
	}
	got, err := EnvelopeAvailableAnchored(b, txns, time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC), time.Sunday, time.Time{}, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 41270 {
		t.Errorf("November envelope = %d, want $412.70 after carrying October's $37.30 overage", got.Amount)
	}
}

func TestEnvelopeAndCardUseSameRolloverStart(t *testing.T) {
	start := mustDate("2026-01-01")
	budget := domain.Budget{
		ID: "fund", CategoryID: "food", Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, Limit: usd(30000), Rollover: true, RolloverStart: start,
	}
	transactions := []domain.Transaction{expense(10000, "USD", "food", "", "2026-03-10")}
	got, err := EnvelopeAvailableAnchored(budget, transactions, mustDate("2026-04-15"), time.Sunday, time.Time{}, currency.Rates{Base: "USD"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Jan and Feb each carry $300; March carries $200 after spending $100;
	// April adds another $300. The card's rollover chain reports $1,100.
	if got.Amount != 110000 {
		t.Fatalf("envelope available = %d; want card-consistent $1,100.00", got.Amount)
	}
}

func TestEnvelopeAvailableRespectsScope(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	curRef := mustDate("2026-06-15")
	indiv := domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: usd(10000)}
	txns := []domain.Transaction{
		expense(5000, "USD", "food", "m1", "2026-05-10"), // counts
		expense(9000, "USD", "food", "m2", "2026-05-11"), // other member, excluded
	}
	got, err := EnvelopeAvailableAnchored(indiv, txns, curRef, time.Sunday, time.Time{}, rates, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// May leftover = 10000 - 5000 = 5000; June leftover = 10000 (no m1 spend) → 15000.
	if got.Amount != 15000 {
		t.Errorf("scoped envelope = %d, want 15000", got.Amount)
	}
}

func TestEnvelopeAvailableUsesReportAndTagInclusion(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	curRef := mustDate("2026-06-15")
	b := domain.Budget{Period: domain.PeriodMonthly, Scope: domain.ScopeShared, Limit: usd(10000), CategoryID: "dining"}
	excluded := expense(4000, "USD", "dining", "", "2026-01-05")
	excluded.ExcludeFromReports = true
	got, err := EnvelopeAvailableAnchored(b, []domain.Transaction{excluded}, curRef, time.Sunday, time.Time{}, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 10000 {
		t.Fatalf("excluded old spend must not start funding history; got %d want 10000", got.Amount)
	}

	tagBudget := domain.Budget{Period: domain.PeriodMonthly, Scope: domain.ScopeShared, Limit: usd(10000), TrackedTags: []string{"trip"}}
	tagged := expense(2000, "USD", "travel", "", "2026-05-05")
	tagged.Tags = []string{"trip"}
	got, err = EnvelopeAvailableAnchored(tagBudget, []domain.Transaction{tagged}, curRef, time.Sunday, time.Time{}, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 18000 {
		t.Fatalf("tracked tag spend should establish May funding history; got %d want 18000", got.Amount)
	}
}

func TestEnvelopeAvailableIncludesHistoriesBeyondFormerWeeklyCap(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	b := domain.Budget{Period: domain.PeriodWeekly, Scope: domain.ScopeShared, Limit: usd(10000), CategoryID: "food"}
	old := expense(1, "USD", "food", "", "2020-01-01")
	got, err := EnvelopeAvailableAnchored(b, []domain.Transaction{old}, mustDate("2026-10-18"), time.Sunday, time.Time{}, rates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount <= 240*10000 {
		t.Fatalf("weekly history beyond 240 periods was truncated: got %d", got.Amount)
	}
}
