package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func newStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAccountCRUD(t *testing.T) {
	s := newStore(t)

	if err := s.PutAccount(domain.Account{ID: "a1", Name: "Checking", Currency: "USD"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetAccount("a1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Name != "Checking" {
		t.Errorf("name = %q, want Checking", got.Name)
	}

	// Update via Put (upsert).
	if err := s.PutAccount(domain.Account{ID: "a1", Name: "Renamed", Currency: "USD"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _, _ = s.GetAccount("a1")
	if got.Name != "Renamed" {
		t.Errorf("after update name = %q, want Renamed", got.Name)
	}

	list, _ := s.ListAccounts()
	if len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}

	deleted, err := s.DeleteAccount("a1")
	if err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetAccount("a1"); ok {
		t.Error("account still present after delete")
	}
}

func TestGetAndDeleteMissing(t *testing.T) {
	s := newStore(t)
	if _, ok, err := s.GetGoal("nope"); ok || err != nil {
		t.Errorf("missing get: ok=%v err=%v", ok, err)
	}
	if deleted, err := s.DeleteGoal("nope"); deleted || err != nil {
		t.Errorf("missing delete: deleted=%v err=%v", deleted, err)
	}
}

func TestPutRequiresID(t *testing.T) {
	s := newStore(t)
	if err := s.PutMember(domain.Member{Name: "noid"}); err == nil {
		t.Error("expected error putting entity without id")
	}
}

func TestTransactionQueries(t *testing.T) {
	s := newStore(t)
	usd := func(n int64) money.Money { return money.New(n, "USD") }
	mk := func(id, acc, cat, member, day string, amt int64) domain.Transaction {
		d, _ := dateutil.ParseDate(day)
		return domain.Transaction{ID: id, AccountID: acc, CategoryID: cat, MemberID: member, Date: d, Amount: usd(amt), Desc: id}
	}
	txns := []domain.Transaction{
		mk("t1", "a1", "food", "m1", "2026-06-03", -100),
		mk("t2", "a1", "rent", "m2", "2026-06-10", -200),
		mk("t3", "a2", "food", "m1", "2026-07-05", -300),
	}
	for _, tx := range txns {
		if err := s.PutTransaction(tx); err != nil {
			t.Fatalf("put: %v", err)
		}
	}

	if got, _ := s.TransactionsByAccount("a1"); len(got) != 2 {
		t.Errorf("by account a1 = %d, want 2", len(got))
	}
	if got, _ := s.TransactionsByCategory("food"); len(got) != 2 {
		t.Errorf("by category food = %d, want 2", len(got))
	}
	if got, _ := s.TransactionsByMember("m1"); len(got) != 2 {
		t.Errorf("by member m1 = %d, want 2", len(got))
	}

	start, end := dateutil.MonthRange(mustMonth("2026-06-15"))
	if got, _ := s.TransactionsByDateRange(start, end); len(got) != 2 {
		t.Errorf("by date range June = %d, want 2", len(got))
	}
}

func TestTasksByStatus(t *testing.T) {
	s := newStore(t)
	_ = s.PutTask(domain.Task{ID: "k1", Title: "a", Status: domain.StatusOpen})
	_ = s.PutTask(domain.Task{ID: "k2", Title: "b", Status: domain.StatusDone})
	_ = s.PutTask(domain.Task{ID: "k3", Title: "c", Status: domain.StatusOpen})

	if got, _ := s.TasksByStatus(domain.StatusOpen); len(got) != 2 {
		t.Errorf("open tasks = %d, want 2", len(got))
	}
	if got, _ := s.TasksByStatus(domain.StatusDone); len(got) != 1 {
		t.Errorf("done tasks = %d, want 1", len(got))
	}
}

func mustMonth(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}
