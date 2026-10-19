// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBR2TBUsesGlobalMemberLensAndCountsRenderedBudgetRows(t *testing.T) {
	app := bud2TestApp(t)
	if err := app.PutMember(domain.Member{ID: "m-priya", Name: "Priya"}); err != nil {
		t.Fatalf("PutMember: %v", err)
	}
	for i := 0; i < 7; i++ {
		if err := app.PutBudget(domain.Budget{
			ID: "shared-" + string(rune('a'+i)), Name: "Shared", CategoryID: "c-food",
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Period: domain.PeriodMonthly,
			Limit: money.New(10000, "USD"),
		}); err != nil {
			t.Fatalf("PutBudget shared %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := app.PutBudget(domain.Budget{
			ID: "other-" + string(rune('a'+i)), Name: "Other", CategoryID: "c-food",
			OwnerID: "m-other", Scope: domain.ScopeIndividual, Period: domain.PeriodMonthly,
			Limit: money.New(10000, "USD"),
		}); err != nil {
			t.Fatalf("PutBudget individual %d: %v", i, err)
		}
	}
	f := render.New(t)
	f.Render(ui.CreateElement(br2tbBudgetListHarness, br2tbBudgetListHarnessProps{App: app}))

	if got := byTestID(f, "select", "budgets-member-lens"); got != nil {
		t.Fatal("duplicate list-level member selector rendered")
	}
	if got := byTestID(f, "p", "budgets-member-scope"); got == nil || got.Text() != "Showing Priya's budgets: 7 of 9 include Priya; amounts are household totals" {
		t.Fatalf("member scope note = %v", got)
	}
	rows := f.AllByTag("div")
	rendered := 0
	for _, row := range rows {
		if row.Attr("data-testid") != "" && len(row.Attr("data-testid")) >= len("budget-card-") && row.Attr("data-testid")[:len("budget-card-")] == "budget-card-" {
			rendered++
		}
	}
	if rendered != 7 {
		t.Fatalf("rendered budget rows = %d, want 7", rendered)
	}
	settings := byTestID(f, "button", "budgets-settings")
	if settings == nil || !strings.HasPrefix(settings.Attr("aria-label"), "Settings") {
		t.Fatalf("Settings accessible name = %v, want it to start with visible text", settings)
	}
	adjust := byTestID(f, "button", "budgets-adjust-all")
	if adjust == nil || adjust.Text() != "Adjust every budget" {
		t.Fatalf("Tools adjust action = %v, want the dialog title", adjust)
	}
	monthClose := byTestID(f, "button", "budgets-month-close")
	if monthClose == nil || strings.Contains(monthClose.Text(), "✓") {
		t.Fatalf("Review the month action should not look completed: %v", monthClose)
	}
}

func TestBV6BMobileToolbarPrecedesFirstBudgetInNaturalDocumentFlow(t *testing.T) {
	app := bud2TestApp(t)
	if err := app.PutBudget(domain.Budget{
		ID: "mobile-flow", Name: "Groceries", CategoryID: "c-food",
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Period: domain.PeriodMonthly,
		Limit: money.New(45000, "USD"),
	}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(br2tbBudgetListHarness, br2tbBudgetListHarnessProps{App: app}))

	add := byTestID(f, "button", "budgets-add")
	if add == nil {
		t.Fatal("Add budget is missing from the list toolbar")
	}
	settings := byTestID(f, "button", "budgets-settings")
	tools := byTestID(f, "button", "budgets-tools")
	density := byTestID(f, "button", "budgets-density")
	if settings == nil || tools == nil || density == nil {
		t.Fatalf("toolbar controls missing: settings=%v tools=%v density=%v", settings, tools, density)
	}

	order := make([]string, 0, 8)
	var visit func(*render.QueryNode)
	visit = func(node *render.QueryNode) {
		if node == nil {
			return
		}
		if strings.Contains(node.Attr("class"), "budgets-tb") {
			order = append(order, "toolbar")
		}
		if strings.HasPrefix(node.Attr("data-testid"), "budget-card-") {
			order = append(order, "first budget")
		}
		for _, child := range node.Children() {
			visit(child)
		}
	}
	visit(f.Container())
	if len(order) < 2 || order[0] != "toolbar" || order[1] != "first budget" {
		t.Fatalf("toolbar and first budget document order = %v, want toolbar before first budget", order)
	}
}

type br2tbBudgetListHarnessProps struct{ App *appstate.App }

func br2tbBudgetListHarness(props br2tbBudgetListHarnessProps) ui.Node {
	scopeAtom := uistate.UseActiveScope()
	scopeAtom.Set(scope.ReportScope{Owners: []string{"m-priya"}})
	return ui.CreateElement(budgetListWidget, budgetListProps{App: props.App})
}
