// SPDX-License-Identifier: MIT

package engineenv

import (
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"math"
	"testing"
	"time"
)

func TestCreditVarsCheckedMixedCurrencyMatchesNativeAndRefreshesRates(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	d := creditTestData(now)
	d.Accounts[0].Currency = "EUR"
	d.Accounts[0].OpeningBalance = money.New(-80000, "EUR")
	d.Accounts[0].CreditLimit = money.New(100000, "EUR")
	for _, rate := range []float64{1, 2} {
		d.Rates.Rates = map[string]float64{"EUR": rate}
		bals, err := ledger.Balances(d.Accounts, d.Transactions)
		if err != nil {
			t.Fatal(err)
		}
		native := credithealth.EvaluateWithRates(creditInputs(d, bals), d.Rates)
		vars := Vars(d)
		if native.UtilErr != nil || vars["credit_util_score"] != float64(native.UtilScore) || vars["credit_pay_to_30"] != float64(native.PayTo30.Amount)/100 || vars["credit_proxy"] != float64(native.ProxyScore) {
			t.Fatalf("rate%v vars mismatch: %v %+v", rate, vars, native)
		}
		if vars["credit_pay_to_30"] != 500*rate {
			t.Errorf("native target relabeled at rate%v: %v", rate, vars["credit_pay_to_30"])
		}
	}
	d.Rates.Rates = nil
	for _, name := range []string{"credit_util_score", "credit_pay_to_30", "credit_pay_to_10"} {
		d.Molecules = append(d.Molecules, domain.Molecule{Name: name, Formula: "999"})
	}
	vars := Vars(d)
	for _, name := range []string{"credit_util_score", "credit_pay_to_30", "credit_pay_to_10", "credit_proxy"} {
		if got, ok := vars[name]; ok {
			t.Errorf("unknown currency %s=%v", name, got)
		}
	}
	for _, name := range []string{"credit_ontime_score", "credit_ontime_weight", "credit_age_score", "credit_age_weight", "credit_util_weight"} {
		if _, ok := vars[name]; !ok {
			t.Errorf("independent factor %s missing", name)
		}
	}
}
func TestCreditVarsCheckedUsesFXWeightedUtilizationAndExplicitForeignLimit(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	d := creditTestData(now)
	d.Accounts[1].Currency = "EUR"
	d.Accounts[1].OpeningBalance = money.Zero("EUR")
	d.Accounts[1].CreditLimit = money.New(100000, "EUR")
	d.Rates.Rates = map[string]float64{"EUR": 2}
	vars := Vars(d)
	if got := vars["credit_util_score"]; got != float64(credithealth.UtilScore(27)) {
		t.Errorf("aggregate native units mixed: score%v", got)
	}
	d.Accounts = d.Accounts[1:]
	d.Accounts[0].OpeningBalance = money.New(-8000, "EUR")
	d.Accounts[0].CreditLimit = money.New(10000, "USD")
	vars = Vars(d)
	if vars["credit_pay_to_30"] != 130 || vars["credit_pay_to_10"] != 150 {
		t.Errorf("foreign limit currency lost: %v", vars)
	}
}
func TestCreditVarsCheckedUnavailableBalancesAndPaydownSums(t *testing.T) {
	for _, kind := range []string{"balance", "minimum", "paydown overflow"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			d := creditTestData(now)
			switch kind {
			case "balance":
				d.Accounts[0].OpeningBalance = money.New(math.MinInt64, "USD")
				d.Transactions = []domain.Transaction{{ID: "underflow", AccountID: d.Accounts[0].ID, Amount: money.New(-1, "USD"), Date: now, Desc: "Charge"}}
			case "minimum":
				d.Accounts[0].OpeningBalance = money.New(math.MinInt64, "USD")
			case "paydown overflow":
				for i := range d.Accounts {
					d.Accounts[i].OpeningBalance = money.New(-(math.MaxInt64 / 5 * 4), "USD")
					d.Accounts[i].CreditLimit = money.New(math.MaxInt64, "USD")
				}
			}
			vars := Vars(d)
			for _, name := range []string{"credit_util_score", "credit_pay_to_10", "credit_proxy"} {
				if got, ok := vars[name]; ok {
					t.Errorf("%s unavailable %s=%v", kind, name, got)
				}
			}
			if kind == "paydown overflow" {
				if got, ok := vars["credit_pay_to_30"]; !ok || got <= 0 {
					t.Errorf("independent target30 lost: %v %v", got, ok)
				}
			} else if _, ok := vars["credit_pay_to_30"]; ok {
				t.Error("unavailable balance became known target")
			}
			if _, ok := vars["credit_age_score"]; !ok {
				t.Error("independent age lost")
			}
		})
	}
}
func TestCreditVarsCheckedKnownZeroTargetsDoNotRequireUnknownFX(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	d := creditTestData(now)
	d.Accounts = d.Accounts[:1]
	d.Accounts[0].Currency = "EUR"
	d.Accounts[0].OpeningBalance = money.Zero("EUR")
	d.Accounts[0].CreditLimit = money.New(100000, "EUR")
	vars := Vars(d)
	if _, ok := vars["credit_util_score"]; ok {
		t.Error("unknown aggregate FX became known utilization")
	}
	for _, name := range []string{"credit_pay_to_30", "credit_pay_to_10"} {
		if got, ok := vars[name]; !ok || got != 0 {
			t.Errorf("known zero target %s=%v present%v", name, got, ok)
		}
	}
}
func TestCreditVarsCheckedRejectsWrongBalanceCurrencyWithoutTolerantFallback(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	d := creditTestData(now)
	d.Accounts = d.Accounts[:1]
	out := map[string]float64{}
	addCreditVars(out, d, func(x int64) float64 { return float64(x) / 100 }, func(int64, string) int64 { t.Fatal("tolerant FX converter called"); return 0 }, map[string]money.Money{d.Accounts[0].ID: money.New(-80000, "EUR")})
	for _, name := range []string{"credit_util_score", "credit_pay_to_30", "credit_pay_to_10"} {
		if _, ok := out[name]; ok {
			t.Errorf("wrong native balance currency fabricated %s", name)
		}
	}
	if _, ok := out["credit_age_score"]; !ok {
		t.Error("known age disappeared")
	}
}
