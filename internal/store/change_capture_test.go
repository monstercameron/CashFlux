// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func captureAll(t *testing.T, s *SQLiteStore, from, target uint64, size int) []RecordChange {
	t.Helper()
	var changes []RecordChange
	for from < target {
		page, err := s.CaptureChanges(from, target, size)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Changes) > size || page.Through <= from || page.Through > target {
			t.Fatalf("unbounded or stalled capture: %+v", page)
		}
		changes = append(changes, page.Changes...)
		from = page.Through
	}
	return changes
}

func cursor(t *testing.T, s *SQLiteStore) uint64 {
	t.Helper()
	n, err := s.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestChangeCaptureEveryMutationPathAndReplay(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		mutate func(*SQLiteStore) error
	}{
		{"crud", func(s *SQLiteStore) error {
			if err := s.PutTransaction(domain.Transaction{ID: "new"}); err != nil {
				return err
			}
			if err := s.PutTransaction(domain.Transaction{ID: "new", Desc: "edited"}); err != nil {
				return err
			}
			_, err := s.DeleteTransaction("new")
			return err
		}},
		{"batch", func(s *SQLiteStore) error {
			return s.PutTransactionsAtomic([]domain.Transaction{{ID: "one"}, {ID: "two"}})
		}},
		{"transfer receipt", func(s *SQLiteStore) error {
			_, err := s.PutTransferBatchAtomic("op", []domain.Transaction{{ID: "one"}, {ID: "two"}})
			return err
		}},
		{"import batch", func(s *SQLiteStore) error {
			return s.PutImportBatch([]domain.Transaction{{ID: "one"}}, domain.Document{ID: "doc"})
		}},
		{"kv", func(s *SQLiteStore) error {
			if err := s.SetKV("line", "quotes \" and\nnewlines"); err != nil {
				return err
			}
			if err := s.SetSettingKV("preference", "value"); err != nil {
				return err
			}
			return s.DeleteKV("line")
		}},
		{"settings", func(s *SQLiteStore) error { return s.PutSettings(Settings{BaseCurrency: "EUR"}) }},
		{"load", func(s *SQLiteStore) error { return s.Load(SampleDataset()) }},
		{"wipe", func(s *SQLiteStore) error { return s.Wipe() }},
		{"debt membership", func(s *SQLiteStore) error {
			return s.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "debt", TransactionIDs: []string{"t1", "t2"}})
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			initial := SampleDataset()
			if err := s.Load(initial); err != nil {
				t.Fatal(err)
			}
			from := cursor(t, s)
			if err := scenario.mutate(s); err != nil {
				t.Fatal(err)
			}
			target := cursor(t, s)
			changes := captureAll(t, s, from, target, 3)
			if len(changes) == 0 {
				t.Fatal("mutation escaped capture")
			}
			replay, err := NewMemory()
			if err != nil {
				t.Fatal(err)
			}
			defer replay.Close()
			if err := replay.Load(initial); err != nil {
				t.Fatal(err)
			}
			if err := replay.ReplayChanges(changes); err != nil {
				t.Fatal(err)
			}
			want, err := s.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			got, err := replay.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatal("replay does not match live dataset")
			}
			if scenario.name == "transfer receipt" {
				changed, err := replay.PutTransferBatchAtomic("op", []domain.Transaction{{ID: "one"}, {ID: "two"}})
				if err != nil || changed {
					t.Fatalf("receipt lost on replay: %v %v", changed, err)
				}
			}
		})
	}
}

func TestChangeCaptureBoundariesRollbackPruneAndRedaction(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutTransaction(domain.Transaction{ID: "a"}); err != nil {
		t.Fatal(err)
	}
	target := cursor(t, s)
	if err := s.PutTransaction(domain.Transaction{ID: "b"}); err != nil {
		t.Fatal(err)
	}
	page, err := s.CaptureChanges(0, target, 1)
	if err != nil || len(page.Changes) != 1 || page.Changes[0].ID != "a" {
		t.Fatalf("revision escaped: %+v %v", page, err)
	}
	beforeRollback := cursor(t, s)
	if err := s.PutImportBatch([]domain.Transaction{{ID: "new"}, {ID: "a"}}, domain.Document{}); err == nil {
		t.Fatal("expected duplicate rejection")
	}
	if cursor(t, s) != beforeRollback {
		t.Fatal("rollback leaked changes")
	}
	if err := s.PruneChanges(target); err != nil {
		t.Fatal(err)
	}
	if cursor(t, s) != beforeRollback {
		t.Fatal("pruning reset sequence")
	}
	if err := s.PutSettings(Settings{OpenAIKey: "device-secret", BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	changes := captureAll(t, s, beforeRollback, cursor(t, s), 8)
	var settings Settings
	if err := json.Unmarshal(changes[0].After, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.OpenAIKey != "" {
		t.Fatal("captured a device credential")
	}
	if err := s.ReplayChanges([]RecordChange{{Table: "transactions; DROP TABLE accounts", ID: "bad", After: json.RawMessage(`{}`)}}); err == nil {
		t.Fatal("accepted non-schema table")
	}
	if err := s.ReplayChanges([]RecordChange{{Table: "transactions", ID: "valid", After: json.RawMessage(`{}`)}, {Table: "invalid", ID: "bad"}}); err == nil {
		t.Fatal("accepted invalid replay")
	}
	if _, found, _ := s.GetTransaction("valid"); found {
		t.Fatal("partially applied invalid replay")
	}
}

func TestCapturedUndoRefusesInterveningEditAndKeepsDeviceCredential(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutTransaction(domain.Transaction{ID: "row", Desc: "before"}); err != nil {
		t.Fatal(err)
	}
	from := cursor(t, s)
	if err := s.PutTransaction(domain.Transaction{ID: "row", Desc: "after"}); err != nil {
		t.Fatal(err)
	}
	changes := captureAll(t, s, from, cursor(t, s), 4)
	undo := RecordChange{Table: changes[0].Table, ID: changes[0].ID, Before: changes[0].After, After: changes[0].Before}
	if err := s.PutTransaction(domain.Transaction{ID: "row", Desc: "intervening"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplayChangesExpected([]RecordChange{undo}); err == nil {
		t.Fatal("undo silently overwrote an intervening edit")
	}
	row, _, _ := s.GetTransaction("row")
	if row.Desc != "intervening" {
		t.Fatal("intervening edit lost")
	}
	if err := s.PutSettings(Settings{OpenAIKey: "device-credential", BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	from = cursor(t, s)
	if err := s.PutSettings(Settings{OpenAIKey: "device-credential", BaseCurrency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	changes = captureAll(t, s, from, cursor(t, s), 4)
	undo = RecordChange{Table: "settings", ID: "app", Before: changes[0].After, After: changes[0].Before}
	if err := s.ReplayChangesExpected([]RecordChange{undo}); err != nil {
		t.Fatal(err)
	}
	settings, err := s.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.OpenAIKey != "device-credential" || settings.BaseCurrency != "USD" {
		t.Fatalf("undo mixed device credentials with household settings: %+v", settings)
	}
}

func TestTransferReceiptsSurviveFullCheckpointAndDoNotCrossImports(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	txns := []domain.Transaction{{ID: "one"}, {ID: "two"}}
	if _, err := s.PutTransferBatchAtomic("operation", txns); err != nil {
		t.Fatal(err)
	}
	ds, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := Export(ds)
	if err != nil {
		t.Fatal(err)
	}
	ds, err = Import(data)
	if err != nil {
		t.Fatal(err)
	}
	rehydrated, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer rehydrated.Close()
	if err := rehydrated.Load(ds); err != nil {
		t.Fatal(err)
	}
	if changed, err := rehydrated.PutTransferBatchAtomic("operation", txns); err != nil || changed {
		t.Fatalf("receipt did not survive checkpoint: %v %v", changed, err)
	}
	if err := rehydrated.Load(EmptyDataset()); err != nil {
		t.Fatal(err)
	}
	if changed, err := rehydrated.PutTransferBatchAtomic("operation", txns); err != nil || !changed {
		t.Fatalf("receipt crossed a replacement import: %v %v", changed, err)
	}
}

func TestCaptureWatermarkAndBoundedTransactionPages(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"a", "b", "c"} {
		if err := s.PutTransaction(domain.Transaction{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	target := cursor(t, s)
	if err := s.PruneChanges(target); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CaptureChanges(0, target, 64); !errors.Is(err, ErrCaptureExpired) {
		t.Fatalf("expired history was silently skipped: %v", err)
	}
	if _, err := s.CaptureChanges(target, target+1, 64); err == nil {
		t.Fatal("accepted a future capture target")
	}
	var ids []string
	after := ""
	for {
		page, err := s.TransactionsPage(after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) > 2 {
			t.Fatal("unbounded page")
		}
		for _, txn := range page {
			ids = append(ids, txn.ID)
			after = txn.ID
		}
		if len(page) < 2 {
			break
		}
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "c"}) {
		t.Fatalf("keyset paging dropped ledger rows: %v", ids)
	}
	from := cursor(t, s)
	if err := s.SetKV("", "empty keys are valid KV state"); err != nil {
		t.Fatal(err)
	}
	changes := captureAll(t, s, from, cursor(t, s), 4)
	if err := s.ReplayChanges(changes); err != nil {
		t.Fatalf("KV mutation escaped replay: %v", err)
	}
}

func TestPruneFailureLeavesCaptureStateUsable(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.PutTransaction(domain.Transaction{ID: "before", Desc: "kept"}); err != nil {
		t.Fatal(err)
	}
	before := cursor(t, s)
	var journalBefore int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM store_change_journal").Scan(&journalBefore); err != nil {
		t.Fatal(err)
	}
	if err := s.PruneChanges(before + 1); err == nil {
		t.Fatal("pruning beyond the committed cursor succeeded")
	}
	var journalAfter int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM store_change_journal").Scan(&journalAfter); err != nil {
		t.Fatal(err)
	}
	if journalAfter != journalBefore {
		t.Fatalf("failed prune changed journal size: before=%d after=%d", journalBefore, journalAfter)
	}
	page, err := s.CaptureChanges(0, before, 8)
	if err != nil || len(page.Changes) != 1 || page.Changes[0].ID != "before" {
		t.Fatalf("failed prune damaged captured changes: %+v, %v", page, err)
	}

	if err := s.PutTransaction(domain.Transaction{ID: "after", Desc: "still writable"}); err != nil {
		t.Fatalf("write after failed prune: %v", err)
	}
	if got := cursor(t, s); got <= before {
		t.Fatalf("cursor did not advance after failed prune: before=%d after=%d", before, got)
	}
}

func TestSQLiteTemporaryStorageUsesMemory(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var tempStore int
	if err := s.db.QueryRow("PRAGMA temp_store").Scan(&tempStore); err != nil {
		t.Fatal(err)
	}
	if tempStore != 2 {
		t.Fatalf("PRAGMA temp_store = %d, want MEMORY (2)", tempStore)
	}
}

func TestCoalescedCaptureKeepsOnlyFinalChangedRecords(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutTransaction(domain.Transaction{ID: "row", Desc: "before"}); err != nil {
		t.Fatal(err)
	}
	from := cursor(t, s)
	if err := s.PutTransaction(domain.Transaction{ID: "row", Desc: "before"}); err != nil {
		t.Fatal(err)
	}
	if cursor(t, s) != from {
		t.Fatal("identical upsert created a persistence record")
	}
	for _, desc := range []string{"middle", "after"} {
		if err := s.PutTransaction(domain.Transaction{ID: "row", Desc: desc}); err != nil {
			t.Fatal(err)
		}
	}
	changes := CoalesceChanges(captureAll(t, s, from, cursor(t, s), 1))
	if len(changes) != 1 {
		t.Fatalf("captured %d versions of one record", len(changes))
	}
	var before, after domain.Transaction
	_ = json.Unmarshal(changes[0].Before, &before)
	_ = json.Unmarshal(changes[0].After, &after)
	if before.Desc != "before" || after.Desc != "after" {
		t.Fatal("coalescing lost the action's boundary images")
	}
	from = cursor(t, s)
	if err := s.PutTransaction(domain.Transaction{ID: "temporary"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteTransaction("temporary"); err != nil {
		t.Fatal(err)
	}
	if got := CoalesceChanges(captureAll(t, s, from, cursor(t, s), 1)); len(got) != 0 {
		t.Fatal("net no-op escaped coalescing")
	}
	txns := []domain.Transaction{{ID: "already-existing"}}
	if err := s.PutTransactionsAtomic(txns); err != nil {
		t.Fatal(err)
	}
	beforeRev := s.Rev()
	if changed, err := s.PutTransferBatchAtomic("new-receipt", txns); err != nil || changed {
		t.Fatalf("receipt adoption: %v %v", changed, err)
	}
	if s.Rev() <= beforeRev {
		t.Fatal("receipt-only mutation did not wake persistence")
	}
}
