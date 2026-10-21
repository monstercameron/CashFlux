// SPDX-License-Identifier: MIT

package credithealth_test

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func checkedCreditCard(id, code string, limit int64) domain.Account {
	a := makeCard(id, id, 0, 24, 15, 0)
	a.Currency = code
	a.CreditLimit = money.New(limit, code)
	return a
}
func TestCreditCheckedSameCurrencyParityAndExplicitTargets(t *testing.T) {
	for _, amount := range []int64{0, -5000, -25000, -80000, 80000, -150000} {
		a := checkedCreditCard("card", "USD", 100000)
		in := credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: amount}, Transactions: paymentsOnTime(a.ID, 15, baseNow), Now: baseNow}
		got := credithealth.EvaluateWithRates(in, currency.Rates{Base: "USD"})
		old := credithealth.Evaluate(in)
		if got.UtilErr != nil || got.PayTo30Err != nil || got.PayTo10Err != nil || !got.ProxyKnown || got.UtilPct != int64(old.Agg.UtilPct) || got.ProxyScore != old.ProxyScore || got.Weights != old.Weights || got.OnTimeScore != old.OnTimeScore || got.AgeScore != old.AgeScore {
			t.Fatalf("parity amount%d: %+v / %+v", amount, got, old)
		}
		if got.Cards[0].Target30 != money.New(old.Cards[0].Target30Minor, "USD") || got.PayTo30 != got.Cards[0].Target30 || got.PayTo10 != got.Cards[0].Target10 {
			t.Fatalf("target units: %+v", got)
		}
	}
}
func TestCreditCheckedMixedCurrencyAggregateAndForeignLimit(t *testing.T) {
	usd, eur := checkedCreditCard("usd", "USD", 100000), checkedCreditCard("eur", "EUR", 100000)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	got := credithealth.EvaluateWithRates(credithealth.Inputs{Accounts: []domain.Account{usd, eur}, Balances: map[string]int64{"usd": -80000, "eur": 0}, Now: baseNow}, rates)
	if got.UtilErr != nil || got.UtilPct != 27 || got.Balance != money.New(80000, "USD") || got.Limit != money.New(300000, "USD") || got.PayTo30 != money.New(50000, "USD") || got.Cards[1].Limit.Currency != "EUR" {
		t.Fatalf("mixed native units: %+v", got)
	}
	eur.CreditLimit = money.New(10000, "USD")
	got = credithealth.EvaluateWithRates(credithealth.Inputs{Accounts: []domain.Account{eur}, Balances: map[string]int64{"eur": -8000}, Now: baseNow}, rates)
	if got.UtilErr != nil || got.UtilPct != 160 || got.Cards[0].Limit != money.New(10000, "USD") || got.Cards[0].Target30 != money.New(6500, "EUR") || got.PayTo30 != money.New(13000, "USD") || got.PayTo10 != money.New(15000, "USD") {
		t.Fatalf("foreign limit mislabeled: %+v", got)
	}
}
func TestCreditCheckedDifferentMinorUnitsAndRoundedZeroLimit(t *testing.T) {
	jpy := checkedCreditCard("yen", "JPY", 1000)
	in := credithealth.Inputs{Accounts: []domain.Account{jpy}, Balances: map[string]int64{"yen": -800}, Now: baseNow}
	got := credithealth.EvaluateWithRates(in, currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.01}})
	if got.UtilErr != nil || got.UtilPct != 80 || got.PayTo30 != money.New(500, "USD") || got.Cards[0].Target30 != money.New(500, "JPY") {
		t.Fatalf("minor units: %+v", got)
	}
	in.Balances["yen"] = 0
	jpy.CreditLimit = money.New(1, "JPY")
	in.Accounts = []domain.Account{jpy}
	got = credithealth.EvaluateWithRates(in, currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.00001}})
	if got.UtilErr == nil || got.ProxyKnown || got.PayTo30Err != nil || got.PayTo30.Amount != 0 {
		t.Fatalf("rounded-away limit became known utilization: %+v", got)
	}
}
func TestCreditCheckedMissingFXPreservesIndependentFactsAndZeroTargets(t *testing.T) {
	a := checkedCreditCard("card", "EUR", 100000)
	for _, amount := range []int64{-80000, 0} {
		got := credithealth.EvaluateWithRates(credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: amount}, Now: baseNow}, currency.Rates{Base: "USD"})
		if !errors.Is(got.UtilErr, currency.ErrUnknownRate) || got.ProxyKnown || got.AgeScore < 0 || got.OnTimeScore < 0 || got.Weights.Util == 0 {
			t.Fatalf("independent factors lost: %+v", got)
		}
		if amount == 0 {
			if got.PayTo30Err != nil || got.PayTo10Err != nil || got.PayTo30.Amount != 0 || got.PayTo10.Amount != 0 {
				t.Fatalf("native known zero required FX: %+v", got)
			}
		} else if !errors.Is(got.PayTo30Err, currency.ErrUnknownRate) || !errors.Is(got.PayTo10Err, currency.ErrUnknownRate) {
			t.Fatalf("unknown paydown fabricated: %+v", got)
		}
		if got.Cards[0].Err != nil || got.Cards[0].Balance.Currency != "EUR" {
			t.Fatalf("native card facts lost: %+v", got.Cards[0])
		}
	}
}
func TestCreditCheckedInvalidSourceMissingBalanceAndOverflow(t *testing.T) {
	for _, kind := range []string{"missing balance", "minimum", "limit currency", "FX overflow", "sum overflow"} {
		t.Run(kind, func(t *testing.T) {
			a := checkedCreditCard("card", "USD", 100000)
			in := credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: -80000}, Now: baseNow}
			rates := currency.Rates{Base: "USD"}
			switch kind {
			case "missing balance":
				in.Balances = nil
			case "minimum":
				in.Balances[a.ID] = math.MinInt64
			case "limit currency":
				a.CreditLimit.Currency = ""
				in.Accounts[0] = a
			case "FX overflow":
				a.Currency = "EUR"
				a.CreditLimit = money.New(math.MaxInt64, "EUR")
				in.Accounts[0] = a
				in.Balances[a.ID] = -math.MaxInt64
				rates.Rates = map[string]float64{"EUR": 2}
			case "sum overflow":
				a.CreditLimit = money.New(math.MaxInt64, "USD")
				in.Accounts[0] = a
				other := a
				other.ID = "other"
				in.Accounts = append(in.Accounts, other)
				in.Balances = map[string]int64{a.ID: 0, other.ID: 0}
			}
			got := credithealth.EvaluateWithRates(in, rates)
			if got.UtilErr == nil || got.ProxyKnown || got.Balance.Amount != 0 || got.Limit.Amount != 0 || got.AgeScore < 0 {
				t.Fatalf("invalid aggregate published: %+v", got)
			}
			if kind == "missing balance" && !errors.Is(got.UtilErr, credithealth.ErrMissingBalance) {
				t.Fatalf("missing vs zero: %v", got.UtilErr)
			}
			if kind == "minimum" && !errors.Is(got.UtilErr, money.ErrOverflow) {
				t.Fatalf("minimum overflow: %v", got.UtilErr)
			}
			if kind == "sum overflow" && (got.PayTo30Err != nil || got.PayTo10Err != nil) {
				t.Fatalf("independent native zero paydown lost: %+v", got)
			}
		})
	}
}
func TestCreditCheckedPaydownTotalsFailIndependently(t *testing.T) {
	a, b := checkedCreditCard("a", "USD", math.MaxInt64), checkedCreditCard("b", "USD", math.MaxInt64)
	owed := int64(math.MaxInt64 / 5 * 4)
	got := credithealth.EvaluateWithRates(credithealth.Inputs{Accounts: []domain.Account{a, b}, Balances: map[string]int64{a.ID: -owed, b.ID: -owed}, Now: baseNow}, currency.Rates{Base: "USD"})
	if got.UtilErr == nil || got.PayTo30Err != nil || got.PayTo30.Amount <= 0 || !errors.Is(got.PayTo10Err, money.ErrOverflow) || got.PayTo10.Amount != 0 {
		t.Fatalf("independent totals: %+v", got)
	}
}
func TestCreditCheckedNoCardsAndArchivedNoLimitConventions(t *testing.T) {
	a := checkedCreditCard("archived", "EUR", 100000)
	a.Archived = true
	noLimit := checkedCreditCard("no limit", "USD", 0)
	got := credithealth.EvaluateWithRates(credithealth.Inputs{Accounts: []domain.Account{a, noLimit}, Balances: map[string]int64{noLimit.ID: -1000}, Now: baseNow}, currency.Rates{Base: "USD"})
	if got.UtilErr != nil || !got.ProxyKnown || got.UtilPct != -1 || got.UtilScore != 0 || got.CardsMissingLimit != 1 || len(got.Cards) != 1 || got.PayTo30Err != nil || got.PayTo30.Amount != 0 {
		t.Fatalf("no-limit convention: %+v", got)
	}
}

func TestCreditCheckedNativeRatioOverflowDoesNotHideKnownPaydown(t *testing.T) {
	a := checkedCreditCard("wide", "USD", 1)
	got := credithealth.EvaluateWithRates(credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: -math.MaxInt64}, Now: baseNow}, currency.Rates{Base: "USD"})
	if !errors.Is(got.UtilErr, money.ErrOverflow) || got.Cards[0].Err != nil || !errors.Is(got.Cards[0].UtilErr, money.ErrOverflow) || got.PayTo30Err != nil || got.PayTo10Err != nil || got.PayTo30.Amount != math.MaxInt64 || got.PayTo10.Amount != math.MaxInt64 {
		t.Fatalf("ratio overflow hid payable native amounts: %+v", got)
	}
}
func TestCreditCheckedRejectsNegativeLimitAndMissingBase(t *testing.T) {
	a := checkedCreditCard("negative", "USD", -1)
	in := credithealth.Inputs{Accounts: []domain.Account{a}, Balances: map[string]int64{a.ID: 0}, Now: baseNow}
	got := credithealth.EvaluateWithRates(in, currency.Rates{Base: "USD"})
	if got.UtilErr == nil || got.PayTo30Err == nil || got.ProxyKnown {
		t.Fatalf("invalid limit became no data: %+v", got)
	}
	got = credithealth.EvaluateWithRates(credithealth.Inputs{Now: baseNow}, currency.Rates{})
	if got.UtilErr == nil || got.PayTo30Err == nil || got.PayTo10Err == nil || got.ProxyKnown {
		t.Fatalf("base-free money became available: %+v", got)
	}
}
