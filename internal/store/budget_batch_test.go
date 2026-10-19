// SPDX-License-Identifier: MIT

package store

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPutBudgetsAtomicWritesEveryRow(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()
	initial := []domain.Budget{
		batchBudget("one", 100),
		batchBudget("two", 200),
	}
	for _, budget := range initial {
		if err := st.PutBudget(budget); err != nil {
			t.Fatalf("PutBudget(%s): %v", budget.ID, err)
		}
	}
	rev := st.Rev()
	updates := []domain.Budget{batchBudget("one", 150), batchBudget("two", 250)}
	if err := st.PutBudgetsAtomic(updates); err != nil {
		t.Fatalf("PutBudgetsAtomic: %v", err)
	}
	for _, want := range updates {
		got, ok, err := st.GetBudget(want.ID)
		if err != nil || !ok {
			t.Fatalf("GetBudget(%s) = %v, %v", want.ID, ok, err)
		}
		if got.Limit.Amount != want.Limit.Amount {
			t.Errorf("budget %s limit = %d, want %d", got.ID, got.Limit.Amount, want.Limit.Amount)
		}
	}
	if got := st.Rev() - rev; got != uint64(len(updates)) {
		t.Errorf("mutation revision advanced by %d, want one per committed budget row (%d)", got, len(updates))
	}
}

func TestPutBudgetsAtomicSecondRowFailureRollsBack(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()
	for _, budget := range []domain.Budget{batchBudget("first", 100), batchBudget("second", 200)} {
		if err := st.PutBudget(budget); err != nil {
			t.Fatalf("PutBudget(%s): %v", budget.ID, err)
		}
	}
	if _, err := st.db.Exec(`CREATE TRIGGER fail_second_budget BEFORE UPDATE ON budgets
		WHEN NEW.id = 'second' BEGIN SELECT RAISE(ABORT, 'injected second-row failure'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	err = st.PutBudgetsAtomic([]domain.Budget{batchBudget("first", 999), batchBudget("second", 999)})
	if err == nil {
		t.Fatal("PutBudgetsAtomic succeeded, want injected failure")
	}
	for id, want := range map[string]int64{"first": 100, "second": 200} {
		got, ok, err := st.GetBudget(id)
		if err != nil || !ok {
			t.Fatalf("GetBudget(%s) = %v, %v", id, ok, err)
		}
		if got.Limit.Amount != want {
			t.Errorf("budget %s changed to %d after rollback, want %d", id, got.Limit.Amount, want)
		}
	}
}

func BenchmarkPutBudgetsAtomicWith3300Transactions(b *testing.B) {
	st, err := NewMemory()
	if err != nil {
		b.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()
	ds := EmptyDataset()
	ds.Transactions = make([]domain.Transaction, 3300)
	for i := range ds.Transactions {
		ds.Transactions[i].ID = fmt.Sprintf("txn-%04d", i)
	}
	ds.Budgets = []domain.Budget{batchBudget("one", 100), batchBudget("two", 200)}
	if err := st.Load(ds); err != nil {
		b.Fatalf("Load seed: %v", err)
	}
	updates := []domain.Budget{batchBudget("one", 101), batchBudget("two", 201)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updates[0].Limit.Amount++
		updates[1].Limit.Amount++
		if err := st.PutBudgetsAtomic(updates); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	var got int
	if err := st.db.QueryRow("SELECT count(*) FROM transactions").Scan(&got); err != nil || got != 3300 {
		b.Fatalf("transaction rows = %d, err=%v; want untouched 3300-row dataset", got, err)
	}
}

func batchBudget(id string, amount int64) domain.Budget {
	return domain.Budget{ID: id, Name: id, CategoryID: "category-" + id, Limit: money.Money{Amount: amount, Currency: "USD"}}
}
