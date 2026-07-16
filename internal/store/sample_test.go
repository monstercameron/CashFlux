// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestEmptyDataset checks that a new workspace's starting point is genuinely empty
// (no financial data), valid, and survives the export→import a new workspace does.
func TestEmptyDataset(t *testing.T) {
	ds := EmptyDataset()
	if len(ds.Accounts) != 0 || len(ds.Transactions) != 0 || len(ds.Budgets) != 0 || len(ds.Goals) != 0 {
		t.Errorf("empty dataset should carry no financial data: %d acct, %d txn, %d budget, %d goal",
			len(ds.Accounts), len(ds.Transactions), len(ds.Budgets), len(ds.Goals))
	}
	if len(ds.Members) != 1 || ds.Settings.BaseCurrency != "USD" {
		t.Errorf("want one default member + USD base, got %d members, base %q", len(ds.Members), ds.Settings.BaseCurrency)
	}
	// A new workspace persists Export(EmptyDataset()) and boot Imports it.
	data, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	back, err := Import(data)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(back.Members) != 1 || len(back.Accounts) != 0 {
		t.Errorf("round-trip: %d members, %d accounts (want 1, 0)", len(back.Members), len(back.Accounts))
	}
}

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
	transactions := idSet(len(ds.Transactions))
	for _, tx := range ds.Transactions {
		transactions[tx.ID] = true
	}
	budgets := idSet(len(ds.Budgets))
	for _, b := range ds.Budgets {
		budgets[b.ID] = true
	}
	goals := idSet(len(ds.Goals))
	for _, g := range ds.Goals {
		goals[g.ID] = true
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
		// A budget tracks a single category, several categories, and/or tags (cross-
		// category) — but it must track SOMETHING, and any category it names must be real.
		if b.CategoryID != "" && !categories[b.CategoryID] {
			t.Errorf("budget %s: unknown category %q", b.ID, b.CategoryID)
		}
		for _, cid := range b.CategoryIDs {
			if !categories[cid] {
				t.Errorf("budget %s: unknown tracked category %q", b.ID, cid)
			}
		}
		if b.CategoryID == "" && len(b.CategoryIDs) == 0 && len(b.TrackedTags) == 0 {
			t.Errorf("budget %s tracks nothing (no category or tag)", b.ID)
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

	// Every linked task points at a real entity of its related type (a follow-up chip /
	// drill-through with a dangling id would render nowhere).
	for _, tk := range ds.Tasks {
		switch tk.RelatedType {
		case domain.RelatedAccount:
			if !accounts[tk.RelatedID] {
				t.Errorf("task %s: unknown related account %q", tk.ID, tk.RelatedID)
			}
		case domain.RelatedTransaction:
			if !transactions[tk.RelatedID] {
				t.Errorf("task %s: unknown related transaction %q", tk.ID, tk.RelatedID)
			}
		case domain.RelatedBudget:
			if !budgets[tk.RelatedID] {
				t.Errorf("task %s: unknown related budget %q", tk.ID, tk.RelatedID)
			}
		case domain.RelatedGoal:
			if !goals[tk.RelatedID] {
				t.Errorf("task %s: unknown related goal %q", tk.ID, tk.RelatedID)
			}
		}
	}
}

func idSet(capacity int) map[string]bool { return make(map[string]bool, capacity) }
