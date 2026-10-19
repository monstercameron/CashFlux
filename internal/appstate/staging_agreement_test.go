// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/staging"
)

// StageCSV is the public preflight path; the legacy importer delegates to the
// same model. Compare every row outcome so preview agreement cannot be a
// coincidence between a test-only parser and a different production parser.
func stageSameCSV(t *testing.T, a *App, csv, fallback string) staging.Batch {
	t.Helper()
	batch, err := a.StageCSV([]byte(csv), fallback)
	if err != nil {
		t.Fatalf("StageCSV: %v", err)
	}
	return batch.Batch
}

func seedAgreementAccount(t *testing.T, a *App) {
	t.Helper()
	if err := a.PutAccount(domain.Account{
		ID: "chk", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(0, "USD"),
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
}

func TestStagingAndTheImporterAgreeOnWhatIsNew(t *testing.T) {
	const fresh = "date,account_id,desc,amount\n" +
		"2026-06-10,Checking,Coffee,-10\n" +
		"2026-06-11,Checking,Lunch,-20\n" +
		"2026-06-12,Checking,Books,-30\n"

	cases := []struct {
		name string
		seed string // imported first, to create duplicates
		file string
	}{
		{"an entirely new file", "", fresh},
		{"an unknown account is a row error", "", "date,account,desc,amount\n2026-06-10,Chase Visa,Coffee,-10\n"},
		{"a file already fully imported", fresh, fresh},
		{"a file overlapping an earlier one", "date,account_id,desc,amount\n2026-06-10,Checking,Coffee,-10\n", fresh},
		{"a file repeating a row within itself", "",
			fresh + "2026-06-10,Checking,Coffee,-10\n"},
		{"two same-file rows with one existing match", "date,account_id,desc,amount\n2026-06-10,Checking,Coffee,-10\n",
			"date,account_id,desc,amount\n2026-06-10,Checking,Coffee,-10\n2026-06-10,Checking,Coffee,-10\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := newApp(t, false)
			seedAgreementAccount(t, a)
			if c.seed != "" {
				if _, err := a.ImportTransactionsCSV([]byte(c.seed), "chk", "seed"); err != nil {
					t.Fatalf("seed import: %v", err)
				}
			}

			batch := stageSameCSV(t, a, c.file, "chk")
			before := len(a.Transactions())
			res, err := a.ImportTransactionsCSV([]byte(c.file), "chk", "run")
			if err != nil {
				t.Fatalf("import: %v", err)
			}
			written := len(a.Transactions()) - before

			if res.Imported != written {
				t.Fatalf("the importer reported %d written but the ledger grew by %d", res.Imported, written)
			}
			if got := len(batch.Approved()); got != written {
				t.Errorf("staging approved %d rows, the importer wrote %d — the preview and the import disagree",
					got, written)
			}
			if batch.Counts.Unusable != len(res.Failed) {
				t.Errorf("staging rejected %d, importer rejected %d", batch.Counts.Unusable, len(res.Failed))
			}
			// The skipped halves must agree too, or the preview's summary is a
			// different story from the one the history records.
			stagedSkipped := batch.Counts.Duplicate + batch.Counts.RepeatedInFile
			if stagedSkipped != res.Duplicates {
				t.Errorf("staging skipped %d as already-present, the importer skipped %d",
					stagedSkipped, res.Duplicates)
			}
		})
	}
}

// A row the file assigns to no account rides the caller's chosen one, in both.
func TestStagingAndTheImporterAgreeOnTheFallbackAccount(t *testing.T) {
	a := newApp(t, false)
	seedAgreementAccount(t, a)
	const csv = "date,desc,amount\n2026-06-10,Coffee,-10\n2026-06-11,Lunch,-20\n"

	batch := stageSameCSV(t, a, csv, "chk")
	before := len(a.Transactions())
	if _, err := a.ImportTransactionsCSV([]byte(csv), "chk", "run"); err != nil {
		t.Fatalf("import: %v", err)
	}
	written := len(a.Transactions()) - before
	if len(batch.Approved()) != written {
		t.Errorf("staging approved %d, the importer wrote %d", len(batch.Approved()), written)
	}
	for _, r := range batch.Approved() {
		if r.AccountID != "chk" {
			t.Errorf("staged row %d landed on %q, want the fallback", r.Index, r.AccountID)
		}
	}
}
