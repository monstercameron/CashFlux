// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// legacySeries deliberately retains the original range-by-range algorithms as
// an independent reference and benchmark for the one-pass implementations.
func legacySeries(txns []domain.Transaction, bounds []time.Time, rates currency.Rates) ([]PeriodFlow, []CategoryTrend, []PayeeTrend, error) {
	n := len(bounds) - 1
	if n < 1 {
		return nil, nil, nil, nil
	}
	flows := make([]PeriodFlow, 0, n)
	catBuckets := make([]map[string]int64, n)
	payeeBuckets := make([]map[string]int64, n)
	names, catIDs := map[string]string{}, map[string]bool{}
	payeeSums := map[string]int64{}
	for i := 0; i < n; i++ {
		flow, err := IncomeVsExpense(txns, bounds[i], bounds[i+1], rates)
		if err != nil {
			return nil, nil, nil, err
		}
		flows = append(flows, flow)
		cats, err := categoryTotals(txns, bounds[i], bounds[i+1], rates)
		if err != nil {
			return nil, nil, nil, err
		}
		catBuckets[i] = cats
		for id := range cats {
			catIDs[id] = true
		}
		payees, spellings, err := payeeTotals(txns, bounds[i], bounds[i+1], rates)
		if err != nil {
			return nil, nil, nil, err
		}
		payeeBuckets[i] = payees
		for key, name := range spellings {
			if _, ok := names[key]; !ok {
				names[key] = name
			}
		}
		for key, amount := range payees {
			payeeSums[key] += amount
		}
	}
	cats := make([]CategoryTrend, 0, len(catIDs))
	for id := range catIDs {
		row := CategoryTrend{CategoryID: id, Spend: make([]int64, n)}
		for i, b := range catBuckets {
			row.Spend[i] = b[id]
			row.Total += b[id]
		}
		row.DeltaPct, row.HasDelta = ledger.PercentChange(row.Spend[n-1], row.Spend[0])
		cats = append(cats, row)
	}
	sort.Slice(cats, func(i, j int) bool {
		if cats[i].Total != cats[j].Total {
			return cats[i].Total > cats[j].Total
		}
		return cats[i].CategoryID < cats[j].CategoryID
	})
	keys := make([]string, 0, len(payeeSums))
	for key := range payeeSums {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if payeeSums[keys[i]] != payeeSums[keys[j]] {
			return payeeSums[keys[i]] > payeeSums[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var payees []PayeeTrend
	for _, key := range keys {
		row := PayeeTrend{Payee: names[key], Spend: make([]int64, n)}
		for i, b := range payeeBuckets {
			row.Spend[i] = b[key]
		}
		payees = append(payees, row)
	}
	return flows, cats, payees, nil
}

func TestBucketSeriesMatchesRangeReference(t *testing.T) {
	start := dt(2026, time.January, 1)
	ordered := []time.Time{start, start.AddDate(0, 1, 0), start.AddDate(0, 2, 0), start.AddDate(0, 3, 0)}
	rng := rand.New(rand.NewPCG(17, 39))
	txns := make([]domain.Transaction, 0, 500)
	for i := 0; i < 500; i++ {
		currencyCode := []string{"USD", "EUR", "JPY"}[i%3]
		amount := int64(rng.IntN(50000) + 1)
		if i%4 != 0 {
			amount = -amount
		}
		txn := domain.Transaction{ID: fmt.Sprint(i), Date: start.Add(time.Duration(rng.IntN(120)-15) * 24 * time.Hour), Amount: money.New(amount, currencyCode), CategoryID: fmt.Sprint(i % 7), Desc: []string{" Shop ", "SHOP", "other", ""}[i%4]}
		if i%11 == 0 {
			txn.TransferAccountID = "other"
		}
		if i%13 == 0 {
			txn.ExcludeFromReports = true
		}
		if i%17 == 0 && amount < 0 {
			txn.Splits = []domain.CategorySplit{{CategoryID: "a", Amount: money.New(amount/2, currencyCode)}, {CategoryID: "b", Amount: money.New(amount-amount/2, currencyCode)}}
		}
		txns = append(txns, txn)
	}
	// Exact boundaries and unsorted spelling changes expose bucket/label drift.
	txns = append(txns, domain.Transaction{Date: ordered[1], Amount: money.New(-100, "USD"), Desc: "shop"}, domain.Transaction{Date: ordered[3], Amount: money.New(-999999, "USD")})
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.13, "JPY": 0.0067}}
	for name, bounds := range map[string][]time.Time{"ordered": ordered, "repeated": {ordered[0], ordered[1], ordered[1], ordered[3]}, "overlap": {ordered[0], ordered[2], ordered[1], ordered[3]}, "none": nil, "single": {start}} {
		t.Run(name, func(t *testing.T) {
			wantFlow, wantCats, wantPayees, err := legacySeries(txns, bounds, rates)
			if err != nil {
				t.Fatal(err)
			}
			gotFlow, err := IncomeExpenseSeries(txns, bounds, rates)
			if err != nil {
				t.Fatal(err)
			}
			gotCats, err := CategoryTrends(txns, bounds, rates)
			if err != nil {
				t.Fatal(err)
			}
			gotPayees, err := PayeeTrends(txns, bounds, rates, 0)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotFlow, wantFlow) || !reflect.DeepEqual(gotCats, wantCats) || !reflect.DeepEqual(gotPayees, wantPayees) {
				t.Fatalf("series diverged from independent range reference\nflow: %v / %v\ncategories: %v / %v\npayees: %v / %v", gotFlow, wantFlow, gotCats, wantCats, gotPayees, wantPayees)
			}
		})
	}
}

func TestBucketSeriesRefundsNetOnceAcrossMonths(t *testing.T) {
	start := dt(2026, time.March, 1)
	txns := []domain.Transaction{
		{ID: "buy", Date: start.AddDate(0, 0, 2), Amount: money.New(-12000, "USD"), CategoryID: "clothing"},
		{ID: "refund", Date: start.AddDate(0, 1, 2), Amount: money.New(4000, "USD"), CategoryID: "clothing"},
	}
	SetRefundLinks([]domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: money.New(4000, "USD")}})
	defer SetRefundLinks(nil)
	bounds := []time.Time{start, start.AddDate(0, 1, 0), start.AddDate(0, 2, 0)}
	flows, err := IncomeExpenseSeries(txns, bounds, usdRates())
	if err != nil {
		t.Fatal(err)
	}
	cats, err := CategoryTrends(txns, bounds, usdRates())
	if err != nil {
		t.Fatal(err)
	}
	if flows[0].Expense != 8000 || flows[1].Income != 0 || flows[1].Expense != 0 || !reflect.DeepEqual(cats[0].Spend, []int64{8000, 0}) {
		t.Fatalf("refund drift: %v / %v", flows, cats)
	}
	if txns[0].Amount.Amount != -12000 || txns[1].Amount.Amount != 4000 {
		t.Fatal("input mutated")
	}
}

func TestBucketSeriesSkipsExcludedMissingFX(t *testing.T) {
	start := dt(2026, time.June, 1)
	bounds := []time.Time{start, start.AddDate(0, 1, 0)}
	txns := []domain.Transaction{
		{Date: start, Amount: money.New(-100, "EUR"), ExcludeFromReports: true},
		{Date: bounds[1], Amount: money.New(-100, "EUR")},
		{Date: start, Amount: money.New(-100, "EUR"), TransferAccountID: "transfer"},
	}
	for _, bad := range []bool{false, true} {
		if bad {
			txns = append(txns, domain.Transaction{Date: start, Amount: money.New(-100, "EUR")})
		}
		_, flowErr := IncomeExpenseSeries(txns, bounds, usdRates())
		_, catErr := CategoryTrends(txns, bounds, usdRates())
		_, payeeErr := PayeeTrends(txns, bounds, usdRates(), 0)
		if (flowErr != nil) != bad || (catErr != nil) != bad || (payeeErr != nil) != bad {
			t.Fatalf("bad=%t errors: %v / %v / %v", bad, flowErr, catErr, payeeErr)
		}
	}
}

func BenchmarkReportSeries(b *testing.B) {
	for _, size := range []int{1000, 25000} {
		start := dt(2024, time.January, 1)
		bounds := make([]time.Time, 25)
		for i := range bounds {
			bounds[i] = start.AddDate(0, i, 0)
		}
		txns := make([]domain.Transaction, size)
		for i := range txns {
			amount := -int64(100 + i%50000)
			if i%7 == 0 {
				amount = -amount
			}
			txns[i] = domain.Transaction{Date: start.AddDate(0, 0, i%1095-365), Amount: money.New(amount, "USD"), CategoryID: fmt.Sprint(i % 24), Desc: fmt.Sprintf("Merchant %d", i%150), AccountID: fmt.Sprint(i % 20)}
		}
		b.Run(fmt.Sprintf("%d/legacy", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, _, err := legacySeries(txns, bounds, usdRates()); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("%d/bucketed", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				flows, err := IncomeExpenseSeries(txns, bounds, usdRates())
				if err != nil {
					b.Fatal(err)
				}
				_ = SavingsRates(flows)
				if _, err := CategoryTrends(txns, bounds, usdRates()); err != nil {
					b.Fatal(err)
				}
				if _, err := PayeeTrends(txns, bounds, usdRates(), 0); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
