// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func bud2TestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "c-food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	prev := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = prev })
	return app
}

func TestBudgetStatusStripHasOneHeadlineFigure(t *testing.T) {
	app := bud2TestApp(t)
	if err := app.PutCategory(domain.Category{ID: "c-travel", Name: "Travel", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	if err := app.PutBudget(domain.Budget{
		ID: "b-food", Name: "Food", CategoryID: "c-food", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly,
		Limit: money.New(50000, "USD"),
	}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	if err := app.PutBudget(domain.Budget{
		ID: "b-travel", Name: "Travel", CategoryID: "c-travel", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodYearly,
		Limit: money.New(600000, "USD"),
	}); err != nil {
		t.Fatalf("PutBudget yearly: %v", err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(budgetSummaryWidget, budgetSummaryProps{App: app}))
	if strip := byTestID(f, "div", "budgets-status-strip"); strip == nil {
		t.Fatal("the status strip did not render")
	}
	figures := 0
	for _, node := range f.AllByTag("span") {
		if node.Attr("data-testid") == "budgets-status-figure" {
			figures++
		}
	}
	if figures != 1 {
		t.Fatalf("headline figures = %d, want one", figures)
	}
	if sentence := byTestID(f, "span", "budgets-status-sentence"); sentence == nil || !strings.Contains(sentence.Text(), "spent of $1,000.00 monthly plan") {
		t.Fatalf("the status strip does not show the normalized monthly plan in its headline: %v", sentence)
	}
	if meter := byTestID(f, "div", "budgets-status-meter"); meter == nil || meter.Attr("role") != "meter" {
		t.Fatalf("the status strip has no accessible spending meter: %v", meter)
	}
}

func TestBudgetAttentionCountAndFilterUseSameSet(t *testing.T) {
	statuses := []budgeting.Status{
		{State: budgeting.StateOver},
		{State: budgeting.StateNear},
		{State: budgeting.StateOK},
		{State: "healthy"},
	}
	count := len(budgetAttentionStatuses(statuses))
	filtered := make([]budgeting.Status, 0, len(statuses))
	for _, status := range statuses {
		if budgetNeedsAttention(status) {
			filtered = append(filtered, status)
		}
	}
	if count != len(filtered) || count != 2 {
		t.Fatalf("attention count = %d, filtered rows = %d; want both 2", count, len(filtered))
	}
}

func TestBudgetMemberLensAndMenuSplit(t *testing.T) {
	app := bud2TestApp(t)
	if err := app.PutMember(domain.Member{ID: "m-priya", Name: "Priya"}); err != nil {
		t.Fatalf("PutMember: %v", err)
	}
	for _, b := range []domain.Budget{
		{ID: "b-shared", Name: "Shared", CategoryID: "c-food", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
		{ID: "b-priya", Name: "Priya", CategoryID: "c-food", OwnerID: "m-priya", Scope: domain.ScopeIndividual, Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
	} {
		if err := app.PutBudget(b); err != nil {
			t.Fatalf("PutBudget(%s): %v", b.ID, err)
		}
	}
	uistate.SetActiveMember("m-priya")
	t.Cleanup(func() { uistate.SetActiveMember("") })
	f := render.New(t)
	f.Render(ui.CreateElement(bud2GlobalLensToolbarHarness, br2tbBudgetListHarnessProps{App: app}))
	if got := byTestID(f, "select", "budgets-member-lens"); got != nil {
		t.Fatal("duplicate list-level member selector rendered")
	}
	if header := byTestID(f, "p", "budgets-member-scope"); header == nil ||
		!strings.HasPrefix(header.Text(), "Showing Priya's budgets:") ||
		!strings.Contains(header.Text(), "amounts are household totals") {
		if header == nil {
			t.Fatal("global-lens header did not render")
		}
		t.Fatalf("global-lens header = %q", header.Text())
	}
	if method := byTestID(f, "select", "budgets-method"); method == nil {
		t.Fatal("Settings menu does not contain the household budgeting method")
	}
	if settingsMenu := byTestID(f, "div", "budgets-settings-menu"); settingsMenu == nil || settingsMenu.Attr("role") != "menu" {
		t.Fatal("Settings menu was not rendered as its own menu")
	}
	if tools := byTestID(f, "button", "budgets-tools"); tools == nil || tools.Text() != uistate.T("bud2.tools") {
		t.Fatalf("Tools menu trigger = %v", tools)
	}
	if toolsMenu := byTestID(f, "div", "budgets-tools-menu"); toolsMenu == nil || toolsMenu.Attr("role") != "menu" {
		t.Fatal("Tools menu was not rendered separately from Settings")
	}
	if adjust := byTestID(f, "button", "budgets-adjust-all"); adjust == nil || adjust.Text() != "Adjust every budget" {
		t.Fatalf("Tools menu does not expose the plain-language adjust-all action: %v", adjust)
	}
}

func bud2GlobalLensToolbarHarness(props br2tbBudgetListHarnessProps) ui.Node {
	scopeAtom := uistate.UseActiveScope()
	scopeAtom.Set(scope.ReportScope{Owners: []string{"m-priya"}})
	return Fragment(
		ui.CreateElement(budgetListWidget, budgetListProps{App: props.App}),
		ui.CreateElement(budgetToolbarContent, budgetToolbarProps{App: props.App}),
	)
}
