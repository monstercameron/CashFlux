// SPDX-License-Identifier: MIT

package reviewqueue_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
)

func usd(n int64) money.Money { return money.Money{Amount: n, Currency: "USD"} }

func tx(id, payee string, minor int64, day int) domain.Transaction {
	return domain.Transaction{
		ID: id, Payee: payee, Desc: payee, Amount: usd(minor), AccountID: "a",
		Date: time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC),
	}
}

// --- grouping is a partition -------------------------------------------------

// TestGroupByMerchantPartitionsTheInput: every queued charge must appear in
// exactly ONE group. A charge that lands in two groups gets categorized twice
// (and double-counted in the footer); a charge in none silently disappears from
// a surface whose whole job is to show you everything that needs attention.
func TestGroupByMerchantPartitionsTheInput(t *testing.T) {
	in := []reviewqueue.Ranked{
		{Txn: tx("a1", "AMZN MKTP US*1", -100, 1), Confidence: 3},
		{Txn: tx("a2", "AMZN MKTP US*2", -200, 2), Confidence: 3},
		{Txn: tx("s1", "SHELL OIL 1", -300, 3), Confidence: 2},
		{Txn: tx("v1", "VENMO PAYMENT 9", -400, 4), Confidence: 0},
		{Txn: tx("e1", "", -500, 5), Confidence: 0}, // no payee and no desc
	}
	groups := reviewqueue.GroupByMerchant(in)

	seen := map[string]int{}
	for _, g := range groups {
		for _, item := range g.Items {
			seen[item.ID]++
		}
	}
	if len(seen) != len(in) {
		t.Errorf("grouped %d distinct charges, input had %d", len(seen), len(in))
	}
	for _, r := range in {
		switch seen[r.Txn.ID] {
		case 1: // exactly once, as required
		case 0:
			t.Errorf("charge %q vanished — it is in no group", r.Txn.ID)
		default:
			t.Errorf("charge %q appears in %d groups", r.Txn.ID, seen[r.Txn.ID])
		}
	}

	// Totals must add up too, or the footer lies about what a confirm will touch.
	var groupSum, inputSum int64
	for _, g := range groups {
		groupSum += g.Total()
	}
	for _, r := range in {
		inputSum += r.Txn.Amount.Amount
	}
	if groupSum != inputSum {
		t.Errorf("group totals sum to %d, input sums to %d", groupSum, inputSum)
	}
}

// TestGroupByMerchantOrderingIsATotalOrder: the display order must be stable and
// deterministic, or the row under the keyboard cursor moves between renders.
func TestGroupByMerchantOrderingIsATotalOrder(t *testing.T) {
	in := []reviewqueue.Ranked{
		{Txn: tx("a1", "AMZN MKTP US*1", -100, 1), Confidence: 3},
		{Txn: tx("b1", "SHELL OIL 1", -100, 2), Confidence: 3},
		{Txn: tx("c1", "NETFLIX.COM", -100, 3), Confidence: 3},
		{Txn: tx("d1", "VENMO PAYMENT", -100, 4), Confidence: 1},
	}
	first := reviewqueue.GroupByMerchant(in)
	for i := 0; i < 50; i++ {
		again := reviewqueue.GroupByMerchant(in)
		if len(again) != len(first) {
			t.Fatalf("group count changed between runs")
		}
		for j := range first {
			if again[j].Key != first[j].Key {
				t.Fatalf("order changed at %d: %q then %q", j, first[j].Key, again[j].Key)
			}
		}
	}
	// Confidence descending is the documented primary key.
	for i := 1; i < len(first); i++ {
		if first[i-1].Confidence < first[i].Confidence {
			t.Errorf("group %d (conf %d) sorts before %d (conf %d)",
				i-1, first[i-1].Confidence, i, first[i].Confidence)
		}
	}
}

// TestGroupConfidenceNeverExceedsAnyMember: confirming a merchant must never
// look safer than its least certain charge.
func TestGroupConfidenceNeverExceedsAnyMember(t *testing.T) {
	in := []reviewqueue.Ranked{
		{Txn: tx("a1", "AMZN MKTP US*1", -100, 1), Confidence: 3},
		{Txn: tx("a2", "AMZN MKTP US*2", -100, 2), Confidence: 1},
		{Txn: tx("a3", "AMZN MKTP US*3", -100, 3), Confidence: 2},
	}
	for _, g := range reviewqueue.GroupByMerchant(in) {
		for _, item := range g.Items {
			for _, r := range in {
				if r.Txn.ID == item.ID && g.Confidence > r.Confidence {
					t.Errorf("group %q claims confidence %d but member %q has %d",
						g.Key, g.Confidence, item.ID, r.Confidence)
				}
			}
		}
	}
}

// --- resolutions -------------------------------------------------------------

// TestResolutionsSurviveJSONRoundTrip: decisions are persisted as JSON in the
// preserved KV. If the shape does not round-trip, every snooze silently becomes
// permanent or evaporates on the next boot.
func TestResolutionsSurviveJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	rs := reviewqueue.Resolutions{}
	rs.Snooze("snoozed", now.Add(48*time.Hour))
	rs.Dismiss("dismissed")

	blob, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back reviewqueue.Resolutions
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.Suppressed("dismissed", now) {
		t.Error("a dismissal did not survive the round trip")
	}
	if !back.Suppressed("snoozed", now) {
		t.Error("a live snooze did not survive the round trip")
	}
	if back.Suppressed("snoozed", now.Add(72*time.Hour)) {
		t.Error("the snooze lost its expiry and became permanent")
	}
	if back.Suppressed("never-seen", now) {
		t.Error("an unknown id must not be suppressed")
	}
}

// TestPruneIsIdempotentAndKeepsDecisions: pruning runs on every load, so it must
// converge and must never quietly discard a dismissal.
func TestPruneIsIdempotentAndKeepsDecisions(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	rs := reviewqueue.Resolutions{}
	rs.Snooze("expired", now.Add(-time.Hour))
	rs.Snooze("live", now.Add(time.Hour))
	rs.Dismiss("kept")

	rs.Prune(now)
	afterFirst := len(rs)
	rs.Prune(now)
	if len(rs) != afterFirst {
		t.Errorf("Prune is not idempotent: %d then %d", afterFirst, len(rs))
	}
	if _, ok := rs["kept"]; !ok {
		t.Error("Prune dropped a dismissal — the user's standing decision")
	}
	if _, ok := rs["live"]; !ok {
		t.Error("Prune dropped a snooze that has not expired")
	}
}

// TestSuppressionShrinksTheQueueMonotonically: adding a decision can only ever
// remove charges from the queue, never add them.
func TestSuppressionShrinksTheQueueMonotonically(t *testing.T) {
	now := time.Now()
	txns := []domain.Transaction{
		tx("t1", "A", -100, 1), tx("t2", "B", -100, 2), tx("t3", "C", -100, 3),
	}
	rs := reviewqueue.Resolutions{}
	prev := len(reviewqueue.QueueOpen(txns, rs, now))
	for _, id := range []string{"t1", "t2", "t3"} {
		rs.Dismiss(id)
		got := len(reviewqueue.QueueOpen(txns, rs, now))
		if got > prev {
			t.Fatalf("dismissing %q GREW the queue: %d -> %d", id, prev, got)
		}
		if reviewqueue.CountOpen(txns, rs, now) != got {
			t.Errorf("CountOpen disagrees with QueueOpen after dismissing %q", id)
		}
		prev = got
	}
	if prev != 0 {
		t.Errorf("dismissing everything left %d queued", prev)
	}
}

// TestQueueOpenIsASubsetOfQueue: the resolution filter may only remove.
func TestQueueOpenIsASubsetOfQueue(t *testing.T) {
	now := time.Now()
	txns := []domain.Transaction{tx("t1", "A", -100, 1), tx("t2", "B", -100, 2)}
	rs := reviewqueue.Resolutions{}
	rs.Snooze("t1", now.Add(time.Hour))

	all := map[string]bool{}
	for _, t2 := range reviewqueue.Queue(txns) {
		all[t2.ID] = true
	}
	for _, t2 := range reviewqueue.QueueOpen(txns, rs, now) {
		if !all[t2.ID] {
			t.Errorf("QueueOpen returned %q, which Queue does not contain", t2.ID)
		}
	}
}

// --- cross-package: the two merchant keys must agree -------------------------

// TestMerchantKeysAgreeAcrossPackages is the invariant that keeps a batch honest.
//
// reviewqueue.MerchantKey decides which charges a bulk action TOUCHES.
// learntally.MerchantKey decides which charges the app LEARNS from.
// If they disagree, the app confirms one set and remembers a different one, and
// the next import is suggested from history that never matched what happened.
func TestMerchantKeysAgreeAcrossPackages(t *testing.T) {
	groups := [][]string{
		{"AMZN MKTP US*2H4RT9", "AMZN MKTP US*8K1QP2", "AMZN.COM/BILL WA"},
		{"SQ *BLUE BOTTLE COFFE", "SQ *BLUE BOTTLE COFFE 2"},
		{"SHELL OIL 57445208", "SHELL OIL 99112233"},
		{"NETFLIX.COM 866-579-7172", "NETFLIX.COM"},
	}
	for _, group := range groups {
		var rqKeys, ltKeys []string
		for _, d := range group {
			rqKeys = append(rqKeys, reviewqueue.MerchantKey(tx("x", d, -100, 1)))
			ltKeys = append(ltKeys, learntally.MerchantKey(d))
		}
		// Within a group both packages must collapse every descriptor to ONE key.
		for i := 1; i < len(group); i++ {
			if rqKeys[i] != rqKeys[0] {
				t.Errorf("reviewqueue split %q from %q (%q vs %q)",
					group[i], group[0], rqKeys[i], rqKeys[0])
			}
			if ltKeys[i] != ltKeys[0] {
				t.Errorf("learntally split %q from %q (%q vs %q)",
					group[i], group[0], ltKeys[i], ltKeys[0])
			}
		}
		// And the two packages must agree with EACH OTHER about the boundary.
		// learntally namespaces its key with an unforgeable prefix, so compare on
		// the payload rather than hardcoding the marker here — the assertion is
		// "same grouping", not "same spelling".
		if !strings.HasSuffix(ltKeys[0], rqKeys[0]) {
			t.Errorf("the two merchant keys have drifted for %q: learntally %q does not carry "+
				"reviewqueue's %q", group[0], ltKeys[0], rqKeys[0])
		}
	}

	// Different merchants must NOT collapse together in either package.
	a, b := "SHELL OIL 1", "NETFLIX.COM"
	if reviewqueue.MerchantKey(tx("x", a, -1, 1)) == reviewqueue.MerchantKey(tx("y", b, -1, 1)) {
		t.Error("reviewqueue merged two unrelated merchants")
	}
	if learntally.MerchantKey(a) == learntally.MerchantKey(b) {
		t.Error("learntally merged two unrelated merchants")
	}
}

// TestMerchantKeyIsStableAndTotal: the key feeds a map and a UI test id, so it
// must be deterministic and never depend on which charge of a group is sampled.
func TestMerchantKeyIsStableAndTotal(t *testing.T) {
	for _, d := range []string{"", "   ", "AMZN MKTP US*1", "\x00", "＊＊＊"} {
		k := reviewqueue.MerchantKey(tx("x", d, -1, 1))
		for i := 0; i < 50; i++ {
			if got := reviewqueue.MerchantKey(tx("x", d, -1, 1)); got != k {
				t.Fatalf("MerchantKey(%q) is not deterministic: %q vs %q", d, k, got)
			}
		}
	}
	// Payee wins over Desc, and Desc is the fallback — a group must not split
	// just because one charge populated only one of the two fields.
	withPayee := domain.Transaction{ID: "1", Payee: "SHELL OIL 1", Desc: "something else", Amount: usd(-1)}
	descOnly := domain.Transaction{ID: "2", Desc: "SHELL OIL 2", Amount: usd(-1)}
	if reviewqueue.MerchantKey(withPayee) != reviewqueue.MerchantKey(descOnly) {
		t.Errorf("payee-only and desc-only charges from one merchant produced different keys: %q vs %q",
			reviewqueue.MerchantKey(withPayee), reviewqueue.MerchantKey(descOnly))
	}
}

// TestQueueOrderingIsStableForEqualDates: ties break on id, so the head of the
// queue cannot flip between renders.
func TestQueueOrderingIsStableForEqualDates(t *testing.T) {
	txns := []domain.Transaction{
		tx("zzz", "A", -100, 5), tx("aaa", "B", -100, 5), tx("mmm", "C", -100, 5),
	}
	var ids []string
	for _, q := range reviewqueue.Queue(txns) {
		ids = append(ids, q.ID)
	}
	want := append([]string(nil), ids...)
	sort.Strings(want)
	for i := range ids {
		if ids[i] != want[i] {
			t.Fatalf("equal-dated charges are not id-ordered: %v", ids)
		}
	}
}
