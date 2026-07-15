// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/changeset"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func newCsApp(t *testing.T) *App {
	t.Helper()
	app, err := New(nil, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return app
}

func seedChecking(t *testing.T, app *App, id, name string) {
	t.Helper()
	ac := domain.Account{
		ID: id, Name: name, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}
	if err := app.PutAccount(ac); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
}

// TestApplyChangesetInOrder applies a multi-op changeset (create category, then
// two transactions that use it, categorize) and checks every op landed in order.
func TestApplyChangesetInOrder(t *testing.T) {
	app := newCsApp(t)
	seedChecking(t, app, "acc1", "Checking")

	cs := changeset.New("Set up groceries")
	cs.Add("create_category", "Create Groceries", json.RawMessage(`{"name":"Groceries"}`))
	cs.Add("add_transaction", "Record Trader Joe's", json.RawMessage(`{"amount":-40,"account":"Checking","payee":"Trader Joe's"}`))
	cs.Add("categorize_transactions", "Categorize Trader Joe's", json.RawMessage(`{"match":"Trader Joe","category":"Groceries"}`))

	rec := app.ApplyChangeset(*cs)
	if !rec.OK() {
		t.Fatalf("expected OK receipt, got failure: %+v", rec.Failed)
	}
	if rec.AppliedCount() != 3 {
		t.Fatalf("AppliedCount = %d, want 3", rec.AppliedCount())
	}
	if got := len(app.Categories()); got != 1 {
		t.Fatalf("categories = %d, want 1", got)
	}
	txns := app.Transactions()
	if len(txns) != 1 {
		t.Fatalf("transactions = %d, want 1", len(txns))
	}
	if txns[0].CategoryID == "" {
		t.Fatal("transaction was not categorized by the categorize op")
	}
}

// TestApplyChangesetStopsOnFailure verifies apply halts at the first failure and
// does not run later ops (no silent partial state), while keeping earlier ops.
func TestApplyChangesetStopsOnFailure(t *testing.T) {
	app := newCsApp(t)

	cs := changeset.New("mixed")
	cs.Add("create_category", "Create Fuel", json.RawMessage(`{"name":"Fuel"}`))
	cs.Add("add_transaction", "Record to a missing account", json.RawMessage(`{"amount":-10,"account":"Nope"}`))
	cs.Add("add_task", "Should never run", json.RawMessage(`{"title":"later"}`))

	rec := app.ApplyChangeset(*cs)
	if rec.OK() {
		t.Fatal("expected failure receipt")
	}
	if rec.Failed.Index != 1 {
		t.Fatalf("Failed.Index = %d, want 1", rec.Failed.Index)
	}
	if rec.AppliedCount() != 1 {
		t.Fatalf("AppliedCount = %d, want 1 (first op only)", rec.AppliedCount())
	}
	// The third op (add_task) must NOT have run.
	if len(app.Tasks()) != 0 {
		t.Fatalf("tasks = %d, want 0 — op after failure ran", len(app.Tasks()))
	}
}

// TestApplyChangesetSkipsDisabled verifies a disabled op is skipped.
func TestApplyChangesetSkipsDisabled(t *testing.T) {
	app := newCsApp(t)
	cs := changeset.New("partial")
	cs.Add("create_category", "Create A", json.RawMessage(`{"name":"A"}`))
	cs.Add("create_category", "Create B", json.RawMessage(`{"name":"B"}`))
	cs.SetEnabled(1, false)

	rec := app.ApplyChangeset(*cs)
	if !rec.OK() || rec.AppliedCount() != 1 {
		t.Fatalf("AppliedCount = %d, want 1; failed=%+v", rec.AppliedCount(), rec.Failed)
	}
	if len(app.Categories()) != 1 {
		t.Fatalf("categories = %d, want 1 (disabled op skipped)", len(app.Categories()))
	}
}

// TestApplyChangesetUnknownKind reports an unknown op kind as a failure.
func TestApplyChangesetUnknownKind(t *testing.T) {
	app := newCsApp(t)
	cs := changeset.New("bad")
	cs.Add("teleport_money", "Do the impossible", json.RawMessage(`{}`))
	rec := app.ApplyChangeset(*cs)
	if rec.OK() || rec.Failed.Kind != "teleport_money" {
		t.Fatalf("expected unknown-kind failure, got %+v", rec)
	}
}

// TestApplyGoalContribution exercises the goal dispatcher.
func TestApplyGoalContribution(t *testing.T) {
	app := newCsApp(t)
	g := domain.Goal{ID: "g1", Name: "Vacation", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TargetAmount: money.New(100000, "USD"), CurrentAmount: money.New(0, "USD")}
	if err := app.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}
	cs := changeset.New("fund it")
	cs.Add("add_goal_contribution", "Add $250 to Vacation", json.RawMessage(`{"goal":"Vacation","amount":250}`))
	rec := app.ApplyChangeset(*cs)
	if !rec.OK() {
		t.Fatalf("failed: %+v", rec.Failed)
	}
	var got domain.Goal
	for _, gg := range app.Goals() {
		if gg.ID == "g1" {
			got = gg
		}
	}
	if got.CurrentAmount.Amount != 25000 {
		t.Fatalf("goal current = %d, want 25000", got.CurrentAmount.Amount)
	}
}
