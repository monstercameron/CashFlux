// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/taxgather"
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

func TestAssistantTaxGatherChunksLargeCSVLosslesslyAndRejectsStaleCursor(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	const rows = 900
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ds := store.Dataset{Settings: store.Settings{BaseCurrency: "USD"}}
	for i := 0; i < rows; i++ {
		id := fmt.Sprintf("tax-category-%04d", i)
		label := fmt.Sprintf("Category %04d %s", i, strings.Repeat("Long retained label ", 30))
		ds.Categories = append(ds.Categories, domain.Category{ID: id, Name: label, Deductible: true})
		ds.Transactions = append(ds.Transactions, domain.Transaction{
			ID: fmt.Sprintf("tax-transaction-%04d", i), Date: start, CategoryID: id, Amount: money.New(-1234, "USD"),
		})
	}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	tool := assistantFinanceTestTool(t, agToolsTax(app, "USD", currency.Rates{Base: "USD"}), "gather_tax_records")
	first := tool.run(json.RawMessage(`{"year":2025,"csv_limit":3000}`))
	if len(first) >= taxToolOutputMaxBytes {
		t.Fatalf("first chunk output is %d bytes, want < %d", len(first), taxToolOutputMaxBytes)
	}
	var response taxCSVChunkOutput
	if err := json.Unmarshal([]byte(first), &response); err != nil {
		t.Fatalf("large CSV response is not chunk JSON: %v; response prefix=%q", err, first[:min(200, len(first))])
	}
	if response.Year != 2025 || response.CSV.Collection != "tax_csv" || response.CSV.ID != "tax-year-2025" || response.CSV.Offset != 0 || response.CSV.NextOffset == nil {
		t.Fatalf("unexpected first chunk metadata: year=%d csv=%+v", response.Year, response.CSV)
	}
	if response.Counts.DeductibleCategories != rows || response.Counts.ReceiptGaps != rows ||
		!strings.Contains(response.Summary, fmt.Sprintf("across %d categories", rows)) || !strings.Contains(response.Summary, fmt.Sprintf("Gaps (%d entries", rows)) {
		t.Fatalf("summary did not keep full-dataset counts: counts=%+v summary=%q", response.Counts, response.Summary)
	}
	snapshot := response.CSV.Snapshot
	// Provider usage is persisted between tool calls and advances App.Rev, but
	// does not alter the retained tax dataset or its canonical CSV.
	beforeUsageWrite := app.Rev()
	app.RecordAISpend("assistant", "unpriced-test-model", ai.Usage{TotalTokens: 1})
	if app.Rev() <= beforeUsageWrite {
		t.Fatal("recording provider usage did not advance the app revision")
	}
	var joined strings.Builder
	joined.WriteString(response.CSV.JSONText)
	offset := *response.CSV.NextOffset
	for pages := 0; ; pages++ {
		if pages > 1000 {
			t.Fatal("CSV continuation did not terminate")
		}
		raw, _ := json.Marshal(struct {
			Year        int    `json:"year"`
			CSVOffset   int    `json:"csv_offset"`
			CSVLimit    int    `json:"csv_limit"`
			CSVSnapshot string `json:"csv_snapshot"`
		}{2025, offset, taxCSVChunkCharacters, snapshot})
		out := tool.run(raw)
		if len(out) >= taxToolOutputMaxBytes {
			t.Fatalf("chunk at offset %d is %d bytes, want < %d", offset, len(out), taxToolOutputMaxBytes)
		}
		var page taxCSVChunkOutput
		if err := json.Unmarshal([]byte(out), &page); err != nil {
			t.Fatalf("chunk at offset %d is invalid JSON: %v: %s", offset, err, out)
		}
		if page.CSV.Offset != offset || page.CSV.Snapshot != snapshot {
			t.Fatalf("chunk cursor mismatch: asked offset %d, got %+v", offset, page.CSV)
		}
		joined.WriteString(page.CSV.JSONText)
		if page.CSV.NextOffset == nil {
			break
		}
		if *page.CSV.NextOffset <= offset {
			t.Fatalf("continuation did not advance: offset=%d next=%d", offset, *page.CSV.NextOffset)
		}
		offset = *page.CSV.NextOffset
	}
	var reconstructed string
	if err := json.Unmarshal([]byte(joined.String()), &reconstructed); err != nil {
		t.Fatalf("concatenated JSON character chunks are not a complete JSON string: %v", err)
	}
	cats, err := app.CategoriesChecked()
	if err != nil {
		t.Fatal(err)
	}
	txns, err := app.Store().ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	links, err := app.Store().ListTxnLinks()
	if err != nil {
		t.Fatal(err)
	}
	end := start.AddDate(1, 0, 0)
	summary, err := taxgather.GatherWithLinks(txns, cats, links, 2025, start, end, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]string, len(cats))
	for _, c := range cats {
		names[c.ID] = c.Name
	}
	expected, err := taxgather.GatherCSVExact(summary, func(id string) string { return names[id] }, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) <= 64*1024 {
		t.Fatalf("fixture CSV is only %d bytes, want a payload larger than the relay's 64 KiB limit", len(expected))
	}
	if reconstructed != string(expected) {
		t.Fatalf("reconstructed CSV differs from canonical CSV: got %d bytes, want %d", len(reconstructed), len(expected))
	}
	if err := app.PutCategory(domain.Category{ID: "tax-category-0000", Name: "Changed retained tax category", Kind: domain.KindExpense, Deductible: true}); err != nil {
		t.Fatal(err)
	}
	staleArgs, _ := json.Marshal(struct {
		Year        int    `json:"year"`
		CSVOffset   int    `json:"csv_offset"`
		CSVSnapshot string `json:"csv_snapshot"`
	}{2025, 1, snapshot})
	stale := tool.run(staleArgs)
	var staleResponse map[string]string
	if err := json.Unmarshal([]byte(stale), &staleResponse); err != nil || !strings.Contains(staleResponse["error"], "snapshot changed") {
		t.Fatalf("stale CSV cursor was accepted: %s (err=%v)", stale, err)
	}
}
