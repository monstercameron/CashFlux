// SPDX-License-Identifier: MIT

package engineenv

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func healthCheckedData() Data { return healthTestData(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)) }

func assertHealthUnavailable(t *testing.T, d Data, key string) {
	t.Helper()
	in := HealthInputs(d)
	if in.Unavailable[key] == nil {
		t.Fatalf("%s should be unavailable: %+v", key, in)
	}
	for _, f := range healthscore.Evaluate(in).Factors {
		if f.Key == key && (f.Applicable || f.TargetMet || f.Weight != 0 || f.Score != 0) {
			t.Fatalf("failed factor scored: %+v", f)
		}
	}
	bals, _ := ledger.Balances(d.Accounts, d.Transactions)
	out := map[string]float64{}
	addHealthVars(out, d, bals)
	for _, name := range []string{healthFactorVar(key), healthFactorVar(key) + "_weight"} {
		if _, ok := out[name]; ok {
			t.Fatalf("unavailable %s became a valid atom", name)
		}
	}
	if _, ok := out["health_savings"]; !ok && key != "savings" {
		t.Fatal("independent savings factor disappeared")
	}
}

func TestHealthCheckedMissingPaymentFXPreservesIndependentFactors(t *testing.T) {
	d := healthCheckedData()
	d.Accounts = append(d.Accounts, domain.Account{ID: "foreign-loan", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "EUR", OpeningBalance: money.New(-10000, "EUR"), MinPayment: money.New(1000, "EUR")})
	assertHealthUnavailable(t, d, "debt")
	in := HealthInputs(d)
	if !in.HasIncome || in.SavingsRatePct != 25 || !in.HasCredit || in.AggUtilizationPct != 30 || in.Unavailable["savings"] != nil || in.Unavailable["utilization"] != nil {
		t.Fatalf("independent factors changed: %+v", in)
	}
	d.Rates.Rates = map[string]float64{"EUR": 2}
	in = HealthInputs(d)
	if in.Unavailable["debt"] != nil || in.ObligationRatioPct != 1 {
		t.Fatalf("fresh complete payment ratio not restored: %+v", in)
	}
}

func TestHealthCheckedUnavailableAtomCannotBeRefilledByMolecule(t *testing.T) {
	d := healthCheckedData()
	d.Accounts = append(d.Accounts, domain.Account{ID: "foreign-loan", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "EUR", OpeningBalance: money.New(-10000, "EUR"), MinPayment: money.New(1000, "EUR")})
	d.Molecules = []domain.Molecule{{Name: "health_debt", Formula: "100"}}
	variables := Vars(d)
	for _, name := range []string{"health_debt", "health_obligation_pct", "health_score"} {
		if value, ok := variables[name]; ok {
			t.Fatalf("unavailable %s was refilled as %v", name, value)
		}
	}
	if value, ok := variables["health_savings"]; !ok || value != 100 {
		t.Fatalf("independent savings factor lost: %v, %v", value, ok)
	}
}

func TestHealthCheckedMinimumMagnitudeAndAggregateOverflow(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		payment, limit, balance int64
		count                   int
		key                     string
	}{
		{"minimum payment", math.MinInt64, 0, 0, 1, "debt"},
		{"payment sum", math.MaxInt64/2 + 1, 0, 0, 2, "debt"},
		{"minimum card balance", 0, 1000, math.MinInt64, 1, "utilization"},
		{"card limits", 0, math.MaxInt64/2 + 1, 0, 2, "utilization"},
		{"card balances", 0, 1000, -(math.MaxInt64/2 + 1), 2, "utilization"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := healthCheckedData()
			for i := 0; i < tc.count; i++ {
				d.Accounts = append(d.Accounts, domain.Account{ID: tc.name + string(rune('a'+i)), Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(tc.balance, "USD"), MinPayment: money.New(tc.payment, "USD"), CreditLimit: money.New(tc.limit, "USD")})
			}
			assertHealthUnavailable(t, d, tc.key)
			if err := HealthInputs(d).Unavailable[tc.key]; !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow cause lost: %v", err)
			}
		})
	}
}

func TestHealthCheckedBudgetRequiresEveryEvaluation(t *testing.T) {
	d := healthCheckedData()
	d.Categories = []domain.Category{{ID: "food", Kind: domain.KindExpense}, {ID: "other", Kind: domain.KindExpense}}
	d.Budgets = []domain.Budget{{ID: "known", CategoryID: "food", Limit: money.New(10000, "USD"), Period: domain.PeriodMonthly}, {ID: "unknown", CategoryID: "other", Limit: money.New(10000, "EUR"), Period: domain.PeriodMonthly}}
	// The known USD spend requires FX to evaluate the EUR budget. An empty
	// budget needs no FX just to establish that zero is below its native limit.
	d.Transactions = append(d.Transactions, domain.Transaction{ID: "other-spend", AccountID: "chk", CategoryID: "other", Date: d.Now.Add(-24 * time.Hour), Amount: money.New(-100, "USD")})
	assertHealthUnavailable(t, d, "budget")
	in := HealthInputs(d)
	if in.HasBudgets || !errors.Is(in.Unavailable["budget"], currency.ErrUnknownRate) {
		t.Fatalf("partial budget set claimed adherence: %+v", in)
	}
	d.Rates.Rates = map[string]float64{"EUR": 2}
	in = HealthInputs(d)
	if !in.HasBudgets || in.BudgetAdherencePct != 100 || in.Unavailable["budget"] != nil {
		t.Fatalf("complete budgets did not restore adherence: %+v", in)
	}
}

func TestHealthCheckedFlowFailurePreservesCreditAndOmitsDependentAtoms(t *testing.T) {
	d := healthCheckedData()
	d.Transactions = append(d.Transactions, domain.Transaction{ID: "foreign-expense", AccountID: "orphan", Date: d.Now.AddDate(0, -1, 0), Amount: money.New(-1000, "EUR")})
	in := HealthInputs(d)
	for _, key := range []string{"savings", "emergency", "debt"} {
		if !errors.Is(in.Unavailable[key], currency.ErrUnknownRate) {
			t.Fatalf("%s failure not retained: %+v", key, in)
		}
	}
	if !in.HasCredit || in.AggUtilizationPct != 30 || in.Unavailable["utilization"] != nil {
		t.Fatalf("independent credit removed: %+v", in)
	}
	bals, _ := ledger.Balances(d.Accounts, d.Transactions)
	out := map[string]float64{}
	addHealthVars(out, d, bals)
	for _, name := range []string{"health_savings", "health_debt", "health_emergency_months", "health_obligation_pct", "health_penalty"} {
		if _, ok := out[name]; ok {
			t.Fatalf("failed input published %s", name)
		}
	}
	if _, ok := out["health_utilization"]; !ok {
		t.Fatal("known utilization atom absent")
	}
}

func TestHealthCheckedWidePercentagesAndTrend(t *testing.T) {
	pct, err := healthPercent(math.MaxInt64, math.MaxInt64, money.RoundTowardZero)
	if err != nil || pct != 100 {
		t.Fatalf("representable wide ratio: %d %v", pct, err)
	}
	if _, err = healthPercent(math.MaxInt64, 1, money.RoundTowardZero); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("oversized percent accepted: %v", err)
	}
	pct, err = healthPercent(1783, 10000, money.RoundHalfAwayFromZero)
	if err != nil || pct != 18 {
		t.Fatalf("utilization rounding changed: %d %v", pct, err)
	}
	trend, err := healthTrendPercent(math.MaxInt64, math.MinInt64)
	if err != nil || trend != -200 {
		t.Fatalf("wide negative change wrapped: %v %v", trend, err)
	}
	trend, err = healthTrendPercent(1000, 1051)
	if err != nil || math.Abs(trend-5.1) > 1e-12 {
		t.Fatalf("fractional trend precision changed: %v %v", trend, err)
	}
}

func TestHealthCheckedSavingsOverflowDoesNotInvalidateKnownDebt(t *testing.T) {
	now := healthCheckedData().Now
	d := Data{Now: now, Rates: currency.Rates{Base: "USD"}, Transactions: []domain.Transaction{{Date: now.AddDate(0, -1, 0), Amount: money.New(3, "USD")}, {Date: now.AddDate(0, -1, 0), Amount: money.New(-math.MaxInt64, "USD")}}}
	in := HealthInputs(d)
	if !errors.Is(in.Unavailable["savings"], money.ErrOverflow) || !in.HasIncome || in.Unavailable["debt"] != nil {
		t.Fatalf("shared income hid independent debt: %+v", in)
	}
	r := healthscore.Evaluate(in)
	for _, f := range r.Factors {
		if f.Key == "savings" && f.Applicable {
			t.Fatal("overflow savings factor scored")
		}
		if f.Key == "debt" && (!f.Applicable || f.Score != 100) {
			t.Fatalf("known no-debt factor removed: %+v", f)
		}
	}
	if r.NegativeCashFlow {
		t.Fatal("unavailable savings imposed a definite penalty")
	}
}
