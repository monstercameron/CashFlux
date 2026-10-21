// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func planHealthBoundaryFixture(t *testing.T, accounts []domain.Account) *appstate.App {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	now := time.Now()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	dataset := store.Dataset{Settings: store.Settings{BaseCurrency: "USD"}, Accounts: accounts}
	for i := 1; i <= 3; i++ {
		date := month.AddDate(0, -i, 10)
		dataset.Transactions = append(dataset.Transactions,
			domain.Transaction{ID: date.Format("2006-01") + "-income", AccountID: "cash", Date: date, Amount: money.New(20_000, "USD")},
			domain.Transaction{ID: date.Format("2006-01") + "-expense", AccountID: "cash", Date: date, Amount: money.New(-10_000, "USD")},
		)
	}
	if err := app.Store().Load(dataset); err != nil {
		t.Fatal(err)
	}
	return app
}

func planHealthRenderHost(t *testing.T, node ui.Node) *render.Fixture {
	t.Helper()
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	t.Cleanup(func() { css.SetSink(previousCSS) })
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	f.Render(node)
	return f
}

func TestPlanHealthBoundaryPlanHostKeepsFailedLiabilityFXNotAssessable(t *testing.T) {
	check := func(t *testing.T, minPaymentCurrency, wantCurrent string) {
		t.Helper()
		card := domain.Account{ID: "card", Name: "Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(10_000, "USD"), InterestRateAPR: domain.APR(9), MinPayment: money.New(1_000, minPaymentCurrency)}
		app := planHealthBoundaryFixture(t, []domain.Account{
			{ID: "cash", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1_000, "USD")},
			card,
		})
		for key, value := range map[string]string{"plan.framework": "foo", "plan.q.match": "yes", "plan.q.deductible": "yes"} {
			if err := app.SetKV(key, value); err != nil {
				t.Fatal(err)
			}
		}
		f := planHealthRenderHost(t, ui.CreateElement(PlanScreen))
		current := byTestID(f, "div", "plan-current")
		if current == nil || !strings.Contains(current.Text(), wantCurrent) {
			t.Fatalf("plan next move=%v; want %q; rendered=%s", current, wantCurrent, f.Text())
		}
	}
	t.Run("known liability facts", func(t *testing.T) { check(t, "USD", "Pay off high-interest debt") })
	t.Run("liability minimum FX failure", func(t *testing.T) { check(t, "EUR", "Answer 2 quick questions") })
}

func TestPlanHealthBoundaryHealthHostKeepsResilienceSourcesIndependent(t *testing.T) {
	t.Run("known liquidity and card balance with unknown APR", func(t *testing.T) {
		planHealthBoundaryFixture(t, []domain.Account{
			{ID: "cash", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1_000, "USD")},
			{ID: "card", Name: "Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(10_000, "USD"), CreditLimit: money.New(100_000, "USD")},
		})
		f := planHealthRenderHost(t, ui.CreateElement(HealthScreen))
		runway := byTestID(f, "p", "stress-runway")
		surprise := byTestID(f, "p", "stress-surprise")
		rate := byTestID(f, "p", "stress-rate")
		if runway == nil || !strings.Contains(runway.Text(), "cash buffer") || surprise == nil || !strings.Contains(surprise.Text(), "estimate is unavailable") || rate == nil || !strings.Contains(rate.Text(), "Rates +5 points would add") {
			t.Fatalf("unknown card APR obscured independent resilience facts: runway=%v surprise=%v rate=%v rendered=%s", runway, surprise, rate, f.Text())
		}
	})

	t.Run("liquidity FX failure leaves card balance known", func(t *testing.T) {
		planHealthBoundaryFixture(t, []domain.Account{
			{ID: "cash", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1_000, "USD")},
			{ID: "foreign-cash", Name: "Foreign checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR", OpeningBalance: money.New(5_000, "EUR")},
			{ID: "card", Name: "Card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(10_000, "USD"), CreditLimit: money.New(100_000, "USD"), InterestRateAPR: domain.APR(20)},
		})
		f := planHealthRenderHost(t, ui.CreateElement(HealthScreen))
		runway := byTestID(f, "p", "stress-runway")
		rate := byTestID(f, "p", "stress-rate")
		if runway == nil || !strings.Contains(runway.Text(), "estimate is unavailable") || rate == nil || !strings.Contains(rate.Text(), "Rates +5 points would add") {
			t.Fatalf("liquidity FX failure contaminated card balance availability: runway=%v rate=%v rendered=%s", runway, rate, f.Text())
		}
	})

	t.Run("card FX failure leaves liquidity known", func(t *testing.T) {
		planHealthBoundaryFixture(t, []domain.Account{
			{ID: "cash", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1_000, "USD")},
			{ID: "card", Name: "Foreign card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "JPY", OpeningBalance: money.New(10_000, "JPY"), CreditLimit: money.New(100_000, "JPY"), InterestRateAPR: domain.APR(20)},
		})
		f := planHealthRenderHost(t, ui.CreateElement(HealthScreen))
		runway := byTestID(f, "p", "stress-runway")
		rate := byTestID(f, "p", "stress-rate")
		if runway == nil || !strings.Contains(runway.Text(), "cash buffer") || rate == nil || !strings.Contains(rate.Text(), "estimate is unavailable") {
			t.Fatalf("card FX failure contaminated known liquidity or concealed card unavailability: runway=%v rate=%v rendered=%s", runway, rate, f.Text())
		}
	})
}
