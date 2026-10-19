// SPDX-License-Identifier: MIT

package budgeting

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
)

func TestBCS1NettedLedgerIsIdempotentAndScopeLocal(t *testing.T) {
	raw := []domain.Transaction{
		{ID: "buy", CategoryID: "food", Date: mustDate("2026-10-10"), Amount: usd(-10000)},
		{ID: "refund", CategoryID: "food", Date: mustDate("2026-11-10"), Amount: usd(6000)},
	}
	links := []domain.TxnLink{{ID: "link", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: usd(6000)}}
	once := NewNettedLedger(raw, links)
	twice := NewNettedLedger(once, links)
	if !reflect.DeepEqual(once, twice) || &once[0] != &twice[0] {
		t.Fatal("netting an existing ledger changed or copied it")
	}

	if raw[0].Amount.Amount != -10000 || raw[1].Amount.Amount != 6000 {
		t.Fatal("raw ledger changed")
	}
	b := domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Limit: usd(20000)}
	rates := currency.Rates{Base: "USD"}
	tests := []struct {
		name   string
		ledger NettedLedger
		month  time.Time
		want   int64
	}{
		{"October once", once, mustDate("2026-10-01"), 4000},
		{"October twice", twice, mustDate("2026-10-01"), 4000},
		{"November once", once, mustDate("2026-11-01"), 0},
		{"November twice", twice, mustDate("2026-11-01"), 0},
		{"different scope has no links", NewNettedLedger(raw), mustDate("2026-10-01"), 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			end := tt.month.AddDate(0, 1, 0)
			spent, err := SpentRollup(b, tt.ledger, tt.month, end, rates, nil)
			if err != nil || spent.Amount != tt.want {
				t.Fatalf("spend=%v err=%v want=%d", spent, err, tt.want)
			}
			status, err := EvaluateRollup(b, tt.ledger, tt.month, end, rates, DefaultNearThreshold, nil)
			if err != nil || status.Spent.Amount != tt.want {
				t.Fatalf("status=%+v err=%v", status, err)
			}
			slices, err := CompositionSlices(b, tt.ledger, tt.month, end, rates, nil)
			var total int64
			for _, slice := range slices {
				total += slice.Amount.Amount
			}
			if err != nil || total != tt.want {
				t.Fatalf("composition=%+v err=%v", slices, err)
			}
			rows, err := ContributingTransactions(b, tt.ledger, tt.month, end, nil)
			total = 0
			for _, row := range rows {
				total -= row.Amount.Amount
			}
			if err != nil || total != tt.want {
				t.Fatalf("contributions=%+v err=%v", rows, err)
			}
		})
	}
}

func TestBCS1EvaluationDoesNotCopyLedger(t *testing.T) {
	budgets, _, txns, links := bcs1Fixture()
	ledger := NewNettedLedger(txns, links)
	start, end := mustDate("2026-01-01"), mustDate("2027-01-01")
	allocs := testing.AllocsPerRun(3, func() {
		if &ledger.Transactions()[0] != &ledger[0] {
			t.Fatal("Transactions copied the ledger")
		}
		if _, err := EvaluateRollup(budgets[0], ledger, start, end, currency.Rates{Base: "USD"}, DefaultNearThreshold, nil); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("evaluation allocated %.0f objects; want a borrowed ledger view", allocs)
	}
}

func TestBCS1TrueUpsUseNettedHistoryAndCurrentBase(t *testing.T) {
	b := domain.Budget{ID: "b", CategoryID: "food", Period: domain.PeriodMonthly, Limit: usd(10000), LimitHistory: map[string]int64{"2026-10-01": 20000}}
	raw := []domain.Transaction{{ID: "buy", CategoryID: "food", Date: mustDate("2026-09-10"), Amount: usd(-18000)},
		{ID: "refund", CategoryID: "food", Date: mustDate("2026-10-10"), Amount: usd(6000)}}
	links := []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: usd(6000)}}
	for _, tt := range []struct {
		name   string
		budget domain.Budget
		want   int
	}{
		{"current history cap", b, 0},
		{"saving is not a spending true-up", func() domain.Budget { v := b; v.Direction = domain.DirectionSave; v.LimitHistory = nil; return v }(), 0},
		{"spending net refund", func() domain.Budget { v := b; v.LimitHistory = nil; return v }(), 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ups, err := SuggestTrueUps([]domain.Budget{tt.budget}, NewNettedLedger(raw, links), nil, mustDate("2026-10-19"), currency.Rates{Base: "USD"})
			if err != nil || len(ups) != tt.want {
				t.Fatalf("true-ups=%+v err=%v want count %d", ups, err, tt.want)
			}
			if len(ups) > 0 && ups[0].LearnedMinor != 12000 {
				t.Fatalf("learned=%d want 12000", ups[0].LearnedMinor)
			}
		})
	}
}

func TestBCS1SparseNettingPreservesRefundResolver(t *testing.T) {
	raw := []domain.Transaction{{ID: "buy", CategoryID: "food", Amount: usd(-10000)}, {ID: "refund", CategoryID: "food", Amount: usd(6000)}, {ID: "unrelated", Amount: usd(-500)}}
	for _, tt := range []struct {
		name  string
		links []domain.TxnLink
	}{
		{"none", nil},
		{"explicit", []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: usd(6000)}}},
		{"inferred", []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}}}},
		{"purchase cap", []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: usd(20000)}}},
		{"missing member", []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "absent"}, Amount: usd(6000)}}},
		{"currency mismatch", []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: money.New(6000, "EUR")}}},
		{"order group", []domain.TxnLink{{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"buy", "refund"}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := NewNettedLedger(raw, tt.links)
			want := txnlinks.NetTransactions(raw, tt.links)
			if !reflect.DeepEqual([]domain.Transaction(got), want) {
				t.Fatalf("sparse netting=%+v want=%+v", got, want)
			}
		})
	}
}

func TestBCS1RecommendationAcceptsTagOnlyLedgerScope(t *testing.T) {
	b := domain.Budget{ID: "tags", TrackedTags: []string{"trip"}, Period: domain.PeriodMonthly, Limit: usd(10000)}
	txns := NewNettedLedger([]domain.Transaction{{ID: "trip", Tags: []string{"TRIP"}, CategoryID: "other", Date: mustDate("2026-09-10"), Amount: usd(-5000)}})
	recommendation, err := RecommendBudgetLimitWithContext(b, nil, txns, mustDate("2026-10-19"), 3, usd(0), currency.Rates{Base: "USD"}, time.Monday, time.Time{})
	if err != nil || !recommendation.HasEvidence || recommendation.Suggested != 5000 {
		t.Fatalf("tag recommendation=%+v err=%v", recommendation, err)
	}
}
