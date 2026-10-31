// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func seedDefaultsFixture(t *testing.T) *App {
	t.Helper()
	app := newApp(t, false)
	for _, account := range []domain.Account{
		{ID: "chk", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "old", Name: "Old", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Archived: true},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatalf("PutAccount %s: %v", account.ID, err)
		}
	}
	if err := app.PutMember(domain.Member{ID: "m1", Name: "Priya"}); err != nil {
		t.Fatalf("PutMember: %v", err)
	}
	return app
}

func memberDefault(t *testing.T, app *App, id string) string {
	t.Helper()
	for _, m := range app.Members() {
		if m.ID == id {
			return m.Prefs.DefaultAccountID
		}
	}
	t.Fatalf("member %q missing", id)
	return ""
}

func TestSetAndClearMemberDefaultAccount(t *testing.T) {
	app := seedDefaultsFixture(t)
	name, err := app.SetMemberDefaultAccount("m1", "chk")
	if err != nil || name != "Priya" {
		t.Fatalf("SetMemberDefaultAccount = %q, %v", name, err)
	}
	if got := memberDefault(t, app, "m1"); got != "chk" {
		t.Fatalf("default = %q, want chk", got)
	}
	name, err = app.ClearMemberDefaultAccount("m1")
	if err != nil || name != "Priya" {
		t.Fatalf("ClearMemberDefaultAccount = %q, %v", name, err)
	}
	if got := memberDefault(t, app, "m1"); got != "" {
		t.Fatalf("default after clear = %q", got)
	}
}

func TestSetMemberDefaultAccountRejectsUnusableTargets(t *testing.T) {
	app := seedDefaultsFixture(t)
	cases := []struct {
		name              string
		member, account   string
		want              error
		clearInsteadOfSet bool
	}{
		{name: "archived account", member: "m1", account: "old", want: ErrDefaultAccountArchived},
		{name: "unknown account", member: "m1", account: "ghost", want: ErrAccountNotFound},
		{name: "unknown member", member: "ghost", account: "chk", want: ErrMemberNotFound},
		{name: "clearing an unknown member", member: "ghost", want: ErrMemberNotFound, clearInsteadOfSet: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.clearInsteadOfSet {
				_, err = app.ClearMemberDefaultAccount(tc.member)
			} else {
				_, err = app.SetMemberDefaultAccount(tc.member, tc.account)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if got := memberDefault(t, app, "m1"); got != "" {
				t.Fatalf("a rejected call changed the default to %q", got)
			}
		})
	}
	if _, err := app.SetMemberDefaultAccount("m1", "  "); err == nil {
		t.Fatal("a blank account id must be rejected")
	}
}

func TestMarkBalancesUpdated(t *testing.T) {
	app := seedDefaultsFixture(t)
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	written, err := app.MarkBalancesUpdated([]string{"chk", "old"}, now)
	if err != nil || written != 2 {
		t.Fatalf("MarkBalancesUpdated = %d, %v", written, err)
	}
	for _, account := range app.Accounts() {
		if !account.BalanceAsOf.Equal(now) {
			t.Errorf("%s BalanceAsOf = %v, want %v", account.ID, account.BalanceAsOf, now)
		}
	}
}

func TestMarkBalancesUpdatedWritesNothingForAnUnknownID(t *testing.T) {
	app := seedDefaultsFixture(t)
	written, err := app.MarkBalancesUpdated([]string{"chk", "ghost"}, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrAccountNotFound) || written != 0 {
		t.Fatalf("MarkBalancesUpdated = %d, %v; want 0 and ErrAccountNotFound", written, err)
	}
	for _, account := range app.Accounts() {
		if !account.BalanceAsOf.IsZero() {
			t.Errorf("%s was stamped despite the failed batch", account.ID)
		}
	}
}

func TestMarkBalancesUpdatedWithNoIDsIsANoOp(t *testing.T) {
	app := seedDefaultsFixture(t)
	if written, err := app.MarkBalancesUpdated(nil, time.Now()); written != 0 || err != nil {
		t.Fatalf("MarkBalancesUpdated(nil) = %d, %v", written, err)
	}
}
