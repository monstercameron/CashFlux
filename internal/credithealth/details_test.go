// SPDX-License-Identifier: MIT

package credithealth_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCheckedDetailsOrdinaryPolicyAndNativePayment(t *testing.T) {
	a := checkedCreditCard("card", "USD", 100000)
	in := credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: -80000}, Now: baseNow}
	rates := currency.Rates{Base: "USD"}
	r := credithealth.EvaluateWithRates(in, rates)
	got := credithealth.PrepareDetails(in, rates, r)
	old := credithealth.Evaluate(in)
	if len(got.Demerits) != len(old.Demerits) || len(got.Advice) != len(old.Advice) {
		t.Fatalf("policy changed: %+v %+v", got, old)
	}
	for i, d := range got.Demerits {
		prior := old.Demerits[i]
		if d.Kind != prior.Kind || d.PointsLost != prior.PointsLost || d.Pct != int64(prior.Pct) {
			t.Fatalf("demerit changed: %+v %+v", d, prior)
		}
	}
	for _, advice := range got.Advice {
		if advice.Kind == credithealth.AdviceLowerUtilization {
			if advice.Pay != money.New(50000, "USD") || !advice.ImpactKnown || !advice.ProjectedKnown {
				t.Fatalf("invalid advice %+v", advice)
			}
		}
	}
}
func TestCheckedDetailsFXImpactUsesCanonicalHypotheticalAndCopiesInputs(t *testing.T) {
	usd, eur := checkedCreditCard("usd", "USD", 100000), checkedCreditCard("eur", "EUR", 100000)
	in := credithealth.Inputs{Accounts: []domain.Account{usd, eur}, Balances: map[string]int64{"usd": -80000, "eur": -50000}, Now: baseNow}
	before := map[string]int64{"usd": -80000, "eur": -50000}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	r := credithealth.EvaluateWithRates(in, rates)
	details := credithealth.PrepareDetails(in, rates, r)
	if !reflect.DeepEqual(before, in.Balances) {
		t.Fatal("input balances mutated")
	}
	for _, advice := range details.Advice {
		if advice.Kind != credithealth.AdviceLowerUtilization {
			continue
		}
		next := in
		next.Balances = map[string]int64{"usd": -80000, "eur": -50000}
		next.Balances[advice.AccountID] = -30000
		expected := credithealth.EvaluateWithRates(next, rates)
		if !advice.ProjectedKnown || advice.ProjectedScore != expected.ProxyScore || advice.ImpactPts != max(0, expected.ProxyScore-r.ProxyScore) {
			t.Fatalf("hypothetical drift: %+v %+v", advice, expected)
		}
		if advice.AccountID == "eur" && advice.Pay != money.New(20000, "EUR") {
			t.Fatalf("foreign payment relabeled: %+v", advice)
		}
	}
}
func TestCheckedDetailsUnknownAggregateRetainsNativePaymentAndIndependentFactors(t *testing.T) {
	a := checkedCreditCard("eur", "EUR", 100000)
	in := credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: -80000}, Now: baseNow}
	rates := currency.Rates{Base: "USD"}
	r := credithealth.EvaluateWithRates(in, rates)
	d := credithealth.PrepareDetails(in, rates, r)
	if !d.Incomplete {
		t.Fatal("unknown FX asserted complete advice")
	}
	found := false
	for _, advice := range d.Advice {
		if advice.Kind == credithealth.AdviceLowerUtilization {
			found = true
			if advice.Pay != money.New(50000, "EUR") || advice.ImpactKnown || advice.ProjectedKnown {
				t.Fatalf("unknown score impact invented: %+v", advice)
			}
		}
	}
	if !found {
		t.Fatal("known native payment disappeared")
	}
	for _, demerit := range d.Demerits {
		if demerit.Kind == credithealth.DemeritUtilization {
			t.Fatal("unavailable aggregate became confirmed demerit")
		}
	}
	in.TransactionsErr = errors.New("unread history")
	r = credithealth.EvaluateWithRates(in, rates)
	d = credithealth.PrepareDetails(in, rates, r)
	if r.OnTimeErr == nil || r.OnTimeScore != -1 || r.ProxyKnown || r.Weights.OnTime == 0 || r.AgeScore < 0 {
		t.Fatalf("unread transactions became empty payments: %+v", r)
	}
	for _, advice := range d.Advice {
		if advice.Kind == credithealth.AdviceOnTime {
			t.Fatal("unknown on-time generated advice")
		}
	}
}

func TestCheckedDetailsUnsupportedCurrencyAndWideRatioAreExplicit(t *testing.T) {
	for _, code := range []string{"", "NOTACURRENCY"} {
		a := checkedCreditCard("card", code, 10000)
		in := credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: -8000}, Now: baseNow}
		rates := currency.Rates{Base: "USD", Rates: map[string]float64{code: 1}}
		r := credithealth.EvaluateWithRates(in, rates)
		if r.ProxyKnown || !errors.Is(r.Cards[0].Err, credithealth.ErrUnsupportedCurrency) || !credithealth.PrepareDetails(in, rates, r).Incomplete {
			t.Fatalf("unsupported currency %q fabricated facts: %+v", code, r)
		}
	}
	a := checkedCreditCard("card", "USD", 1)
	in := credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: -9223372036854775807}, Now: baseNow}
	rates := currency.Rates{Base: "USD"}
	r := credithealth.EvaluateWithRates(in, rates)
	d := credithealth.PrepareDetails(in, rates, r)
	if r.ProxyKnown || r.Cards[0].UtilErr == nil || !d.Incomplete {
		t.Fatalf("unrepresentable ratio became ordinary utilization: %+v %+v", r, d)
	}
	for _, a := range d.Advice {
		if a.Kind == credithealth.AdviceLowerUtilization && (a.Pay.Amount != 9223372036854775807 || a.ImpactKnown || a.ProjectedKnown) {
			t.Fatalf("wide payment lost exactness or gained a fake score: %+v", a)
		}
	}
}
