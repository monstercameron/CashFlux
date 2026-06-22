package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func newAllocApp(t *testing.T) *App {
	t.Helper()
	app, err := New(nil, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return app
}

// allocSeedGoal adds a goal to the store and returns it.
func allocSeedGoal(t *testing.T, app *App, id, name string, target, current int64) domain.Goal {
	t.Helper()
	g := domain.Goal{
		ID:            id,
		Name:          name,
		Scope:         domain.ScopeShared,
		OwnerID:       domain.GroupOwnerID,
		TargetAmount:  money.New(target, "USD"),
		CurrentAmount: money.New(current, "USD"),
	}
	if err := app.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}
	return g
}

// allocSeedAccount adds an account to the store and returns it.
// For liability accounts, pass domain.TypeCreditCard as accType; for assets, pass domain.TypeChecking.
func allocSeedAccount(t *testing.T, app *App, id, name string, accType domain.AccountType) domain.Account {
	t.Helper()
	ac := domain.Account{
		ID:       id,
		Name:     name,
		Scope:    domain.ScopeShared,
		OwnerID:  domain.GroupOwnerID,
		Class:    accType.Class(),
		Type:     accType,
		Currency: "USD",
	}
	if err := app.PutAccount(ac); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	return ac
}

func TestApplyAllocationGoalContribution(t *testing.T) {
	app := newAllocApp(t)
	undoSnapshot = nil

	allocSeedGoal(t, app, "g1", "Vacation", 200000, 50000) // target 2000, current 500 USD

	actions := []allocate.Action{
		{Kind: allocate.GoalContribution, DestinationID: "g1", DestinationName: "Vacation", Amount: 30000},
	}
	result, err := app.ApplyAllocation(actions)
	if err != nil {
		t.Fatalf("ApplyAllocation: %v", err)
	}
	if result.GoalsFunded != 1 {
		t.Errorf("GoalsFunded = %d, want 1", result.GoalsFunded)
	}
	if result.GoalDollars != 30000 {
		t.Errorf("GoalDollars = %d, want 30000", result.GoalDollars)
	}
	if result.Overflow != 0 {
		t.Errorf("Overflow = %d, want 0", result.Overflow)
	}

	goals := app.Goals()
	var g domain.Goal
	for _, x := range goals {
		if x.ID == "g1" {
			g = x
		}
	}
	if g.CurrentAmount.Amount != 80000 {
		t.Errorf("CurrentAmount = %d, want 80000", g.CurrentAmount.Amount)
	}
}

func TestApplyAllocationGoalCapsAtTarget(t *testing.T) {
	app := newAllocApp(t)
	undoSnapshot = nil

	// target=100000 current=90000 → headroom = 10000
	allocSeedGoal(t, app, "g2", "Car", 100000, 90000)

	actions := []allocate.Action{
		// Send 15000, but only 10000 headroom → 5000 overflow
		{Kind: allocate.GoalContribution, DestinationID: "g2", DestinationName: "Car", Amount: 15000},
	}
	result, err := app.ApplyAllocation(actions)
	if err != nil {
		t.Fatalf("ApplyAllocation: %v", err)
	}
	if result.Overflow != 5000 {
		t.Errorf("Overflow = %d, want 5000", result.Overflow)
	}
	if result.GoalDollars != 10000 {
		t.Errorf("GoalDollars = %d, want 10000 (capped)", result.GoalDollars)
	}
	for _, g := range app.Goals() {
		if g.ID == "g2" && g.CurrentAmount.Amount != 100000 {
			t.Errorf("CurrentAmount = %d, want 100000 (capped at target)", g.CurrentAmount.Amount)
		}
	}
}

func TestApplyAllocationEarmark(t *testing.T) {
	app := newAllocApp(t)
	undoSnapshot = nil

	allocSeedAccount(t, app, "acc1", "Savings", domain.TypeChecking)

	actions := []allocate.Action{
		{Kind: allocate.AccountEarmark, DestinationID: "acc1", DestinationName: "Savings", Amount: 50000},
	}
	result, err := app.ApplyAllocation(actions)
	if err != nil {
		t.Fatalf("ApplyAllocation: %v", err)
	}
	if result.EarmarksMade != 1 {
		t.Errorf("EarmarksMade = %d, want 1", result.EarmarksMade)
	}
	if result.EarmarkDollars != 50000 {
		t.Errorf("EarmarkDollars = %d, want 50000", result.EarmarkDollars)
	}
	earmarks := app.Earmarks()
	if len(earmarks) != 1 {
		t.Fatalf("Earmarks count = %d, want 1", len(earmarks))
	}
	if earmarks[0].DestinationID != "acc1" {
		t.Errorf("DestinationID = %q, want acc1", earmarks[0].DestinationID)
	}
	if earmarks[0].DestinationKind != domain.EarmarkKindAccount {
		t.Errorf("DestinationKind = %q, want account", earmarks[0].DestinationKind)
	}
}

func TestApplyAllocationDebtEarmark(t *testing.T) {
	app := newAllocApp(t)
	undoSnapshot = nil

	allocSeedAccount(t, app, "visa", "Visa Card", domain.TypeCreditCard)

	actions := []allocate.Action{
		{Kind: allocate.DebtPaydownEarmark, DestinationID: "visa", DestinationName: "Visa Card", Amount: 15000},
	}
	result, err := app.ApplyAllocation(actions)
	if err != nil {
		t.Fatalf("ApplyAllocation: %v", err)
	}
	earmarks := app.Earmarks()
	if len(earmarks) != 1 {
		t.Fatalf("Earmarks count = %d, want 1", len(earmarks))
	}
	if earmarks[0].DestinationKind != domain.EarmarkKindDebt {
		t.Errorf("DestinationKind = %q, want debt", earmarks[0].DestinationKind)
	}
	if result.EarmarkDollars != 15000 {
		t.Errorf("EarmarkDollars = %d, want 15000", result.EarmarkDollars)
	}
}

func TestApplyAllocationUndo(t *testing.T) {
	app := newAllocApp(t)
	undoSnapshot = nil

	allocSeedGoal(t, app, "g3", "Fund", 100000, 0)

	actions := []allocate.Action{
		{Kind: allocate.GoalContribution, DestinationID: "g3", DestinationName: "Fund", Amount: 5000},
	}
	if _, err := app.ApplyAllocation(actions); err != nil {
		t.Fatalf("ApplyAllocation: %v", err)
	}

	// Goal should be bumped.
	for _, g := range app.Goals() {
		if g.ID == "g3" && g.CurrentAmount.Amount != 5000 {
			t.Errorf("pre-undo CurrentAmount = %d, want 5000", g.CurrentAmount.Amount)
		}
	}

	if err := app.UndoLastAllocation(); err != nil {
		t.Fatalf("UndoLastAllocation: %v", err)
	}

	// Goal should be back to 0.
	for _, g := range app.Goals() {
		if g.ID == "g3" && g.CurrentAmount.Amount != 0 {
			t.Errorf("post-undo CurrentAmount = %d, want 0", g.CurrentAmount.Amount)
		}
	}
	// Earmarks should be gone too (they were not created, but confirm list empty).
	if len(app.Earmarks()) != 0 {
		t.Errorf("post-undo earmarks = %d, want 0", len(app.Earmarks()))
	}
}

func TestUndoWithNoSnapshotErrors(t *testing.T) {
	app := newAllocApp(t)
	undoSnapshot = nil

	if err := app.UndoLastAllocation(); err == nil {
		t.Error("expected error when no snapshot to undo")
	}
}

func TestApplyAllocationMissingGoalErrors(t *testing.T) {
	app := newAllocApp(t)
	undoSnapshot = nil

	actions := []allocate.Action{
		{Kind: allocate.GoalContribution, DestinationID: "nonexistent", DestinationName: "Ghost", Amount: 100},
	}
	_, err := app.ApplyAllocation(actions)
	if err == nil {
		t.Error("expected error for missing goal")
	}
	// Rollback should leave store clean — no earmarks leaked.
	if len(app.Earmarks()) != 0 {
		t.Errorf("post-rollback earmarks = %d, want 0", len(app.Earmarks()))
	}
}
