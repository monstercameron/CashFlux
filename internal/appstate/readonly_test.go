// SPDX-License-Identifier: MIT

package appstate_test

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

// makeApp builds a test App seeded with one member of the given role and
// installs that role as the active identity via SetActiveRoleFunc.
func makeApp(t *testing.T, role domain.MemberRole) *appstate.App {
	t.Helper()
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	// Wire the active-role function to return a fixed role — no uistate import.
	app.SetActiveRoleFunc(func() domain.MemberRole { return role })
	return app
}

// validAccount returns a minimal account that passes ValidateAccount.
func validAccount() domain.Account {
	return domain.Account{
		ID:             id.New(),
		Name:           "Checking",
		OwnerID:        domain.GroupOwnerID,
		Scope:          domain.ScopeShared,
		Class:          domain.ClassAsset,
		Type:           domain.TypeChecking,
		Currency:       "USD",
		OpeningBalance: money.Money{Amount: 0, Currency: "USD"},
	}
}

// ── ActiveRole / CanEdit / CanManageMembers ────────────────────────────────

func TestActiveRoleDefaultsToOwner(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if got := app.ActiveRole(); got != domain.RoleOwner {
		t.Errorf("ActiveRole (no fn wired) = %q, want %q", got, domain.RoleOwner)
	}
}

func TestCanEditByRole(t *testing.T) {
	cases := []struct {
		role domain.MemberRole
		want bool
	}{
		{domain.RoleOwner, true},
		{domain.RoleAdmin, true},
		{domain.RoleViewer, false},
	}
	for _, tc := range cases {
		app := makeApp(t, tc.role)
		if got := app.CanEdit(); got != tc.want {
			t.Errorf("role %q: CanEdit() = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestCanManageMembersByRole(t *testing.T) {
	cases := []struct {
		role domain.MemberRole
		want bool
	}{
		{domain.RoleOwner, true},
		{domain.RoleAdmin, false},
		{domain.RoleViewer, false},
	}
	for _, tc := range cases {
		app := makeApp(t, tc.role)
		if got := app.CanManageMembers(); got != tc.want {
			t.Errorf("role %q: CanManageMembers() = %v, want %v", tc.role, got, tc.want)
		}
	}
}

// ── ErrReadOnly sentinel ───────────────────────────────────────────────────

func TestErrReadOnlyIsSentinel(t *testing.T) {
	app := makeApp(t, domain.RoleViewer)
	err := app.PutAccount(validAccount())
	if err == nil {
		t.Fatal("expected ErrReadOnly, got nil")
	}
	if !errors.Is(err, appstate.ErrReadOnly) {
		t.Errorf("expected errors.Is(err, ErrReadOnly); got %v", err)
	}
}

// ── Financial entity guard (Viewer is blocked, Owner/Admin succeed) ────────

func TestPutAccountRoleGating(t *testing.T) {
	cases := []struct {
		role    domain.MemberRole
		wantErr bool
	}{
		{domain.RoleOwner, false},
		{domain.RoleAdmin, false},
		{domain.RoleViewer, true},
	}
	for _, tc := range cases {
		app := makeApp(t, tc.role)
		err := app.PutAccount(validAccount())
		gotReadOnly := errors.Is(err, appstate.ErrReadOnly)
		if tc.wantErr && !gotReadOnly {
			t.Errorf("role %q: PutAccount expected ErrReadOnly, got %v", tc.role, err)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("role %q: PutAccount unexpected error: %v", tc.role, err)
		}
	}
}

func TestDeleteAccountRoleGating(t *testing.T) {
	cases := []struct {
		role    domain.MemberRole
		wantErr bool
	}{
		{domain.RoleOwner, false},  // no such id → not ErrReadOnly
		{domain.RoleViewer, true},  // blocked before even checking the id
	}
	for _, tc := range cases {
		app := makeApp(t, tc.role)
		err := app.DeleteAccount("nonexistent-id")
		gotReadOnly := errors.Is(err, appstate.ErrReadOnly)
		if tc.wantErr && !gotReadOnly {
			t.Errorf("role %q: DeleteAccount expected ErrReadOnly, got %v", tc.role, err)
		}
		if !tc.wantErr && gotReadOnly {
			t.Errorf("role %q: DeleteAccount unexpected ErrReadOnly", tc.role)
		}
	}
}

func TestPutTransactionRoleGating(t *testing.T) {
	cases := []struct {
		role    domain.MemberRole
		wantErr bool
	}{
		{domain.RoleOwner, false},
		{domain.RoleAdmin, false},
		{domain.RoleViewer, true},
	}
	for _, tc := range cases {
		app := makeApp(t, tc.role)
		// Seed an account first (as owner) so the transaction has a valid account.
		ownerApp := makeApp(t, domain.RoleOwner)
		acct := domain.Account{
			ID:             id.New(),
			Name:           "Cash",
			Currency:       "USD",
			OpeningBalance: money.Money{Amount: 0, Currency: "USD"},
		}
		_ = ownerApp.PutAccount(acct)

		// Build the transaction against any account (validation checks the ID format,
		// not existence for this test purpose — we care about the role gate).
		txn := domain.Transaction{
			ID:        id.New(),
			AccountID: acct.ID,
			Amount:    money.Money{Amount: 100, Currency: "USD"},
		}

		err := app.PutTransaction(txn)
		gotReadOnly := errors.Is(err, appstate.ErrReadOnly)
		if tc.wantErr && !gotReadOnly {
			t.Errorf("role %q: PutTransaction expected ErrReadOnly, got %v", tc.role, err)
		}
		if !tc.wantErr && gotReadOnly {
			t.Errorf("role %q: PutTransaction unexpected ErrReadOnly", tc.role)
		}
	}
}

// ── Member management guard (only Owner may manage) ───────────────────────

func TestPutMemberRoleGating(t *testing.T) {
	validMember := func() domain.Member {
		return domain.Member{ID: id.New(), Name: "Alice", Color: "#aabbcc"}
	}
	cases := []struct {
		role    domain.MemberRole
		wantErr bool
	}{
		{domain.RoleOwner, false},
		{domain.RoleAdmin, true},
		{domain.RoleViewer, true},
	}
	for _, tc := range cases {
		app := makeApp(t, tc.role)
		err := app.PutMember(validMember())
		gotReadOnly := errors.Is(err, appstate.ErrReadOnly)
		if tc.wantErr && !gotReadOnly {
			t.Errorf("role %q: PutMember expected ErrReadOnly, got %v", tc.role, err)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("role %q: PutMember unexpected error: %v", tc.role, err)
		}
	}
}

func TestDeleteMemberRoleGating(t *testing.T) {
	cases := []struct {
		role    domain.MemberRole
		wantErr bool
	}{
		{domain.RoleOwner, false},
		{domain.RoleAdmin, true},
		{domain.RoleViewer, true},
	}
	for _, tc := range cases {
		app := makeApp(t, tc.role)
		err := app.DeleteMember("nonexistent-id")
		gotReadOnly := errors.Is(err, appstate.ErrReadOnly)
		if tc.wantErr && !gotReadOnly {
			t.Errorf("role %q: DeleteMember expected ErrReadOnly, got %v", tc.role, err)
		}
		if !tc.wantErr && gotReadOnly {
			t.Errorf("role %q: DeleteMember unexpected ErrReadOnly", tc.role)
		}
	}
}

// ── SetActiveRoleFunc wiring ───────────────────────────────────────────────

func TestSetActiveRoleFuncDynamic(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	current := domain.RoleOwner
	app.SetActiveRoleFunc(func() domain.MemberRole { return current })

	// Owner → expect success (no ErrReadOnly).
	if err := app.PutAccount(validAccount()); err != nil {
		t.Fatalf("owner PutAccount: %v", err)
	}

	// Switch to Viewer → expect ErrReadOnly.
	current = domain.RoleViewer
	if err := app.PutAccount(validAccount()); !errors.Is(err, appstate.ErrReadOnly) {
		t.Errorf("viewer PutAccount: expected ErrReadOnly, got %v", err)
	}
}
