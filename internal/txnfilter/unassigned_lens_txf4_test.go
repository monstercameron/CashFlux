// SPDX-License-Identifier: MIT

package txnfilter_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

const txf4UnassignedMemberID = "__cashflux_unassigned_member__"

func TestUnassignedOwnerLensShowsOnlyUnassignedTransactions(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "marcus", MemberID: "m-marcus"},
		{ID: "unassigned-one"},
		{ID: "priya", MemberID: "m-priya"},
		{ID: "unassigned-two", MemberID: ""},
	}

	c, applied := (txnfilter.Criteria{}).WithOwnerLens([]string{txf4UnassignedMemberID})
	if !applied {
		t.Fatal("unassigned member lens was not applied")
	}
	got := txnfilter.Apply(txns, c)
	if len(got) != 2 {
		t.Fatalf("unassigned lens kept %d rows, want the 2 rows without a member", len(got))
	}
	for _, txn := range got {
		if txn.MemberID != "" {
			t.Errorf("unassigned lens kept transaction %q with member %q", txn.ID, txn.MemberID)
		}
	}
}

func TestUnassignedCanJoinNamedOwnerInOneLens(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "marcus", MemberID: "m-marcus"},
		{ID: "unassigned"},
		{ID: "priya", MemberID: "m-priya"},
	}
	c, applied := (txnfilter.Criteria{}).WithOwnerLens([]string{"m-marcus", txf4UnassignedMemberID})
	if !applied {
		t.Fatal("mixed owner lens was not applied")
	}
	got := txnfilter.Apply(txns, c)
	if len(got) != 2 || got[0].ID != "marcus" || got[1].ID != "unassigned" {
		t.Fatalf("mixed owner lens returned %#v, want Marcus and the unassigned transaction", got)
	}
}

func TestUnassignedOwnerLensStillYieldsToPageMemberFilter(t *testing.T) {
	base := txnfilter.Criteria{Member: "m-marcus"}
	got, applied := base.WithOwnerLens([]string{txf4UnassignedMemberID})
	if applied {
		t.Fatal("ambient unassigned lens overrode an explicit page member filter")
	}
	if got.Member != "m-marcus" || got.Members != "" {
		t.Fatalf("page member filter changed: Member=%q Members=%q", got.Member, got.Members)
	}
}

func TestUnassignedLensFilterPreservesOtherCriteria(t *testing.T) {
	txn := domain.Transaction{
		ID: "unassigned-coffee", Desc: "Coffee", Amount: money.Money{Amount: -250, Currency: "USD"},
		Date: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
	}
	c, _ := (txnfilter.Criteria{Text: "coffee"}).WithOwnerLens([]string{txf4UnassignedMemberID})
	if got := txnfilter.Apply([]domain.Transaction{txn}, c); len(got) != 1 {
		t.Fatalf("unassigned lens lost other criteria or rejected the unassigned row: got %d rows", len(got))
	}
}
