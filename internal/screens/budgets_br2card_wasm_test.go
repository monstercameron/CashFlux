// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalfunded"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBR2CardFiguresPrecedeBarAndPaceLegend(t *testing.T) {
	p := budgetRowCardProps{
		s:        budgeting.Status{Budget: domain.Budget{ID: "br2-layout", Name: "Groceries"}},
		props:    budgetRowProps{Status: budgeting.Status{Budget: domain.Budget{ID: "br2-layout", Name: "Groceries"}}},
		barSpent: "$242.50", limit: money.New(20520, "USD"), barPct: 118, width: 100,
		limitEditing: ui.State[bool]{},
	}
	f := render.New(t)
	f.Render(ui.CreateElement(br2FiguresHarness, p))

	var classes []string
	var visit func(*render.QueryNode)
	visit = func(node *render.QueryNode) {
		if node == nil {
			return
		}
		classes = append(classes, node.Attr("class"))
		for _, child := range node.Children() {
			visit(child)
		}
	}
	visit(f.Container())
	figures, bar, legend := -1, -1, -1
	for i, class := range classes {
		switch class {
		case "budget-card-loader-figs":
			figures = i
		case "budget-card-loader":
			bar = i
		case "budget-pace-legend":
			legend = i
		}
	}
	if figures < 0 || bar < 0 || legend < 0 || !(figures < bar && bar < legend) {
		t.Fatalf("DOM order figures=%d bar=%d pace legend=%d; want figures, bar, legend", figures, bar, legend)
	}
}

func br2FiguresHarness(p budgetRowCardProps) ui.Node {
	p.limitEditing = ui.UseState(false)
	p.limitDraft = ui.UseState("")
	return shorthand.Fragment(budgetRowFigures(p), shorthand.Span(css.Class("budget-pace-legend"), "Today's pace: 58% of the month"))
}

func TestBR2CardGoalFundingNamesGoalAndReconcilesLimit(t *testing.T) {
	app := bud2TestApp(t)
	if err := app.PutGoal(domain.Goal{ID: "goal-couch", Name: "New couch", CategoryID: "c-food", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		TargetAmount: money.New(50000, "USD"), CurrentAmount: money.New(50000, "USD")}); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}
	anchor := time.Now()
	if err := app.PutTransaction(domain.Transaction{
		ID: "txn-couch", AccountID: "account", Desc: "Couch", Date: anchor, CategoryID: "c-food", Amount: money.New(-42000, "USD"), GoalID: "goal-couch",
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	b := domain.Budget{ID: "br2-goal", Name: "Shopping", CategoryID: "c-food", Period: domain.PeriodMonthly, Limit: money.New(20000, "USD"), Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID}
	if err := app.PutBudget(b); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	line, chip := budgetRowGoalFunding(false, budgeting.Status{Budget: b, Limit: b.Limit}, budgetRowProps{Anchor: anchor}, int(app.Rev()))
	f := render.New(t)
	f.Render(line)
	got := byTestID(f, "span", "budget-goalfunded-br2-goal")
	if got == nil {
		t.Fatal("goal-funded note did not render")
	}
	want := "$80.00 available from New couch"
	if got.Text() != want {
		t.Fatalf("goal note = %q, want %q", got.Text(), want)
	}
	if chip != want {
		t.Fatalf("compact marker name = %q, want the same explanatory sentence", chip)
	}
	if got.Text() == "" || got.Text() == uistate.T("budgets.goalFunded") {
		t.Fatalf("goal note fell back to the old vague copy: %q", got.Text())
	}
}

func TestBR2CardExpensiveHelpersRunOncePerBudgetRevisionAndPeriod(t *testing.T) {
	app := bud2TestApp(t)
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var goalCalls, followUpCalls int
	for range 3 {
		memoGoalFunded(app, 44001, "br2-counter", start, end, func() (goalfunded.Split, error) {
			goalCalls++
			return goalfunded.Split{FundedMinor: 1}, nil
		})
		memoBudgetFollowUps(app, app.Rev(), "br2-counter", func() []followUpItem {
			followUpCalls++
			return []followUpItem{{ID: "one"}}
		})
	}
	if goalCalls != 1 || followUpCalls != 1 {
		t.Fatalf("same-key render work: goal=%d follow-ups=%d, want one calculation each", goalCalls, followUpCalls)
	}
	if err := app.PutCategory(domain.Category{ID: "br2-revision", Name: "Revision", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	newRevision := int(app.Rev())
	memoGoalFunded(app, newRevision, "br2-counter", start, end, func() (goalfunded.Split, error) {
		goalCalls++
		return goalfunded.Split{}, nil
	})
	memoBudgetFollowUps(app, app.Rev(), "br2-counter", func() []followUpItem {
		followUpCalls++
		return nil
	})
	if goalCalls != 2 || followUpCalls != 2 {
		t.Fatalf("new-revision render work: goal=%d follow-ups=%d, want one refresh each", goalCalls, followUpCalls)
	}
}

func TestBR2CardCopyCatalogIncludesNamedGoalVariants(t *testing.T) {
	for _, key := range []string{"budgets.goalFundedAvailableOne", "budgets.goalFundedAvailableMany", "budgets.goalFundedUnnamed"} {
		if got := uistate.T(key); got == key || got == "" {
			t.Errorf("copy key %q is missing", key)
		}
	}
}

func TestBR2CardFollowUpButtonOwnsAccessibleName(t *testing.T) {
	app := bud2TestApp(t)
	b := domain.Budget{ID: "br2-followups", Name: "Shopping"}
	for _, task := range []domain.Task{
		{ID: "br2-open", Title: "Review receipt", Status: domain.StatusOpen, Priority: domain.PriorityMedium, RelatedType: domain.RelatedBudget, RelatedID: b.ID},
		{ID: "br2-done", Title: "Check monthly plan", Status: domain.StatusDone, Priority: domain.PriorityMedium, RelatedType: domain.RelatedBudget, RelatedID: b.ID},
	} {
		if err := app.PutTask(task); err != nil {
			t.Fatalf("PutTask(%s): %v", task.ID, err)
		}
	}
	f := render.New(t)
	f.Render(ui.CreateElement(br2FollowUpHarness, budgeting.Status{Budget: b}))
	button := byTestID(f, "button", "budget-todos-toggle-"+b.ID)
	if button == nil {
		t.Fatal("follow-ups button did not render")
	}
	want := uistate.T("budgets.followUpsHead", 1, 2)
	if got := button.Attr("aria-label"); got != want {
		t.Fatalf("button accessible name = %q, want %q", got, want)
	}
	for _, child := range f.AllByTag("span") {
		if child.Attr("class") == "budget-followup-count" && child.Attr("aria-label") != "" {
			t.Fatalf("accessible name belongs on the button, child has %q", child.Attr("aria-label"))
		}
	}
}

func br2FollowUpHarness(status budgeting.Status) ui.Node {
	open := ui.UseState(false)
	panel, _ := budgetRowFollowUps(budgetRowFollowUpsProps{s: status, todosOpen: open})
	return panel
}

var _ = ui.Node(nil)
