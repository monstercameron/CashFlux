// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func workflowHostApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	for _, account := range []domain.Account{
		{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(100_000, "USD")},
		{ID: "savings", Name: "Savings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: money.Zero("USD")},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatalf("PutAccount(%s): %v", account.ID, err)
		}
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() {
		appstate.Default = previous
		_ = app.Store().Close()
	})
	return app
}

func workflowHostButton(f *render.Fixture, label string) *render.QueryNode {
	for _, node := range f.AllByTag("button") {
		if strings.TrimSpace(node.Text()) == label {
			return node
		}
	}
	return nil
}

func workflowHostTestID(f *render.Fixture, tag, testID string) *render.QueryNode {
	for _, node := range f.AllByTag(tag) {
		if node.Attr("data-testid") == testID {
			return node
		}
	}
	return nil
}

func TestWorkflowComposerScheduledTransferUsesPreparedScheduleAndDedupe(t *testing.T) {
	app := workflowHostApp(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Workflows))
	f.Stabilize()
	before := time.Now()

	name := workflowHostTestID(f, "input", "wf-draft-name")
	trigger := workflowHostTestID(f, "select", "wf-draft-trigger")
	if name == nil || trigger == nil {
		t.Fatal("workflow composer name or trigger control is missing")
	}
	name.Input("Weekly savings move")
	trigger.Change(string(workflow.TriggerScheduled))
	f.Stabilize()
	if cadence := workflowHostTestID(f, "select", "wf-draft-cadence"); cadence == nil {
		t.Fatal("scheduled workflow cadence control is missing")
	} else {
		cadence.Change(string(domain.CadenceWeekly))
	}
	action := workflowHostTestID(f, "select", "wf-draft-action")
	if action == nil {
		t.Fatal("workflow action type control is missing")
	}
	action.Change(string(workflow.ActionTransfer))
	f.Stabilize()
	from := workflowHostTestID(f, "select", "wf-draft-transfer-from")
	to := workflowHostTestID(f, "select", "wf-draft-transfer-to")
	amount := workflowHostTestID(f, "input", "wf-draft-transfer-amount")
	if from == nil || to == nil || amount == nil {
		t.Fatal("scheduled transfer controls are missing")
	}
	from.Change("checking")
	to.Change("savings")
	amount.Input("25.00")
	save := workflowHostButton(f, "Save workflow")
	if save == nil {
		t.Fatal("workflow save button is missing")
	}
	save.Click()
	f.Stabilize()

	workflows := app.Workflows()
	if len(workflows) != 1 {
		t.Fatalf("saved workflows = %d, want 1; screen: %s", len(workflows), f.Text())
	}
	saved := workflows[0]
	if saved.Trigger.Kind != workflow.TriggerScheduled || saved.Trigger.Cadence != domain.CadenceWeekly {
		t.Fatalf("saved trigger = %+v", saved.Trigger)
	}
	if !saved.Trigger.NextRun.After(before) || saved.Trigger.NextRun.After(before.Add(8*24*time.Hour)) {
		t.Fatalf("scheduled NextRun was not prepared from the current clock: %v (before %v)", saved.Trigger.NextRun, before)
	}
	if len(saved.Actions) != 1 || saved.Actions[0].Kind != workflow.ActionTransfer {
		t.Fatalf("saved actions = %+v", saved.Actions)
	}
	if want := "wf:" + saved.ID + ":{period}"; saved.Actions[0].DedupeKey != want {
		t.Fatalf("transfer dedupe key = %q, want %q", saved.Actions[0].DedupeKey, want)
	}
}

func TestWorkflowComposerSameAccountTransferKeepsNeedsValueMessage(t *testing.T) {
	app := workflowHostApp(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Workflows))
	f.Stabilize()
	name := workflowHostTestID(f, "input", "wf-draft-name")
	action := workflowHostTestID(f, "select", "wf-draft-action")
	if name == nil || action == nil {
		t.Fatal("workflow composer controls are missing")
	}
	name.Input("Transfer to itself")
	action.Change(string(workflow.ActionTransfer))
	f.Stabilize()
	from := workflowHostTestID(f, "select", "wf-draft-transfer-from")
	to := workflowHostTestID(f, "select", "wf-draft-transfer-to")
	amount := workflowHostTestID(f, "input", "wf-draft-transfer-amount")
	if from == nil || to == nil || amount == nil {
		t.Fatal("transfer draft controls are missing")
	}
	from.Change("checking")
	to.Change("checking")
	amount.Input("25.00")
	add := workflowHostButton(f, "Add action")
	if add == nil {
		t.Fatal("add action button is missing")
	}
	add.Click()
	f.Stabilize()
	if !strings.Contains(f.Text(), "Fill in the action's value before adding it.") {
		t.Fatalf("same-account transfer changed the existing draft error: %s", f.Text())
	}
	if len(app.Workflows()) != 0 {
		t.Fatal("same-account action unexpectedly created a workflow")
	}
}
