package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestMemberCRUD(t *testing.T) {
	s := newStore(t)
	if err := s.PutMember(domain.Member{ID: "m1", Name: "Alice"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetMember("m1")
	if err != nil || !ok || got.Name != "Alice" {
		t.Fatalf("Get: %+v ok=%v err=%v", got, ok, err)
	}
	if list, _ := s.ListMembers(); len(list) != 1 {
		t.Errorf("list = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteMember("m1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetMember("m1"); ok {
		t.Error("member still present after delete")
	}
}

func TestCategoryCRUD(t *testing.T) {
	s := newStore(t)
	if err := s.PutCategory(domain.Category{ID: "c1", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetCategory("c1")
	if err != nil || !ok || got.Name != "Food" {
		t.Fatalf("Get: %+v ok=%v err=%v", got, ok, err)
	}
	if list, _ := s.ListCategories(); len(list) != 1 {
		t.Errorf("list = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteCategory("c1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetCategory("c1"); ok {
		t.Error("category still present after delete")
	}
}

func TestTransactionCRUD(t *testing.T) {
	s := newStore(t)
	if err := s.PutTransaction(domain.Transaction{ID: "t1", AccountID: "a1", Desc: "Coffee"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetTransaction("t1")
	if err != nil || !ok || got.Desc != "Coffee" {
		t.Fatalf("Get: %+v ok=%v err=%v", got, ok, err)
	}
	if list, _ := s.ListTransactions(); len(list) != 1 {
		t.Errorf("list = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteTransaction("t1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetTransaction("t1"); ok {
		t.Error("transaction still present after delete")
	}
}

func TestBudgetCRUD(t *testing.T) {
	s := newStore(t)
	if err := s.PutBudget(domain.Budget{ID: "b1", CategoryID: "food"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetBudget("b1")
	if err != nil || !ok || got.CategoryID != "food" {
		t.Fatalf("Get: %+v ok=%v err=%v", got, ok, err)
	}
	if list, _ := s.ListBudgets(); len(list) != 1 {
		t.Errorf("list = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteBudget("b1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetBudget("b1"); ok {
		t.Error("budget still present after delete")
	}
}

func TestGoalCRUD(t *testing.T) {
	s := newStore(t)
	if err := s.PutGoal(domain.Goal{ID: "g1", Name: "Emergency fund"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetGoal("g1")
	if err != nil || !ok || got.Name != "Emergency fund" {
		t.Fatalf("Get: %+v ok=%v err=%v", got, ok, err)
	}
	if list, _ := s.ListGoals(); len(list) != 1 {
		t.Errorf("list = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteGoal("g1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetGoal("g1"); ok {
		t.Error("goal still present after delete")
	}
}

func TestTaskCRUD(t *testing.T) {
	s := newStore(t)
	if err := s.PutTask(domain.Task{ID: "k1", Title: "Pay rent", Status: domain.StatusOpen}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetTask("k1")
	if err != nil || !ok || got.Title != "Pay rent" {
		t.Fatalf("Get: %+v ok=%v err=%v", got, ok, err)
	}
	if list, _ := s.ListTasks(); len(list) != 1 {
		t.Errorf("list = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteTask("k1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetTask("k1"); ok {
		t.Error("task still present after delete")
	}
}
