// SPDX-License-Identifier: MIT

package engineenv

// This file exposes the credit-health proxy as engine variables: the three
// factor scores (utilization / on-time / account-age, each 0–100) and their
// EXACT normalized weights as atoms, plus the actionable pay-down targets. The
// headline is deliberately NOT an atom — it is the credit_proxy MOLECULE in
// DefaultMolecules, a real formula over these atoms (floor(Σ score×weight),
// clamped) — so the number is auditable via Explain, referenceable in any
// formula or dashboard widget, and even re-weightable by editing the molecule.
// The inputs derivation (CreditInputs) is the single source shared by the
// /credit page, the /debt embed, and these variables.

import (
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
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
// the pure port of the /credit screen's assembly: per-card running balances via
// ledger.Balance plus the transaction history for the on-time proxy.
func CreditInputs(d Data) credithealth.Inputs {
	balances := make(map[string]int64, len(d.Accounts))
	for _, a := range d.Accounts {
		if a.Type != domain.TypeCreditCard || a.Archived {
			continue
		}
		bal, err := ledger.Balance(a, d.Transactions)
		if err != nil {
			continue
		}
		balances[a.ID] = bal.Amount
	}
	return credithealth.Inputs{
		Accounts:     d.Accounts,
		Balances:     balances,
		Transactions: d.Transactions,
		Now:          d.Now,
	}
}

// addCreditVars runs the credit-health model over the shared CreditInputs
// derivation and exposes each factor's score + exact weight (a missing on-time
// or age factor scores 0 with zero weight), plus the pay-down targets summed
// across cards in the base currency. The credit_proxy molecule then reproduces
// credithealth.Evaluate's ProxyScore exactly.
func addCreditVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	for _, name := range CreditVarNames {
		out[name] = 0
	}
	r := credithealth.Evaluate(CreditInputs(d))
	out["credit_util_score"] = float64(credithealth.UtilScore(r.Agg.UtilPct))
	out["credit_util_weight"] = r.Weights.Util
	if r.OnTimeScore >= 0 {
		out["credit_ontime_score"] = float64(r.OnTimeScore)
		out["credit_ontime_weight"] = r.Weights.OnTime
	}
	if r.AgeScore >= 0 {
		out["credit_age_score"] = float64(r.AgeScore)
		out["credit_age_weight"] = r.Weights.Age
	}
	cur := func(id string) string {
		for _, a := range d.Accounts {
			if a.ID == id {
				return a.Currency
			}
		}
		return d.Rates.Base
	}
	var to30, to10 int64
	for _, c := range r.Cards {
		to30 += toBase(c.Target30Minor, cur(c.AccountID))
		to10 += toBase(c.Target10Minor, cur(c.AccountID))
	}
	out["credit_pay_to_30"] = major(to30)
	out["credit_pay_to_10"] = major(to10)
}
