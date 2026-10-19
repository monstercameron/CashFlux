// SPDX-License-Identifier: MIT

package appstate

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importaudit"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
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

func TestCSVImportHistoryCollisionRollsBackEveryTransaction(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)
	if err := a.PutDocument(domain.Document{ID: "occupied", Kind: domain.DocCSV}); err != nil {
		t.Fatal(err)
	}
	result, err := a.ImportTransactionsCSV([]byte(importCSV), "", "occupied")
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 0 || len(result.Failed) != 3 {
		t.Fatalf("result = %+v, want all three rows failed after atomic rollback", result)
	}
	if got := len(a.Transactions()); got != 0 {
		t.Fatalf("transactions after failed import = %d, want 0", got)
	}
}

func TestCSVImportWithoutHistoryCannotOverwriteTransactionID(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)
	prior := domain.Transaction{ID: "fixed", AccountID: "chk", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Desc: "Original", Amount: money.New(-100, "USD")}
	if err := a.Store().PutTransaction(prior); err != nil {
		t.Fatal(err)
	}
	csv := []byte("id,date,account_id,desc,amount\nfixed,2026-01-02,chk,Replacement,-2.00\n")
	result, err := a.ImportTransactionsCSV(csv, "", "")
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := a.Store().GetTransaction("fixed")
	if err != nil || !found || got.Desc != "Original" {
		t.Fatalf("transaction after conflicting import = %+v found=%v err=%v", got, found, err)
	}
	if result.Imported != 0 || len(result.Failed) != 1 || !strings.Contains(result.Failed[0].Reason, "not saved") {
		t.Fatalf("import result = %+v", result)
	}
}

func TestReviewedDocumentInvalidDateIsRejectedWithoutClockFallback(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)
	rows := []extract.Row{{Date: "2026-02-30", Description: "Rent", Amount: "-1200.00"}}
	batch, err := a.Stage(domain.DocImage, rows, "chk")
	preview, failed := batch.Approved(), imaUnusableRows(batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != 0 || len(failed) != 1 || !strings.Contains(failed[0].Reason, "date") {
		t.Fatalf("preview/failures = %+v / %+v", preview, failed)
	}
	result, err := a.ImportReviewedDocumentRows(domain.DocImage, "chk", rows)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 0 || len(result.Failed) != 1 || len(a.Transactions()) != 0 {
		t.Fatalf("import result/state = %+v / %+v", result, a.Transactions())
	}
}

func TestCSVImportKeepsDistinctSameDayPurchasesAndPreviewMatchesCommit(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)
	if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRule(rules.Rule{ID: "food-rule", Match: "coffee", SetCategoryID: "food"}); err != nil {
		t.Fatal(err)
	}
	csv := []byte("date,account_id,desc,amount\n2026-01-02,chk,Coffee Shop,-4.50\n2026-01-02,chk,Parking,-4.50\n")
	batch, err := a.StageCSV(csv, "")
	preview := imaUsableTransactions(batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != 2 || preview[0].CategoryID != "food" {
		t.Fatalf("preview = %+v", preview)
	}
	if len(batch.Rows) != 2 || !batch.Rows[1].PossibleDuplicate {
		t.Fatalf("preview details = %+v", batch)
	}
	result, err := a.ImportTransactionsCSV(csv, "", "purchases")
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 2 || result.Duplicates != 0 || len(result.PossibleDuplicates) != 1 {
		t.Fatalf("import result = %+v", result)
	}
	byDesc := map[string]string{}
	for _, txn := range a.Transactions() {
		byDesc[txn.Desc] = txn.CategoryID
	}
	if len(byDesc) != 2 || byDesc["Coffee Shop"] != "food" || byDesc["Parking"] != "" {
		t.Fatalf("committed categories = %+v", byDesc)
	}
}

func TestReviewedDocumentPreviewAndCommitShareAutoCategory(t *testing.T) {
	a := newApp(t, false)
	seedImportAccount(t, a)
	if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRule(rules.Rule{ID: "food-rule", Match: "coffee", SetCategoryID: "food"}); err != nil {
		t.Fatal(err)
	}
	rows := []extract.Row{{Date: "2026-01-02", Description: "Coffee Shop", Amount: "-4.50"}}
	batch, err := a.Stage(domain.DocImage, rows, "chk")
	preview, failed := batch.Approved(), imaUnusableRows(batch)
	if err != nil || len(failed) != 0 {
		t.Fatalf("preview error/failures = %v / %+v", err, failed)
	}
	if len(preview) != 1 || preview[0].Transaction.CategoryID != "food" {
		t.Fatalf("review preview = %+v", preview)
	}
	result, err := a.ImportReviewedDocumentRows(domain.DocImage, "chk", rows)
	if err != nil || result.Imported != 1 {
		t.Fatalf("import result/error = %+v / %v", result, err)
	}
	if got := a.Transactions()[0].CategoryID; got != "food" {
		t.Fatalf("committed category = %q, want food", got)
	}
}
