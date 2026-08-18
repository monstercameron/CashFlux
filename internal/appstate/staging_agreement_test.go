// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/staging"
)

// C689 stages a file to preview it and then imports the raw bytes, so the two
// decide independently which rows are new. They agree today because both ask
// dedupe.Key — but agreement by coincidence is exactly how the duplicates screen
// and the importer came to disagree in the first place (C688), and nothing was
// checking it.
//
// This is that check: for the same file and the same ledger, what staging says
// will import and what the importer actually writes must match.
func stageSameCSV(t *testing.T, a *App, csv, fallback string) staging.Batch {
	t.Helper()
	parsed, err := a.ParseCSVForPreview([]byte(csv), fallback)
	if err != nil {
		t.Fatalf("ParseCSVForPreview: %v", err)
	}
	inputs := make([]staging.Input, 0, len(parsed))
	for _, x := range parsed {
		inputs = append(inputs, staging.Input{
			Date: x.Date, Description: x.Desc, AmountMinor: x.Amount.Amount,
			HasAmount: true, AccountID: x.AccountID,
		})
	}
	return staging.Stage(inputs, a.Transactions(), fallback, "USD")
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
		{"a file already fully imported", fresh, fresh},
		{"a file overlapping an earlier one", "date,account_id,desc,amount\n2026-06-10,Checking,Coffee,-10\n", fresh},
		{"a file repeating a row within itself", "",
			fresh + "2026-06-10,Checking,Coffee,-10\n"},
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
