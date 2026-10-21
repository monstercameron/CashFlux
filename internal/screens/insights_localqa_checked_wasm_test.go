// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/insights/localqa"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestInsightsQASourceMissingFXIsUnavailable(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutAccount(netWorthFixtureAccount("qa-eur", "EUR", 12345, false)); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = map[string]float64{}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	source := newInsightsQASource(app, "USD", currency.Rates{Base: "USD"})
	answer, ok := localqa.Answer(localqa.IntentBalance, source, "", insightsMoneyFmtForSource(source))
	if !ok || !strings.Contains(answer, "unknown, not zero") || strings.Contains(answer, "$0") {
		t.Fatalf("answer=%q ok=%v, want unavailable without fabricated zero", answer, ok)
	}
	if source.LastError() == nil {
		t.Fatal("missing FX did not reach checked source error state")
	}
}

func TestInsightsQASourceClosedStoreDiffersFromSavedEmpty(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	source := newInsightsQASource(app, "USD", currency.Rates{Base: "USD"})
	clockDelta := source.now.Sub(app.ClockNow())
	if source.now.IsZero() || clockDelta > time.Second || clockDelta < -time.Second {
		t.Fatalf("source clock=%v app clock=%v", source.now, app.ClockNow())
	}
	empty, ok := localqa.Answer(localqa.IntentBalance, source, "", insightsMoneyFmtForSource(source))
	if !ok || !strings.Contains(empty, "$0.00") || source.LastError() != nil {
		t.Fatalf("empty saved ledger should be a known zero, answer=%q error=%v", empty, source.LastError())
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	answer, ok := localqa.Answer(localqa.IntentBalance, source, "", insightsMoneyFmtForSource(source))
	if !ok || !strings.Contains(answer, "unknown, not zero") || strings.Contains(answer, "$0") {
		t.Fatalf("closed-store answer=%q ok=%v, want unavailable", answer, ok)
	}
}

func TestInsightsQASourceRefreshesBoundStoreSettingsAndFX(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutAccount(netWorthFixtureAccount("qa-usd", "USD", 10000, false)); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.BaseCurrency = "EUR"
	settings.FXRates = map[string]float64{"USD": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	// Constructor values are stale on purpose: every calculation must use the
	// currently bound store's base and rate table.
	source := newInsightsQASource(app, "USD", currency.Rates{Base: "USD"})
	if got := source.LiquidBalanceMinor(); got != 20000 || source.LastError() != nil || source.base != "EUR" {
		t.Fatalf("balance=%d base=%q error=%v, want 20000 EUR from bound settings", got, source.base, source.LastError())
	}
}

func TestInsightsQACategoryUsesRefundSplitsAndReportExclusions(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutAccount(netWorthFixtureAccount("qa-checking", "USD", 100000, false)); err != nil {
		t.Fatal(err)
	}
	for _, category := range []domain.Category{
		{ID: "qa-food", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "qa-home", Name: "Household", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	now := app.ClockNow()
	for _, txn := range []domain.Transaction{
		{ID: "qa-purchase", AccountID: "qa-checking", Date: now, Desc: "Purchase", CategoryID: "qa-food", Amount: money.New(-10000, "USD")},
		{ID: "qa-refund", AccountID: "qa-checking", Date: now, Desc: "Refund", CategoryID: "qa-food", Amount: money.New(3000, "USD")},
		{ID: "qa-split", AccountID: "qa-checking", Date: now, Desc: "Split", Amount: money.New(-6000, "USD"),
			Splits: []domain.CategorySplit{{CategoryID: "qa-food", Amount: money.New(-4000, "USD")}, {CategoryID: "qa-home", Amount: money.New(-2000, "USD")}}},
		{ID: "qa-excluded", AccountID: "qa-checking", Date: now, Desc: "Excluded", CategoryID: "qa-food", Amount: money.New(-9000, "USD"), ExcludeFromReports: true},
	} {
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutTxnLink(domain.TxnLink{ID: "qa-refund-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"qa-purchase", "qa-refund"}, Amount: money.New(3000, "USD")}); err != nil {
		t.Fatal(err)
	}
	source := newInsightsQASource(app, "USD", currency.Rates{Base: "USD"})
	answer, ok := localqa.Answer(localqa.IntentSpendingByCategory, source, "How much did I spend on Groceries this month?", insightsMoneyFmtForSource(source))
	if !ok || !strings.Contains(answer, "$110.00") || source.LastError() != nil {
		t.Fatalf("category answer=%q error=%v, want $110.00 refund-netted split spend", answer, source.LastError())
	}
}

func TestInsightsQASourceOverflowIsUnavailable(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutAccount(netWorthFixtureAccount("qa-overflow", "USD", 1, false)); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "qa-overflow-income", AccountID: "qa-overflow", Desc: "Overflow income", Date: app.ClockNow(), Amount: money.New(math.MaxInt64, "USD")}); err != nil {
		t.Fatal(err)
	}
	source := newInsightsQASource(app, "USD", currency.Rates{Base: "USD"})
	answer, ok := localqa.Answer(localqa.IntentBalance, source, "", insightsMoneyFmtForSource(source))
	if !ok || !strings.Contains(answer, "unknown, not zero") || source.LastError() == nil {
		t.Fatalf("overflow answer=%q ok=%v error=%v, want explicit unavailable", answer, ok, source.LastError())
	}
}

func insightsMoneyFmtForSource(source *insightsQASource) func(int64) string {
	return func(minor int64) string { return insightsMoneyFmt(minor, source.base) }
}
