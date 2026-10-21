// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nlauthor"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func TestWorkflowVariableNamesForMatchesActualTransactionContext(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	account := seedAccount(t, a, 0)
	for _, def := range []customfields.Def{
		{ID: "tip", EntityType: "transaction", Key: "tip", Label: "Tip", Type: customfields.TypeNumber},
		{ID: "reimbursable", EntityType: "transaction", Key: "reimbursable", Label: "Reimbursable", Type: customfields.TypeBool},
		{ID: "project", EntityType: "transaction", Key: "project", Label: "Project", Type: customfields.TypeText},
		{ID: "reviewed_on", EntityType: "transaction", Key: "reviewed_on", Label: "Reviewed on", Type: customfields.TypeDate},
		{ID: "team", EntityType: "transaction", Key: "team", Label: "Team", Type: customfields.TypeSelect, Options: []string{"A", "B"}},
	} {
		if err := a.PutCustomFieldDef(def); err != nil {
			t.Fatal(err)
		}
	}
	txn := domain.Transaction{ID: "first", AccountID: account.ID, Desc: "Market", Date: now, Amount: money.New(-12345, "USD"),
		Custom: map[string]any{"tip": float64(3), "reimbursable": true, "project": "Trip", "reviewed_on": "2026-10-10", "team": "A"}}
	if err := a.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	if err := a.PutTransaction(domain.Transaction{ID: "second", AccountID: account.ID, Desc: "Other", Date: now,
		Amount: money.New(-500, "USD"), Custom: map[string]any{"tip": float64(5)}}); err != nil {
		t.Fatal(err)
	}
	ctx := a.txnContext(txn)
	declared := a.WorkflowVariableNamesFor(workflow.TriggerTxnAdded)
	if !sort.StringsAreSorted(declared) || !reflect.DeepEqual(a.WorkflowVariableNames(), declared) {
		t.Fatal("trigger declaration/discovery union lost sorted compatibility")
	}
	for _, name := range declared {
		_, number := ctx.Vars[name]
		_, text := ctx.Strs[name]
		if !number && !text {
			t.Errorf("declared %s missing from the actual transaction runtime", name)
		}
	}
	for _, name := range workflow.TransactionVariableNames() {
		if !slices.Contains(declared, name) {
			t.Errorf("runtime transaction variable %s not discoverable", name)
		}
	}
	if ctx.Vars["txn_abs"] != 123.45 || ctx.Vars["txn_amount"] != -123.45 || ctx.Vars["cf_txn_tip"] != 3 ||
		ctx.Vars["cf_txn_reimbursable"] != 1 || ctx.Strs["cf_txn_project"] != "Trip" || ctx.Strs["cf_txn_reviewed_on"] != "2026-10-10" || ctx.Strs["cf_txn_team"] != "A" {
		t.Fatalf("runtime transaction fields/custom values disagree: %+v", ctx)
	}
	for _, trigger := range []workflow.TriggerKind{workflow.TriggerManual, workflow.TriggerScheduled, workflow.TriggerBudgetExceeded, workflow.TriggerGoalReached, workflow.TriggerBillDue} {
		names := a.WorkflowVariableNamesFor(trigger)
		for _, name := range workflow.TransactionVariableNames() {
			if slices.Contains(names, name) {
				t.Errorf("%s advertises transaction-only %s", trigger, name)
			}
		}
		for _, name := range []string{"cf_txn_reimbursable", "cf_txn_project", "cf_txn_reviewed_on", "cf_txn_team", "cf_txn_unknown"} {
			if slices.Contains(names, name) {
				t.Errorf("%s advertises unavailable per-row custom field %s", trigger, name)
			}
		}
		if !slices.Contains(names, "cf_txn_tip") || a.engineVars()["cf_txn_tip"] != 8 {
			t.Errorf("%s lost existing numeric aggregate custom-field semantics", trigger)
		}
	}
}

func TestWorkflowVariableNamesForCannotBeSpoofedByReservedMolecule(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	// Legacy/imported data can contain a name the transaction runner reserves.
	if err := a.Store().PutMolecule(domain.Molecule{Name: "txn_abs", Formula: "999"}); err != nil {
		t.Fatal(err)
	}
	if a.engineVars()["txn_abs"] != 999 {
		t.Fatal("fixture did not publish a colliding legacy molecule")
	}
	for _, trigger := range []workflow.TriggerKind{workflow.TriggerManual, workflow.TriggerScheduled, workflow.TriggerBudgetExceeded, workflow.TriggerGoalReached, workflow.TriggerBillDue} {
		names := a.WorkflowVariableNamesFor(trigger)
		if slices.Contains(names, "txn_abs") {
			t.Fatalf("%s accepted a reserved row variable via a molecule", trigger)
		}
	}
	knownNames := a.WorkflowVariableNamesFor(workflow.TriggerTxnAdded)
	_, errs := nlauthor.CompileWorkflow(nlauthor.WorkflowInput{ID: "large", Name: "Large charge", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "txn_abs > 500",
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Review"}}},
		func(name string) bool { return slices.Contains(knownNames, name) })
	if len(errs) > 0 {
		t.Fatalf("advertised transaction condition still fails canonical compilation: %v", errs)
	}
	ctx := a.txnContext(domain.Transaction{ID: "t", Amount: money.New(-12345, "USD")})
	if ctx.Vars["txn_abs"] != 123.45 {
		t.Fatal("reserved transaction context did not override colliding molecule")
	}
}

func TestTransactionContextDefinesOptionalStringsWhenLinksAreMissing(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	ctx := a.txnContext(domain.Transaction{ID: "unlinked", Amount: money.New(-500, "USD")})
	for _, name := range []string{"txn_account", "txn_category"} {
		value, exists := ctx.Strs[name]
		if !exists || value != "" {
			t.Fatalf("optional context field %s = %q, present=%v", name, value, exists)
		}
	}
	matched, err := workflow.Eval(`and(txn_account == "", txn_category == "")`, ctx)
	if err != nil || !matched {
		t.Fatalf("valid empty-link condition cannot run: matched=%v err=%v", matched, err)
	}
}
