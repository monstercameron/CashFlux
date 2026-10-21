// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestPrimaryGoalExplicitNameSortSpansDefaultSections(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	for _, g := range []domain.Goal{
		{ID: "last", Name: "Zed overdue", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("USD"), TargetAmount: money.New(10000, "USD"), TargetDate: time.Now().AddDate(0, 0, -1)},
		{ID: "first", Name: "Baby fund", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("USD"), TargetAmount: money.New(10000, "USD")},
		{ID: "middle", Name: "Emergency fund", IsSinkingFund: true, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.Zero("USD"), TargetAmount: money.New(10000, "USD")},
	} {
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	defer f.Cleanup()
	// Render the real host and surfaceTiles boundary. A test parent subscribing
	// to UseGoalSort would itself force both sibling tiles to render and could
	// conceal a missed descendant subscription in the production hierarchy.
	f.Render(ui.CreateElement(Goals))
	f.Stabilize()
	byTestID(f, "button", "goals-tab-goals").Click()
	f.Stabilize()
	picker := byTestID(f, "select", "goals-sort")
	if picker == nil {
		t.Fatal("missing goal sort picker")
	}
	picker.Change(uistate.GoalSortActionable)
	f.Stabilize()
	if byTestID(f, "section", "goals-needsplan-section") == nil && byTestID(f, "div", "goals-needsplan-section") == nil {
		t.Fatal("default sort lost the needs-a-plan section")
	}
	picker = byTestID(f, "select", "goals-sort")
	if picker == nil {
		t.Fatal("missing goal sort picker")
	}
	picker.Change(uistate.GoalSortName)
	f.Stabilize()
	text := f.Text()
	first, middle, last := strings.Index(text, "Baby fund"), strings.Index(text, "Emergency fund"), strings.Index(text, "Zed overdue")
	if first < 0 || middle <= first || last <= middle {
		t.Fatalf("explicit name sort still pinned the overdue section first: %s", text)
	}
	if byTestID(f, "section", "goals-needsplan-section") != nil || byTestID(f, "div", "goals-needsplan-section") != nil {
		t.Fatal("explicit sort retained a separately ordered lead section")
	}
	byTestID(f, "select", "goals-sort").Change(uistate.GoalSortActionable)
	f.Stabilize()
	if byTestID(f, "section", "goals-needsplan-section") == nil && byTestID(f, "div", "goals-needsplan-section") == nil {
		t.Fatal("switching back did not restore default sections")
	}
}
