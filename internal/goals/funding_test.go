// SPDX-License-Identifier: MIT

package goals

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func fundingGoal(id string, order, priority int) domain.Goal {
	return domain.Goal{ID: id, FundingOrder: order, Priority: priority}
}

func TestFundingOrdered(t *testing.T) {
	gs := []domain.Goal{
		fundingGoal("stored-first", 0, 0),  // unordered, unprioritized → last, by stored order
		fundingGoal("prio-high", 0, 1),     // unordered but high priority → after explicit
		fundingGoal("explicit-2", 2, 3),    // explicit order beats priority
		fundingGoal("explicit-1", 1, 0),    // explicit 1 → first
		fundingGoal("stored-second", 0, 0), // stable vs stored-first
	}
	got := FundingOrdered(gs)
	want := []string{"explicit-1", "explicit-2", "prio-high", "stored-first", "stored-second"}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("order[%d] = %s, want %s (full: %v)", i, got[i].ID, id, ids(got))
		}
	}
	if gs[0].ID != "stored-first" {
		t.Error("input slice mutated")
	}
}

func ids(gs []domain.Goal) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.ID
	}
	return out
}

func TestMoveFunding(t *testing.T) {
	gs := []domain.Goal{
		fundingGoal("a", 1, 0),
		fundingGoal("b", 2, 0),
		fundingGoal("c", 3, 0),
	}
	// Move b up: b, a, c → renumbered 1..3.
	plan, ok := MoveFunding(gs, "b", -1)
	if !ok {
		t.Fatal("move up failed")
	}
	if plan["b"] != 1 || plan["a"] != 2 || plan["c"] != 3 {
		t.Errorf("renumber = %v, want b1 a2 c3", plan)
	}
	// Move a (first) up: off the end → no-op.
	if _, ok := MoveFunding(gs, "a", -1); ok {
		t.Error("moving the first goal up must report ok=false")
	}
	// Move c (last) down: off the end → no-op.
	if _, ok := MoveFunding(gs, "c", 1); ok {
		t.Error("moving the last goal down must report ok=false")
	}
	// Unknown id.
	if _, ok := MoveFunding(gs, "nope", 1); ok {
		t.Error("unknown id must report ok=false")
	}
	// Unordered goals get a concrete position after any move.
	gs2 := []domain.Goal{fundingGoal("x", 0, 0), fundingGoal("y", 0, 0)}
	plan2, ok := MoveFunding(gs2, "y", -1)
	if !ok || plan2["y"] != 1 || plan2["x"] != 2 {
		t.Errorf("unordered move = %v ok=%t, want y1 x2 true", plan2, ok)
	}
}
