package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestSampleDatasetIntegrity checks the sample dataset's internal references hold
// together (beyond per-entity validity): every transaction/budget/goal/task points
// at entities that actually exist, and the transfer legs balance. This guards
// against a future seed edit silently breaking a reference or leaving a transfer
// one-legged.
func TestSampleDatasetIntegrity(t *testing.T) {
	ds := SampleDataset()

	accounts := idSet(len(ds.Accounts))
	for _, a := range ds.Accounts {
		accounts[a.ID] = true
	}
	categories := idSet(len(ds.Categories))
	for _, c := range ds.Categories {
		categories[c.ID] = true
	}
	members := idSet(len(ds.Members))
	for _, m := range ds.Members {
		members[m.ID] = true
	}

	owns := func(id string) bool { return id == domain.GroupOwnerID || members[id] }

	// Transactions: real account + member; transfers reference a real (distinct)
	// account, and non-transfers reference a real category.
	var transferLegs int
	var transferSum int64
	for _, tx := range ds.Transactions {
		if !accounts[tx.AccountID] {
			t.Errorf("tx %s: unknown account %q", tx.ID, tx.AccountID)
		}
		if tx.MemberID != "" && !members[tx.MemberID] {
			t.Errorf("tx %s: unknown member %q", tx.ID, tx.MemberID)
		}
		if tx.TransferAccountID != "" {
			transferLegs++
			transferSum += tx.Amount.Amount
			if !accounts[tx.TransferAccountID] {
				t.Errorf("tx %s: unknown transfer account %q", tx.ID, tx.TransferAccountID)
			}
			if tx.TransferAccountID == tx.AccountID {
				t.Errorf("tx %s: transfer to the same account", tx.ID)
			}
			continue
		}
		if tx.CategoryID != "" && !categories[tx.CategoryID] {
			t.Errorf("tx %s: unknown category %q", tx.ID, tx.CategoryID)
		}
	}
	if transferLegs == 0 {
		t.Error("expected the sample to include transfers")
	}
	if transferLegs%2 != 0 {
		t.Errorf("transfer legs should pair up, got an odd count: %d", transferLegs)
	}
	if transferSum != 0 {
		t.Errorf("transfer legs should net to zero, got %d", transferSum)
	}

	for _, b := range ds.Budgets {
		if !categories[b.CategoryID] {
			t.Errorf("budget %s: unknown category %q", b.ID, b.CategoryID)
		}
		if !owns(b.OwnerID) {
			t.Errorf("budget %s: unknown owner %q", b.ID, b.OwnerID)
		}
	}

	for _, g := range ds.Goals {
		if !owns(g.OwnerID) {
			t.Errorf("goal %s: unknown owner %q", g.ID, g.OwnerID)
		}
		if g.AccountID != "" && !accounts[g.AccountID] {
			t.Errorf("goal %s: unknown linked account %q", g.ID, g.AccountID)
		}
	}

	for _, tk := range ds.Tasks {
		if tk.RelatedType == domain.RelatedAccount && !accounts[tk.RelatedID] {
			t.Errorf("task %s: unknown related account %q", tk.ID, tk.RelatedID)
		}
	}
}

func idSet(capacity int) map[string]bool { return make(map[string]bool, capacity) }
