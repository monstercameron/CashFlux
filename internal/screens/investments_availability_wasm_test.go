// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestInvestSummaryShowsIndependentBalanceAndFXAvailability(t *testing.T) {
	t.Run("missing FX does not render a zero traditional balance", func(t *testing.T) {
		app := primaryAccountsGoalsApp(t)
		settings := app.Settings()
		settings.BaseCurrency, settings.FXRates = "USD", nil
		if err := app.PutSettings(settings); err != nil {
			t.Fatal(err)
		}
		if err := app.PutAccount(domain.Account{ID: "retirement-eur", Name: "Retirement", OwnerID: domain.GroupOwnerID,
			Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeRetirement,
			Currency: "EUR", OpeningBalance: money.New(12500, "EUR")}); err != nil {
			t.Fatal(err)
		}
		v := computeInvestViewRaw(app)
		if v.TotalKnown || v.TraditionalKnown || len(v.BalByID) != 0 {
			t.Fatalf("missing FX was presented as a base-currency amount: %+v", v)
		}
		f := render.New(t)
		defer f.Cleanup()
		f.Render(ui.CreateElement(investSummaryWidget, investPanelProps{App: app}))
		f.Stabilize()
		if !strings.Contains(f.Text(), "Unavailable") || strings.Contains(f.Text(), "$0.00") {
			t.Fatalf("unavailable total must not be rendered as zero: %s", f.Text())
		}
	})

	t.Run("security value remains known when account reconciliation FX is missing", func(t *testing.T) {
		app := primaryAccountsGoalsApp(t)
		settings := app.Settings()
		settings.BaseCurrency, settings.FXRates = "USD", nil
		if err := app.PutSettings(settings); err != nil {
			t.Fatal(err)
		}
		if err := app.PutAccount(domain.Account{ID: "broker-eur", Name: "Brokerage", OwnerID: domain.GroupOwnerID,
			Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeInvestment,
			Currency: "EUR", OpeningBalance: money.New(10000, "EUR")}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutAccount(domain.Account{ID: "retirement-usd", Name: "Retirement", OwnerID: domain.GroupOwnerID,
			Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeRetirement,
			Currency: "USD", OpeningBalance: money.New(1000, "USD")}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutHolding(domain.Holding{ID: "fund", AccountID: "broker-eur", Ticker: "FND",
			Shares: 1, CurrentPriceMinorPerShare: 5000, CostBasisMinor: 4000}); err != nil {
			t.Fatal(err)
		}
		v := computeInvestViewRaw(app)
		if !v.TotalKnown || v.TotalValueMinor != 6000 || v.ReconcileKnown {
			t.Fatalf("independent security/total/reconciliation availability lost: %+v", v)
		}
		f := render.New(t)
		defer f.Cleanup()
		f.Render(ui.CreateElement(investSummaryWidget, investPanelProps{App: app}))
		f.Stabilize()
		if byTestID(f, "p", "invest-reconcile-unavailable") == nil {
			t.Fatalf("missing reconciliation availability note: %s", f.Text())
		}
	})

	t.Run("invalid ledger balance does not render a zero total", func(t *testing.T) {
		app := primaryAccountsGoalsApp(t)
		account := domain.Account{ID: "broker-usd", Name: "Brokerage", OwnerID: domain.GroupOwnerID,
			Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeInvestment,
			Currency: "USD", OpeningBalance: money.New(1<<63-1, "USD")}
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
		if err := app.PutTransaction(domain.Transaction{ID: "overflow", AccountID: account.ID,
			Desc: "Overflow check", Date: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), Amount: money.New(1, "USD")}); err != nil {
			t.Fatal(err)
		}
		v := computeInvestViewRaw(app)
		if v.TotalKnown {
			t.Fatalf("invalid account balance marked known: %+v", v)
		}
		f := render.New(t)
		defer f.Cleanup()
		f.Render(ui.CreateElement(investSummaryWidget, investPanelProps{App: app}))
		f.Stabilize()
		if !strings.Contains(f.Text(), "Unavailable") || strings.Contains(f.Text(), "$0.00") {
			t.Fatalf("invalid balance must not be rendered as zero: %s", f.Text())
		}
	})
}
