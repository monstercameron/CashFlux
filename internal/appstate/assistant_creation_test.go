// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func creationFixture(t *testing.T) (*App, domain.Account, domain.Category) {
	t.Helper()
	a := newApp(t, false)
	t.Cleanup(func() { _ = a.Store().Close() })
	account := domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.Zero("USD")}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	account = a.Accounts()[0]
	category := domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}
	if err := a.PutCategory(category); err != nil {
		t.Fatal(err)
	}
	return a, account, category
}
func creationTxn(id string) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "bank", Date: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), Desc: "Market", Payee: "Market", Amount: money.New(-100, "USD"), Source: domain.TxnSourceAssistant}
}

func TestExpectedCreationTransactionNormalizesDedupeAndPreservesRole(t *testing.T) {
	a, account, category := creationFixture(t)
	row := creationTxn("new")
	row.CategoryID = category.ID
	got, created, err := a.CreateTransactionExpected(row, account, &category)
	if err != nil || !created || got.Date.Hour() != 0 {
		t.Fatalf("create: %+v %v %v", got, created, err)
	}
	revision := a.Rev()
	row.ID = "retry"
	row.Payee = "  MARKET  "
	dup, created, err := a.CreateTransactionExpected(row, account, &category)
	if err != nil || created || dup.ID != got.ID || a.Rev() != revision || len(a.Transactions()) != 1 {
		t.Fatalf("duplicate mutated: %+v %v %v", dup, created, err)
	}
	a.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	row.ID = "viewer"
	row.Payee = "Other"
	if _, _, err := a.CreateTransactionExpected(row, account, &category); !errors.Is(err, ErrReadOnly) || a.Rev() != revision {
		t.Fatalf("role bypass: %v", err)
	}
}

func TestExpectedCreationRejectsStaleAccountCategoryCollisionAndBalanceOverflow(t *testing.T) {
	for _, kind := range []string{"account", "category", "id collision", "balance overflow"} {
		t.Run(kind, func(t *testing.T) {
			a, account, category := creationFixture(t)
			row := creationTxn("new")
			row.CategoryID = category.ID
			switch kind {
			case "account":
				next := account
				next.Name = "Changed"
				if err := a.PutAccount(next); err != nil {
					t.Fatal(err)
				}
			case "category":
				next := category
				next.Name = "Changed"
				if err := a.PutCategory(next); err != nil {
					t.Fatal(err)
				}
			case "id collision":
				other := row
				other.Amount = money.New(-200, "USD")
				if err := a.PutTransaction(other); err != nil {
					t.Fatal(err)
				}
			case "balance overflow":
				account.OpeningBalance = money.New(math.MaxInt64, "USD")
				if err := a.PutAccount(account); err != nil {
					t.Fatal(err)
				}
				row.Amount = money.New(1, "USD")
			}
			before, revision := a.Transactions(), a.Rev()
			_, _, err := a.CreateTransactionExpected(row, account, &category)
			if err == nil || a.Rev() != revision || !reflect.DeepEqual(a.Transactions(), before) {
				t.Fatalf("failed create changed state: %v", err)
			}
		})
	}
}

func TestExpectedCategorizationCommitsTogetherAndRejectsLastStaleRow(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "stale last"}[stale], func(t *testing.T) {
			a, _, category := creationFixture(t)
			first, second := creationTxn("first"), creationTxn("second")
			second.Payee = "Second"
			if err := a.PutTransaction(first); err != nil {
				t.Fatal(err)
			}
			if err := a.PutTransaction(second); err != nil {
				t.Fatal(err)
			}
			expected := a.Transactions()
			if stale {
				changed := expected[len(expected)-1]
				changed.Desc = "Edited"
				if err := a.PutTransaction(changed); err != nil {
					t.Fatal(err)
				}
			}
			before, revision := a.Transactions(), a.Rev()
			err := a.CategorizeTransactionsExpected(expected, category)
			if stale {
				if err == nil || a.Rev() != revision || !reflect.DeepEqual(a.Transactions(), before) {
					t.Fatalf("partial stale batch: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range a.Transactions() {
				if row.CategoryID != category.ID {
					t.Fatal("batch skipped row")
				}
			}
		})
	}
}

func TestExpectedCategorizationRejectsTransferAndChangedCategory(t *testing.T) {
	for _, kind := range []string{"transfer", "category"} {
		t.Run(kind, func(t *testing.T) {
			a, _, category := creationFixture(t)
			row := creationTxn("row")
			if err := a.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			expected := a.Transactions()
			if kind == "transfer" {
				expected[0].TransferAccountID = "elsewhere"
			} else {
				next := category
				next.Name = "Changed"
				if err := a.PutCategory(next); err != nil {
					t.Fatal(err)
				}
			}
			before, revision := a.Transactions(), a.Rev()
			err := a.CategorizeTransactionsExpected(expected, category)
			if err == nil || a.Rev() != revision || !reflect.DeepEqual(a.Transactions(), before) {
				t.Fatalf("invalid categorization changed state: %v", err)
			}
		})
	}
}

func TestExpectedCreationCategoryCanonicalDuplicateAndIDCollision(t *testing.T) {
	a, _, existing := creationFixture(t)
	revision := a.Rev()
	for _, id := range []string{"new", existing.ID} {
		got, created, err := a.CreateCategoryExpected(domain.Category{ID: id, Name: " FOOD ", Kind: domain.KindIncome})
		if err != nil || created || got.ID != existing.ID || a.Rev() != revision {
			t.Fatalf("duplicate category changed: %+v %v %v", got, created, err)
		}
	}
	if _, _, err := a.CreateCategoryExpected(domain.Category{ID: existing.ID, Name: "Different", Kind: domain.KindExpense}); err == nil || a.Rev() != revision {
		t.Fatalf("existing ID overwritten: %v", err)
	}
	got, created, err := a.CreateCategoryExpected(domain.Category{ID: "new", Name: "New category", Kind: domain.KindIncome})
	if err != nil || !created || got.Name != "New category" {
		t.Fatalf("create category failed: %+v %v %v", got, created, err)
	}
}

func TestAssistantTransactionAmountUsesFixedLiabilitySignChecked(t *testing.T) {
	for _, tc := range []struct {
		name       string
		account    domain.Account
		flow, want int64
	}{
		{"asset expense", domain.Account{}, -100, -100}, {"positive owed charge", domain.Account{Class: domain.ClassLiability, OpeningBalance: money.New(100, "USD")}, -100, 100}, {"positive owed payment", domain.Account{Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed}, 100, -100}, {"negative owed charge", domain.Account{Class: domain.ClassLiability, OpeningBalance: money.New(-100, "USD")}, -100, -100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.account.Currency = "USD"
			got, err := AssistantTransactionAmount(tc.account, money.New(tc.flow, "USD"))
			if err != nil || got.Amount != tc.want {
				t.Fatalf("booking sign: %v %v", got, err)
			}
		})
	}
	_, err := AssistantTransactionAmount(domain.Account{Class: domain.ClassLiability, Currency: "USD"}, money.New(math.MinInt64, "USD"))
	if !errors.Is(err, money.ErrOverflow) || !strings.Contains(err.Error(), "booking") {
		t.Fatalf("unrepresentable sign accepted: %v", err)
	}
}
