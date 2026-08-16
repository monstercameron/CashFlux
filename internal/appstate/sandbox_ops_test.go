// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func sandboxApp(t *testing.T) *App {
	t.Helper()
	a := newTestAppAt(t, time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC))
	putAccount(t, a, "a1", "Checking")
	return a
}

func TestASandboxStartsAsACopyOfTheRealThing(t *testing.T) {
	a := sandboxApp(t)
	sb, err := a.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	if len(sb.App.Accounts()) != len(a.Accounts()) {
		t.Fatalf("sandbox has %d accounts, real has %d", len(sb.App.Accounts()), len(a.Accounts()))
	}
	// Nothing has changed yet, so nothing should show up as changed.
	if got := sb.Diff(0.5); len(got) != 0 {
		t.Fatalf("an untouched sandbox reported %d changes: %+v", len(got), got)
	}
}

func TestAChangeInTheSandboxNeverReachesTheRealData(t *testing.T) {
	// This is the guarantee the whole feature rests on. If it fails, the assistant
	// is editing the household's money while claiming to be exploring.
	a := sandboxApp(t)
	sb, err := a.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	before := len(a.Transactions())

	if err := sb.App.PutTransaction(domain.Transaction{
		ID: "hypothetical", AccountID: "a1", Payee: "New flat", Desc: "Rent",
		Amount: money.New(-180000, "USD"), Date: time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("sandbox write: %v", err)
	}
	if len(sb.App.Transactions()) != 1 {
		t.Fatalf("the sandbox did not take the write: %d", len(sb.App.Transactions()))
	}
	if got := len(a.Transactions()); got != before {
		t.Fatalf("the real dataset has %d transactions, had %d — a sandbox write escaped", got, before)
	}
	for _, tx := range a.Transactions() {
		if tx.ID == "hypothetical" {
			t.Fatal("the hypothetical transaction is in the real ledger")
		}
	}
}

func TestTheDiffReportsWhatTheScenarioMoved(t *testing.T) {
	a := sandboxApp(t)
	sb, err := a.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	if err := sb.App.PutTransaction(domain.Transaction{
		ID: "rent", AccountID: "a1", Payee: "New flat", Desc: "Rent",
		Amount: money.New(-180000, "USD"), Date: time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("sandbox write: %v", err)
	}
	changes := sb.Diff(1)
	if len(changes) == 0 {
		t.Fatal("an £1,800 expense moved nothing in the diff")
	}
	// Largest movement first: the point of the diff is which figure this actually
	// hits, and a list sorted by anything else buries it.
	for i := 1; i < len(changes); i++ {
		if abs(changes[i-1].Delta) < abs(changes[i].Delta) {
			t.Fatalf("changes are not ordered by size: %v then %v", changes[i-1], changes[i])
		}
	}
}

func TestTheBaselineIsFrozenWhenTheSandboxIsTaken(t *testing.T) {
	// An ordinary edit made while somebody explores a scenario must not show up as
	// an effect OF that scenario — the most misleading thing this could do.
	a := sandboxApp(t)
	sb, err := a.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	// The REAL dataset moves after the sandbox was taken.
	if err := a.PutTransaction(domain.Transaction{
		ID: "real", AccountID: "a1", Payee: "Groceries", Desc: "Shopping",
		Amount: money.New(-5000, "USD"), Date: time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("real write: %v", err)
	}
	// The sandbox itself was not touched, so it must still report no change.
	if got := sb.Diff(0.5); len(got) != 0 {
		t.Fatalf("a real edit leaked into the scenario's diff: %+v", got)
	}
}

func TestASandboxCannotNotifyTheHousehold(t *testing.T) {
	a := sandboxApp(t)
	a.Notifier = func(string) { t.Fatal("the sandbox surfaced a notice about hypothetical money") }
	sb, err := a.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	if sb.App.Notifier != nil {
		t.Fatal("the sandbox kept a notifier pointing at the real UI")
	}
}

func TestANilAppSandboxesSafely(t *testing.T) {
	var a *App
	if _, err := a.NewSandbox(); err == nil {
		t.Fatal("sandboxing nothing reported success")
	}
	var sb *Sandbox
	if got := sb.Diff(1); got != nil {
		t.Fatalf("diffing a nil sandbox returned %v", got)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestASandboxWriteNeverFiresAWorkflow(t *testing.T) {
	// The hole that made "nothing is saved" untrue: PutTransaction fires
	// transaction-added workflows, and one of the actions a workflow can take
	// reaches a package-level runner wired to the REAL app — with the real key and
	// the real spend meter. Sixty synthetic transactions would have meant sixty
	// live model calls, paid for, from a question the tool promised was discarded.
	a := sandboxApp(t)
	if err := a.PutWorkflow(workflow.Workflow{
		ID: "w1", Name: "on every transaction", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded},
		Actions: []workflow.Action{{Kind: workflow.ActionAgentRun, Prompt: "summarise"}},
	}); err != nil {
		t.Fatalf("PutWorkflow: %v", err)
	}
	prior := AgentRunner
	fired := 0
	AgentRunner = func(string, func(string)) { fired++ }
	defer func() { AgentRunner = prior }()

	sb, err := a.NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := sb.App.PutTransaction(domain.Transaction{
			ID: fmt.Sprintf("hypo-%d", i), AccountID: "a1", Payee: "New flat", Desc: "Rent",
			Amount: money.New(-180000, "USD"), Date: time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("sandbox write: %v", err)
		}
	}
	if fired != 0 {
		t.Fatalf("a hypothetical fired %d real workflow run(s) — each one a paid model call", fired)
	}
	// The workflow itself must still exist in the sandbox; it is suspended, not
	// deleted, so a scenario over a household WITH automations is still their data.
	if len(sb.App.Workflows()) != 1 {
		t.Fatalf("the sandbox lost the household's workflows: %d", len(sb.App.Workflows()))
	}
}
