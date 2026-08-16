// SPDX-License-Identifier: MIT

package txnfilter_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func lensTxn(id, member string) domain.Transaction {
	return domain.Transaction{
		ID: id, MemberID: member, AccountID: "a", Desc: id,
		Amount: money.Money{Amount: -100, Currency: "USD"},
		Date:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}
}

// The whole point of the lens is that it FILTERS. Before this existed the
// transactions surface read a dead atom, so choosing "View as Priya" in the top
// bar changed nothing about the ledger and said nothing about why.
func TestOwnerLensNarrowsTheLedger(t *testing.T) {
	txns := []domain.Transaction{lensTxn("t1", "m-priya"), lensTxn("t2", "m-marcus"), lensTxn("t3", "m-priya")}

	c, applied := txnfilter.Criteria{}.WithOwnerLens([]string{"m-priya"})
	if !applied {
		t.Fatal("lens did not apply to an unfiltered criteria")
	}
	got := txnfilter.Apply(txns, c)
	if len(got) != 2 {
		t.Fatalf("lens kept %d rows, want 2 (Priya's)", len(got))
	}
	for _, g := range got {
		if g.MemberID != "m-priya" {
			t.Errorf("row %q belongs to %q, which the Priya lens should have excluded", g.ID, g.MemberID)
		}
	}
}

// A member the user picked in the page's own filter panel is a local, deliberate
// decision; the ambient top-bar perspective must not silently override it.
func TestOwnerLensYieldsToAnExplicitMemberFilter(t *testing.T) {
	base := txnfilter.Criteria{Member: "m-marcus"}
	c, applied := base.WithOwnerLens([]string{"m-priya"})
	if applied {
		t.Error("lens reported that it applied over an explicit member filter")
	}
	if c.Members != "" || c.Member != "m-marcus" {
		t.Errorf("lens mutated an explicitly filtered criteria: Member=%q Members=%q", c.Member, c.Members)
	}
}

// Multi-owner scopes come from the report scope selector; the ledger honours all
// of them rather than silently picking one.
func TestOwnerLensHonoursEveryOwnerInTheScope(t *testing.T) {
	txns := []domain.Transaction{lensTxn("t1", "m-priya"), lensTxn("t2", "m-marcus"), lensTxn("t3", "m-sam")}
	c, applied := txnfilter.Criteria{}.WithOwnerLens([]string{"m-priya", "m-marcus"})
	if !applied {
		t.Fatal("a two-owner lens did not apply")
	}
	if got := len(txnfilter.Apply(txns, c)); got != 2 {
		t.Fatalf("two-owner lens kept %d rows, want 2", got)
	}
}

// "Everyone" is the empty scope, and whitespace must not fake a selection.
func TestOwnerLensIsInertWhenNobodyIsSelected(t *testing.T) {
	for _, owners := range [][]string{nil, {}, {""}, {"  "}} {
		c, applied := txnfilter.Criteria{Text: "coffee"}.WithOwnerLens(owners)
		if applied {
			t.Errorf("lens %#v reported applied", owners)
		}
		if c.Members != "" {
			t.Errorf("lens %#v wrote Members=%q", owners, c.Members)
		}
		if c.Text != "coffee" {
			t.Errorf("lens %#v disturbed the rest of the criteria", owners)
		}
	}
}
