// SPDX-License-Identifier: MIT

package txnfilter

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestToDateIncludesRowsWithClockTimes(t *testing.T) {
	row := domain.Transaction{ID: "late", Date: time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)}
	got := Apply([]domain.Transaction{row}, Criteria{To: "2026-10-31"})
	if len(got) != 1 || got[0].ID != row.ID {
		t.Fatalf("To-day 23:00 transaction was excluded: %+v", got)
	}
}

func TestAmountFiltersAndSearchUseBaseCurrency(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.0067, "EUR": 1.08}}
	labels := Labels{BaseCurrency: "USD", ToBase: rates.ToBase}
	tests := []struct {
		name string
		row  domain.Transaction
		crit Criteria
		want int
	}{
		{
			name: "5000 yen is below 100 US dollars",
			row:  domain.Transaction{ID: "yen", Amount: money.New(-5000, "JPY")},
			crit: Criteria{AmountMin: "100"},
			want: 0,
		},
		{
			name: "90 euros is above 95 US dollars",
			row:  domain.Transaction{ID: "euro", Amount: money.New(-9000, "EUR")},
			crit: Criteria{AmountMax: "95"},
			want: 0,
		},
		{
			name: "numeric search compares converted value",
			row:  domain.Transaction{ID: "euro-search", Amount: money.New(-9000, "EUR")},
			crit: Criteria{Text: "$97.20"},
			want: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyWithLabels([]domain.Transaction{tc.row}, tc.crit, labels)
			if len(got) != tc.want {
				t.Fatalf("got %d rows, want %d", len(got), tc.want)
			}
		})
	}
}

func TestAmountFilterReportsFailedConversion(t *testing.T) {
	row := domain.Transaction{ID: "no-rate", Amount: money.New(-5000, "JPY")}
	var reports []string
	labels := Labels{
		BaseCurrency: "USD",
		OnConversionError: func(txn domain.Transaction, err error) {
			reports = append(reports, fmt.Sprintf("%s: %v", txn.ID, err))
		},
	}
	got, issues := ApplyWithLabelsChecked([]domain.Transaction{row}, Criteria{AmountMin: "100"}, labels)
	if len(got) != 0 {
		t.Fatalf("unconvertible row matched: %+v", got)
	}
	if len(reports) != 1 || reports[0] == "" {
		t.Fatalf("conversion failures reported = %v, want one row-specific error", reports)
	}
	if len(issues) != 1 || issues[0].Transaction.ID != row.ID || issues[0].Err == nil {
		t.Fatalf("checked result issues = %+v, want the row and conversion error", issues)
	}
}

func TestLargePresetUsesBaseCurrency(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.0067, "EUR": 1.08}}
	rows := []domain.Transaction{
		{ID: "yen", Amount: money.New(-5000, "JPY")},
		{ID: "euro", Amount: money.New(-9000, "EUR")},
		{ID: "large-euro", Amount: money.New(-15000, "EUR")},
	}
	got := CountPresetsWithLabels(rows, Criteria{}, PresetScope{LargeMinor: 10000}, Labels{
		BaseCurrency: "USD",
		ToBase:       rates.ToBase,
	})
	if got.Large != 1 {
		t.Fatalf("Large count = %d, want only the €150 charge to clear $100", got.Large)
	}
}

func TestCountPresetsUsesPayeeLabels(t *testing.T) {
	row := domain.Transaction{
		ID: "alias", Payee: "OLD MERCHANT NAME", Desc: "charge",
		Amount: money.New(-25000, "USD"), Date: time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
	}
	labels := Labels{Payee: func(string) string { return "Clean Merchant" }}
	rows := []domain.Transaction{row}
	base := Criteria{Text: "clean merchant"}
	filterLabels := Labels{Payee: labels.Payee, BaseCurrency: "USD"}
	got := CountPresetsWithLabels(rows, base, PresetScope{
		LargeMinor: 20000,
		MonthStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		MonthEnd:   time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}, filterLabels)
	if got.Uncategorized != 1 || got.Large != 1 || got.ThisMonth != 1 {
		t.Fatalf("preset counts with alias search = %+v; labels must match the displayed rows", got)
	}
	large := base
	large.AmountMin = "200"
	if n := len(ApplyWithLabels(rows, large, filterLabels)); got.Large != n {
		t.Fatalf("Large chip promised %d rows, applying the base-currency preset yields %d", got.Large, n)
	}
}

func benchmarkPresetLedger() []domain.Transaction {
	rows := make([]domain.Transaction, 3300)
	for i := range rows {
		rows[i] = domain.Transaction{
			ID: fmt.Sprintf("txn-%04d", i), Desc: fmt.Sprintf("Merchant %d", i%37),
			Date:       time.Date(2026, time.August, i%31+1, 12, 0, 0, 0, time.UTC),
			Amount:     money.New(-int64((i%250+1)*325), "USD"),
			Tags:       []string{[]string{"needs-review", "ordinary"}[i%2]},
			CategoryID: []string{"", "food", "home"}[i%3],
		}
	}
	return rows
}

// BenchmarkCountPresets3300Before preserves the old four Apply+sort passes for
// comparable measurement against the one-pass implementation below.
func BenchmarkCountPresets3300Before(b *testing.B) {
	rows := benchmarkPresetLedger()
	scope := PresetScope{ReviewTag: "needs-review", LargeFor: func(domain.Transaction) int64 { return 10000 },
		MonthStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), MonthEnd: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	crit := Criteria{Text: "merchant"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var counts PresetCounts
		for _, txn := range Apply(rows, crit.Without(FieldUncategorized)) {
			if !txn.IsTransfer() && txn.CategoryID == "" {
				counts.Uncategorized++
			}
		}
		for _, txn := range Apply(rows, crit.Without(FieldTag)) {
			for _, tag := range txn.Tags {
				if tag == scope.ReviewTag {
					counts.NeedsReview++
					break
				}
			}
		}
		for _, txn := range Apply(rows, crit.Without(FieldAmountMin)) {
			if AbsAmount(txn) >= scope.LargeFor(txn) {
				counts.Large++
			}
		}
		for _, txn := range Apply(rows, crit.Without(FieldFrom).Without(FieldTo)) {
			if !txn.Date.Before(scope.MonthStart) && txn.Date.Before(scope.MonthEnd) {
				counts.ThisMonth++
			}
		}
		_ = counts
	}
}

func BenchmarkCountPresets3300After(b *testing.B) {
	rows := benchmarkPresetLedger()
	scope := PresetScope{ReviewTag: "needs-review", LargeMinor: 10000,
		MonthStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), MonthEnd: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	crit := Criteria{Text: "merchant"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CountPresetsWithLabels(rows, crit, scope, Labels{BaseCurrency: "USD"})
	}
}
