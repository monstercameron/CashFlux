// SPDX-License-Identifier: MIT

package engineenv

// This file exposes the credit-health proxy as engine variables: the three
// factor scores (utilization / on-time / account-age, each 0–100) and their
// EXACT normalized weights as atoms, plus the actionable pay-down targets. The
// headline is deliberately NOT an atom — it is the credit_proxy MOLECULE in
// DefaultMolecules, a real formula over these atoms (floor(Σ score×weight),
// clamped) — so the number is auditable via Explain, referenceable in any
// formula or dashboard widget, and even re-weightable by editing the molecule.
// The engine uses checked currency-aware aggregates; legacy credit-page inputs
// retain their existing shape for callers of CreditInputs.

import (
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// CreditVarNames are the fixed credit atoms addCreditVars exposes, in a stable
// order: a score + weight pair per factor, plus the pay-down targets.
var CreditVarNames = []string{
	"credit_util_score", "credit_util_weight", // aggregate-utilization factor (always weighted; 0 with no limits)
	"credit_ontime_score", "credit_ontime_weight", // on-time payment proxy (weight 0 when no due days are set)
	"credit_age_score", "credit_age_weight", // account-age proxy (weight 0 when no card has a balance date)
	"credit_pay_to_30", // Σ amount to pay so every card sits at ≤30% utilization (major units)
	"credit_pay_to_10", // …and at ≤10% (major units)
}

func init() { Names = append(Names, CreditVarNames...) }

// CreditInputs derives the credit-health signals from the fundamental Data —
// the pure port of the /credit screen's assembly: per-card running balances
// plus the transaction history for the on-time proxy.
func CreditInputs(d Data) credithealth.Inputs {
	bals, _ := ledger.Balances(d.Accounts, d.Transactions)
	return creditInputs(d, bals)
}

// creditInputs is CreditInputs over a precomputed balance map, so Vars() can
// share its single-pass Balances result instead of re-scanning the ledger.
func creditInputs(d Data, bals map[string]money.Money) credithealth.Inputs {
	balances := make(map[string]int64, len(d.Accounts))
	for _, a := range d.Accounts {
		if a.Type != domain.TypeCreditCard || a.Archived {
			continue
		}
		if bal, ok := bals[a.ID]; ok && bal.Currency == a.Currency {
			balances[a.ID] = bal.Amount
		}
	}
	return credithealth.Inputs{
		Accounts:     d.Accounts,
		Balances:     balances,
		Transactions: d.Transactions,
		Now:          d.Now,
	}
}

// addCreditVars uses native card targets and a checked base-currency aggregate.
// Unknown required FX, missing balances and arithmetic errors omit monetary
// factors; independent on-time/age scores and their known weights remain usable.
// The retained compatibility converter is deliberately unused: tolerant FX
// conversion cannot establish an available credit-health result.
func addCreditVars(out map[string]float64, d Data, major func(int64) float64, _ func(int64, string) int64, bals map[string]money.Money) {
	rates := d.Rates
	if rates.Base == "" {
		rates.Base = "USD"
	}
	r := credithealth.EvaluateWithRates(creditInputs(d, bals), rates)
	out["credit_util_weight"] = r.Weights.Util
	out["credit_ontime_score"] = 0
	out["credit_ontime_weight"] = r.Weights.OnTime
	if r.OnTimeScore >= 0 {
		out["credit_ontime_score"] = float64(r.OnTimeScore)
	}
	out["credit_age_score"] = 0
	out["credit_age_weight"] = r.Weights.Age
	if r.AgeScore >= 0 {
		out["credit_age_score"] = float64(r.AgeScore)
	}
	if r.UtilErr == nil {
		out["credit_util_score"] = float64(r.UtilScore)
	}
	if r.PayTo30Err == nil {
		out["credit_pay_to_30"] = major(r.PayTo30.Amount)
	}
	if r.PayTo10Err == nil {
		out["credit_pay_to_10"] = major(r.PayTo10.Amount)
	}
}
