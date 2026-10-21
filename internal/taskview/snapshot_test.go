// SPDX-License-Identifier: MIT

package taskview

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasklink"
)

func TestSnapshotCountsChildrenAndOpenLinksIgnoreViewFilters(t *testing.T) {
	tasks := []domain.Task{
		{ID: "parent", Status: domain.StatusDone},
		{ID: "child", ParentID: "parent", Status: domain.StatusOpen, Due: day("2026-10-20"), RelatedType: domain.RelatedAccount, RelatedID: "a"},
		{ID: "done", ParentID: "parent", Status: domain.StatusDone, RelatedType: domain.RelatedGoal, RelatedID: "g"},
		{ID: "late", ParentID: "missing", Status: domain.StatusOpen, Due: day("2026-10-19")},
	}
	before := append([]domain.Task(nil), tasks...)
	got := BuildSnapshot(SnapshotInput{Tasks: tasks, TodayISO: "2026-10-20"})
	if got.Counts != (Counts{Open: 2, Done: 2, Total: 4, Today: 1, Overdue: 1, DonePct: 50}) {
		t.Fatalf("counts %+v", got.Counts)
	}
	if got.Children["parent"].Total != 2 || got.Children["parent"].Done != 1 || got.Children["missing"].Total != 1 {
		t.Fatalf("child counts %+v", got.Children)
	}
	if !got.OpenLinks[Link{domain.RelatedAccount, "a"}] || got.OpenLinks[Link{domain.RelatedGoal, "g"}] {
		t.Fatalf("open-link index %+v", got.OpenLinks)
	}
	if !reflect.DeepEqual(tasks, before) {
		t.Fatal("snapshot changed input tasks")
	}
	copy := got.ByID["child"]
	copy.Title = "Copy changed"
	got.ByID["child"] = copy
	if tasks[1].Title != "" {
		t.Fatal("snapshot task value aliases the input struct")
	}
}

func TestSnapshotDisplayLookupsMatchOriginalSearchAndRetainOnlyReferences(t *testing.T) {
	in := SnapshotInput{
		Tasks: []domain.Task{
			{ID: "same", Title: "first", RelatedType: domain.RelatedAccount, RelatedID: "a", MemberID: "m"},
			{ID: "same", Title: "last", RelatedType: domain.RelatedBudget, RelatedID: "b"},
			{RelatedType: domain.RelatedGoal, RelatedID: "g"},
			{RelatedType: domain.RelatedTransaction, RelatedID: "t"},
			{RelatedType: domain.RelatedTransaction, RelatedID: "desc"},
			{RelatedType: domain.RelatedTransaction, RelatedID: "missing"},
			{RelatedType: domain.RelatedReviewQueue, RelatedID: "inbox"},
		},
		Accounts:     []domain.Account{{ID: "a", Name: "first account"}, {ID: "a", Name: "second account"}, {ID: "unreferenced", Name: "Other"}},
		Budgets:      []domain.Budget{{ID: "b", Name: ""}, {ID: "b", Name: "Later"}},
		Goals:        []domain.Goal{{ID: "g", Name: "Goal"}},
		Transactions: []domain.Transaction{{ID: "t", Payee: "Merchant", Desc: "Description"}, {ID: "t", Payee: "Later"}, {ID: "desc", Desc: "Description"}, {ID: "unreferenced", Payee: "Unused"}},
		Members:      []domain.Member{{ID: "m", Name: "First member"}, {ID: "m", Name: "Later"}, {ID: "unused", Name: "Unused"}},
	}
	snapshot := BuildSnapshot(in)
	for _, task := range in.Tasks {
		name, ok := snapshot.EntityName(task.RelatedType, task.RelatedID)
		want, wantOK := tasklink.EntityName(task.RelatedType, task.RelatedID, in.Accounts, in.Budgets, in.Goals, in.Transactions)
		if name != want || ok != wantOK {
			t.Fatalf("%s/%s lookup=(%q,%t), want (%q,%t)", task.RelatedType, task.RelatedID, name, ok, want, wantOK)
		}
	}
	if snapshot.ByID["same"].Title != "last" || snapshot.MemberNames["m"] != "First member" {
		t.Fatal("duplicate lookup semantics changed")
	}
	if len(snapshot.EntityNames) != 5 || len(snapshot.MemberNames) != 1 {
		t.Fatalf("unreferenced entities retained: %d entities, %d members", len(snapshot.EntityNames), len(snapshot.MemberNames))
	}
}

func BenchmarkSnapshotEntityLookup(b *testing.B) {
	in := SnapshotInput{Transactions: make([]domain.Transaction, 10000), Tasks: make([]domain.Task, 100)}
	for i := range in.Transactions {
		in.Transactions[i] = domain.Transaction{ID: fmt.Sprint(i), Payee: "Merchant"}
	}
	for i := range in.Tasks {
		in.Tasks[i] = domain.Task{ID: fmt.Sprint(i), RelatedType: domain.RelatedTransaction, RelatedID: fmt.Sprint(9900 + i)}
	}
	b.Run("prepared", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			snapshot := BuildSnapshot(in)
			for _, task := range in.Tasks {
				_, _ = snapshot.EntityName(task.RelatedType, task.RelatedID)
			}
		}
	})
	b.Run("previous-per-row-search", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, task := range in.Tasks {
				_, _ = tasklink.EntityName(task.RelatedType, task.RelatedID, nil, nil, nil, in.Transactions)
			}
		}
	})
}
