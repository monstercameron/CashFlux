// SPDX-License-Identifier: MIT

package integrity

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/money"
)

var hygNow = time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)

func hygTxn(id, cat, transferTo string) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "a", CategoryID: cat,
		TransferAccountID: transferTo, Desc: id, Date: hygNow,
		Amount: money.New(-1000, "USD")}
}

func countOf(counts []HygieneCount, kind string) int {
	for _, c := range counts {
		if c.Kind == kind {
			return c.N
		}
	}
	return -1
}

// A transfer has no category to assign, so counting it would present permanent,
// unfixable work — the worst thing to put in a list of things to do.
func TestUncategorizedExcludesTransfers(t *testing.T) {
	in := HygieneInput{
		Transactions: []domain.Transaction{
			hygTxn("t1", "", ""),       // genuinely uncategorized
			hygTxn("t2", "", "b"),      // a transfer — not work
			hygTxn("t3", "c-food", ""), // filed
			hygTxn("t4", "", ""),       // genuinely uncategorized
		},
		Now: hygNow,
	}
	if got := countOf(Hygiene(in), HygieneUncategorized); got != 2 {
		t.Errorf("uncategorized = %d, want 2", got)
	}
}

// Only accounts that CAN be reconciled: a property valuation has no statement to
// match, so listing it would be asking for work that does not exist.
func TestUnreconciledCountsOnlyStatementAccounts(t *testing.T) {
	in := HygieneInput{
		Accounts: []domain.Account{
			{ID: "chk", Type: domain.TypeChecking},
			{ID: "card", Type: domain.TypeCreditCard},
			{ID: "house", Type: domain.TypeProperty},
			{ID: "inv", Type: domain.TypeInvestment},
			{ID: "old", Type: domain.TypeChecking, Archived: true},
			{ID: "done", Type: domain.TypeSavings, Reconciliations: []domain.Reconciliation{{At: hygNow}}},
		},
		Now: hygNow,
	}
	if got := countOf(Hygiene(in), HygieneUnreconciled); got != 2 {
		t.Errorf("unreconciled = %d, want 2 (checking + card only)", got)
	}
}

// Every count comes back, including the zeroes, so a caller can render a
// "nothing to do" state rather than guessing from a short slice.
func TestHygieneAlwaysReturnsEveryCount(t *testing.T) {
	counts := Hygiene(HygieneInput{Now: hygNow})
	if len(counts) != 3 {
		t.Fatalf("got %d counts, want 3", len(counts))
	}
	for _, c := range counts {
		if c.N != 0 {
			t.Errorf("%s = %d on empty input", c.Kind, c.N)
		}
		if c.Route == "" {
			t.Errorf("%s has nowhere to go — a count with no route is a complaint", c.Kind)
		}
	}
	if HygieneTotal(counts) != 0 {
		t.Errorf("HygieneTotal on empty input = %d", HygieneTotal(counts))
	}
}

// The staleness window comes from the caller so it matches the one the accounts
// page applies — computing it twice with two windows is how two screens come to
// disagree about the same account.
func TestStaleAccountsUsesTheCallersWindows(t *testing.T) {
	old := hygNow.AddDate(0, 0, -60)
	accounts := []domain.Account{{ID: "a", Type: domain.TypeChecking, BalanceAsOf: old}}

	tight := Hygiene(HygieneInput{Accounts: accounts, Now: hygNow,
		Windows: freshness.Windows{domain.TypeChecking: 7}})
	loose := Hygiene(HygieneInput{Accounts: accounts, Now: hygNow,
		Windows: freshness.Windows{domain.TypeChecking: 365}})

	if countOf(tight, HygieneStaleAccounts) == countOf(loose, HygieneStaleAccounts) {
		t.Error("the window had no effect — the count is not using the caller's rule")
	}
}

func TestHygieneTotalSums(t *testing.T) {
	got := HygieneTotal([]HygieneCount{{N: 12}, {N: 3}, {N: 2}})
	if got != 17 {
		t.Errorf("HygieneTotal = %d, want 17", got)
	}
}
