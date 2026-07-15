// SPDX-License-Identifier: MIT

package events

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func usd(minor int64) money.Money { return money.Money{Amount: minor, Currency: "USD"} }

func TestCovers(t *testing.T) {
	ev := domain.Event{Start: day(2026, 6, 1), End: day(2026, 6, 10)}
	open := domain.Event{Start: day(2026, 6, 1)}
	cases := []struct {
		name string
		ev   domain.Event
		when time.Time
		want bool
	}{
		{"before start", ev, day(2026, 5, 31), false},
		{"on start", ev, day(2026, 6, 1), true},
		{"inside", ev, day(2026, 6, 5), true},
		{"on end exclusive", ev, day(2026, 6, 10), false},
		{"after end", ev, day(2026, 6, 11), false},
		{"open ended after", open, day(2027, 1, 1), true},
		{"open ended before", open, day(2025, 1, 1), false},
	}
	for _, c := range cases {
		if got := c.ev.Covers(c.when); got != c.want {
			t.Errorf("%s: Covers=%v want %v", c.name, got, c.want)
		}
	}
}

func TestAutoAssociate(t *testing.T) {
	ev := domain.Event{Start: day(2026, 6, 1), End: day(2026, 6, 10)}
	txns := []domain.Transaction{
		{ID: "a", Date: day(2026, 5, 30), Amount: usd(-100)},                            // before
		{ID: "b", Date: day(2026, 6, 1), Amount: usd(-200)},                             // on start
		{ID: "c", Date: day(2026, 6, 5), Amount: usd(-300)},                             // inside
		{ID: "t", Date: day(2026, 6, 5), Amount: usd(-400), TransferAccountID: "acct2"}, // transfer excluded
		{ID: "d", Date: day(2026, 6, 10), Amount: usd(-500)},                            // on end excluded
	}
	got := AutoAssociate(ev, txns)
	want := []string{"b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestMembers(t *testing.T) {
	links := []domain.TxnLink{
		{Kind: domain.TxnLinkEventTxn, EventID: "ev1", TxnIDs: []string{"a"}},
		{Kind: domain.TxnLinkEventTxn, EventID: "ev1", TxnIDs: []string{"b"}},
		{Kind: domain.TxnLinkEventTxn, EventID: "ev2", TxnIDs: []string{"c"}},
		{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "d"}}, // wrong kind
	}
	m := Members(links, "ev1")
	if len(m) != 2 || !m["a"] || !m["b"] {
		t.Fatalf("ev1 members=%v", m)
	}
	if len(Members(links, "")) != 0 {
		t.Fatalf("empty eventID should yield no members")
	}
}

func TestTotals(t *testing.T) {
	members := map[string]bool{"a": true, "b": true, "c": true}
	txns := []domain.Transaction{
		{ID: "a", Amount: usd(-1000), CategoryID: "food"},
		{ID: "b", Amount: usd(-3000), Splits: []domain.CategorySplit{
			{CategoryID: "food", Amount: usd(-1000)},
			{CategoryID: "lodging", Amount: usd(-2000)},
		}},
		{ID: "c", Amount: usd(500), CategoryID: "refund"}, // income line
		{ID: "z", Amount: usd(-9999), CategoryID: "food"}, // not a member
	}
	total, byCat := Totals(members, txns)
	if total != -3500 {
		t.Fatalf("total=%d want -3500", total)
	}
	want := map[string]int64{"food": -2000, "lodging": -2000, "refund": 500}
	if len(byCat) != len(want) {
		t.Fatalf("byCat=%v", byCat)
	}
	for _, ca := range byCat {
		if want[ca.CategoryID] != ca.Minor {
			t.Fatalf("category %s=%d want %d", ca.CategoryID, ca.Minor, want[ca.CategoryID])
		}
	}
	// deterministic sort by category id
	if byCat[0].CategoryID != "food" || byCat[1].CategoryID != "lodging" || byCat[2].CategoryID != "refund" {
		t.Fatalf("order=%v", byCat)
	}
	if got := SpendMinor(members, txns); got != 3500 {
		t.Fatalf("SpendMinor=%d want 3500", got)
	}
}
