// SPDX-License-Identifier: MIT

package reviewqueue

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func rqUSD(minor int64) money.Money { return money.Money{Amount: minor, Currency: "USD"} }

func rqTxn(id, payee string, minor int64) domain.Transaction {
	return domain.Transaction{
		ID: id, Payee: payee, Desc: payee, Amount: rqUSD(minor),
		AccountID: "acct", Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}
}

// --- C492: splits ------------------------------------------------------------

func TestSplitThatReconcilesIsNotQueued(t *testing.T) {
	tx := rqTxn("t1", "Costco", -10000)
	tx.Splits = []domain.CategorySplit{
		{CategoryID: "c-groceries", Amount: rqUSD(-6000)},
		{CategoryID: "c-household", Amount: rqUSD(-4000)},
	}
	if !tx.SplitsReconcile() {
		t.Fatal("test fixture does not reconcile; fix the fixture")
	}
	if Needs(tx) {
		t.Error("a split that adds up is fully categorized — queueing it invites a " +
			"flat assignment that changes no budget math")
	}
}

func TestSplitThatDoesNotReconcileStaysQueued(t *testing.T) {
	tx := rqTxn("t2", "Costco", -10000)
	tx.Splits = []domain.CategorySplit{{CategoryID: "c-groceries", Amount: rqUSD(-6000)}}
	if !Needs(tx) {
		t.Error("an unbalanced split leaves money unaccounted for and must stay queued")
	}
	if got := ReasonFor(tx); got != ReasonSplitUnbalanced {
		t.Errorf("ReasonFor = %v, want ReasonSplitUnbalanced — assigning a category "+
			"would not fix it, so the UI must not offer that as the primary action", got)
	}
}

func TestFlatTransactionsUnaffectedBySplitRule(t *testing.T) {
	if !Needs(rqTxn("t3", "Anywhere", -500)) {
		t.Error("an uncategorized flat charge must still queue")
	}
	done := rqTxn("t4", "Anywhere", -500)
	done.CategoryID = "c-x"
	if Needs(done) {
		t.Error("a categorized flat charge must not queue")
	}
}

// --- C493: durable resolutions ----------------------------------------------

func TestDismissedNeverReturns(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	rs := Resolutions{}
	rs.Dismiss("t1")
	txns := []domain.Transaction{rqTxn("t1", "A", -100), rqTxn("t2", "B", -100)}

	got := QueueOpen(txns, rs, now)
	if len(got) != 1 || got[0].ID != "t2" {
		t.Fatalf("QueueOpen = %v, want only t2", ids(got))
	}
	// Still gone a year later.
	if len(QueueOpen(txns, rs, now.AddDate(1, 0, 0))) != 1 {
		t.Error("a dismissal is the user's standing decision and must not expire")
	}
	if CountOpen(txns, rs, now) != 1 {
		t.Error("CountOpen must agree with QueueOpen")
	}
}

func TestSnoozeExpires(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	rs := Resolutions{}
	rs.Snooze("t1", now.Add(24*time.Hour))
	txns := []domain.Transaction{rqTxn("t1", "A", -100)}

	if len(QueueOpen(txns, rs, now)) != 0 {
		t.Error("a snoozed charge must be hidden while the snooze holds")
	}
	if len(QueueOpen(txns, rs, now.Add(48*time.Hour))) != 1 {
		t.Error("a snoozed charge must come back after its date — this is the fix for " +
			"skips that were lost on reload")
	}
}

func TestReopenAndPrune(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	rs := Resolutions{}
	rs.Snooze("t1", now.Add(time.Hour))
	rs.Dismiss("t2")
	rs.Reopen("t1")
	if rs.Suppressed("t1", now) {
		t.Error("Reopen should clear the decision")
	}

	rs = Resolutions{}
	rs.Snooze("expired", now.Add(-time.Hour))
	rs.Snooze("live", now.Add(time.Hour))
	rs.Dismiss("kept")
	rs.Prune(now)
	if _, ok := rs["expired"]; ok {
		t.Error("an expired snooze should be pruned so the map cannot grow forever")
	}
	if _, ok := rs["live"]; !ok {
		t.Error("a live snooze must survive pruning")
	}
	if _, ok := rs["kept"]; !ok {
		t.Error("a dismissal must survive pruning")
	}
}

func TestNilResolutionsIsUnfiltered(t *testing.T) {
	txns := []domain.Transaction{rqTxn("t1", "A", -100)}
	if len(QueueOpen(txns, nil, time.Now())) != 1 {
		t.Error("a nil Resolutions must behave as no decisions at all")
	}
}

// --- C494: grouping + ordering ----------------------------------------------

// TestMerchantKeyGroupsVaryingDescriptors is the fix for a batch that promised a
// grouping it would not honour.
func TestMerchantKeyGroupsVaryingDescriptors(t *testing.T) {
	a := MerchantKey(rqTxn("1", "AMZN MKTP US*2H4RT9", -100))
	b := MerchantKey(rqTxn("2", "AMZN MKTP US*8K1QP2", -100))
	if a == "" || a != b {
		t.Errorf("MerchantKey: %q vs %q — charges displayed as one merchant must group", a, b)
	}
	if MerchantKey(rqTxn("3", "", 0)) != "" {
		t.Error("an empty descriptor must produce an empty key")
	}
	// Desc is the fallback when Payee is blank.
	d := domain.Transaction{ID: "4", Desc: "SHELL OIL 5744", Amount: rqUSD(-100)}
	if MerchantKey(d) == "" {
		t.Error("MerchantKey should fall back to Desc")
	}
}

func TestGroupByMerchantOrdersByConfidenceThenSize(t *testing.T) {
	in := []Ranked{
		{Txn: rqTxn("a1", "AMZN MKTP US*1", -100), Confidence: 3},
		{Txn: rqTxn("a2", "AMZN MKTP US*2", -100), Confidence: 3},
		{Txn: rqTxn("s1", "SHELL OIL 1", -100), Confidence: 3},
		{Txn: rqTxn("v1", "VENMO PAYMENT 9", -100), Confidence: 0},
	}
	got := GroupByMerchant(in)
	if len(got) != 3 {
		t.Fatalf("got %d groups, want 3: %+v", len(got), got)
	}
	// Amazon: same confidence as Shell but more items, so it clears more work.
	if len(got[0].Items) != 2 {
		t.Errorf("first group = %q with %d items, want the 2-item Amazon group",
			got[0].Merchant, len(got[0].Items))
	}
	// The unresolvable merchant sinks to the bottom — the head of the queue
	// should not be the one charge nothing can answer.
	if got[len(got)-1].Confidence != 0 {
		t.Errorf("last group confidence = %d, want the 0-confidence group last", got[len(got)-1].Confidence)
	}
}

// TestGroupConfidenceIsTheWorstMember: confirming a whole merchant must never
// look safer than its least certain row.
func TestGroupConfidenceIsTheWorstMember(t *testing.T) {
	got := GroupByMerchant([]Ranked{
		{Txn: rqTxn("a1", "AMZN MKTP US*1", -100), Confidence: 3},
		{Txn: rqTxn("a2", "AMZN MKTP US*2", -100), Confidence: 1},
	})
	if len(got) != 1 {
		t.Fatalf("want 1 group, got %d", len(got))
	}
	if got[0].Confidence != 1 {
		t.Errorf("group confidence = %d, want 1 (the worst member)", got[0].Confidence)
	}
}

func TestGroupTotal(t *testing.T) {
	got := GroupByMerchant([]Ranked{
		{Txn: rqTxn("a1", "AMZN MKTP US*1", -2500)},
		{Txn: rqTxn("a2", "AMZN MKTP US*2", -1000)},
	})
	if len(got) != 1 || got[0].Total() != -3500 {
		t.Errorf("Total = %d, want -3500", got[0].Total())
	}
}

func ids(in []domain.Transaction) []string {
	out := make([]string, len(in))
	for i, t := range in {
		out[i] = t.ID
	}
	return out
}
