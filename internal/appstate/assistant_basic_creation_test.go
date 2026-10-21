// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestCreateTaskExpectedRequiresReviewedCollectionAndNeverOverwrites(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	expected, err := a.Store().ListTasks()
	if err != nil {
		t.Fatal(err)
	}
	draft := domain.Task{ID: "assistant-task", Title: "Call the bank", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceAI}
	got, err := a.CreateTaskExpected(draft, expected)
	if err != nil || got.ID != draft.ID || got.Title != draft.Title {
		t.Fatalf("create task: %+v, %v", got, err)
	}
	revision := a.Rev()
	if _, err := a.CreateTaskExpected(draft, expected); err == nil || a.Rev() != revision {
		t.Fatalf("same ID overwrote task: err=%v revision=%d want=%d", err, a.Rev(), revision)
	}
	stale := append([]domain.Task(nil), expected...)
	if err := a.Store().PutTask(domain.Task{ID: "other", Title: "changed", Status: domain.StatusOpen, Priority: domain.PriorityLow}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateTaskExpected(domain.Task{ID: "another", Title: "Another", Status: domain.StatusOpen, Priority: domain.PriorityMedium}, stale); err == nil {
		t.Fatal("expected changed task collection to reject creation")
	}
}

func storeSettingsUSD() store.Settings { return store.Settings{BaseCurrency: "USD"} }

func storeSettingsEUR() store.Settings { return store.Settings{BaseCurrency: "EUR"} }

func TestCreateTaskExpectedChecksRoleAndValidation(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	expected, _ := a.Store().ListTasks()
	draft := domain.Task{ID: "assistant-task", Title: "Valid", Status: domain.StatusOpen, Priority: domain.PriorityMedium}
	a.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	if _, err := a.CreateTaskExpected(draft, expected); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer write error = %v", err)
	}
	a.SetActiveRoleFunc(nil)
	draft.Title = " "
	if _, err := a.CreateTaskExpected(draft, expected); err == nil {
		t.Fatal("expected invalid task to be rejected")
	}
}

func TestCreateAccountExpectedBindsAccountsAndBaseCurrency(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	if err := a.Store().PutSettings(storeSettingsUSD()); err != nil {
		t.Fatal(err)
	}
	expected, err := a.Store().ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	draft := domain.Account{
		ID: "assistant-account", Name: "Family loan", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD", OpeningBalance: money.New(-12345, "USD"),
		BalanceAsOf: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
	}
	got, err := a.CreateAccountExpected(draft, expected, "USD")
	if err != nil || got.LiabilitySign != domain.LiabilityNegativeOwed {
		t.Fatalf("create account: %+v, %v", got, err)
	}
	revision := a.Rev()
	if _, err := a.CreateAccountExpected(draft, expected, "USD"); err == nil || a.Rev() != revision {
		t.Fatalf("same ID overwrote account: err=%v revision=%d want=%d", err, a.Rev(), revision)
	}
	if _, err := a.CreateAccountExpected(domain.Account{ID: "stale", Name: "Stale", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.Zero("USD")}, expected, "EUR"); err == nil {
		t.Fatal("expected changed currency context to reject creation")
	}
	if err := a.Store().PutSettings(storeSettingsEUR()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAccountExpected(domain.Account{ID: "changed-settings", Name: "Changed settings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.Zero("USD")}, a.Accounts(), "USD"); err == nil {
		t.Fatal("expected saved base currency change to reject creation")
	}
}
