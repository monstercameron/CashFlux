package budgeting

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func budget(id, name string, limit money.Money) domain.Budget {
	return domain.Budget{ID: id, Name: name, Limit: limit}
}

func TestTransfer(t *testing.T) {
	tests := []struct {
		name          string
		from, to      domain.Budget
		amt           money.Money
		allowNegative bool
		wantErr       error
		wantFrom      money.Money // expected source limit after
		wantTo        money.Money // expected destination limit after
	}{
		{
			name:     "cover overspend pulls from an under-budget envelope",
			from:     budget("shop", "Shopping", usd(40000)),
			to:       budget("groc", "Groceries", usd(60000)),
			amt:      usd(30400),
			wantFrom: usd(9600),
			wantTo:   usd(90400),
		},
		{
			name:     "exact-to-zero source is allowed",
			from:     budget("a", "A", usd(5000)),
			to:       budget("b", "B", usd(1000)),
			amt:      usd(5000),
			wantFrom: usd(0),
			wantTo:   usd(6000),
		},
		{
			name:    "insufficient source is rejected",
			from:    budget("a", "A", usd(5000)),
			to:      budget("b", "B", usd(1000)),
			amt:     usd(5001),
			wantErr: ErrInsufficientSource,
		},
		{
			name:          "insufficient source allowed when overdraw permitted",
			from:          budget("a", "A", usd(5000)),
			to:            budget("b", "B", usd(1000)),
			amt:           usd(8000),
			allowNegative: true,
			wantFrom:      usd(-3000),
			wantTo:        usd(9000),
		},
		{
			name:    "same budget is rejected",
			from:    budget("a", "A", usd(5000)),
			to:      budget("a", "A", usd(5000)),
			amt:     usd(100),
			wantErr: ErrTransferSameBudget,
		},
		{
			name:    "non-positive amount is rejected",
			from:    budget("a", "A", usd(5000)),
			to:      budget("b", "B", usd(1000)),
			amt:     usd(0),
			wantErr: ErrTransferNonPositive,
		},
		{
			name:    "currency mismatch is rejected",
			from:    budget("a", "A", usd(5000)),
			to:      budget("b", "B", money.New(1000, "EUR")),
			amt:     usd(100),
			wantErr: ErrTransferCurrency,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Transfer(tc.from, tc.to, tc.amt, tc.allowNegative)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !res.From.Limit.Equal(tc.wantFrom) {
				t.Errorf("from limit = %s, want %s", res.From.Limit, tc.wantFrom)
			}
			if !res.To.Limit.Equal(tc.wantTo) {
				t.Errorf("to limit = %s, want %s", res.To.Limit, tc.wantTo)
			}
			// Balanced: total budgeted amount is unchanged.
			beforeTotal := tc.from.Limit.Amount + tc.to.Limit.Amount
			afterTotal := res.From.Limit.Amount + res.To.Limit.Amount
			if beforeTotal != afterTotal {
				t.Errorf("total budgeted changed: %d -> %d", beforeTotal, afterTotal)
			}
			// Breakdown legs are populated and consistent.
			if !res.FromLimitBefore.Equal(tc.from.Limit) || !res.ToLimitBefore.Equal(tc.to.Limit) {
				t.Errorf("breakdown 'before' legs wrong: %+v", res)
			}
			if !res.Amount.Equal(tc.amt) {
				t.Errorf("result amount = %s, want %s", res.Amount, tc.amt)
			}
		})
	}
}

func TestTransferDoesNotMutateInputs(t *testing.T) {
	from := budget("a", "A", usd(5000))
	to := budget("b", "B", usd(1000))
	if _, err := Transfer(from, to, usd(2000), false); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if from.Limit.Amount != 5000 || to.Limit.Amount != 1000 {
		t.Errorf("inputs mutated: from=%s to=%s", from.Limit, to.Limit)
	}
}

func TestCoverAmount(t *testing.T) {
	tests := []struct {
		name      string
		spent     money.Money
		limit     money.Money
		remaining money.Money
		want      money.Money
	}{
		{name: "overspent returns the shortfall", remaining: usd(-30400), want: usd(30400)},
		{name: "within budget returns zero", remaining: usd(5000), want: usd(0)},
		{name: "exactly at limit returns zero", remaining: usd(0), want: usd(0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CoverAmount(Status{Remaining: tc.remaining})
			if !got.Equal(tc.want) {
				t.Errorf("CoverAmount = %s, want %s", got, tc.want)
			}
		})
	}
}
