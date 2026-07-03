// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// creditTestData builds a household with two credit cards — one carrying a hot
// balance against its limit with a due day and an old balance date, one cooler —
// so every proxy factor has data.
func creditTestData(now time.Time) Data {
	old := time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC)
	hot := domain.Account{ID: "hot", Type: domain.TypeCreditCard, Class: domain.ClassLiability, Currency: "USD",
		OpeningBalance: money.New(-80000, "USD"), BalanceAsOf: old, DueDayOfMonth: 15}
	hot.CreditLimit = money.New(100000, "USD") // $800 owed of $1,000 → 80%
	cool := domain.Account{ID: "cool", Type: domain.TypeCreditCard, Class: domain.ClassLiability, Currency: "USD",
		OpeningBalance: money.New(-5000, "USD"), BalanceAsOf: old, DueDayOfMonth: 5}
	cool.CreditLimit = money.New(100000, "USD") // $50 of $1,000 → 5%
	return Data{
		Accounts: []domain.Account{hot, cool},
		Rates:    currency.Rates{Base: "USD"},
		Now:      now,
	}
}

// TestCreditProxyMoleculeMatchesModel is the headline guarantee: the
// credit_proxy MOLECULE evaluated through the normal Vars() pipeline equals
// credithealth.Evaluate's ProxyScore for the same derived inputs.
func TestCreditProxyMoleculeMatchesModel(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	d := creditTestData(now)
	v := Vars(d)
	r := credithealth.Evaluate(CreditInputs(d))

	if got := v["credit_proxy"]; got != float64(r.ProxyScore) {
		t.Errorf("credit_proxy molecule = %v, model ProxyScore = %d", got, r.ProxyScore)
	}
	if got := v["credit_util_score"]; got != float64(credithealth.UtilScore(r.Agg.UtilPct)) {
		t.Errorf("credit_util_score = %v, want %d", got, credithealth.UtilScore(r.Agg.UtilPct))
	}
	if got := v["credit_util_weight"]; got != r.Weights.Util {
		t.Errorf("credit_util_weight = %v, want %v", got, r.Weights.Util)
	}
	// $850 owed of $2,000 → 42% aggregate; the pay-to-30 target is the excess
	// over 30% on the hot card only: $800 − $300 = $500.
	if got := v["credit_pay_to_30"]; got != 500 {
		t.Errorf("credit_pay_to_30 = %v, want 500", got)
	}
	// Pay-to-10: hot 800−100=700, cool already at 5% → 700 total.
	if got := v["credit_pay_to_10"]; got != 700 {
		t.Errorf("credit_pay_to_10 = %v, want 700", got)
	}
}

// TestCreditVarsNoCards: with no credit cards every credit_* variable still
// exists, on-time/age carry zero weight, and the molecule floors to 0 —
// mirroring the model.
func TestCreditVarsNoCards(t *testing.T) {
	d := Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()}
	v := Vars(d)
	for _, k := range CreditVarNames {
		if _, ok := v[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
	r := credithealth.Evaluate(CreditInputs(d))
	if got := v["credit_proxy"]; got != float64(r.ProxyScore) {
		t.Errorf("credit_proxy = %v, model = %d", got, r.ProxyScore)
	}
	if v["credit_ontime_weight"] != 0 || v["credit_age_weight"] != 0 {
		t.Errorf("missing factors should carry zero weight")
	}
}
