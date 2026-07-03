// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/money"
)

// healthTestData builds a household with income, spending, liquid cash, a debt
// with a minimum payment, a credit card with a limit, and enough history for
// the net-worth trend — so every factor is applicable.
func healthTestData(now time.Time) Data {
	asOf := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	mk := func(id string, ty domain.AccountType, class domain.AccountClass, minor int64) domain.Account {
		return domain.Account{ID: id, Type: ty, Class: class, Currency: "USD",
			OpeningBalance: money.New(minor, "USD"), BalanceAsOf: asOf}
	}
	card := mk("cc", domain.TypeCreditCard, domain.ClassLiability, -30000) // $300 owed
	card.CreditLimit = money.New(100000, "USD")                           // $1,000 limit → 30% util
	card.MinPayment = money.New(5000, "USD")                              // $50/mo minimum
	var txns []domain.Transaction
	// Three full trailing months of $4,000 income / $3,000 spend → 25% savings rate.
	for m := 1; m <= 3; m++ {
		d := time.Date(2026, time.Month(3+m), 10, 0, 0, 0, 0, time.UTC)
		txns = append(txns,
			domain.Transaction{ID: "i" + string(rune('0'+m)), AccountID: "chk", Date: d, Amount: money.New(400000, "USD")},
			domain.Transaction{ID: "e" + string(rune('0'+m)), AccountID: "chk", Date: d.AddDate(0, 0, 2), Amount: money.New(-300000, "USD")},
		)
	}
	return Data{
		Accounts: []domain.Account{
			mk("chk", domain.TypeChecking, domain.ClassAsset, 600000), // $6,000 liquid
			card,
		},
		Transactions: txns,
		Rates:        currency.Rates{Base: "USD"},
		Now:          now,
	}
}

// TestHealthScoreMoleculeMatchesModel is the headline guarantee: the
// health_score MOLECULE (a formula over the health_* atoms) evaluated through
// the normal Vars() pipeline equals healthscore.Evaluate's Score for the same
// derived inputs — the score the /health page shows IS the formula's value.
func TestHealthScoreMoleculeMatchesModel(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	d := healthTestData(now)
	v := Vars(d)
	r := healthscore.Evaluate(HealthInputs(d))

	if r.Band == healthscore.BandNoData {
		t.Fatalf("test data should produce a scoreable household, got NoData")
	}
	if got := v["health_score"]; got != float64(r.Score) {
		t.Errorf("health_score molecule = %v, model Score = %d", got, r.Score)
	}
	// Factor atoms mirror the model's factors exactly.
	for _, f := range r.Factors {
		name := healthFactorVar(f.Key)
		if got := v[name]; got != float64(f.Score) {
			t.Errorf("%s = %v, want %d", name, got, f.Score)
		}
		if got := v[name+"_weight"]; got != f.Weight {
			t.Errorf("%s_weight = %v, want %v", name, got, f.Weight)
		}
	}
	// 25% savings rate → the savings factor saturates at 100.
	if v["health_savings"] != 100 {
		t.Errorf("health_savings = %v, want 100 (25%% rate saturates)", v["health_savings"])
	}
	// $6,000 opening + $3,000 net inflow = $9,000 liquid ÷ $3,000/mo spend = 3 months.
	if m := v["health_emergency_months"]; m < 2.9 || m > 3.1 {
		t.Errorf("health_emergency_months = %v, want ~3", m)
	}
	// No deficit → no penalty.
	if v["health_penalty"] != 0 {
		t.Errorf("health_penalty = %v, want 0", v["health_penalty"])
	}
}

// TestHealthVarsNoData: an empty dataset yields zero weights everywhere, so the
// molecule lands on 0 — mirroring the model's BandNoData — and every health_*
// variable still exists (formulas never hit undefined variables).
func TestHealthVarsNoData(t *testing.T) {
	v := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	for _, k := range HealthVarNames {
		if _, ok := v[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
	if v["health_score"] != 0 {
		t.Errorf("health_score = %v, want 0 for an empty dataset", v["health_score"])
	}
	if v["health_savings_weight"] != 0 {
		t.Errorf("weights should be zero in the NoData case")
	}
}

// TestHealthPenaltyAppliesInFormula: a deficit household pays the flat penalty
// through the molecule, matching the model.
func TestHealthPenaltyAppliesInFormula(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	d := healthTestData(now)
	// Invert the cash flow: $3,000 in / $4,000 out each month → negative savings.
	for i := range d.Transactions {
		d.Transactions[i].Amount = d.Transactions[i].Amount.Neg()
	}
	v := Vars(d)
	r := healthscore.Evaluate(HealthInputs(d))
	if !r.NegativeCashFlow {
		t.Fatalf("inverted flows should read as a deficit")
	}
	if v["health_penalty"] != healthscore.NegativeCashFlowPenalty {
		t.Errorf("health_penalty = %v, want %d", v["health_penalty"], healthscore.NegativeCashFlowPenalty)
	}
	if got := v["health_score"]; got != float64(r.Score) {
		t.Errorf("health_score molecule = %v, model Score = %d", got, r.Score)
	}
}
