// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestInstitutionReassignOnDelete(t *testing.T) {
	app := newApp(t, false)
	if err := app.PutInstitution(domain.Institution{ID: "chase", Name: "Chase"}); err != nil {
		t.Fatalf("put institution: %v", err)
	}
	acc := domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, InstitutionID: "chase"}
	if err := app.PutAccount(acc); err != nil {
		t.Fatalf("put account: %v", err)
	}
	if err := app.DeleteInstitution("chase"); err != nil {
		t.Fatalf("delete institution: %v", err)
	}
	got := app.Accounts()
	if len(got) != 1 || got[0].InstitutionID != "" {
		t.Errorf("account should fall back to no-institution, got %+v", got)
	}
	if len(app.Institutions()) != 0 {
		t.Errorf("institution should be deleted")
	}
}

func TestProjectAccountLowPoint(t *testing.T) {
	app := newApp(t, false)
	asOf := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	acc := domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		OpeningBalance: money.New(234000, "USD"), BalanceAsOf: asOf}
	if err := app.PutAccount(acc); err != nil {
		t.Fatalf("put account: %v", err)
	}
	// Rent −$1,400 on the 3rd, scoped to this account.
	rent := domain.Recurring{ID: "r1", Label: "Rent", Amount: money.New(-140000, "USD"), Cadence: domain.CadenceMonthly,
		NextDue: time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC), AccountID: "a1"}
	if err := app.PutRecurring(rent); err != nil {
		t.Fatalf("put recurring: %v", err)
	}
	p := app.ProjectAccount("a1", asOf, 30)
	if p.Low != 94000 {
		t.Errorf("Low = %d, want 94000", p.Low)
	}
	if len(p.Drivers) == 0 || p.Drivers[0].Label != "Rent" {
		t.Errorf("expected rent driver: %+v", p.Drivers)
	}
}

func TestDocExpiryReconcile(t *testing.T) {
	app := newApp(t, false)
	now := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	acc := domain.Account{ID: "a1", Name: "Car", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeOther, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		DocRefs: []domain.AccountDocRef{{ArtifactID: "pol", Label: "Registration", AttachedAt: now.AddDate(0, -2, 0), ExpiresAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}}}
	if err := app.PutAccount(acc); err != nil {
		t.Fatalf("put account: %v", err)
	}
	created, resolved, err := app.ReconcileDocExpiryTasks(now, 30)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if created != 1 || resolved != 0 {
		t.Fatalf("first reconcile created=%d resolved=%d, want 1/0", created, resolved)
	}
	// Idempotent: running again creates nothing.
	created2, _, _ := app.ReconcileDocExpiryTasks(now, 30)
	if created2 != 0 {
		t.Errorf("second reconcile should be a no-op, created=%d", created2)
	}
	// File a newer Registration → the old reminder auto-resolves.
	acc.DocRefs = append(acc.DocRefs, domain.AccountDocRef{ArtifactID: "pol2", Label: "Registration", AttachedAt: now, ExpiresAt: time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC)})
	if err := app.PutAccount(acc); err != nil {
		t.Fatalf("put account 2: %v", err)
	}
	_, resolved3, err := app.ReconcileDocExpiryTasks(now, 30)
	if err != nil {
		t.Fatalf("reconcile 3: %v", err)
	}
	if resolved3 != 1 {
		t.Errorf("renewing the doc should resolve the reminder, resolved=%d", resolved3)
	}
}

func TestBuildEmergencyPackNoCredentials(t *testing.T) {
	app := newApp(t, false)
	if err := app.PutInstitution(domain.Institution{ID: "chase", Name: "Chase", SupportPhone: "1-800-935-9935"}); err != nil {
		t.Fatalf("put institution: %v", err)
	}
	acc := domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		InstitutionID: "chase", BeneficiaryNote: "TOD to Jane"}
	if err := app.PutAccount(acc); err != nil {
		t.Fatalf("put account: %v", err)
	}
	pack := app.BuildEmergencyPack("Cam", "Take care.", nil, true, time.Now())
	if len(pack.Accounts) != 1 || pack.Accounts[0].BeneficiaryNote != "TOD to Jane" {
		t.Errorf("pack account wrong: %+v", pack.Accounts)
	}
	if len(pack.Institutions) != 1 {
		t.Errorf("pack should list the institution: %+v", pack.Institutions)
	}
}
