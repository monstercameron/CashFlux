// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type budgetFundingBoundaryProps struct {
	app               *appstate.App
	start             time.Time
	end               time.Time
	actualIncomeKnown bool
}

func budgetFundingBoundaryHost(p budgetFundingBoundaryProps) ui.Node {
	prepared := budgetSummaryFunding(budgetSummaryFundingProps{
		app: p.app, wStart: p.start, wEnd: p.end, normalizationOK: true,
		normalizedBudgetTotal: 1200, v: budgetView{Base: "USD", BannerIncome: 1000, Method: budgeting.MethodZeroBased},
	})
	allocation := budgetSummaryAllocation(budgetSummaryAllocationProps{
		futurePeriod: true, hist: true, wEnd: p.end,
		actualIncome: prepared.prepared.Received, actualIncomeKnown: p.actualIncomeKnown,
		overAssigned: prepared.prepared.OverAssigned, overAssignedKnown: prepared.prepared.OverAssignedKnown,
		v: budgetView{BannerIncome: 1000}, pr: prefs.Prefs{},
	})
	details := budgetSummaryDetails(budgetSummaryDetailsProps{
		v: budgetView{Base: "USD", BannerIncome: 1000}, normalizationOK: true,
		futurePeriod: false, funding: prepared.prepared.Funding,
		fundingKnown: prepared.prepared.FundingKnown, fundingEligible: prepared.prepared.FundingEligible,
		hist: true, vw: period.NewWindow(period.Month, p.start, time.Monday), app: p.app,
		pr: prefs.Prefs{}, alloc: Fragment(), rail: Fragment(), incomeActual: prepared.incomeActual,
		lastMonthTag: Fragment(), closeOffer: allocation.closeOffer, toAssignChip: Fragment(), hasAlloc: true,
		basisBtn: Fragment(), ageChip: Fragment(),
	})
	return Div(details,
		Span(Attr("data-testid", "budgets-funding-known"), strconv.FormatBool(prepared.prepared.FundingKnown)),
	)
}

func TestBudgetFundingBoundaryShowsUnavailableAndSuppressesMonthClose(t *testing.T) {
	run := func(t *testing.T, rates map[string]float64, wantKnown, wantCloseOffer, wantCallout bool) {
		t.Helper()
		app, err := appstate.New(&bytes.Buffer{}, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.PutAccount(domain.Account{ID: "checking", Name: "Checking", Currency: "EUR", Type: domain.TypeChecking,
			Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.Zero("EUR")}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: rates}); err != nil {
			t.Fatal(err)
		}
		start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)
		if err := app.PutTransaction(domain.Transaction{ID: "income", AccountID: "checking", Desc: "Paycheck", Date: start.AddDate(0, 0, 1), Amount: money.New(100, "EUR")}); err != nil {
			t.Fatal(err)
		}
		previous := appstate.Default
		appstate.Default = app
		f := render.New(t)
		finished := false
		defer func() {
			if !finished {
				f.Render(ui.Fragment())
				f.Stabilize()
				f.Cleanup()
				appstate.Default = previous
				_ = app.Store().Close()
			}
		}()
		f.Render(ui.CreateElement(budgetFundingBoundaryHost, budgetFundingBoundaryProps{app: app, start: start, end: end, actualIncomeKnown: wantKnown}))
		if got := byTestID(f, "span", "budgets-funding-known"); got == nil || (got.Text() == "true") != wantKnown {
			t.Fatalf("funding availability = %v, want known=%v", got, wantKnown)
		}
		if wantKnown {
			if byTestID(f, "p", "budgets-income-unavailable") != nil {
				t.Fatal("valid income source showed unavailable notice")
			}
		} else if byTestID(f, "p", "budgets-income-unavailable") == nil {
			t.Fatal("missing FX income should show an unavailable notice")
		}
		if (byTestID(f, "button", "budgets-monthclose-offer") != nil) != wantCloseOffer {
			t.Fatalf("month-close offer presence = %v, want %v", byTestID(f, "button", "budgets-monthclose-offer") != nil, wantCloseOffer)
		}
		if (byTestID(f, "div", "budgets-funded-callout") != nil) != wantCallout {
			t.Fatalf("funded callout presence = %v, want %v", byTestID(f, "div", "budgets-funded-callout") != nil, wantCallout)
		}
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
		finished = true
		appstate.Default = previous
		if err := app.Store().Close(); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("missing FX suppresses funding and month-close actions", func(t *testing.T) {
		run(t, nil, false, false, false)
	})
	t.Run("known income preserves eligible actions", func(t *testing.T) {
		run(t, map[string]float64{"EUR": 1}, true, true, true)
	})
	t.Run("warm closed-store reads remain unavailable", func(t *testing.T) {
		app, err := appstate.New(&bytes.Buffer{}, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.PutAccount(domain.Account{ID: "checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking,
			Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
			t.Fatal(err)
		}
		start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)
		if err := app.PutTransaction(domain.Transaction{ID: "income", AccountID: "checking", Desc: "Paycheck", Date: start.AddDate(0, 0, 1), Amount: money.New(100, "USD")}); err != nil {
			t.Fatal(err)
		}
		_ = app.Transactions()
		_ = app.Settings()
		if err := app.Store().Close(); err != nil {
			t.Fatal(err)
		}
		prepared := budgetSummaryFunding(budgetSummaryFundingProps{app: app, wStart: start, wEnd: end,
			normalizationOK: true, normalizedBudgetTotal: 1200,
			v: budgetView{Base: "USD", BannerIncome: 1000, Method: budgeting.MethodZeroBased}})
		if prepared.prepared.IncomeKnown || prepared.prepared.FundingKnown {
			t.Fatalf("warm closed-store income treated as available: %+v", prepared.prepared)
		}
		if summary := buildMonthCloseSummary(app, budgetView{Base: "USD", BannerIncome: 1000}, start, end); summary.Error == nil {
			t.Fatalf("warm closed-store month close has no source error: %+v", summary)
		}
	})
}
