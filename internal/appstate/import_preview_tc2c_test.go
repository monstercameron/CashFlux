// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestImportPreviewUsesImporterDuplicateCountAndAccountCurrency(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutAccount(domain.Account{
		ID: "jpy", Name: "JPY account", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "JPY", OpeningBalance: money.New(0, "JPY"),
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	const csv = "date,account_id,desc,amount\n" +
		"2026-06-10,jpy,Train fare,1500\n" +
		"2026-06-10,jpy,Train fare,1500\n"

	batch, err := a.StageCSV([]byte(csv), "jpy")
	rows := imaUsableTransactions(batch)
	if err != nil {
		t.Fatalf("StageCSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("preview rows = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.Amount.Amount != 1500 || row.Amount.Currency != "JPY" {
			t.Errorf("preview amount = %+v, want 1500 JPY", row.Amount)
		}
	}
	previewCount, previewDuplicates := batch.Counts.New+batch.Counts.Duplicate, batch.Counts.Duplicate
	want := dedupe.CountIncomingDuplicates(rows, a.Transactions(), "jpy")
	if previewCount != len(rows) || previewDuplicates != want || want != 0 {
		t.Fatalf("preview (rows=%d, duplicates=%d), importer count=%d; want 2, 0, 0", previewCount, previewDuplicates, want)
	}
	result, err := a.ImportTransactionsCSV([]byte(csv), "jpy", "tc2c")
	if err != nil {
		t.Fatalf("ImportTransactionsCSV: %v", err)
	}
	if result.Duplicates != want || result.Imported != len(rows) {
		t.Fatalf("import = %+v, want %d imported and %d duplicates", result, len(rows), want)
	}
}
