// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestNormalizedLimitDefaultsCurrency checks that a budget whose limit has an
// empty currency code is evaluated in the rate table's base currency.
func TestNormalizedLimitDefaultsCurrency(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: money.New(50000, "")}
	all := []domain.Transaction{expense(10000, "USD", "food", "", "2026-06-03")}

	st, err := Evaluate(budget, all, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if st.Spent.Currency != "USD" || !st.Spent.Equal(usd(10000)) {
		t.Errorf("spent = %v, want 10000 USD (base-defaulted)", st.Spent)
	}
	if !st.Remaining.Equal(usd(40000)) {
		t.Errorf("remaining = %v, want 40000 USD", st.Remaining)
	}
}

// TestSpentConvertError exercises the conversion-error path: a covered expense in
// a currency the rate table can't resolve makes Spent fail.
func TestSpentConvertError(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"} // no JPY rate
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(50000)}
	all := []domain.Transaction{expense(1000, "JPY", "food", "", "2026-06-03")}

	if _, err := Spent(budget, all, start, end, rates); err == nil {
		t.Error("Spent should error when a covered expense can't be converted")
	}
}

// TestEvaluateConvertError checks the error propagates through Evaluate too.
func TestEvaluateConvertError(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(50000)}
	all := []domain.Transaction{expense(1000, "JPY", "food", "", "2026-06-03")}

	if _, err := Evaluate(budget, all, start, end, rates, DefaultNearThreshold); err == nil {
		t.Error("Evaluate should surface the conversion error")
	}
}

func TestEvaluateAll(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budgets := []domain.Budget{
		{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(50000)},
		{CategoryID: "rent", Scope: domain.ScopeShared, Limit: usd(100000)},
	}
	all := []domain.Transaction{
		expense(45000, "USD", "food", "", "2026-06-03"), // near (90% of 50000)
		expense(20000, "USD", "rent", "", "2026-06-04"), // ok
	}

	statuses, err := EvaluateAll(budgets, all, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}
	if statuses[0].State != StateNear {
		t.Errorf("food state = %q, want near", statuses[0].State)
	}
	if statuses[1].State != StateOK {
		t.Errorf("rent state = %q, want ok", statuses[1].State)
	}
}

func TestEvaluateAllError(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budgets := []domain.Budget{{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(50000)}}
	all := []domain.Transaction{expense(1000, "JPY", "food", "", "2026-06-03")}

	if _, err := EvaluateAll(budgets, all, start, end, rates, DefaultNearThreshold); err == nil {
		t.Error("EvaluateAll should surface a per-budget evaluation error")
	}
}

// TestEnvelopeAvailableConvertError covers the error path inside the envelope
// accumulation loop (a covered expense that can't be converted).
func TestEnvelopeAvailableConvertError(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: usd(50000)}
	all := []domain.Transaction{expense(1000, "JPY", "food", "", "2026-06-03")}

	if _, err := EnvelopeAvailable(budget, all, mustDate("2026-06-15"), time.Sunday, rates, nil); err == nil {
		t.Error("EnvelopeAvailable should surface the conversion error from the loop")
	}
}
