// SPDX-License-Identifier: MIT

package taxgather

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func netIncomeFixture() NetIncomeInput {
	from := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	return NetIncomeInput{Year: 2026, From: from, To: from.AddDate(0, 2, 0), Rates: currency.Rates{Base: "USD"},
		Categories: []domain.Category{{ID: "ded", Deductible: true}, {ID: "other"}}}
}

func netIncomeRow(id, category, code string, at time.Time, amount int64) domain.Transaction {
	return domain.Transaction{ID: id, CategoryID: category, Date: at, Amount: money.New(amount, code)}
}

func TestNetIncomeWithLinksPreservesExactWindowAndKnownZeroOrLoss(t *testing.T) {
	in := netIncomeFixture()
	for _, tc := range []struct {
		name                    string
		rows                    []domain.Transaction
		income, deductible, net int64
	}{
		{"empty", nil, 0, 0, 0},
		{"income", []domain.Transaction{netIncomeRow("income", "", "USD", in.From, 10000)}, 10000, 0, 10000},
		{"net zero", []domain.Transaction{netIncomeRow("income", "", "USD", in.From, 10000), netIncomeRow("expense", "ded", "USD", in.From, -10000)}, 10000, 10000, 0},
		{"loss", []domain.Transaction{netIncomeRow("expense", "ded", "USD", in.From, -10000)}, 0, 10000, -10000},
		{"half open", []domain.Transaction{netIncomeRow("before", "", "NOFX", in.From.Add(-time.Nanosecond), math.MaxInt64), netIncomeRow("start", "", "USD", in.From, 10000), netIncomeRow("last", "ded", "USD", in.To.Add(-time.Nanosecond), -3000), netIncomeRow("end", "ded", "NOFX", in.To, math.MinInt64)}, 10000, 3000, 7000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in.Transactions = tc.rows
			got, err := NetIncomeWithLinks(in)
			if err != nil || got.IncomeMinor != tc.income || got.DeductibleMinor != tc.deductible || got.NetMinor != tc.net {
				t.Fatalf("got %+v / %v", got, err)
			}
			if got.Year != in.Year || !got.From.Equal(in.From) || !got.To.Equal(in.To) || got.Currency != "USD" {
				t.Fatalf("window/currency changed: %+v", got)
			}
		})
	}
}

func TestNetIncomeWithLinksUsesCanonicalSplitLinesAndExclusions(t *testing.T) {
	in := netIncomeFixture()
	in.Transactions = []domain.Transaction{
		netIncomeRow("income", "", "USD", in.From, 10000),
		{ID: "mixed-other-parent", Date: in.From, CategoryID: "other", Amount: money.New(-1000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-600, "USD")}, {CategoryID: "other", Amount: money.New(-400, "USD")}}},
		{ID: "mixed-deductible-parent", Date: in.From, CategoryID: "ded", Amount: money.New(-1000, "USD"), Splits: []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-300, "USD")}, {CategoryID: "other", Amount: money.New(-900, "USD")}, {CategoryID: "ded", Amount: money.New(200, "USD")}}},
		{ID: "excluded", Date: in.From, ExcludeFromReports: true, CategoryID: "ded", Amount: money.New(math.MinInt64, "NOFX")},
		{ID: "transfer", Date: in.From, TransferAccountID: "other", CategoryID: "ded", Amount: money.New(math.MinInt64, "NOFX")},
	}
	before, _ := json.Marshal(in)
	got, err := NetIncomeWithLinks(in)
	if err != nil || got.IncomeMinor != 10000 || got.DeductibleMinor != 900 || got.NetMinor != 9100 {
		t.Fatalf("split/report policy: %+v / %v", got, err)
	}
	collection, err := GatherWithLinks(in.Transactions, in.Categories, in.Links, in.Year, in.From, in.To, in.Rates)
	if err != nil || got.DeductibleMinor != collection.Deductible.Total {
		t.Fatalf("gather parity: %+v / %v", collection, err)
	}
	flows, err := reports.IncomeExpenseSeriesFromLedger(budgeting.NewNettedLedger(in.Transactions, in.Links), []time.Time{in.From, in.To}, in.Rates)
	if err != nil || got.IncomeMinor != flows[0].Income {
		t.Fatalf("report parity: %+v / %v", flows, err)
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("input or nested split slice mutated")
	}
}

func TestNetIncomeWithLinksKeepsOutOfWindowRefundCounterparts(t *testing.T) {
	in := netIncomeFixture()
	in.Transactions = []domain.Transaction{netIncomeRow("income", "", "USD", in.From, 10000), netIncomeRow("purchase", "ded", "USD", in.From, -4000), netIncomeRow("refund", "ded", "USD", in.To, 1000)}
	in.Links = []domain.TxnLink{{ID: "refund-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}}}
	before, _ := json.Marshal(in)
	got, err := NetIncomeWithLinks(in)
	if err != nil || got.IncomeMinor != 10000 || got.DeductibleMinor != 3000 || got.NetMinor != 7000 {
		t.Fatalf("original-period attribution: %+v / %v", got, err)
	}
	// Selecting the later refund period must not turn that paired reimbursement
	// into unrelated income after its purchase has already been offset.
	in.From, in.To = in.To, in.To.AddDate(0, 1, 0)
	got, err = NetIncomeWithLinks(in)
	if err != nil || got.IncomeMinor != 0 || got.DeductibleMinor != 0 || got.NetMinor != 0 {
		t.Fatalf("refund double counted as income: %+v / %v", got, err)
	}
	in.From, in.To = netIncomeFixture().From, netIncomeFixture().To
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("refund preparation mutated caller records")
	}
	// Canonical legacy policy does not net incompatible native currencies just
	// because a link exists. The out-of-window refund needs no unrelated FX.
	in.Transactions[2].Amount = money.New(1000, "EUR")
	got, err = NetIncomeWithLinks(in)
	if err != nil || got.IncomeMinor != 10000 || got.DeductibleMinor != 4000 || got.NetMinor != 6000 {
		t.Fatalf("cross-currency link policy changed: %+v / %v", got, err)
	}
}

func TestNetIncomeWithLinksRefusesAmbiguousRefundAllocation(t *testing.T) {
	in := netIncomeFixture()
	original := netIncomeRow("purchase", "other", "USD", in.From, -100)
	original.Splits = []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-60, "USD")}, {CategoryID: "other", Amount: money.New(-40, "USD")}}
	in.Transactions = []domain.Transaction{original, netIncomeRow("refund", "", "USD", in.To, 40)}
	in.Links = []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}}}
	if got, err := NetIncomeWithLinks(in); err == nil || !strings.Contains(err.Error(), "category allocation") || !reflect.DeepEqual(got, NetIncome{}) {
		t.Fatalf("partial split refund guessed: %+v / %v", got, err)
	}
	in.Transactions[1].Amount = money.New(100, "USD")
	if got, err := NetIncomeWithLinks(in); err != nil || got.DeductibleMinor != 0 {
		t.Fatalf("fully refunded split: %+v / %v", got, err)
	}
	in.Transactions[0].Splits = nil
	in.Transactions[1].Amount = money.New(80, "USD")
	in.Links = append(in.Links, domain.TxnLink{ID: "second", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}})
	if got, err := NetIncomeWithLinks(in); err == nil || !reflect.DeepEqual(got, NetIncome{}) {
		t.Fatalf("overallocated refund: %+v / %v", got, err)
	}
}

func TestNetIncomeWithLinksCurrencyPrecisionAndSameCurrencyIndependence(t *testing.T) {
	in := netIncomeFixture()
	in.Rates = currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2, "JPY": 0.01}}
	in.Transactions = []domain.Transaction{netIncomeRow("income", "", "EUR", in.From, 100), netIncomeRow("expense", "ded", "JPY", in.From, -100)}
	got, err := NetIncomeWithLinks(in)
	if err != nil || got.IncomeMinor != 200 || got.DeductibleMinor != 100 || got.NetMinor != 100 {
		t.Fatalf("FX/JPY decimals: %+v / %v", got, err)
	}
	in.Rates = currency.Rates{Base: "JPY"}
	in.Transactions = []domain.Transaction{netIncomeRow("income", "", "JPY", in.From, 1234), netIncomeRow("expense", "ded", "JPY", in.From, -34)}
	got, err = NetIncomeWithLinks(in)
	if err != nil || got.NetMinor != 1200 || got.Currency != "JPY" {
		t.Fatalf("same-currency dependency/precision: %+v / %v", got, err)
	}
}

func TestNetIncomeWithLinksErrorsNeverReturnPartialFinancialFacts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rows  []domain.Transaction
		rates currency.Rates
		want  error
	}{
		{"income FX", []domain.Transaction{{Amount: money.New(1, "EUR")}}, currency.Rates{Base: "USD"}, currency.ErrUnknownRate},
		{"expense FX strict complete fold", []domain.Transaction{{CategoryID: "other", Amount: money.New(-1, "EUR")}}, currency.Rates{Base: "USD"}, currency.ErrUnknownRate},
		{"invalid FX", []domain.Transaction{{Amount: money.New(1, "EUR")}}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": math.NaN()}}, currency.ErrInvalidRate},
		{"minimum magnitude", []domain.Transaction{{CategoryID: "ded", Amount: money.New(math.MinInt64, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"income sum", []domain.Transaction{{Amount: money.New(math.MaxInt64, "USD")}, {Amount: money.New(1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"expense/category sum", []domain.Transaction{{CategoryID: "ded", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "ded", Amount: money.New(-1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"grand expense sum", []domain.Transaction{{CategoryID: "ded", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "other", Amount: money.New(-1, "USD")}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"deductible split sum", []domain.Transaction{{Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "ded", Amount: money.New(-1, "USD")}, {CategoryID: "other", Amount: money.New(math.MaxInt64, "USD")}}}}, currency.Rates{Base: "USD"}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := netIncomeFixture()
			in.Rates = tc.rates
			for i := range tc.rows {
				tc.rows[i].Date = in.From
			}
			in.Transactions = tc.rows
			got, err := NetIncomeWithLinks(in)
			if !errors.Is(err, tc.want) || !reflect.DeepEqual(got, NetIncome{}) {
				t.Fatalf("partial/zero fact on failure: %+v / %v", got, err)
			}
		})
	}
	// Both valid signed endpoints remain exact, rather than passing through float.
	in := netIncomeFixture()
	in.Transactions = []domain.Transaction{netIncomeRow("max", "", "USD", in.From, math.MaxInt64)}
	got, err := NetIncomeWithLinks(in)
	if err != nil || got.NetMinor != math.MaxInt64 {
		t.Fatalf("max income precision: %+v %v", got, err)
	}
	in.Transactions = []domain.Transaction{netIncomeRow("loss", "ded", "USD", in.From, -math.MaxInt64)}
	got, err = NetIncomeWithLinks(in)
	if err != nil || got.NetMinor != -math.MaxInt64 {
		t.Fatalf("max loss precision: %+v %v", got, err)
	}
}

func TestNetIncomeWithLinksValidatesMetadataWithoutRoundingWindow(t *testing.T) {
	for _, change := range []func(*NetIncomeInput){func(i *NetIncomeInput) { i.Year = 0 }, func(i *NetIncomeInput) { i.Year = 10000 }, func(i *NetIncomeInput) { i.From = time.Time{} }, func(i *NetIncomeInput) { i.To = i.From }, func(i *NetIncomeInput) { i.To = i.From.Add(-time.Hour) }, func(i *NetIncomeInput) { i.Rates.Base = "" }, func(i *NetIncomeInput) { i.Rates.Base = "FAKE" }} {
		in := netIncomeFixture()
		change(&in)
		if got, err := NetIncomeWithLinks(in); err == nil || !reflect.DeepEqual(got, NetIncome{}) {
			t.Fatalf("invalid metadata accepted: %+v / %v", got, err)
		}
	}
	in := netIncomeFixture()
	in.Year = 2025
	got, err := NetIncomeWithLinks(in)
	if err != nil || got.Year != 2025 || !got.From.Equal(in.From) || !got.To.Equal(in.To) {
		t.Fatalf("year label changed actual bounds: %+v %v", got, err)
	}
}
