// SPDX-License-Identifier: MIT

package dedupe

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func on(id, acct, desc string, minor int64) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Desc: desc,
		Amount: money.New(minor, "USD"),
		Date:   time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
	}
}

// C688: the same subscription billed to two cards on the same day is two real
// payments. Grouping them offered a Merge button whose result is deleting one.
func TestChargesOnDifferentAccountsAreNotDuplicates(t *testing.T) {
	txns := []domain.Transaction{
		on("a", "personal-card", "OpenRouter", -2000),
		on("b", "business-card", "OpenRouter", -2000),
	}
	if got := FindDuplicates(txns); len(got) != 0 {
		t.Errorf("grouped charges on two accounts as duplicates: %+v", got)
	}
}

// The real double entry — same account — is still caught.
func TestTwoEntriesOnOneAccountAreStillDuplicates(t *testing.T) {
	txns := []domain.Transaction{
		on("a", "personal-card", "FAL Features Labels", -1500),
		on("b", "personal-card", "fal features labels", -1500),
	}
	got := FindDuplicates(txns)
	if len(got) != 1 {
		t.Fatalf("got %d groups, want 1: %+v", len(got), got)
	}
	if got[0].AccountID != "personal-card" {
		t.Errorf("AccountID = %q, want the shared account named on the group", got[0].AccountID)
	}
	if len(got[0].IDs) != 2 {
		t.Errorf("group has %d members, want 2", len(got[0].IDs))
	}
}

// The grouping rule and the importer's skip rule must be the same rule, because
// the screen's grouping is the input to a destructive action while the
// importer's is the input to a harmless skip. They used to differ by exactly the
// account.
func TestGroupingAndImportUseOneKey(t *testing.T) {
	existing := []domain.Transaction{on("a", "personal-card", "OpenRouter", -2000)}
	incoming := []domain.Transaction{on("b", "business-card", "OpenRouter", -2000)}

	if n := CountIncomingDuplicates(incoming, existing, ""); n != 0 {
		t.Errorf("importer called a different account's charge a duplicate (%d)", n)
	}
	if got := FindDuplicates(append(existing, incoming...)); len(got) != 0 {
		t.Errorf("the screen disagreed with the importer: %+v", got)
	}

	sameAcct := []domain.Transaction{on("c", "personal-card", "OpenRouter", -2000)}
	if n := CountIncomingDuplicates(sameAcct, existing, ""); n != 1 {
		t.Errorf("importer missed a same-account duplicate (%d)", n)
	}
	if got := FindDuplicates(append(existing, sameAcct...)); len(got) != 1 {
		t.Errorf("the screen missed a same-account duplicate: %+v", got)
	}
}

func TestKeyIncludesTheAccount(t *testing.T) {
	a := on("a", "one", "OpenRouter", -2000)
	b := on("b", "two", "OpenRouter", -2000)
	if Signature(a) != Signature(b) {
		t.Errorf("the within-account halves should match; they are the same charge")
	}
	if Key(a) == Key(b) {
		t.Errorf("Key must separate two accounts: %q", Key(a))
	}
}

// A guard against a caller assembling a group some other way: merging across
// accounts deletes a real payment, and the survivor looks exactly like it, so
// nothing downstream would notice.
func TestSameAccountGuard(t *testing.T) {
	survivor := on("a", "personal-card", "OpenRouter", -2000)
	if !SameAccount(survivor, []domain.Transaction{on("b", "personal-card", "OpenRouter", -2000)}) {
		t.Errorf("SameAccount rejected a legitimate same-account group")
	}
	if SameAccount(survivor, []domain.Transaction{on("b", "business-card", "OpenRouter", -2000)}) {
		t.Errorf("SameAccount allowed a cross-account merge")
	}
	if !SameAccount(survivor, nil) {
		t.Errorf("SameAccount rejected a group of one")
	}
}
