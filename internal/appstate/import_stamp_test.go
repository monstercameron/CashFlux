// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/importaudit"
	"github.com/monstercameron/CashFlux/internal/money"
)

// C687: an imported row has to be able to say which run created it, or the
// history's count and the ledger's count are two unconnected numbers.
func seedImportAccount(t *testing.T, a *App) domain.Account {
	t.Helper()
	acc := domain.Account{
		ID: "chk", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(0, "USD"),
	}
	if err := a.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	return acc
}

const importCSV = "date,account_id,desc,amount\n" +
	"2026-06-10,Checking,Coffee,-10\n" +
	"2026-06-11,Checking,Lunch,-20\n" +
	"2026-06-12,Checking,Books,-30\n"

func TestImportStampsEveryRowWithItsRun(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)

	res, err := a.ImportTransactionsCSV([]byte(importCSV), "", "run-1")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Imported != 3 {
		t.Fatalf("imported = %d, want 3", res.Imported)
	}
	stamped := 0
	for _, x := range a.Transactions() {
		if x.SourceDocID == "run-1" {
			stamped++
		}
	}
	if stamped != 3 {
		t.Errorf("%d of 3 rows carry the run id", stamped)
	}
}

// The duplicate count used to be computed and thrown away, so a re-import that
// wrote nothing reported nothing.
func TestReimportReportsDuplicatesRatherThanSilence(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)

	if _, err := a.ImportTransactionsCSV([]byte(importCSV), "", "run-1"); err != nil {
		t.Fatalf("first import: %v", err)
	}
	res, err := a.ImportTransactionsCSV([]byte(importCSV), "", "run-2")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if res.Imported != 0 {
		t.Errorf("imported = %d, want 0 on a re-import", res.Imported)
	}
	if res.Duplicates != 3 {
		t.Errorf("duplicates = %d, want 3 — the count used to be dropped", res.Duplicates)
	}
	if res.Presented() != 3 {
		t.Errorf("Presented = %d, want 3", res.Presented())
	}
}

// Parse failures and duplicates are different facts and must not be conflated.
func TestFailedRowsAreCountedApartFromDuplicates(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)
	if _, err := a.ImportTransactionsCSV([]byte(importCSV), "", "run-1"); err != nil {
		t.Fatalf("seed import: %v", err)
	}

	mixed := importCSV + "2026-06-13,Checking,Broken,not-a-number\n"
	res, err := a.ImportTransactionsCSV([]byte(mixed), "", "run-2")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Duplicates != 3 {
		t.Errorf("duplicates = %d, want 3", res.Duplicates)
	}
	if len(res.Failed) != 1 {
		t.Errorf("failed = %d, want the unparseable row counted separately", len(res.Failed))
	}
}

// End to end: import, delete some rows, and the audit says where they went.
func TestAuditExplainsRowsRemovedAfterImport(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)

	if _, err := a.ImportTransactionsCSV([]byte(importCSV), "", "run-1"); err != nil {
		t.Fatalf("import: %v", err)
	}
	doc := domain.Document{
		ID: "run-1", Kind: domain.DocCSV, Status: domain.DocImported,
		AccountID: "chk", RowCount: 3,
	}
	if err := a.PutDocument(doc); err != nil {
		t.Fatalf("PutDocument: %v", err)
	}

	var victim string
	for _, x := range a.Transactions() {
		if x.SourceDocID == "run-1" {
			victim = x.ID
			break
		}
	}
	if err := a.DeleteTransaction(victim); err != nil {
		t.Fatalf("delete: %v", err)
	}

	tally := importaudit.For(doc, a.Transactions())
	if !tally.ActiveKnown {
		t.Fatalf("the run is untraceable after stamping")
	}
	if tally.Active != 2 || tally.Removed != 1 {
		t.Errorf("Active/Removed = %d/%d, want 2/1", tally.Active, tally.Removed)
	}
	if tally.Clean() {
		t.Errorf("Clean() = true, but a row has gone since the import")
	}
}

// An import that names no run still works; it simply cannot be traced later.
func TestImportWithoutARunIDStillImports(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)
	res, err := a.ImportTransactionsCSV([]byte(importCSV), "", "")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Imported != 3 {
		t.Errorf("imported = %d, want 3", res.Imported)
	}
	for _, x := range a.Transactions() {
		if x.SourceDocID != "" {
			t.Errorf("row %s was stamped with %q despite no run being named", x.ID, x.SourceDocID)
		}
	}
}
