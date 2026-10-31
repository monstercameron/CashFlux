// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/acctproject"
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
	p, err := app.ProjectAccount("a1", asOf, 30)
	if err != nil {
		t.Fatal(err)
	}
	if p.Low != 94000 {
		t.Errorf("Low = %d, want 94000", p.Low)
	}
	if len(p.Drivers) == 0 || p.Drivers[0].Label != "Rent" {
		t.Errorf("expected rent driver: %+v", p.Drivers)
	}
}

func TestProjectAccountRejectsRecurringCurrencyMismatch(t *testing.T) {
	app := newApp(t, false)
	asOf := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	account := domain.Account{ID: "currency", Name: "Currency", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Currency: "USD", OpeningBalance: money.New(10000, "USD"), BalanceAsOf: asOf}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := app.PutRecurring(domain.Recurring{ID: "foreign", AccountID: account.ID, Label: "Foreign", Amount: money.New(-100, "EUR"), NextDue: asOf.AddDate(0, 0, 1), Cadence: domain.CadenceYearly}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProjectAccount(account.ID, asOf, 30); !errors.Is(err, acctproject.ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch was not surfaced: %v", err)
	}
}

// TestProjectAccountMatchesAccountview checks that the account projection uses the same recurring
// expansion as the account views, for an overdue bill and for an asOf with a time of day.
func TestProjectAccountMatchesAccountview(t *testing.T) {
	cases := []struct {
		name    string
		asOf    time.Time
		nextDue time.Time
	}{
		{"overdue", time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{"intraday", time.Date(2026, 3, 3, 15, 30, 0, 0, time.UTC), time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newApp(t, false)
			account := domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking,
				OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(234000, "USD"), BalanceAsOf: tc.asOf}
			if err := app.PutAccount(account); err != nil {
				t.Fatal(err)
			}
			rent := domain.Recurring{ID: "r1", Label: "Rent", Amount: money.New(-140000, "USD"), Cadence: domain.CadenceMonthly, NextDue: tc.nextDue, AccountID: "a1"}
			if err := app.PutRecurring(rent); err != nil {
				t.Fatal(err)
			}
			got, err := app.ProjectAccount("a1", tc.asOf, 30)
			if err != nil {
				t.Fatal(err)
			}
			want, err := accountview.PrepareProjectionInputs([]domain.Recurring{rent}).Project("a1", money.New(234000, "USD"), tc.asOf, 30)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ProjectAccount = %+v, want %+v", got, want)
			}
			if len(got.Drivers) == 0 {
				t.Errorf("the %s bill produced no driver", tc.name)
			}
		})
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

func TestDeleteInstitutionTouchesOnlyAccountsThatUsedIt(t *testing.T) {
	app := newApp(t, false)
	for _, in := range []domain.Institution{{ID: "chase", Name: "Chase"}, {ID: "ally", Name: "Ally"}} {
		if err := app.PutInstitution(in); err != nil {
			t.Fatal(err)
		}
	}
	for id, inst := range map[string]string{"a1": "chase", "a2": "ally", "a3": ""} {
		acc := refsAccount(id)
		acc.InstitutionID = inst
		if err := app.PutAccount(acc); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.DeleteInstitution("chase"); err != nil {
		t.Fatal(err)
	}
	if got := storedAccount(app, "a1").InstitutionID; got != "" {
		t.Errorf("a1 institution = %q, want cleared", got)
	}
	if got := storedAccount(app, "a2").InstitutionID; got != "ally" {
		t.Errorf("a2 institution = %q, want ally kept", got)
	}
	if len(app.Institutions()) != 1 {
		t.Errorf("institutions = %+v, want only ally", app.Institutions())
	}
	// Deleting an id nothing references changes nothing and is not an error.
	if err := app.DeleteInstitution("ghost"); err != nil {
		t.Errorf("deleting an unknown institution = %v, want nil", err)
	}
	if err := app.DeleteInstitution("ally"); err != nil {
		t.Fatal(err)
	}
	if got := storedAccount(app, "a2").InstitutionID; got != "" {
		t.Errorf("a2 institution = %q after deleting ally, want cleared", got)
	}
}

func TestDeleteInstitutionReportsUnreadableAccountsAndChangesNothing(t *testing.T) {
	app := newApp(t, false)
	if err := app.PutInstitution(domain.Institution{ID: "chase", Name: "Chase"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteInstitution("chase"); err == nil {
		t.Fatal("DeleteInstitution succeeded although accounts could not be read")
	}
}

func TestProjectAccountStatesALiabilityFromItsOwnSign(t *testing.T) {
	app := newApp(t, false)
	asOf := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	card := domain.Account{ID: "card", Name: "Card", Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
		LiabilitySign: domain.LiabilityPositiveOwed, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		OpeningBalance: money.New(50000, "USD"), BalanceAsOf: asOf}
	if err := app.PutAccount(card); err != nil {
		t.Fatal(err)
	}
	if err := app.PutRecurring(domain.Recurring{ID: "sub", Label: "Charge", AccountID: "card", Amount: money.New(-60000, "USD"),
		NextDue: asOf.AddDate(0, 0, 2), Cadence: domain.CadenceYearly}); err != nil {
		t.Fatal(err)
	}
	p, err := app.ProjectAccount("card", asOf, 30)
	if err != nil {
		t.Fatal(err)
	}
	if p.Low != -110000 {
		t.Fatalf("Low = %d, want -110000 (owes 500.00, then a 600.00 charge)", p.Low)
	}
}

func TestReadersOnAnAppWithoutAStoreReturnNothing(t *testing.T) {
	var app *App
	if got := app.SweepRules(); got != nil {
		t.Errorf("SweepRules = %v, want nil", got)
	}
	if got := app.SweepProposals(time.Now()); got != nil {
		t.Errorf("SweepProposals = %v, want nil", got)
	}
	if got := app.Institutions(); got != nil {
		t.Errorf("Institutions = %v, want nil", got)
	}
	if got := app.Members(); got != nil {
		t.Errorf("Members = %v, want nil", got)
	}
	if got := app.Tasks(); got != nil {
		t.Errorf("Tasks = %v, want nil", got)
	}
	if got := app.Earmarks(); got != nil {
		t.Errorf("Earmarks = %v, want nil", got)
	}
	if _, err := app.ProjectAccount("x", time.Now(), 30); !errors.Is(err, ErrDurabilityUnavailable) {
		t.Errorf("ProjectAccount = %v, want ErrDurabilityUnavailable", err)
	}
	pack := app.BuildEmergencyPack("Owner", "note", nil, false, time.Now())
	if pack.OwnerName != "Owner" || len(pack.Accounts) != 0 {
		t.Errorf("BuildEmergencyPack = %+v, want an empty pack for the owner", pack)
	}
}
