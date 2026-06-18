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
			got, err := EnvelopeAvailable(monthly, tt.txns, curRef, time.Sunday, rates, nil)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if got.Amount != tt.want {
				t.Errorf("EnvelopeAvailable = %d, want %d", got.Amount, tt.want)
			}
		})
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
	got, err := EnvelopeAvailable(indiv, txns, curRef, time.Sunday, rates, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// May leftover = 10000 - 5000 = 5000; June leftover = 10000 (no m1 spend) → 15000.
	if got.Amount != 15000 {
		t.Errorf("scoped envelope = %d, want 15000", got.Amount)
	}
}
