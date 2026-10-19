// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestResolveTasksForTxnSkipsEngineVarsWithoutOpenResolveTask(t *testing.T) {
	tests := []struct {
		name  string
		tasks []domain.Task
	}{
		{name: "no tasks"},
		{
			name: "done resolve task",
			tasks: []domain.Task{{
				ID: "done", Title: "Done", Status: domain.StatusDone,
				Priority: domain.PriorityMedium,
				Resolve:  &domain.TaskResolve{Condition: "safe_to_spend < 0"},
			}},
		},
		{
			name:  "open task without resolve rule",
			tasks: []domain.Task{{ID: "open", Title: "Open", Status: domain.StatusOpen, Priority: domain.PriorityMedium}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newApp(t, false)
			for _, task := range tc.tasks {
				if err := a.PutTask(task); err != nil {
					t.Fatalf("PutTask: %v", err)
				}
			}
			taskResolveVarsCache.Lock()
			taskResolveVarsCache.app = nil
			taskResolveVarsCache.vars = nil
			taskResolveVarsCache.Unlock()

			a.resolveTasksForTxn(domain.Transaction{})

			taskResolveVarsCache.Lock()
			defer taskResolveVarsCache.Unlock()
			if taskResolveVarsCache.app != nil || taskResolveVarsCache.vars != nil {
				t.Fatal("engine variables were built without an open task with a resolve rule")
			}
		})
	}
}

func TestTaskResolveVarsCacheIsScopedToAppRevision(t *testing.T) {
	a := newApp(t, false)
	b := newApp(t, false)

	first := a.taskResolveVars()
	second := a.taskResolveVars()
	if reflect.ValueOf(first).Pointer() != reflect.ValueOf(second).Pointer() {
		t.Fatal("same-revision call rebuilt the engine variables")
	}

	// A different App at the same revision must not inherit the first App's data.
	other := b.taskResolveVars()
	if reflect.ValueOf(first).Pointer() == reflect.ValueOf(other).Pointer() {
		t.Fatal("different App reused the prior App's variable map")
	}
	if err := a.PutTask(domain.Task{ID: "revision", Title: "Revision", Status: domain.StatusOpen, Priority: domain.PriorityMedium}); err != nil {
		t.Fatalf("PutTask: %v", err)
	}
	updated := a.taskResolveVars()
	if reflect.ValueOf(other).Pointer() == reflect.ValueOf(updated).Pointer() {
		t.Fatal("new store revision reused the cached variable map")
	}
}

func TestTaskResolveEventVarsAddsTxnFieldsWithoutChangingBase(t *testing.T) {
	base := map[string]float64{"household_total": 12, "txn_amount": 999}
	txn := domain.Transaction{
		Amount: money.Money{Amount: -1234, Currency: "USD"},
		Custom: map[string]any{"reimbursable": true, "project": "Kitchen"},
	}
	defs := []customfields.Def{
		{Key: "reimbursable", Type: customfields.TypeBool},
		{Key: "project", Type: customfields.TypeText},
	}
	got := taskResolveEventVars(base, txn, nil, defs)
	if got["household_total"] != 12 || got["txn_amount"] != -12.34 || got["txn_abs"] != 12.34 {
		t.Fatalf("event vars = %v", got)
	}
	if got["cf_txn_reimbursable"] != 1 {
		t.Errorf("boolean custom field = %v, want 1", got["cf_txn_reimbursable"])
	}
	if base["txn_amount"] != 999 {
		t.Errorf("base vars were mutated: %v", base)
	}
}
