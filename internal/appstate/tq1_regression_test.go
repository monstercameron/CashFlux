// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func TestTQ1ApplyRulesPreservesCategoriesAndReviewedRows(t *testing.T) {
	tests := []struct {
		name         string
		reviewed     bool
		wantCategory string
		wantDesc     string
	}{
		{name: "category-less first match does not shadow category rule", wantCategory: "food", wantDesc: "Cafe"},
		{name: "reviewed transaction is preserved", reviewed: true, wantCategory: "manual", wantDesc: "Cafe"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
				t.Fatal(err)
			}
			category := ""
			if tc.reviewed {
				category = "manual"
			}
			tx := domain.Transaction{ID: "t", AccountID: "acc1", Date: thisMonth(), CategoryID: category, Desc: "Cafe", Payee: "Cafe", Amount: money.New(-500, "USD"), Reviewed: tc.reviewed}
			if err := a.PutTransaction(tx); err != nil {
				t.Fatal(err)
			}
			for _, r := range []rules.Rule{
				{ID: "bill-only", Match: "cafe", SetBillAccountID: "bill"},
				{ID: "category", Match: "cafe", SetCategoryID: "food"},
			} {
				if err := a.PutRule(r); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := a.ApplyRules(); err != nil {
				t.Fatal(err)
			}
			got := a.Transactions()[0]
			if got.CategoryID != tc.wantCategory {
				t.Errorf("category = %q, want %q", got.CategoryID, tc.wantCategory)
			}
			if got.Desc != tc.wantDesc {
				t.Errorf("description = %q, want %q", got.Desc, tc.wantDesc)
			}
		})
	}
}

func TestTQ1EntryCategoryRuleIgnoresCategorylessRule(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutRule(rules.Rule{ID: "bill", Match: "cafe", SetBillAccountID: "bill-account"}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRule(rules.Rule{ID: "category", Match: "cafe", SetCategoryID: "food"}); err != nil {
		t.Fatal(err)
	}
	got := a.AutoCategorizeTransaction(domain.Transaction{Payee: "Cafe", Desc: "Cafe", Amount: money.New(-500, "USD"), Date: thisMonth()})
	if got.CategoryID != "food" {
		t.Fatalf("entry category = %q, want food", got.CategoryID)
	}
}

func TestTQ1CategorylessBackfillPreservesExistingCategory(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	tx := domain.Transaction{ID: "t", AccountID: "acc1", Date: thisMonth(), CategoryID: "manual", Desc: "Cafe", Payee: "Cafe", Amount: money.New(-500, "USD")}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRule(rules.Rule{ID: "bill-only", Match: "cafe", SetBillAccountID: "bill", RenameDesc: "Coffee"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyRules(); err != nil {
		t.Fatal(err)
	}
	got := a.Transactions()[0]
	if got.CategoryID != "manual" || got.Desc != "Coffee" || got.BillAccountID != "bill" {
		t.Fatalf("categoryless backfill result = category %q, desc %q, bill %q", got.CategoryID, got.Desc, got.BillAccountID)
	}
}

func TestTQ1ApplyRulesHonorsRoleGuardAndValidation(t *testing.T) {
	t.Run("role guard", func(t *testing.T) {
		a := newApp(t, false)
		a.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
		if _, err := a.ApplyRules(); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("ApplyRules error = %v, want ErrReadOnly", err)
		}
	})
	t.Run("invalid existing transaction", func(t *testing.T) {
		a := newApp(t, false)
		seedAccount(t, a, 0)
		if err := a.store.PutTransaction(domain.Transaction{ID: "invalid", AccountID: "acc1", Date: thisMonth(), Payee: "Cafe", Amount: money.New(-500, "USD")}); err != nil {
			t.Fatal(err)
		}
		if err := a.PutRule(rules.Rule{ID: "category", Match: "cafe", SetCategoryID: "food"}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.ApplyRules(); err == nil {
			t.Fatal("ApplyRules accepted transaction missing required description")
		}
	})
}

func TestTQ1CSVSameFileRepeatsMatchExistingOccurrences(t *testing.T) {
	for _, existing := range []int{0, 1} {
		t.Run(map[int]string{0: "no prior occurrence", 1: "one prior occurrence"}[existing], func(t *testing.T) {
			a := newApp(t, false)
			seedAccount(t, a, 0)
			one := []byte("date,account_id,desc,amount\n2026-06-10,Checking,Transit,-3\n")
			if existing > 0 {
				if res, err := a.ImportTransactionsCSV(one, "", ""); err != nil || res.Imported != 1 {
					t.Fatalf("seed import: %+v, %v", res, err)
				}
			}
			input := []byte("date,account_id,desc,amount\n2026-06-10,Checking,Transit,-3\n2026-06-10,Checking,Transit,-3\n")
			res, err := a.ImportTransactionsCSV(input, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if wantImported, wantDuplicates := 2-existing, existing; res.Imported != wantImported || res.Duplicates != wantDuplicates {
				t.Fatalf("imported=%d duplicates=%d, want %d and %d", res.Imported, res.Duplicates, wantImported, wantDuplicates)
			}
		})
	}
}

func TestTQ1RestoreDeletedExpenseDoesNotDrawFundAgain(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	if err := a.PutCategory(domain.Category{ID: "repair", Name: "Repair", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutGoal(domain.Goal{ID: "fund", Name: "Fund", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(50000, "USD"), CurrentAmount: money.New(30000, "USD"), IsSinkingFund: true, CategoryID: "repair"}); err != nil {
		t.Fatal(err)
	}
	tx := domain.Transaction{ID: "expense", AccountID: "acc1", Date: thisMonth(), CategoryID: "repair", Desc: "Repair bill", Amount: money.New(-10000, "USD")}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteTransaction(tx.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.RestoreTransactions([]domain.Transaction{tx}); err != nil {
		t.Fatal(err)
	}
	if got := a.Goals()[0].CurrentAmount.Amount; got != 20000 {
		t.Fatalf("fund balance after delete/undo = %d, want 20000", got)
	}
}

func TestTQ1PutTransactionNormalizesUTCDate(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	local := time.Date(2026, 10, 31, 20, 30, 0, 0, time.FixedZone("New York", -4*60*60))
	tx := domain.Transaction{ID: "late", AccountID: "acc1", Date: local, Desc: "late", Amount: money.New(-100, "USD")}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatal(err)
	}
	got := a.Transactions()[0].Date
	want := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("date = %s (%s), want UTC day %s", got, got.Location(), want)
	}
}

func TestTransactionAutoRulesDoNotUseShortCategorySubstring(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCategory(domain.Category{ID: "car", Name: "Car", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	got := a.AutoCategorizeTransaction(domain.Transaction{Payee: "Carl's Cafe", Desc: "Lunch", Amount: money.New(-900, "USD"), Date: thisMonth()})
	if got.CategoryID == "car" {
		t.Fatalf("short category name incorrectly categorized unrelated payee: %+v", got)
	}
}

func TestTQ1CreateTransferPairNormalizesDate(t *testing.T) {
	a := newApp(t, false)
	for _, account := range []domain.Account{
		{ID: "from", Name: "Checking", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "USD"},
		{ID: "to", Name: "Savings", Type: domain.TypeSavings, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "USD"},
	} {
		if err := a.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	local := time.Date(2026, 10, 31, 23, 0, 0, 0, time.FixedZone("local", -4*60*60))
	if _, _, err := a.CreateTransferPair(TransferParams{OperationID: "tq1", FromAccountID: "from", ToAccountID: "to", AmountMinor: 1000, Date: local}); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	for _, txn := range a.Transactions() {
		if !txn.Date.Equal(want) || txn.Date.Location() != time.UTC {
			t.Errorf("transfer leg %q date = %s (%s), want UTC %s", txn.ID, txn.Date, txn.Date.Location(), want)
		}
	}
}

func TestTQ1DeleteTransactionPrunesEveryTxnLinkKind(t *testing.T) {
	a := newApp(t, false)
	for _, tx := range []domain.Transaction{
		{ID: "deleted", AccountID: "acc1", Date: thisMonth(), Desc: "Deleted", Amount: money.New(-100, "USD")},
		{ID: "other", AccountID: "acc1", Date: thisMonth(), Desc: "Other", Amount: money.New(100, "USD")},
	} {
		if err := a.PutTransaction(tx); err != nil {
			t.Fatal(err)
		}
	}
	links := []domain.TxnLink{
		{ID: "bill", Kind: domain.TxnLinkBillMatch, TxnIDs: []string{"deleted"}, RecurringID: "r", OccurrenceDate: thisMonth()},
		{ID: "refund", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"deleted", "other"}},
		{ID: "group", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"deleted", "other"}},
		{ID: "event", Kind: domain.TxnLinkEventTxn, TxnIDs: []string{"deleted"}, EventID: "event"},
	}
	for _, link := range links {
		if err := a.store.PutTxnLink(link); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.DeleteTransaction("deleted"); err != nil {
		t.Fatal(err)
	}
	if got := a.TxnLinks(); len(got) != 0 {
		t.Fatalf("links after deleting member = %+v, want none", got)
	}
}

func TestTQ1CSVDefaultsCurrencyFromResolvedAccount(t *testing.T) {
	for _, tc := range []struct {
		input    string
		currency string
		amount   int64
	}{
		{input: "date,account_id,desc,amount\n2026-06-10,Euro,Market,-10\n", currency: "EUR", amount: -1000},
		{input: "date,account_id,desc,amount,currency\n2026-06-10,Euro,Market,-10,\n", currency: "EUR", amount: -1000},
		{input: "date,account_id,desc,amount,currency\n2026-06-10,Euro,Market,-10,\n", currency: "JPY", amount: -10},
	} {
		a := newApp(t, false)
		if err := a.PutAccount(domain.Account{ID: "eur", Name: "Euro", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: tc.currency}); err != nil {
			t.Fatal(err)
		}
		res, err := a.ImportTransactionsCSV([]byte(tc.input), "", "")
		if err != nil || res.Imported != 1 {
			t.Fatalf("import: %+v, %v", res, err)
		}
		got := a.Transactions()[0].Amount
		if got.Currency != tc.currency || got.Amount != tc.amount {
			t.Fatalf("amount = %+v, want %d %s", got, tc.amount, tc.currency)
		}
	}
}
