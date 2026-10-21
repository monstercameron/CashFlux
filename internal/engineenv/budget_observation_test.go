// SPDX-License-Identifier: MIT

package engineenv

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/money"
)

func observationBudget() domain.Budget {
	return domain.Budget{ID: "food", Name: "Food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}
}

func TestBudgetObservationDistinguishesUnknownSpendingFromZero(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		amount []int64
		code   string
		rates  map[string]float64
		want   error
	}{
		{name: "known zero", code: "USD"},
		{name: "missing FX", amount: []int64{-100}, code: "EUR", want: currency.ErrUnknownRate},
		{name: "invalid FX", amount: []int64{-100}, code: "EUR", rates: map[string]float64{"EUR": math.NaN()}, want: currency.ErrInvalidRate},
		{name: "sum overflow", amount: []int64{-math.MaxInt64, -1}, code: "USD", want: money.ErrOverflow},
		{name: "magnitude overflow", amount: []int64{math.MinInt64}, code: "USD", want: money.ErrOverflow},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := Data{Now: now, Rates: currency.Rates{Base: "USD", Rates: test.rates}}
			for _, amount := range test.amount {
				data.Transactions = append(data.Transactions, domain.Transaction{CategoryID: "food", Date: now, Amount: money.New(amount, test.code)})
			}
			b := observationBudget()
			got := observeBudget(b, data, func(n int64) float64 { return float64(n) / 100 })
			if got.Limit != 100 || !errors.Is(got.Err, test.want) || got.SpendingKnown != (test.want == nil) {
				t.Fatalf("observation = %+v, want error %v and known=%v", got, test.want, test.want == nil)
			}
			out := map[string]float64{}
			data.Budgets = []domain.Budget{b}
			addBudgetVars(out, data, func(n int64) float64 { return float64(n) / 100 })
			if value, present := out["budget_food_limit"]; !present || value != 100 {
				t.Fatal("unknown spending hid the independently known limit")
			}
			for _, field := range []string{"spent", "remaining", "over", "percent"} {
				value, present := out["budget_food_"+field]
				if present != got.SpendingKnown {
					t.Errorf("%s = %v, present=%v; spending known=%v", field, value, present, got.SpendingKnown)
				}
			}
			if test.want == nil {
				if got.Spent != 0 || got.Remaining != 100 || got.Over != 0 || got.Percent != 0 {
					t.Fatalf("measured zero changed: %+v", got)
				}
				value, err := formula.Eval("budget_food_over <= 0", formula.Env{Vars: out})
				if err != nil || value != true {
					t.Fatalf("known zero no longer satisfies existing task condition: %v %v", value, err)
				}
			} else if _, err := formula.Eval("budget_food_over <= 0", formula.Env{Vars: out}); err == nil {
				t.Fatal("unknown overspending became a trustworthy task condition")
			}
		})
	}
}

// Keep the previous successful arithmetic as an independent parity reference;
// this slice changes availability, not period, ownership, netting, FX or units.
func priorKnownBudgetValues(b domain.Budget, data Data, major func(int64) float64) map[string]float64 {
	start, end := budgeting.PeriodRange(b.Period, data.Now, data.WeekStart)
	spent, err := budgeting.Spent(b, data.budgetLedger(), start, end, data.Rates)
	if err != nil {
		panic(err)
	}
	spending, limit := major(spent.Amount), major(b.Limit.Amount)
	over, percent := 0.0, 0.0
	if spending > limit {
		over = spending - limit
	}
	if limit != 0 {
		percent = spending / limit * 100
	}
	return map[string]float64{"limit": limit, "spent": spending, "remaining": limit - spending, "over": over, "percent": percent}
}

func TestBudgetObservationPreservesSuccessfulValuesAndInputs(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		limit  int64
		amount int64
		code   string
	}{
		{"under limit", 10000, -2500, "USD"},
		{"overspent", 10000, -15000, "USD"},
		{"converted", 10000, -15001, "EUR"},
		{"refund", 10000, 2500, "USD"},
		{"zero limit", 0, -2500, "USD"},
		{"large representable", math.MaxInt64, -math.MaxInt64, "USD"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := observationBudget()
			b.Limit = money.New(test.limit, "USD")
			data := Data{Now: now, Budgets: []domain.Budget{b}, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}},
				Transactions: []domain.Transaction{{ID: "match", CategoryID: "food", Date: now, Amount: money.New(test.amount, test.code)},
					{ID: "outside", CategoryID: "food", Date: now.AddDate(0, -1, 0), Amount: money.New(math.MinInt64, "XXX")},
					{ID: "other", CategoryID: "other", Date: now, Amount: money.New(math.MinInt64, "XXX")}}}
			before := append([]domain.Transaction(nil), data.Transactions...)
			major := func(n int64) float64 { return float64(n) / 100 }
			want := priorKnownBudgetValues(b, data, major)
			out := map[string]float64{}
			addBudgetVars(out, data, major)
			for field, value := range want {
				if actual, found := out["budget_food_"+field]; !found || actual != value {
					t.Errorf("%s = %v, present=%v; want prior result %v", field, actual, found, value)
				}
			}
			if !reflect.DeepEqual(data.Transactions, before) || !reflect.DeepEqual(data.Budgets[0], b) {
				t.Fatal("observing a budget changed the caller's inputs")
			}
		})
	}
}

func TestUnavailableBudgetVarsKeepSlugIdentityAndCannotBeReplacedByMolecule(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	b := observationBudget()
	other := b
	other.ID, other.CategoryID, other.Limit = "other", "other", money.New(20000, "USD")
	data := Data{Now: now, Budgets: []domain.Budget{b, other}, Rates: currency.Rates{Base: "USD"},
		Transactions: []domain.Transaction{{ID: "foreign", CategoryID: "food", Date: now, Amount: money.New(-15000, "EUR")}},
		Molecules: []domain.Molecule{{Name: "budget_food_over", Formula: "0"},
			{Name: "food_cleared", Formula: "budget_food_over <= 0"}}}
	vars := Vars(data)
	for _, name := range []string{"budget_food_spent", "budget_food_remaining", "budget_food_over", "budget_food_percent", "food_cleared"} {
		if value, found := vars[name]; found {
			t.Errorf("unavailable %s was replaced with %v", name, value)
		}
	}
	if vars["budget_food_limit"] != 100 || vars["budget_food_2_limit"] != 200 {
		t.Fatal("unknown first budget moved the duplicate slug identities")
	}
	for _, field := range BudgetVarFields {
		if _, present := vars["budget_food_2_"+field]; !present {
			t.Errorf("clean duplicate lost its known %s", field)
		}
	}
	data.Rates.Rates = map[string]float64{"EUR": 1}
	vars = Vars(data)
	if vars["budget_food_over"] != 50 || vars["food_cleared"] != 0 {
		t.Fatalf("known spending or successful molecule-shadow rules changed: over=%v cleared=%v", vars["budget_food_over"], vars["food_cleared"])
	}
}
