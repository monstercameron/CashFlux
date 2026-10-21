// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestDebtLoansNativeDebtSummaryHostParityAndOverflowAvailability(t *testing.T) {
	t.Run("valid liability total remains visible", func(t *testing.T) {
		app := bud2TestApp(t)
		if err := app.PutAccount(domain.Account{ID: "visa", Name: "Visa", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OpeningBalance: money.New(-12345, "USD")}); err != nil {
			t.Fatal(err)
		}
		f := render.New(t)
		f.Render(ui.CreateElement(debtSummaryWidget, debtSummaryProps{App: app}))
		figure := byTestID(f, "div", "debt-total-owed")
		if figure == nil || figure.Text() != "$123.45" {
			t.Fatalf("valid debt total=%q", debtSummaryText(f))
		}
	})
	t.Run("absolute-value overflow is disclosed", func(t *testing.T) {
		app := bud2TestApp(t)
		if err := app.PutAccount(domain.Account{ID: "overflow", Name: "Overflow", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OpeningBalance: money.New(math.MinInt64, "USD")}); err != nil {
			t.Fatal(err)
		}
		f := render.New(t)
		f.Render(ui.CreateElement(debtSummaryWidget, debtSummaryProps{App: app}))
		figure := byTestID(f, "div", "debt-total-owed")
		if figure == nil || figure.Text() != "—" || strings.TrimSpace(figure.Attr("title")) == "" {
			t.Fatalf("overflow was rendered as a plausible balance: node=%v text=%q", figure, debtSummaryText(f))
		}
	})
}

func debtSummaryText(f *render.Fixture) string {
	if n := byTestID(f, "div", "debt-total-owed"); n != nil {
		return n.Text()
	}
	return "<missing>"
}

func TestDebtTunerHostShowsUnavailableForMissingIncludedFX(t *testing.T) {
	app := bud2TestApp(t)
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, account := range []domain.Account{
		{ID: "usd-card", Name: "USD card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OpeningBalance: money.New(-100_000, "USD"), MinPayment: money.New(5_000, "USD")},
		{ID: "eur-card", Name: "EUR card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "EUR", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OpeningBalance: money.New(-1_000, "EUR"), MinPayment: money.New(100, "EUR")},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}

	f := render.New(t)
	f.Render(ui.CreateElement(debtTunerWidget, debtPanelProps{App: app}))
	if byTestID(f, "p", "debt-tuner-unavailable") == nil {
		t.Fatalf("missing included FX did not make the tuner unavailable: %q", f.Text())
	}
	if byTestID(f, "div", "debt-tuner-stats") != nil {
		t.Fatalf("tuner displayed partial payoff facts: %q", f.Text())
	}
	if suggest := byTestID(f, "button", "debt-extra-suggest"); suggest == nil || !isDisabled(suggest) {
		t.Fatalf("suggestion remained available for an incomplete debt set: node=%v", suggest)
	}
}

func TestDebtLoansNativeLoanCardExtraMoneyAndInvalidPrecision(t *testing.T) {
	a := domain.Account{ID: "loan", Name: "Family loan", Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeLoan, TermMonths: 12, InterestRateAPR: loanAPRPtr(0)}
	props := loanCardProps{Account: a, Balance: money.New(120000, "USD"), BaseCur: "USD"}
	f := render.New(t)
	f.Render(ui.CreateElement(loanCard, props))
	f.InputByID("loan-extra-loan", "12.34")
	f.Stabilize()
	if n := byTestID(f, "div", "loan-preview-loan"); n == nil || !strings.Contains(n.Text(), "$112.34") {
		t.Fatalf("valid exact extra-payment preview missing: %q", loanCardText(f))
	}

	f.InputByID("loan-extra-loan", "12.345")
	f.Stabilize()
	if byTestID(f, "p", "loan-extra-error-loan") == nil {
		t.Fatalf("excess precision was silently accepted: %q", loanCardText(f))
	}
	if byTestID(f, "div", "loan-preview-loan") != nil || byTestID(f, "div", "loan-savings-loan") != nil {
		t.Fatalf("invalid extra payment retained plausible savings: %q", loanCardText(f))
	}
}

func loanCardText(f *render.Fixture) string {
	if n := byTestID(f, "div", "loan-preview-loan"); n != nil {
		return n.Text()
	}
	if n := byTestID(f, "p", "loan-extra-error-loan"); n != nil {
		return n.Text()
	}
	return "<no loan preview/error>"
}

func loanAPRPtr(v float64) *float64 { return &v }
