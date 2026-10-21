// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func newWorkflowDuplicateTestApp(t *testing.T) *App {
	t.Helper()
	a, err := New(nil, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = a.store.Close() })
	return a
}

func testPYFWorkflow(id, from, to string, cadence domain.RecurringCadence, enabled bool) workflow.Workflow {
	return workflow.Workflow{
		ID:      id,
		Name:    "PYF " + id,
		Enabled: enabled,
		Trigger: workflow.Trigger{Kind: workflow.TriggerScheduled, Cadence: cadence},
		Actions: []workflow.Action{{
			Kind:                  workflow.ActionTransfer,
			TransferFromAccountID: from,
			TransferToAccountID:   to,
			TransferAmount:        100,
			DedupeKey:             "pyf:" + id + ":{period}",
		}},
	}
}

func TestPutWorkflowRejectsDisabledPYFDuplicateWithoutMutation(t *testing.T) {
	a := newWorkflowDuplicateTestApp(t)
	existing := testPYFWorkflow("existing", "checking", "savings", domain.CadenceMonthly, false)
	if err := a.PutWorkflow(existing); err != nil {
		t.Fatalf("PutWorkflow(existing): %v", err)
	}
	beforeRev := a.Rev()

	candidate := testPYFWorkflow("new", "checking", "savings", domain.CadenceMonthly, true)
	err := a.PutWorkflow(candidate)
	var duplicate *DuplicatePayYourselfFirstError
	if !errors.As(err, &duplicate) {
		t.Fatalf("PutWorkflow duplicate error = %v, want DuplicatePayYourselfFirstError", err)
	}
	if duplicate.ExistingEnabled {
		t.Fatal("duplicate should report that the existing workflow is disabled")
	}
	if got := a.Rev(); got != beforeRev {
		t.Fatalf("revision after rejected duplicate = %d, want unchanged %d", got, beforeRev)
	}
	stored, err := a.store.ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(stored) != 1 || stored[0].ID != existing.ID {
		t.Fatalf("persisted workflows after rejected duplicate = %#v, want only %q", stored, existing.ID)
	}
}

func TestPutWorkflowAllowsPYFUpdateButRejectsUpdateIntoAnotherIdentity(t *testing.T) {
	a := newWorkflowDuplicateTestApp(t)
	existing := testPYFWorkflow("existing", "checking", "savings", domain.CadenceMonthly, true)
	if err := a.PutWorkflow(existing); err != nil {
		t.Fatalf("PutWorkflow(existing): %v", err)
	}

	updatedSelf := existing
	updatedSelf.Name = "Renamed"
	updatedSelf.Actions = append([]workflow.Action(nil), existing.Actions...)
	updatedSelf.Actions[0].TransferAmount = 200
	if err := a.PutWorkflow(updatedSelf); err != nil {
		t.Fatalf("updating the same PYF ID: %v", err)
	}

	other := testPYFWorkflow("other", "checking", "emergency", domain.CadenceMonthly, true)
	if err := a.PutWorkflow(other); err != nil {
		t.Fatalf("PutWorkflow(other): %v", err)
	}
	updatedOther := other
	updatedOther.Actions = append([]workflow.Action(nil), other.Actions...)
	updatedOther.Actions[0].TransferToAccountID = "savings"
	beforeRev := a.Rev()
	err := a.PutWorkflow(updatedOther)
	var duplicate *DuplicatePayYourselfFirstError
	if !errors.As(err, &duplicate) || !duplicate.ExistingEnabled {
		t.Fatalf("update into another workflow's identity error = %v, want enabled duplicate error", err)
	}
	if got := a.Rev(); got != beforeRev {
		t.Fatalf("revision after rejected update = %d, want unchanged %d", got, beforeRev)
	}
	stored, err := a.store.ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("persisted workflow count after rejected update = %d, want 2", len(stored))
	}
	for _, w := range stored {
		if w.ID == other.ID && w.Actions[0].TransferToAccountID != "emergency" {
			t.Fatalf("rejected update changed persisted destination: %#v", w)
		}
	}
}

func TestPutWorkflowDoesNotClassifyOrdinaryTransferAsPYF(t *testing.T) {
	a := newWorkflowDuplicateTestApp(t)
	first := testPYFWorkflow("ordinary-one", "checking", "savings", domain.CadenceMonthly, true)
	first.Actions[0].DedupeKey = "wf:ordinary-one:{period}"
	second := testPYFWorkflow("ordinary-two", "checking", "savings", domain.CadenceMonthly, true)
	second.Actions[0].DedupeKey = "wf:ordinary-two:{period}"
	if err := a.PutWorkflow(first); err != nil {
		t.Fatalf("PutWorkflow(first): %v", err)
	}
	if err := a.PutWorkflow(second); err != nil {
		t.Fatalf("ordinary transfer was classified as PYF: %v", err)
	}
	if got, err := a.store.ListWorkflows(); err != nil || len(got) != 2 {
		t.Fatalf("ListWorkflows = %d, %v; want two ordinary transfers", len(got), err)
	}
}

func TestCreatePayYourselfFirstWorkflowPreservesTypedDuplicateError(t *testing.T) {
	a := newWorkflowDuplicateTestApp(t)
	if _, err := a.CreatePayYourselfFirstWorkflow("checking", "savings", 100, domain.CadenceMonthly); err != nil {
		t.Fatalf("first CreatePayYourselfFirstWorkflow: %v", err)
	}
	beforeRev := a.Rev()
	_, err := a.CreatePayYourselfFirstWorkflow("checking", "savings", 200, domain.CadenceMonthly)
	var duplicate *DuplicatePayYourselfFirstError
	if !errors.As(err, &duplicate) {
		t.Fatalf("duplicate create error = %v, want DuplicatePayYourselfFirstError", err)
	}
	if !duplicate.ExistingEnabled {
		t.Fatal("duplicate create should report enabled existing workflow")
	}
	if got := a.Rev(); got != beforeRev {
		t.Fatalf("revision after duplicate create = %d, want unchanged %d", got, beforeRev)
	}
	stored, err := a.store.ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("persisted workflows after duplicate create = %d, want 1", len(stored))
	}
}

func TestPutWorkflowPropagatesPYFDuplicateReadError(t *testing.T) {
	a := newWorkflowDuplicateTestApp(t)
	if err := a.store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	err := a.PutWorkflow(testPYFWorkflow("new", "checking", "savings", domain.CadenceMonthly, true))
	if err == nil || !strings.Contains(err.Error(), "check pay-yourself-first duplicates") {
		t.Fatalf("PutWorkflow error = %v, want propagated duplicate-scan read error", err)
	}
	var duplicate *DuplicatePayYourselfFirstError
	if errors.As(err, &duplicate) {
		t.Fatalf("read failure must not be reported as duplicate: %v", err)
	}
}
