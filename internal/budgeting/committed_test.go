// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func cmtDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func cmtBudget(period domain.Period, limitMinor int64) domain.Budget {
	return domain.Budget{
		ID: "b1", Name: "Entertainment", CategoryID: "cat1",
		Period: period, Limit: money.New(limitMinor, "USD"),
	}
}

func cmtRec(id string, minor int64, cadence domain.RecurringCadence, next time.Time, smooth bool) domain.Recurring {
	return domain.Recurring{
		ID: id, Label: id, Amount: money.New(minor, "USD"),
		Cadence: cadence, NextDue: next, CategoryID: "cat1",
		SmoothIntoBudgets: smooth,
	}
}

func cmtTxn(minor int64, date time.Time) domain.Transaction {
	return domain.Transaction{
		ID: "t" + date.Format("0102"), AccountID: "a1", CategoryID: "cat1",
		Amount: money.New(minor, "USD"), Date: date,
	}
}

func TestCommitted(t *testing.T) {
	start := cmtDate(2026, 6, 1)
	end := cmtDate(2026, 7, 1)

	tests := []struct {
		name          string
		budget        domain.Budget
		recurrings    []domain.Recurring
		posted        []domain.Transaction
		remaining     int64
		wantCommitted int64
		wantFree      int64
	}{
		{
			name:          "monthly: Netflix+Spotify uncommitted",
			budget:        cmtBudget(domain.PeriodMonthly, 10000),
			recurrings:    []domain.Recurring{cmtRec("netflix", -1600, domain.CadenceMonthly, cmtDate(2026, 6, 15), false), cmtRec("spotify", -1000, domain.CadenceMonthly, cmtDate(2026, 6, 20), false)},
			remaining:     10000,
			wantCommitted: 2600,
			wantFree:      7400,
		},
		{
			name:          "recurring already posted -> not committed",
			budget:        cmtBudget(domain.PeriodMonthly, 10000),
			recurrings:    []domain.Recurring{cmtRec("netflix", -1600, domain.CadenceMonthly, cmtDate(2026, 6, 15), false)},
			posted:        []domain.Transaction{cmtTxn(-1600, cmtDate(2026, 6, 10))},
			remaining:     8400,
			wantCommitted: 0,
			wantFree:      8400,
		},
		{
			name:          "multi-recurring one category",
			budget:        cmtBudget(domain.PeriodMonthly, 20000),
			recurrings:    []domain.Recurring{cmtRec("a", -3000, domain.CadenceMonthly, cmtDate(2026, 6, 5), false), cmtRec("b", -2000, domain.CadenceMonthly, cmtDate(2026, 6, 12), false), cmtRec("c", -1000, domain.CadenceMonthly, cmtDate(2026, 6, 25), false)},
			remaining:     20000,
			wantCommitted: 6000,
			wantFree:      14000,
		},
		{
			name:          "smoothed annual off-month accrues set-aside",
			budget:        cmtBudget(domain.PeriodMonthly, 10000),
			recurrings:    []domain.Recurring{cmtRec("ins", -60000, domain.CadenceYearly, cmtDate(2026, 12, 15), true)},
			remaining:     10000,
			wantCommitted: 5000,
			wantFree:      5000,
		},
		{
			name:          "smoothed annual landing-month: no committed set-aside",
			budget:        cmtBudget(domain.PeriodMonthly, 10000),
			recurrings:    []domain.Recurring{cmtRec("ins", -60000, domain.CadenceYearly, cmtDate(2026, 6, 15), true)},
			remaining:     10000,
			wantCommitted: 0,
			wantFree:      10000,
		},
		{
			name:          "committed capped at remaining",
			budget:        cmtBudget(domain.PeriodMonthly, 10000),
			recurrings:    []domain.Recurring{cmtRec("big", -9000, domain.CadenceMonthly, cmtDate(2026, 6, 15), false)},
			remaining:     5000,
			wantCommitted: 5000,
			wantFree:      0,
		},
		{
			name:          "over budget: no committed, free carries negative",
			budget:        cmtBudget(domain.PeriodMonthly, 10000),
			recurrings:    []domain.Recurring{cmtRec("netflix", -1600, domain.CadenceMonthly, cmtDate(2026, 6, 15), false)},
			remaining:     -500,
			wantCommitted: 0,
			wantFree:      -500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Committed(tt.budget, tt.recurrings, tt.posted, money.New(tt.remaining, "USD"), start, end)
			if got.Committed.Amount != tt.wantCommitted {
				t.Errorf("Committed = %d, want %d", got.Committed.Amount, tt.wantCommitted)
			}
			if got.Free.Amount != tt.wantFree {
				t.Errorf("Free = %d, want %d", got.Free.Amount, tt.wantFree)
			}
			// Committed + Free must reconcile to remaining.
			if got.Committed.Amount+got.Free.Amount != tt.remaining {
				t.Errorf("committed+free = %d, want remaining %d", got.Committed.Amount+got.Free.Amount, tt.remaining)
			}
		})
	}
}

func TestCommittedWeeklyBudget(t *testing.T) {
	// Weekly budget window; a weekly recurring should be expected once.
	start := cmtDate(2026, 6, 1)
	end := cmtDate(2026, 6, 8)
	budget := cmtBudget(domain.PeriodWeekly, 5000)
	rs := []domain.Recurring{cmtRec("gym", -2000, domain.CadenceWeekly, cmtDate(2026, 6, 4), false)}
	got := Committed(budget, rs, nil, money.New(5000, "USD"), start, end)
	if got.Committed.Amount != 2000 {
		t.Errorf("weekly committed = %d, want 2000", got.Committed.Amount)
	}
	if got.Free.Amount != 3000 {
		t.Errorf("weekly free = %d, want 3000", got.Free.Amount)
	}
}
