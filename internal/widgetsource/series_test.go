// SPDX-License-Identifier: MIT

package widgetsource

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func seriesTxn(id string, d time.Time, amt int64, cat string, tags []string, custom map[string]any) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "a", Date: d, Amount: money.New(amt, "USD"), CategoryID: cat, Tags: tags, Custom: custom}
}

// TestTxnFilterMatcher pins the three selector forms (tag / cat by id or name /
// custom-field value) and the transfer exclusion.
func TestTxnFilterMatcher(t *testing.T) {
	cats := []domain.Category{{ID: "cat-biz", Name: "Online business"}}
	d := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	tagged := seriesTxn("t1", d, -500, "", []string{"Business"}, nil)
	inCat := seriesTxn("t2", d, 900, "cat-biz", nil, nil)
	withCF := seriesTxn("t3", d, 700, "", nil, map[string]any{"project": "Side hustle"})
	transfer := seriesTxn("t4", d, -500, "", []string{"business"}, nil)
	transfer.TransferAccountID = "b"

	cases := []struct {
		filter string
		tx     domain.Transaction
		want   bool
	}{
		{"tag:business", tagged, true}, // case-insensitive
		{"tag:business", inCat, false},
		{"tag:business", transfer, false}, // transfers never match
		{"cat:cat-biz", inCat, true},
		{"cat:Online business", inCat, true}, // by display name
		{"cat:cat-biz", tagged, false},
		{"cf:project=Side hustle", withCF, true},
		{"cf:project=Personal", withCF, false},
	}
	for _, tc := range cases {
		match, err := TxnFilterMatcher(tc.filter, cats)
		if err != nil {
			t.Fatalf("%s: %v", tc.filter, err)
		}
		if got := match(tc.tx); got != tc.want {
			t.Errorf("%s on %s: got %v, want %v", tc.filter, tc.tx.ID, got, tc.want)
		}
	}
	for _, bad := range []string{"", "nonsense", "cf:project", "tag:"} {
		if _, err := TxnFilterMatcher(bad, cats); err == nil {
			t.Errorf("filter %q should be rejected", bad)
		}
	}
}

// TestFilteredFlowSeries sums matching transactions per month over the window.
func TestFilteredFlowSeries(t *testing.T) {
	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		seriesTxn("m1a", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), 10000, "cat-biz", nil, nil),
		seriesTxn("m1b", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), 2500, "cat-biz", nil, nil),
		seriesTxn("m2", time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), 4000, "cat-biz", nil, nil),
		seriesTxn("noise", time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC), 99999, "cat-other", nil, nil),
	}
	match, err := TxnFilterMatcher("cat:cat-biz", nil)
	if err != nil {
		t.Fatal(err)
	}
	fr := FilteredFlowSeries(txns, rates, now, 3, match, false)
	if fr.Rows != 3 {
		t.Fatalf("want 3 monthly rows, got %d", fr.Rows)
	}
	val, _ := fr.Column("value")
	// Window ends at the last COMPLETE month: Apr, May, Jun 2026 (July is
	// mid-flight and would plot a misleading cliff).
	if got := val.Num(0); got != 0 {
		t.Errorf("April sum = %v, want 0", got)
	}
	if got := val.Num(1); got != 4000 {
		t.Errorf("May sum = %v, want 4000", got)
	}
	if got := val.Num(2); got != 12500 {
		t.Errorf("June sum = %v, want 12500", got)
	}

	// abs=true plots each month's magnitude — a pure-expense "costs" series
	// reads as positive dollars instead of a chart of negatives.
	neg := []domain.Transaction{
		seriesTxn("e1", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), -8000, "cat-biz", nil, nil),
		seriesTxn("e2", time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), -3000, "cat-biz", nil, nil),
	}
	frAbs := FilteredFlowSeries(neg, rates, now, 3, match, true)
	valAbs, _ := frAbs.Column("value")
	if got := valAbs.Num(1); got != 3000 {
		t.Errorf("abs May = %v, want 3000", got)
	}
	if got := valAbs.Num(2); got != 8000 {
		t.Errorf("abs June = %v, want 8000", got)
	}
}

// TestFormulaSeries evaluates the closure per month and honors the format.
func TestFormulaSeries(t *testing.T) {
	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	var windows []time.Time
	fr := FormulaSeries(now, 3, "", "USD", func(start, end time.Time) (float64, bool) {
		windows = append(windows, start)
		return float64(len(windows)) * 1.5, true // 1.5, 3.0, 4.5 dollars
	})
	if fr.Rows != 3 || len(windows) != 3 {
		t.Fatalf("want 3 rows/windows, got %d/%d", fr.Rows, len(windows))
	}
	val, _ := fr.Column("value")
	if val.Type != domain.FieldMoney {
		t.Errorf("default format should be money, got %s", val.Type)
	}
	if got := val.Num(2); got != 450 { // 4.5 dollars → 450 minor
		t.Errorf("third value = %v minor, want 450", got)
	}
	pct := FormulaSeries(now, 2, "percent", "USD", func(start, end time.Time) (float64, bool) { return 42, true })
	pv, _ := pct.Column("value")
	if pv.Type != domain.FieldPercent || pv.Num(0) != 42 {
		t.Errorf("percent series: type %s value %v, want percent 42", pv.Type, pv.Num(0))
	}
}
