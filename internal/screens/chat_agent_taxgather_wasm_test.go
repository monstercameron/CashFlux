// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestAssistantTaxGatherReadsFreshBoundCurrencyRefundsAndSplits(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)
	ds := store.Dataset{
		Settings:   store.Settings{BaseCurrency: "USD"},
		Categories: []domain.Category{{ID: "ded", Name: "Work supplies", Deductible: true}, {ID: "donate", Name: "Charitable giving", Deductible: true}, {ID: "interest", Name: "Interest"}},
		Transactions: []domain.Transaction{
			{ID: "original", Date: start, CategoryID: "ded", Amount: money.New(-10000, "USD")},
			{ID: "refund", Date: end, CategoryID: "ded", Amount: money.New(4000, "USD")},
			{ID: "mixed", Date: start, CategoryID: "ded", Amount: money.New(-7000, "JPY"), Splits: []domain.CategorySplit{{CategoryID: "donate", Amount: money.New(-5000, "JPY")}, {CategoryID: "interest", Amount: money.New(-3000, "JPY")}, {CategoryID: "ded", Amount: money.New(1000, "JPY")}}},
			{ID: "foreign", Date: start, CategoryID: "ded", Amount: money.New(-500, "EUR"), Attachments: []domain.AttachmentRef{{ArtifactID: "receipt"}}},
			{ID: "excluded", Date: start, CategoryID: "donate", ExcludeFromReports: true, Amount: money.New(math.MinInt64, "NOFX")},
			{ID: "transfer", Date: start, CategoryID: "donate", TransferAccountID: "other", Amount: money.New(-9999, "NOFX")},
		},
		TxnLinks: []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"original", "refund"}}},
	}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	// Capture the real tool before settings change and before another workspace
	// becomes Default. Neither the old parameters nor that global may supply FX.
	tool := assistantFinanceTestTool(t, agToolsTax(app, "USD", currency.Rates{Base: "USD"}), "gather_tax_records")
	if err := app.Store().PutSettings(store.Settings{BaseCurrency: "JPY", FXRates: map[string]float64{"USD": 150, "EUR": 200}}); err != nil {
		t.Fatal(err)
	}
	_ = primaryAccountsGoalsApp(t)
	before, err := app.Store().ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	rev := app.Rev()
	out := tool.run(json.RawMessage(`{"year":2025}`))
	for _, want := range []string{"Household scope; UTC calendar year; amounts in JPY", "Currency,JPY\n", "Work supplies,10000\n", "Charitable giving,5000\n", "TOTAL,15000\n", "Charitable donations,5000\n", "Interest paid,3000\n", "Gaps (2 entries missing a receipt)", "buckets can overlap"} {
		if !strings.Contains(out, want) {
			t.Fatal("fresh scope/refund/split/FX tax gathering missing "+want, out)
		}
	}
	after, err := app.Store().ListTransactions()
	if err != nil || !reflect.DeepEqual(before, after) || app.Rev() != rev {
		t.Fatal("read changed ledger/metadata", err)
	}
	if strings.Contains(out, "NOFX") || strings.Contains(out, "Couldn't gather") {
		t.Fatal("report-excluded/transfer records polluted gathering", out)
	}
}

func TestAssistantTaxGatherKeepsUnknownAndAmbiguousInputsUnavailable(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ds := store.Dataset{Settings: store.Settings{BaseCurrency: "USD"}, Categories: []domain.Category{{ID: "ded", Deductible: true}}, Transactions: []domain.Transaction{{ID: "missing-rate", Date: start, CategoryID: "ded", Amount: money.New(-100, "EUR")}}}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	tool := assistantFinanceTestTool(t, agToolsTax(app, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 100}}), "gather_tax_records")
	checkUnavailable := func(want string) {
		t.Helper()
		out := tool.run(json.RawMessage(`{"year":2025}`))
		if !strings.HasPrefix(out, "Couldn't gather tax records:") || !strings.Contains(out, want) || strings.Contains(out, "CSV export:") || strings.Contains(out, "Gaps: none") {
			t.Fatal("unavailable input became a partial/zero tax result", out)
		}
	}
	checkUnavailable("exchange rate")
	ds.Transactions = []domain.Transaction{{ID: "overflow", Date: start, CategoryID: "ded", Amount: money.New(math.MinInt64, "USD")}}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	checkUnavailable("overflow")
	ds.Transactions = []domain.Transaction{{ID: "ambiguous-split", Date: start, CategoryID: "ded", Amount: money.New(-100, "USD"), Splits: []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-60, "USD")}, {CategoryID: "other", Amount: money.New(-40, "USD")}}}, {ID: "refund", Date: start, Amount: money.New(40, "USD")}}
	ds.TxnLinks = []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"ambiguous-split", "refund"}}}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	checkUnavailable("ambiguous-split")
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	checkUnavailable("settings are unavailable")
}

func TestAssistantTaxGatherExportsExactLargeMinorAmountsAndStrictYear(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ds := store.Dataset{Settings: store.Settings{BaseCurrency: "JPY"}, Categories: []domain.Category{{ID: "ded", Name: "Large exact expense", Deductible: true}}, Transactions: []domain.Transaction{{ID: "near-limit", Date: start, CategoryID: "ded", Amount: money.New(-math.MaxInt64, "JPY")}}}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	tool := assistantFinanceTestTool(t, agToolsTax(app, "USD", currency.Rates{Base: "USD"}), "gather_tax_records")
	out := tool.run(json.RawMessage(`{"year":2025}`))
	if !strings.Contains(out, "Currency,JPY\n") || !strings.Contains(out, "TOTAL,9223372036854775807\n") {
		t.Fatal("JPY precision or valid int64 boundary lost", out)
	}
	ds.Settings.BaseCurrency = "USD"
	ds.Transactions[0].Amount = money.New(-math.MaxInt64, "USD")
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{"year":2025}`))
	if !strings.Contains(out, "Currency,USD\n") || !strings.Contains(out, "TOTAL,92233720368547758.07\n") {
		t.Fatal("USD exact decimal rounded through float", out)
	}
	for _, raw := range []string{"{bad", `{"year":"2025"}`, `{"year":2025.5}`, `{"year":0}`, `{"year":null}`, `{"year":-1}`, `{"year":10000}`, `{"year":2025,"extra":1}`} {
		if out := tool.run(json.RawMessage(raw)); !strings.HasPrefix(out, "Couldn't gather tax records:") || strings.Contains(out, "CSV export:") {
			t.Fatal("malformed year became a plausible gathering", raw, out)
		}
	}
	if out := tool.run(json.RawMessage(`{}`)); !strings.HasPrefix(out, "Tax-year "+strconv.Itoa(app.ClockNow().UTC().Year()-1)+" gathering") {
		t.Fatal("omitted year did not use the bound app's prior UTC calendar year", out)
	}
	ds.Settings.BaseCurrency = "ZZZ"
	ds.Transactions[0].Amount = money.New(-100, "ZZZ")
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{"year":2025}`))
	if !strings.Contains(out, "canonical two-decimal fallback assumption") || !strings.Contains(out, "Currency,ZZZ\n") || !strings.Contains(out, "TOTAL,1.00\n") {
		t.Fatal("unknown currency support lost its explicit precision assumption", out)
	}
}
