package debtview

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
)

func debtAccount(id, name, cur string, amount int64) domain.Account {
	return domain.Account{ID: id, Name: name, Currency: cur, Class: domain.ClassLiability, Type: domain.TypeCreditCard, OpeningBalance: money.New(amount, cur), InterestRateAPR: floatPtr(20)}
}
func floatPtr(v float64) *float64 { return &v }

func TestPrepareCheckedDebtFacts(t *testing.T) {
	t.Run("preserves mortgage exclusion and strategy rank", func(t *testing.T) {
		a := debtAccount("a", "A", "USD", -10000)
		a.MinPayment = money.New(500, "USD")
		b := debtAccount("b", "B", "USD", -20000)
		b.InterestRateAPR = floatPtr(5)
		b.MinPayment = money.New(500, "USD")
		m := debtAccount("m", "Mortgage", "USD", -90000)
		m.Type = domain.TypeMortgage
		r := Prepare([]domain.Account{a, b, m}, nil, currency.Rates{Base: "USD"}, "USD", Config{Strategy: payoff.Avalanche, ExtraMinor: 200, CompareMinimumOnly: true}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		if !r.TotalKnown || r.TotalOwed.Amount != 120000 {
			t.Fatalf("total=%+v known=%v", r.TotalOwed, r.TotalKnown)
		}
		if !r.PlanKnown || r.PlanOwed.Amount != 30000 || len(r.Debts) != 2 {
			t.Fatalf("plan facts: %+v", r)
		}
		if len(r.Accounts) != 3 || r.Accounts[0].Account.ID != "a" || r.Accounts[0].Rank != 1 {
			t.Fatalf("rank/order=%+v", r.Accounts)
		}
		if !r.PlanValid || r.DebtFree.IsZero() {
			t.Fatalf("plan/date missing: %+v", r)
		}
		if !r.MinimumPlanValid || !r.SavingsKnown || r.MonthsSaved < 0 || r.InterestSaved.Amount < 0 {
			t.Fatalf("minimum-only comparison missing or negative: %+v", r)
		}
		wantPlan, wantPlanOK, wantPlanErr := payoff.BuildPlanChecked(r.Debts, 200, payoff.Avalanche)
		wantMinimum, wantMinimumOK, wantMinimumErr := payoff.BuildPlanChecked(r.Debts, 0, payoff.Avalanche)
		if !wantPlanOK || wantPlanErr != nil || !wantMinimumOK || wantMinimumErr != nil {
			t.Fatalf("test plans unexpectedly invalid: chosen=%+v/%v/%v minimum=%+v/%v/%v", wantPlan, wantPlanOK, wantPlanErr, wantMinimum, wantMinimumOK, wantMinimumErr)
		}
		if !reflect.DeepEqual(r.Plan, wantPlan) || !reflect.DeepEqual(r.MinimumPlan, wantMinimum) {
			t.Fatalf("prepared scenarios differ from the same debt set and strategy: chosen=%+v minimum=%+v", r.Plan, r.MinimumPlan)
		}
	})
	t.Run("does not turn unavailable balance or fx into zero", func(t *testing.T) {
		a := debtAccount("bad", "Bad", "EUR", -500)
		r := Prepare([]domain.Account{a}, nil, currency.Rates{Base: "USD"}, "USD", Config{CompareMinimumOnly: true}, time.Now())
		if r.TotalKnown || r.PlanKnown || r.PlanValid || r.MinimumPlanValid || r.SavingsKnown || len(r.Debts) != 0 || r.Accounts[0].BaseKnown || r.TotalErr == nil || r.SavingsErr == nil {
			t.Fatalf("missing FX was hidden: %+v", r)
		}
		if len(r.MissingRates) != 1 || r.MissingRates[0] != "EUR" || len(r.PlanMissingRates) != 1 || r.PlanMissingRates[0] != "EUR" || !r.TotalOwed.IsZero() || !r.PlanOwed.IsZero() {
			t.Fatalf("missing FX exposed a partial total or no currency issue: %+v", r)
		}
		a.OpeningBalance = money.New(math.MinInt64, "USD")
		r = Prepare([]domain.Account{a}, nil, currency.Rates{Base: "USD"}, "USD", Config{}, time.Now())
		if r.TotalKnown || r.PlanKnown || r.PlanValid || r.MinimumPlanValid || r.SavingsKnown || len(r.Debts) != 0 || r.Accounts[0].BalanceErr == nil {
			t.Fatalf("abs overflow was hidden: %+v", r)
		}
	})
	t.Run("included FX failure invalidates the complete payoff set", func(t *testing.T) {
		valid := debtAccount("usd", "USD", "USD", -10000)
		valid.MinPayment = money.New(500, "USD")
		missing := debtAccount("eur", "EUR", "EUR", -5000)
		missing.MinPayment = money.New(250, "EUR")
		r := Prepare([]domain.Account{valid, missing}, nil, currency.Rates{Base: "USD"}, "USD", Config{ExtraMinor: 100, Strategy: payoff.Avalanche, CompareMinimumOnly: true, CompareStrategies: true}, time.Now())
		if r.PlanKnown || r.PlanValid || r.MinimumPlanValid || r.SavingsKnown || len(r.Debts) != 0 || r.PlanErr == nil || r.SavingsErr == nil {
			t.Fatalf("partial included-debt plan escaped preparation: %+v", r)
		}
		if r.Comparison.Known || r.Comparison.Err == nil || len(r.PlanMissingRates) != 1 || r.PlanMissingRates[0] != "EUR" || !r.PlanOwed.IsZero() {
			t.Fatalf("partial strategy comparison escaped preparation: %+v", r.Comparison)
		}
	})
	t.Run("known debt-free is distinct from unavailable", func(t *testing.T) {
		free := Prepare(nil, nil, currency.Rates{Base: "USD"}, "USD", Config{CompareMinimumOnly: true, CompareStrategies: true}, time.Now())
		if !free.PlanKnown || !free.PlanValid || !free.MinimumPlanValid || !free.SavingsKnown || len(free.Debts) != 0 || !free.PlanOwed.IsZero() || free.SavingsErr != nil {
			t.Fatalf("empty known plan not represented as debt-free: %+v", free)
		}
		if !free.Comparison.Known || !free.Comparison.Match || free.Comparison.HasRecommendation {
			t.Fatalf("debt-free strategy comparison is not a known tie: %+v", free.Comparison)
		}
		unknown := Prepare([]domain.Account{debtAccount("eur", "EUR", "EUR", -100)}, nil, currency.Rates{Base: "USD"}, "USD", Config{CompareMinimumOnly: true, CompareStrategies: true}, time.Now())
		if unknown.PlanKnown || unknown.PlanValid || unknown.MinimumPlanValid || unknown.SavingsKnown {
			t.Fatalf("unavailable plan was confused with debt-free: %+v", unknown)
		}
	})
	t.Run("minimum-only nonviability does not invent savings", func(t *testing.T) {
		account := debtAccount("card", "Card", "USD", -10000)
		account.InterestRateAPR = floatPtr(0)
		r := Prepare([]domain.Account{account}, nil, currency.Rates{Base: "USD"}, "USD", Config{ExtraMinor: 1000, Strategy: payoff.Avalanche, CompareMinimumOnly: true}, time.Now())
		if !r.PlanKnown || !r.PlanValid || r.MinimumPlanValid || r.SavingsKnown || r.SavingsErr == nil {
			t.Fatalf("minimum-only baseline availability was lost: %+v", r)
		}
	})
	t.Run("checked total overflow", func(t *testing.T) {
		a := debtAccount("a", "A", "USD", math.MaxInt64)
		b := debtAccount("b", "B", "USD", math.MaxInt64)
		r := Prepare([]domain.Account{a, b}, nil, currency.Rates{Base: "USD"}, "USD", Config{}, time.Now())
		if r.TotalKnown || !errors.Is(r.TotalErr, money.ErrOverflow) || !r.TotalOwed.IsZero() {
			t.Fatalf("overflow not surfaced: %+v", r)
		}
	})
}

func TestPlanSavingsRejectsNegativeComparison(t *testing.T) {
	tests := []struct {
		name    string
		chosen  payoff.Plan
		minimum payoff.Plan
	}{
		{
			name:    "more months",
			chosen:  payoff.Plan{Months: 4, TotalInterest: 100},
			minimum: payoff.Plan{Months: 3, TotalInterest: 100},
		},
		{
			name:    "more interest",
			chosen:  payoff.Plan{Months: 3, TotalInterest: 101},
			minimum: payoff.Plan{Months: 3, TotalInterest: 100},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			months, interest, err := planSavings(tt.chosen, tt.minimum, "USD")
			if err == nil || months != 0 || !interest.IsZero() {
				t.Fatalf("negative benefit was presented: months=%d interest=%+v err=%v", months, interest, err)
			}
		})
	}
}

func TestPrepareStrategyComparisonUsesChosenPlanAndHonorsOptions(t *testing.T) {
	accounts := []domain.Account{debtAccount("small", "Small", "USD", -12000), debtAccount("large", "Large", "USD", -30000)}
	accounts[0].MinPayment = money.New(300, "USD")
	accounts[1].MinPayment = money.New(900, "USD")
	accounts[0].InterestRateAPR = floatPtr(8)
	accounts[1].InterestRateAPR = floatPtr(24)

	r := Prepare(accounts, nil, currency.Rates{Base: "USD"}, "USD", Config{ExtraMinor: 250, Strategy: payoff.Avalanche, CompareStrategies: true}, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if !r.PlanKnown || !r.PlanValid || !r.Comparison.Known || !r.Comparison.Requested || r.MinimumComparisonRequested || r.MinimumPlanValid || r.SavingsKnown {
		t.Fatalf("comparison options did not isolate the requested work: %+v", r)
	}
	if !reflect.DeepEqual(r.Comparison.AvalanchePlan, r.Plan) || !r.Comparison.AvalancheValid {
		t.Fatalf("selected avalanche plan was not reused: chosen=%+v comparison=%+v", r.Plan, r.Comparison)
	}
	wantSnowball, ok, err := payoff.BuildPlanChecked(r.Debts, 250, payoff.Snowball)
	if !ok || err != nil || !reflect.DeepEqual(r.Comparison.SnowballPlan, wantSnowball) {
		t.Fatalf("alternate strategy mismatch: got=%+v want=%+v ok=%v err=%v", r.Comparison.SnowballPlan, wantSnowball, ok, err)
	}
	if !r.SuggestedExtraKnown || r.SuggestedExtraMinor <= 0 {
		t.Fatalf("suggested extra was not prepared with the requested comparison: %+v", r)
	}

	minimal := Prepare(accounts, nil, currency.Rates{Base: "USD"}, "USD", Config{Strategy: payoff.Avalanche}, time.Now())
	if !minimal.PlanKnown || !minimal.PlanValid || minimal.Comparison.Requested || minimal.MinimumComparisonRequested || minimal.MinimumPlanValid || minimal.SavingsKnown || minimal.SuggestedExtraKnown {
		t.Fatalf("unrequested comparison work leaked into the default preparation: %+v", minimal)
	}
}

func TestPrepareExcludedMissingFXKeepsPlanButHidesWholeTotal(t *testing.T) {
	included := debtAccount("card", "Card", "USD", -10000)
	included.MinPayment = money.New(400, "USD")
	excluded := debtAccount("mortgage", "Mortgage", "EUR", -80000)
	excluded.Type = domain.TypeMortgage
	r := Prepare([]domain.Account{included, excluded}, nil, currency.Rates{Base: "USD"}, "USD", Config{Strategy: payoff.Avalanche, CompareStrategies: true}, time.Now())
	if r.TotalKnown || !r.TotalOwed.IsZero() || r.TotalErr == nil || len(r.MissingRates) != 1 || r.MissingRates[0] != "EUR" {
		t.Fatalf("whole-source total should be unavailable without a partial figure: %+v", r)
	}
	if !r.PlanKnown || r.PlanOwed.Amount != 10000 || !r.Comparison.Known || len(r.PlanMissingRates) != 0 || len(r.Debts) != 1 {
		t.Fatalf("excluded missing FX should not corrupt the complete included plan: %+v", r)
	}
}

func TestPreparePreservesInputsAndAttachesLedgerSourceError(t *testing.T) {
	a := debtAccount("card", "Card", "USD", -10000)
	accounts := []domain.Account{a}
	txns := []domain.Transaction{{ID: "wrong-currency", AccountID: "card", Amount: money.New(-25, "EUR")}}
	rateTable := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	accountsBefore, txnsBefore, ratesBefore := append([]domain.Account(nil), accounts...), append([]domain.Transaction(nil), txns...), currency.Rates{Base: rateTable.Base, Rates: map[string]float64{}}
	for k, v := range rateTable.Rates {
		ratesBefore.Rates[k] = v
	}
	r := Prepare(accounts, txns, rateTable, "USD", Config{Strategy: payoff.Avalanche}, time.Time{})
	if len(r.Accounts) != 1 || r.Accounts[0].BalanceErr == nil || r.TotalKnown || r.PlanKnown {
		t.Fatalf("ledger source error was hidden: %+v", r)
	}
	if !reflect.DeepEqual(accounts, accountsBefore) || !reflect.DeepEqual(txns, txnsBefore) || !reflect.DeepEqual(rateTable, ratesBefore) {
		t.Fatal("preparation mutated caller-owned inputs")
	}
}
