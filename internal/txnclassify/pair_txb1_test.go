// SPDX-License-Identifier: MIT

package txnclassify

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestFindReciprocalUsesTransferGroupID(t *testing.T) {
	source := domain.Transaction{ID: "source", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "g1", Date: pairDay, Amount: money.New(-50000, "USD")}
	wrongGroup := domain.Transaction{ID: "lookalike", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "g2", Date: pairDay.AddDate(0, 0, 1), Amount: money.New(50000, "USD")}
	matchingOldDate := domain.Transaction{ID: "real", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "g1", Date: pairDay.AddDate(0, 0, 30), Amount: money.New(45000, "EUR")}

	t.Run("rejects lookalike with another group", func(t *testing.T) {
		if _, ok := FindReciprocal(source, "savings", []domain.Transaction{source, wrongGroup}); ok {
			t.Fatal("different transfer group was treated as the counterpart")
		}
	})
	t.Run("accepts same group outside date heuristic", func(t *testing.T) {
		match, ok := FindReciprocal(source, "savings", []domain.Transaction{source, matchingOldDate})
		if !ok || match.Txn.ID != "real" {
			t.Fatalf("match = %+v, ok=%v; want the same-group leg regardless of date", match, ok)
		}
	})
}

func TestVisibleTransferSummaryPairsAndIndexesRows(t *testing.T) {
	rows := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "g1", Amount: money.New(-10000, "USD")},
		{ID: "in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "g1", Amount: money.New(10000, "USD")},
		{ID: "fx-out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "g-fx", Amount: money.New(-10000, "USD")},
		{ID: "fx-in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "g-fx", Amount: money.New(9000, "EUR")},
		{ID: "orphan", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "g2", Amount: money.New(-2500, "USD")},
		{ID: "legacy-out", AccountID: "checking", TransferAccountID: "savings", Amount: money.New(-5000, "USD"), Date: pairDay},
		{ID: "legacy-in", AccountID: "savings", TransferAccountID: "checking", Amount: money.New(5000, "USD"), Date: pairDay.AddDate(0, 0, 2)},
	}
	got := SummarizeVisibleTransfers(rows)
	wantCounted := []bool{true, false, true, false, true, true, false}
	for i := range wantCounted {
		if got.CountedRows[i] != wantCounted[i] {
			t.Errorf("CountedRows[%d] = %v, want %v", i, got.CountedRows[i], wantCounted[i])
		}
	}
	if got.UnpairedRows != 1 {
		t.Fatalf("UnpairedRows = %d, want 1", got.UnpairedRows)
	}
	rows = make([]domain.Transaction, 3300)
	for i := 0; i < len(rows); i += 2 {
		group := "g" + time.Unix(int64(i), 0).Format("150405")
		rows[i] = domain.Transaction{ID: group + "-out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: group, Amount: money.New(-1, "USD")}
		rows[i+1] = domain.Transaction{ID: group + "-in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: group, Amount: money.New(1, "USD")}
	}
	got = SummarizeVisibleTransfers(rows)
	count := 0
	for _, counted := range got.CountedRows {
		if counted {
			count++
		}
	}
	if count != 1650 || got.UnpairedRows != 0 {
		t.Fatalf("large summary counted=%d unpaired=%d; want 1650 and 0", count, got.UnpairedRows)
	}
}

func TestVisibleTransferSummaryRecognizesLiabilitySignConventions(t *testing.T) {
	tests := []struct {
		name          string
		first, second int64
	}{
		{name: "opposite-sign asset pair", first: -5000, second: 5000},
		{name: "both-negative liability pair", first: -5000, second: -5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := []domain.Transaction{
				{ID: "asset-side", AccountID: "checking", TransferAccountID: "loan", TransferGroupID: "group", Amount: money.New(tt.first, "USD")},
				{ID: "liability-side", AccountID: "loan", TransferAccountID: "checking", TransferGroupID: "group", Amount: money.New(tt.second, "USD")},
			}
			got := SummarizeVisibleTransfers(rows)
			if got.UnpairedRows != 0 {
				t.Errorf("UnpairedRows = %d, want 0", got.UnpairedRows)
			}
			counted := 0
			for _, isCounted := range got.CountedRows {
				if isCounted {
					counted++
				}
			}
			if counted != 1 {
				t.Errorf("counted rows = %d, want 1", counted)
			}
		})
	}
}
