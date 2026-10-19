// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestSOLP3LedgerIgnoresUnrelatedWritesAndPruning(t *testing.T) {
	a := newApp(t, false)
	if err := a.store.PutTransaction(domain.Transaction{ID: "one", AccountID: "a"}); err != nil {
		t.Fatal(err)
	}
	before := a.ledgerSnapshot()
	for _, test := range []struct {
		name  string
		write func() error
	}{
		{"audit", func() error { return a.store.PutAuditEntry(auditlog.Entry{ID: "audit"}) }},
		{"kv", func() error { return a.store.SetKV("ui", "on") }},
		{"settings", func() error { return a.store.SetSettingKV("theme", "dark") }},
		{"account", func() error { return a.store.PutAccount(domain.Account{ID: "a", Name: "Renamed"}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.write(); err != nil {
				t.Fatal(err)
			}
			cursor, err := a.store.ChangeCursor()
			if err != nil {
				t.Fatal(err)
			}
			if err := a.store.PruneChanges(cursor); err != nil {
				t.Fatal(err)
			}
			if got := a.ledgerSnapshot(); got != before {
				t.Fatal("unrelated write/pruning replaced snapshot")
			}
		})
	}
}

func TestSOLP3LedgerIncrementalMutationPaths(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*store.SQLiteStore) error
	}{
		{"edit", func(s *store.SQLiteStore) error {
			return s.PutTransaction(domain.Transaction{ID: "b", AccountID: "a", Desc: "edited", Tags: []string{"new"}})
		}},
		{"move account", func(s *store.SQLiteStore) error {
			return s.PutTransaction(domain.Transaction{ID: "b", AccountID: "new"})
		}},
		{"delete", func(s *store.SQLiteStore) error { _, err := s.DeleteTransaction("b"); return err }},
		{"insert sorted", func(s *store.SQLiteStore) error {
			return s.PutTransactionsAtomic([]domain.Transaction{{ID: "aa", AccountID: "a"}, {ID: "zz", AccountID: "new"}})
		}},
		{"import", func(s *store.SQLiteStore) error {
			return s.PutImportBatch([]domain.Transaction{{ID: "aa", AccountID: "new"}, {ID: "bb", AccountID: "a", Desc: "imported"}}, domain.Document{ID: "doc"})
		}},
		{"workspace replacement", func(s *store.SQLiteStore) error {
			return s.Load(store.Dataset{Transactions: []domain.Transaction{{ID: "b", AccountID: "other", Desc: "new workspace"}}})
		}},
		{"wipe", func(s *store.SQLiteStore) error { return s.Wipe() }},
		{"coalesced delete reinsert", func(s *store.SQLiteStore) error {
			if _, err := s.DeleteTransaction("b"); err != nil {
				return err
			}
			return s.PutTransaction(domain.Transaction{ID: "b", AccountID: "new"})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := newApp(t, false)
			if err := a.store.PutTransactionsAtomic([]domain.Transaction{{ID: "b", AccountID: "a", Tags: []string{"old"}}, {ID: "c", AccountID: "unchanged"}}); err != nil {
				t.Fatal(err)
			}
			before := a.ledgerSnapshot()
			encoded, _ := json.Marshal(before.rows)
			stable := before.byAccount["unchanged"]
			if err := test.mutate(a.store); err != nil {
				t.Fatal(err)
			}
			got := a.ledgerSnapshot()
			want, err := a.store.ListTransactions()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.rows, want) {
				t.Fatalf("rows differ from SQL: got=%+v want=%+v", got.rows, want)
			}
			old, _ := json.Marshal(before.rows)
			if string(old) != string(encoded) {
				t.Fatal("old rows mutated")
			}
			for i, row := range got.rows {
				if got.byID[row.ID] != i {
					t.Fatal("ID index stale")
				}
				indexed, ok := a.TransactionByID(row.ID)
				if !ok || !reflect.DeepEqual(indexed, row) {
					t.Fatal("ID lookup stale")
				}
			}
			for _, account := range []string{"a", "unchanged", "new", "other"} {
				var rows []domain.Transaction
				for _, row := range want {
					if row.AccountID == account {
						rows = append(rows, row)
					}
				}
				if !reflect.DeepEqual(got.byAccount[account], rows) {
					t.Fatalf("account %q stale", account)
				}
			}
			if rows := got.byAccount["unchanged"]; len(rows) > 0 && &rows[0] != &stable[0] {
				t.Fatal("unchanged account index copied")
			}
			if !reflect.DeepEqual(before.byAccount["a"][0].Tags, []string{"old"}) {
				t.Fatal("old account/nested value mutated")
			}
		})
	}
}

func TestSOLP3LedgerUndoPrunedFallbackAndStoreIdentity(t *testing.T) {
	a := newApp(t, false)
	original := domain.Transaction{ID: "one", AccountID: "a", Desc: "original"}
	if err := a.store.PutTransaction(original); err != nil {
		t.Fatal(err)
	}
	before := a.Transactions()
	from, err := a.store.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	changed := original
	changed.Desc = "changed"
	if err := a.store.PutTransaction(changed); err != nil {
		t.Fatal(err)
	}
	target, err := a.store.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	page, err := a.store.CaptureChanges(from, target, 10)
	if err != nil {
		t.Fatal(err)
	}
	if a.Transactions()[0].Desc != "changed" {
		t.Fatal("edit stale")
	}
	undo := page.Changes[0]
	undo.Before, undo.After = undo.After, undo.Before
	if err := a.store.ReplayChangesExpected([]store.RecordChange{undo}); err != nil {
		t.Fatal(err)
	}
	if a.Transactions()[0].Desc != "original" || before[0].Desc != "original" {
		t.Fatal("undo/old snapshot stale")
	}
	// Prune an unread edit: the next read must reload, not skip it.
	if err := a.store.PutTransaction(changed); err != nil {
		t.Fatal(err)
	}
	target, err = a.store.ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.store.PruneChanges(target); err != nil {
		t.Fatal(err)
	}
	if a.Transactions()[0].Desc != "changed" {
		t.Fatal("pruned history lost edit")
	}
	other := newApp(t, false)
	if err := other.store.PutTransaction(domain.Transaction{ID: "one", Desc: "different scope"}); err != nil {
		t.Fatal(err)
	}
	// Exercise a reused cache across store identities even at the same global Rev.
	got, err := a.reads.transactions.get(other.store)
	if err != nil {
		t.Fatal(err)
	}
	if got.rows[0].Desc != "different scope" {
		t.Fatal("store scopes mixed")
	}
}

func BenchmarkSOLP3IncrementalLedger3300(b *testing.B) {
	a, err := New(nil, false)
	if err != nil {
		b.Fatal(err)
	}
	defer a.Store().Close()
	rows := make([]domain.Transaction, 3300)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprintf("row-%04d", i), AccountID: "a"}
	}
	if err := a.store.PutTransactionsAtomic(rows); err != nil {
		b.Fatal(err)
	}
	a.Transactions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := rows[0]
		row.Desc = fmt.Sprint(i)
		if err := a.store.PutTransaction(row); err != nil {
			b.Fatal(err)
		}
		benchmarkTxnRows = a.Transactions()
	}
}

// TestSOLP3LedgerProfile3300 isolates the read-side update from rendering and
// durability. Elapsed timings are evidence; identity tests guard invalidation.
func TestSOLP3LedgerProfile3300(t *testing.T) {
	a := newApp(t, false)
	rows := make([]domain.Transaction, 3300)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprintf("row-%04d", i), AccountID: "checking"}
	}
	if err := a.store.PutTransactionsAtomic(rows); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	a.Transactions()
	cold := time.Since(start)
	row := rows[0]
	row.Desc = "changed"
	start = time.Now()
	if err := a.store.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	updated := a.ledgerSnapshot()
	incremental := time.Since(start)
	// Repeating the same SQL upsert has no captured table change.
	if err := a.store.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if a.ledgerSnapshot() != updated {
		t.Fatal("identical upsert invalidated snapshot")
	}
	original := rows[0]
	if err := a.store.PutTransaction(original); err != nil {
		t.Fatal(err)
	}
	if err := a.store.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if a.ledgerSnapshot() != updated {
		t.Fatal("net no-op invalidated snapshot")
	}
	start = time.Now()
	if err := a.store.SetKV("unrelated", "changed"); err != nil {
		t.Fatal(err)
	}
	if a.ledgerSnapshot() != updated {
		t.Fatal("KV invalidated snapshot")
	}
	unrelated := time.Since(start)
	t.Logf("SOLP3 ledger 3300 cold=%.3f ms single write+read=%.3f ms unrelated write+read=%.3f ms", float64(cold.Microseconds())/1000, float64(incremental.Microseconds())/1000, float64(unrelated.Microseconds())/1000)
}
