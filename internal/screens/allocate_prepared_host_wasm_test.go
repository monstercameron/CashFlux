// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestAllocateHostKeepsValidSplitAndRejectsMalformedMoneyWithoutSaving(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutAccount(domain.Account{
		ID: "allocate-savings", Name: "Savings", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeSavings,
		Currency: "USD", OpeningBalance: money.Zero("USD"), ExpectedReturnAPR: 4,
	}); err != nil {
		t.Fatal(err)
	}
	uistate.SetAllocConfig(uistate.AllocConfig{AmountMinor: 10_000, Profile: "returns", Mode: "weighted"})
	f := render.New(t)
	defer func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	}()
	f.Render(ui.CreateElement(Allocate))
	f.Stabilize()

	row := byTestID(f, "div", "alloc-dest-allocate-savings")
	if row == nil || !strings.Contains(row.Text(), "$100.00") {
		t.Fatalf("valid amount did not produce the expected destination split: %s", f.Text())
	}
	if byTestID(f, "button", "allocate-apply-btn") == nil {
		t.Fatal("valid allocation did not expose its apply action")
	}

	amount := byTestID(f, "input", "allocate-amount")
	if amount == nil {
		t.Fatal("Allocate amount input is missing")
	}
	amount.Input("1.234") // USD accepts at most two decimal places.
	f.Stabilize()
	if !strings.Contains(f.Text(), "Enter valid nonnegative amounts") {
		t.Fatalf("malformed money did not show its validation state: %s", f.Text())
	}
	if byTestID(f, "button", "allocate-apply-btn") != nil {
		t.Fatal("malformed money left the apply action available")
	}
	row = byTestID(f, "div", "alloc-dest-allocate-savings")
	if row == nil || strings.Contains(row.Text(), "$100.00") || strings.Contains(row.Text(), "$1.23") {
		t.Fatalf("malformed money produced a plausible split: %v", row)
	}
	if saved := uistate.AllocConfigGet(); saved.AmountMinor != 10_000 {
		t.Fatalf("malformed money overwrote the last valid saved plan: %+v", saved)
	}
}

func TestAllocateHostDoesNotPrefillFromUnavailableMonthIncome(t *testing.T) {
	tests := []struct {
		name      string
		amounts   []money.Money
		currency  string
		fxMissing bool
	}{
		{name: "missing FX", amounts: []money.Money{money.New(2500, "EUR")}, currency: "EUR", fxMissing: true},
		{name: "period total overflow", amounts: []money.Money{money.New(math.MaxInt64, "USD"), money.New(1, "USD")}, currency: "USD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := primaryAccountsGoalsApp(t)
			if err := app.PutAccount(domain.Account{
				ID: "allocate-income", Name: "Income", OwnerID: domain.GroupOwnerID,
				Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
				Currency: tt.currency, OpeningBalance: money.Zero(tt.currency),
			}); err != nil {
				t.Fatal(err)
			}
			if err := app.PutAccount(domain.Account{
				ID: "allocate-savings", Name: "Savings", OwnerID: domain.GroupOwnerID,
				Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeSavings,
				Currency: "USD", OpeningBalance: money.Zero("USD"), ExpectedReturnAPR: 4,
			}); err != nil {
				t.Fatal(err)
			}
			for i, amount := range tt.amounts {
				if err := app.PutTransaction(domain.Transaction{
					ID: "allocate-income-" + string(rune('a'+i)), AccountID: "allocate-income",
					Date: time.Now(), Desc: "Paycheck", Amount: amount,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if tt.fxMissing {
				settings := app.Settings()
				settings.FXRates = nil
				if err := app.PutSettings(settings); err != nil {
					t.Fatal(err)
				}
			}
			uistate.SetAllocConfig(uistate.AllocConfig{})

			f := render.New(t)
			defer func() {
				f.Render(ui.Fragment())
				f.Stabilize()
				f.Cleanup()
			}()
			f.Render(ui.CreateElement(Allocate))
			f.Stabilize()
			if byTestID(f, "div", "income-nudge") != nil || byTestID(f, "button", "income-nudge-apply") != nil {
				t.Fatalf("unavailable income exposed a prefill action: %s", f.Text())
			}
			if byTestID(f, "p", "income-unavailable") == nil {
				t.Fatalf("unavailable income state was not shown: %s", f.Text())
			}
			amountInput := byTestID(f, "input", "allocate-amount")
			if amountInput == nil {
				t.Fatalf("manual allocation input missing while income is unavailable: %s", f.Text())
			}
			amountInput.Input("12.00")
			f.Stabilize()
			if !strings.Contains(f.Text(), "$12.00") {
				t.Fatalf("manual amount was not accepted while income is unavailable: %s", f.Text())
			}
		})
	}
}

func TestAllocateHostReportsUnavailableSources(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	}()
	f.Render(ui.CreateElement(Allocate))
	f.Stabilize()
	if !strings.Contains(strings.ToLower(f.Text()), "allocation data is unavailable") {
		t.Fatalf("closed source did not render an unavailable allocation state: %s", f.Text())
	}
	if byTestID(f, "button", "allocate-apply-btn") != nil {
		t.Fatal("unavailable source exposed an apply action")
	}
}
