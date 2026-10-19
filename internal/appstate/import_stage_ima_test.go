// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/store"
)

// Exercise the exact account-aware store parser called by buildCSVPreflight,
// rather than constructing a separate test-only staging path.
func TestIMAUIParserStageAndCommitAgree(t *testing.T) {
	cases := []struct {
		name, account string
		failures      int
	}{
		{"blank uses selected account", "", 0},
		{"known name resolves", "Checking", 0},
		{"known ID resolves", "chk", 0},
		{"unknown name is a row error", "Chase Visa", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			seedAgreementAccount(t, a)
			raw := []byte("date,account,desc,amount\n2026-06-01," + tc.account + ",Coffee,-4.50\n")
			parsed, failed, err := store.TransactionsFromCSVResilientForAccountList(raw, "USD", "chk", []store.CSVAccount{{ID: "chk", Name: "Checking", Currency: "USD"}})
			if err != nil {
				t.Fatal(err)
			}
			batch, err := a.StageCSV(raw, "chk")
			if err != nil {
				t.Fatal(err)
			}
			if len(failed) != tc.failures || batch.Counts.Unusable != tc.failures || batch.Counts.New != len(parsed) {
				t.Fatalf("UI=%+v/%+v stage=%+v", parsed, failed, batch)
			}
			result, err := a.Commit(batch, "ui-path", "checkpoint")
			if err != nil {
				t.Fatal(err)
			}
			if result.Imported != len(parsed) || len(result.Failed) != tc.failures || len(result.Outcomes) != 1 {
				t.Fatalf("result=%+v", result)
			}
			for _, tx := range a.Transactions() {
				if tx.AccountID != "chk" {
					t.Fatalf("orphan transaction: %+v", tx)
				}
			}
			if a.Documents()[0].CheckpointID != "checkpoint" {
				t.Fatal("checkpoint was not saved with history")
			}
		})
	}
}

func TestIMAEverySourceCountsLedgerOccurrences(t *testing.T) {
	for _, source := range []string{"csv", "statement", "pdf", "receipt", "ai"} {
		t.Run(source, func(t *testing.T) {
			for _, ledgerCount := range []int{0, 1, 2, 3} {
				t.Run(string(rune('0'+ledgerCount)), func(t *testing.T) {
					a := newApp(t, false)
					seedAgreementAccount(t, a)
					for i := 0; i < ledgerCount; i++ {
						if err := a.Store().PutTransaction(domain.Transaction{ID: string(rune('a' + i)), AccountID: "chk", Date: onIMA(t, "2026-06-01"), Desc: "Metro", Amount: money.New(-250, "USD")}); err != nil {
							t.Fatal(err)
						}
					}
					var batch StagedImport
					var err error
					if source == "csv" {
						batch, err = a.StageCSV([]byte("date,desc,amount\n2026-06-01,Metro,-2.50\n2026-06-01,Metro,-2.50\n"), "chk")
					} else {
						rows := extract.EnsureRowIDs([]extract.Row{{Date: "2026-06-01", Description: "Metro", Amount: "-2.50"}, {Date: "2026-06-01", Description: "Metro", Amount: "-2.50"}})
						batch, err = a.Stage(domain.DocumentKind(source), rows, "chk")
					}
					if err != nil {
						t.Fatal(err)
					}
					wantDup := min(ledgerCount, 2)
					if batch.Counts.Duplicate != wantDup || batch.Counts.New != 2-wantDup || batch.Rows[0].ID == batch.Rows[1].ID || batch.Rows[1].Occurrence != 2 {
						t.Fatalf("stage=%+v", batch)
					}
					result, err := a.Commit(batch, "run", "")
					if err != nil {
						t.Fatal(err)
					}
					if result.Duplicates != wantDup || result.Imported != 2-wantDup || len(a.Transactions()) != ledgerCount+2-wantDup {
						t.Fatalf("result=%+v", result)
					}
					for i, outcome := range result.Outcomes {
						if outcome.ID != batch.Rows[i].ID {
							t.Fatal("row identity changed")
						}
					}
				})
			}
		})
	}
}

func TestIMADocumentRowsValidatedBeforeCommit(t *testing.T) {
	cases := []struct {
		name      string
		row       extract.Row
		want      int64
		errorText string
	}{
		{"symbol", extract.Row{Amount: "$45.00"}, 4500, ""},
		{"grouping", extract.Row{Amount: "1,234.56"}, 123456, ""},
		{"parentheses", extract.Row{Amount: "(45.00)"}, -4500, ""},
		{"unicode minus", extract.Row{Amount: "\u221245.00"}, -4500, ""},
		{"bad date", extract.Row{Date: "2026-02-30", Amount: "1"}, 0, "date"},
		{"ambiguous date", extract.Row{Date: "06/01/2026", Amount: "1"}, 0, "YYYY-MM-DD"},
		{"unknown account", extract.Row{AccountID: "missing", Amount: "1"}, 0, "account"},
		{"wrong currency", extract.Row{Currency: "EUR", Amount: "1"}, 0, "currency"},
		{"overflow", extract.Row{Amount: "92233720368547758.08"}, 0, "amount"},
		{"double sign overflow", extract.Row{Amount: "--92233720368547758.08"}, 0, "amount"},
		{"zero", extract.Row{Amount: "0"}, 0, "zero"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			seedAgreementAccount(t, a)
			row := tc.row
			row.ID = "stable"
			row.Description = "Coffee"
			if row.Date == "" {
				row.Date = "2026-06-01"
			}
			batch, err := a.Stage(domain.DocImage, []extract.Row{row}, "chk")
			if err != nil {
				t.Fatal(err)
			}
			staged := batch.Rows[0]
			if tc.errorText != "" {
				if staged.Verdict != staging.Unusable || !strings.Contains(staged.Reason, tc.errorText) {
					t.Fatalf("row=%+v", staged)
				}
			} else if !staged.Approved() || staged.AmountMinor != tc.want || staged.Currency != "USD" {
				t.Fatalf("row=%+v", staged)
			}
			result, err := a.Commit(batch, "validation", "")
			if err != nil {
				t.Fatal(err)
			}
			if tc.errorText != "" {
				if result.Imported != 0 || len(result.Failed) != 1 || result.Outcomes[0].ID != "stable" || result.Outcomes[0].Status != "failed" {
					t.Fatalf("result=%+v", result)
				}
			} else if result.Imported != 1 || a.Transactions()[0].Amount.Amount != tc.want {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestIMACategoryMatchingIsExact(t *testing.T) {
	a := newApp(t, false)
	for _, cat := range []domain.Category{{ID: "gas", Name: "Gas", Kind: domain.KindExpense}, {ID: "food", Name: "Food", Kind: domain.KindExpense}, {ID: "parent", Name: "Living", Kind: domain.KindExpense}} {
		if err := a.PutCategory(cat); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ input, want string }{{"Vegas trip", ""}, {"Gas & Electric utilities", ""}, {"  gAS  ", "gas"}, {"Food & Drink", ""}, {"Living", "parent"}, {"parent", "parent"}} {
		if got := a.categoryIDForDocumentRow(tc.input); got != tc.want {
			t.Errorf("%q = %q want %q", tc.input, got, tc.want)
		}
	}
}

func TestIMACommitRejectsStaleChangedOrForeignBatch(t *testing.T) {
	for _, change := range []string{"account", "ledger", "batch", "fallback", "foreign", "replay"} {
		t.Run(change, func(t *testing.T) {
			a := newApp(t, false)
			seedAgreementAccount(t, a)
			batch, err := a.StageCSV([]byte("date,desc,amount\n2026-06-01,Coffee,-4.50\n"), "chk")
			if err != nil {
				t.Fatal(err)
			}
			target := a
			switch change {
			case "account":
				ac := a.Accounts()[0]
				ac.Name = "Changed"
				if err := a.PutAccount(ac); err != nil {
					t.Fatal(err)
				}
			case "ledger":
				if err := a.Store().PutTransaction(domain.Transaction{ID: "new", AccountID: "chk", Date: onIMA(t, "2026-06-01"), Desc: "Other", Amount: money.New(-100, "USD")}); err != nil {
					t.Fatal(err)
				}
			case "batch":
				batch.Rows[0].Transaction.Amount.Amount = -999
			case "fallback":
				batch.AccountID = "other"
			case "foreign":
				target = newApp(t, false)
				seedAgreementAccount(t, target)
			case "replay":
				if _, err := a.Commit(batch, "first", ""); err != nil {
					t.Fatal(err)
				}
			}
			before := len(target.Transactions())
			if _, err := target.Commit(batch, "stale", ""); !errors.Is(err, ErrStaleImport) {
				t.Fatalf("error=%v, want stale preview", err)
			}
			if len(target.Transactions()) != before {
				t.Fatal("stale preview changed ledger")
			}
		})
	}
}

func TestIMAAtomicFailureReportsEveryRowAndKeepsHistory(t *testing.T) {
	a := newApp(t, false)
	seedAgreementAccount(t, a)
	prior := domain.Document{ID: "occupied", Kind: domain.DocCSV, CheckpointID: "original"}
	if err := a.PutDocument(prior); err != nil {
		t.Fatal(err)
	}
	batch, err := a.StageCSV([]byte("date,desc,amount\n2026-06-01,Coffee,-4.50\n2026-06-02,Tea,-2.50\n"), "chk")
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.Commit(batch, "occupied", "replacement")
	if err == nil || result.Imported != 0 || result.DocumentID != "" || len(result.Failed) != 2 || len(result.Outcomes) != 2 || len(a.Transactions()) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, outcome := range result.Outcomes {
		if outcome.Status != "failed" || outcome.TransactionID != "" {
			t.Fatalf("outcome=%+v", outcome)
		}
	}
	if !reflect.DeepEqual(a.Documents()[0], prior) {
		t.Fatal("history was overwritten")
	}
}

func TestIMACheckpointAndUnrelatedWritesDoNotInvalidatePreview(t *testing.T) {
	a := newApp(t, false)
	seedAgreementAccount(t, a)
	batch, err := a.StageCSV([]byte("date,desc,amount\n2026-06-01,Coffee,-4.50\n"), "chk")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetKV("checkpoint-metadata", "saved"); err != nil {
		t.Fatal(err)
	}
	result, err := a.Commit(batch, "run", "checkpoint")
	if err != nil || result.Imported != 1 || a.Documents()[0].CheckpointID != "checkpoint" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func onIMA(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := dateutil.ParseDate(value)
	if err != nil {
		t.Fatal(err)
	}
	return date
}

// Test adapters read the public staged model; production has no second preview.
func imaUsableTransactions(batch StagedImport) []domain.Transaction {
	var out []domain.Transaction
	for _, row := range batch.Rows {
		if row.Verdict != staging.Unusable {
			out = append(out, row.Transaction)
		}
	}
	return out
}

func imaUnusableRows(batch StagedImport) []staging.Row {
	var out []staging.Row
	for _, row := range batch.Rows {
		if row.Verdict == staging.Unusable {
			out = append(out, row)
		}
	}
	return out
}
func TestIMAImportSuccessWaitsForDurableCommit(t *testing.T) {
	for _, scenario := range []string{"unavailable", "delayed", "failed", "stale"} {
		t.Run(scenario, func(t *testing.T) {
			a := newApp(t, false)
			seedAgreementAccount(t, a)
			batch, err := a.StageCSV([]byte("date,desc,amount\n2026-06-01,Coffee,-4.50\n"), "chk")
			if err != nil {
				t.Fatal(err)
			}
			var complete func(DurableResult)
			var revision uint64
			if scenario != "unavailable" {
				a.SetDurableWriter(func(rev uint64, done func(DurableResult)) { revision = rev; complete = done })
			}
			calls := 0
			var got CSVImportResult
			var gotError error
			a.CommitDurable(batch, "run", "checkpoint", func(result CSVImportResult, err error) { calls++; got = result; gotError = err })
			if scenario == "unavailable" {
				if calls != 1 || !errors.Is(gotError, ErrDurabilityUnavailable) || len(a.Transactions()) != 0 {
					t.Fatalf("unavailable storage wrote or acknowledged: %+v %v", got, gotError)
				}
				return
			}
			if calls != 0 || complete == nil {
				t.Fatal("import acknowledged before durable completion")
			}
			result := DurableResult{Revision: revision}
			if scenario == "failed" {
				result.Err = errors.New("disk full")
			}
			if scenario == "stale" {
				result.Revision--
			}
			complete(result)
			complete(result)
			if calls != 1 || (gotError != nil) != (scenario != "delayed") {
				t.Fatalf("calls=%d result=%+v error=%v", calls, got, gotError)
			}
			if got.Imported != 1 || len(a.Transactions()) != 1 {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}
func TestIMAInvalidSourceRowsRemainEditable(t *testing.T) {
	a := newApp(t, false)
	seedAgreementAccount(t, a)
	batch, err := a.StageCSV([]byte("date,desc,amount,account\n2026-06-01,Coffee,broken,Checking\n"), "chk")
	if err != nil {
		t.Fatal(err)
	}
	row := batch.Rows[0]
	if row.ID == "" || row.Verdict != staging.Unusable || row.RawDate != "2026-06-01" || row.RawDescription != "Coffee" || row.RawAmount != "broken" || row.RawAccount != "Checking" {
		t.Fatalf("uneditable source row=%+v", row)
	}
	batch, err = a.Stage(domain.DocImage, []extract.Row{{ID: "draft", Date: "2026-02-30", Description: "Coffee", Amount: "-4.50"}}, "chk")
	if err != nil {
		t.Fatal(err)
	}
	row = batch.Rows[0]
	if row.ID != "draft" || row.RawDate != "2026-02-30" || row.RawAmount != "-4.50" || row.AccountID != "chk" || row.Currency != "USD" {
		t.Fatalf("uneditable draft row=%+v", row)
	}
}

func TestIMARepeatedCSVTransactionIDIsARowError(t *testing.T) {
	a := newApp(t, false)
	seedAgreementAccount(t, a)
	batch, err := a.StageCSV([]byte("id,date,desc,amount\nsame,2026-06-01,Coffee,-4.50\nsame,2026-06-02,Tea,-2.50\n"), "chk")
	if err != nil {
		t.Fatal(err)
	}
	if batch.Counts.New != 1 || batch.Counts.Unusable != 1 || batch.Rows[0].ID == batch.Rows[1].ID || !strings.Contains(batch.Rows[1].Reason, "ID") {
		t.Fatalf("batch=%+v", batch)
	}
	result, err := a.Commit(batch, "run", "")
	if err != nil || result.Imported != 1 || len(result.Failed) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestIMARuleHitsUseOriginalDescription(t *testing.T) {
	a := newApp(t, false)
	seedAgreementAccount(t, a)
	if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRule(rules.Rule{ID: "coffee", Match: "CARD COFFEE", RenameDesc: "Coffee", SetCategoryID: "food"}); err != nil {
		t.Fatal(err)
	}
	batch, err := a.StageCSV([]byte("date,desc,amount\n2026-06-01,CARD COFFEE,-4.50\n"), "chk")
	if err != nil {
		t.Fatal(err)
	}
	if batch.Rows[0].Transaction.Desc != "Coffee" || batch.Rows[0].RawDescription != "CARD COFFEE" || a.Rules()[0].HitCount != 0 {
		t.Fatal("preview should transform without crediting a rule hit")
	}
	result, err := a.Commit(batch, "run", "")
	if err != nil || result.Imported != 1 || a.Rules()[0].HitCount != 1 {
		t.Fatalf("result=%+v rule=%+v err=%v", result, a.Rules()[0], err)
	}
}
