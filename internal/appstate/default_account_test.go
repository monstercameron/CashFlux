// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestWorkspaceDefaultAccountSetAndClear(t *testing.T) {
	app := newApp(t, false)
	account := domain.Account{
		ID: "checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}
	if err := app.PutAccount(account); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}

	if err := app.SetDefaultAccountID(account.ID); err != nil {
		t.Fatalf("SetDefaultAccountID: %v", err)
	}
	if got := app.DefaultAccountID(); got != account.ID {
		t.Fatalf("DefaultAccountID() = %q, want %q", got, account.ID)
	}
	ds, err := app.Store().Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after set: %v", err)
	}
	if ds.Settings.DefaultAccountID != account.ID {
		t.Fatalf("persisted default = %q, want %q", ds.Settings.DefaultAccountID, account.ID)
	}

	// Safety checkpoints use the same redacted dataset export as ImportJSON.
	checkpoint, err := app.ExportJSONRedacted()
	if err != nil {
		t.Fatalf("ExportJSONRedacted checkpoint: %v", err)
	}
	checkpointRestore := newApp(t, false)
	if err := checkpointRestore.ImportJSON(checkpoint); err != nil {
		t.Fatalf("ImportJSON checkpoint: %v", err)
	}
	if got := checkpointRestore.DefaultAccountID(); got != account.ID {
		t.Fatalf("checkpoint default account = %q, want %q", got, account.ID)
	}

	// A portable SQLite backup must carry the same workspace setting.
	backup, err := app.ExportSQLite()
	if err != nil {
		t.Fatalf("ExportSQLite backup: %v", err)
	}
	backupRestore := newApp(t, false)
	if err := backupRestore.ImportSQLite(backup); err != nil {
		t.Fatalf("ImportSQLite backup: %v", err)
	}
	if got := backupRestore.DefaultAccountID(); got != account.ID {
		t.Fatalf("backup default account = %q, want %q", got, account.ID)
	}

	if err := app.ClearDefaultAccountID(); err != nil {
		t.Fatalf("ClearDefaultAccountID: %v", err)
	}
	if got := app.DefaultAccountID(); got != "" {
		t.Fatalf("DefaultAccountID() after clear = %q, want empty", got)
	}
}

func TestSetWorkspaceDefaultAccountRejectsMissingOrArchived(t *testing.T) {
	app := newApp(t, false)
	archived := domain.Account{
		ID: "archived", Name: "Archived", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Archived: true,
	}
	if err := app.PutAccount(archived); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}

	for _, accountID := range []string{"missing", archived.ID, "  "} {
		if err := app.SetDefaultAccountID(accountID); err == nil {
			t.Errorf("SetDefaultAccountID(%q) succeeded, want error", accountID)
		}
		if got := app.DefaultAccountID(); got != "" {
			t.Errorf("failed set changed default to %q", got)
		}
	}
}

func TestDeleteAccountClearsDefaultsInSameDatasetWrite(t *testing.T) {
	app := newApp(t, false)
	for _, account := range []domain.Account{
		{ID: "remove", Name: "Remove", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "keep", Name: "Keep", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatalf("PutAccount(%q): %v", account.ID, err)
		}
	}
	for _, member := range []domain.Member{
		{ID: "m1", Name: "One", Prefs: domain.MemberPrefs{DefaultAccountID: "remove"}},
		{ID: "m2", Name: "Two", Prefs: domain.MemberPrefs{DefaultAccountID: "keep"}},
	} {
		if err := app.PutMember(member); err != nil {
			t.Fatalf("PutMember(%q): %v", member.ID, err)
		}
	}
	if err := app.SetDefaultAccountID("remove"); err != nil {
		t.Fatalf("SetDefaultAccountID: %v", err)
	}

	if err := app.DeleteAccount("remove"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	ds, err := app.Store().Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after delete: %v", err)
	}
	if len(ds.Accounts) != 1 || ds.Accounts[0].ID != "keep" {
		t.Fatalf("accounts after delete = %+v", ds.Accounts)
	}
	if ds.Settings.DefaultAccountID != "" {
		t.Errorf("workspace default = %q, want empty", ds.Settings.DefaultAccountID)
	}
	if ds.Members[0].Prefs.DefaultAccountID != "" {
		t.Errorf("deleted account member default = %q, want empty", ds.Members[0].Prefs.DefaultAccountID)
	}
	if ds.Members[1].Prefs.DefaultAccountID != "keep" {
		t.Errorf("unrelated member default = %q, want keep", ds.Members[1].Prefs.DefaultAccountID)
	}
}

func TestArchivingAccountRetainsWorkspaceDefault(t *testing.T) {
	app := newApp(t, false)
	account := domain.Account{
		ID: "checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}
	if err := app.PutAccount(account); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.SetDefaultAccountID(account.ID); err != nil {
		t.Fatalf("SetDefaultAccountID: %v", err)
	}
	account.Archived = true
	if err := app.PutAccount(account); err != nil {
		t.Fatalf("archive with PutAccount: %v", err)
	}
	if got := app.DefaultAccountID(); got != account.ID {
		t.Fatalf("archived account default = %q, want retained %q", got, account.ID)
	}
}
