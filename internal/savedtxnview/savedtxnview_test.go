// SPDX-License-Identifier: MIT

package savedtxnview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func sampleTxns() []domain.Transaction {
	return []domain.Transaction{
		{ID: "a", AccountID: "acc1", CategoryID: "food", Desc: "Amazon order", Payee: "Amazon", Amount: money.New(-4500, "USD"), Date: mustDate("2026-06-01")},
		{ID: "b", AccountID: "acc1", CategoryID: "fees", Desc: "Bank fee", Amount: money.New(-1200, "USD"), Date: mustDate("2026-06-02")},
		{ID: "c", AccountID: "acc2", CategoryID: "food", Desc: "Amazon fresh", Payee: "Amazon", Amount: money.New(-9800, "USD"), Date: mustDate("2026-06-03")},
		{ID: "d", AccountID: "acc2", CategoryID: "pay", Desc: "Payday", Amount: money.New(250000, "USD"), Date: mustDate("2026-06-04")},
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		view    SavedTxnView
		wantErr error
	}{
		{"ok", SavedTxnView{Name: "All fees"}, nil},
		{"blank name", SavedTxnView{Name: "   "}, ErrNameRequired},
		{"empty name", SavedTxnView{}, ErrNameRequired},
		{"negative threshold", SavedTxnView{Name: "x", Threshold: -5}, ErrThresholdNegative},
		{"zero threshold ok", SavedTxnView{Name: "x", Threshold: 0}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.view.Validate(); got != tc.wantErr {
				t.Fatalf("Validate() = %v, want %v", got, tc.wantErr)
			}
		})
	}
}

func TestSummary(t *testing.T) {
	txns := sampleTxns()
	cases := []struct {
		name      string
		criteria  txnfilter.Criteria
		wantCount int
		wantTotal int64
	}{
		{"amazon by text", txnfilter.Criteria{Text: "amazon"}, 2, -14300},
		{"fees category", txnfilter.Criteria{Category: "fees"}, 1, -1200},
		{"all", txnfilter.Criteria{}, 4, 250000 - 4500 - 1200 - 9800},
		{"none", txnfilter.Criteria{Text: "nomatch"}, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := SavedTxnView{Name: tc.name, Criteria: tc.criteria}
			count, total := v.Summary(txns, nil)
			if count != tc.wantCount || total != tc.wantTotal {
				t.Fatalf("Summary() = (%d, %d), want (%d, %d)", count, total, tc.wantCount, tc.wantTotal)
			}
		})
	}
}

func TestSummaryAmountFunc(t *testing.T) {
	txns := sampleTxns()
	// A converter that doubles every amount (stand-in for FX conversion).
	v := SavedTxnView{Name: "amazon", Criteria: txnfilter.Criteria{Text: "amazon"}}
	_, total := v.Summary(txns, func(tx domain.Transaction) int64 { return tx.Amount.Amount * 2 })
	if total != -28600 {
		t.Fatalf("total with amount func = %d, want -28600", total)
	}
}

func TestCrossedThreshold(t *testing.T) {
	cases := []struct {
		name      string
		threshold int64
		total     int64
		want      bool
	}{
		{"no threshold", 0, -99999, false},
		{"under", 50000, -14300, false},
		{"at", 14300, -14300, true},
		{"over positive", 1000, 250000, true},
		{"magnitude of negative", 10000, -14300, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := SavedTxnView{Name: "x", Threshold: tc.threshold}
			if got := v.CrossedThreshold(tc.total); got != tc.want {
				t.Fatalf("CrossedThreshold(%d) = %v, want %v", tc.total, got, tc.want)
			}
		})
	}
}

func TestDismissalKeyEncodesThreshold(t *testing.T) {
	v := SavedTxnView{ID: "v1", Threshold: 500}
	if got := v.DismissalKey(); got != "v1@500" {
		t.Fatalf("DismissalKey() = %q, want v1@500", got)
	}
	v.Threshold = 750
	if got := v.DismissalKey(); got != "v1@750" {
		t.Fatalf("DismissalKey() after change = %q, want v1@750", got)
	}
}

func TestMapRoundTrip(t *testing.T) {
	kv := map[string]string{}
	v1 := SavedTxnView{ID: "1", Name: "Beta", Criteria: txnfilter.Criteria{Text: "amazon"}, Threshold: 500, CreatedAt: mustDate("2026-06-01")}
	v2 := SavedTxnView{ID: "2", Name: "alpha", Criteria: txnfilter.Criteria{Category: "fees"}, CreatedAt: mustDate("2026-06-02")}
	kv = Put(kv, v1)
	kv = Put(kv, v2)

	got := List(kv)
	if len(got) != 2 {
		t.Fatalf("List len = %d, want 2", len(got))
	}
	// Sorted case-insensitively by name: "alpha" before "Beta".
	if got[0].ID != "2" || got[1].ID != "1" {
		t.Fatalf("List order = [%s, %s], want [2, 1]", got[0].ID, got[1].ID)
	}
	// Round-trip fidelity of the criteria + threshold.
	if got[1].Criteria.Text != "amazon" || got[1].Threshold != 500 {
		t.Fatalf("round-trip lost data: %+v", got[1])
	}

	kv = Delete(kv, "1")
	if len(List(kv)) != 1 {
		t.Fatalf("after delete len = %d, want 1", len(List(kv)))
	}
}

func TestListSkipsCorrupt(t *testing.T) {
	kv := map[string]string{"good": `{"id":"good","name":"OK"}`, "bad": "{not json"}
	got := List(kv)
	if len(got) != 1 || got[0].ID != "good" {
		t.Fatalf("List should skip corrupt entries, got %+v", got)
	}
}

func TestNameTaken(t *testing.T) {
	views := []SavedTxnView{{ID: "1", Name: "All fees"}, {ID: "2", Name: "Amazon"}}
	if !NameTaken(views, "all fees", "") {
		t.Fatal("expected case-insensitive name clash")
	}
	if NameTaken(views, "All fees", "1") {
		t.Fatal("a view should not clash with itself")
	}
	if NameTaken(views, "New view", "") {
		t.Fatal("unused name should be free")
	}
}
