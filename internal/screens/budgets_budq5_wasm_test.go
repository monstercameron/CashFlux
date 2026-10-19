// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalfunded"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBUDQ5InlineLimitWriteErrorKeepsDraftAndNotifies(t *testing.T) {
	app := guardTestApp(t)
	b := quickFillFixtureBudget()
	if err := app.PutBudget(b); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	f := render.New(t)
	f.Render(Fragment(ui.CreateElement(budq5Notice), ui.CreateElement(BudgetRow, budgetRowProps{Status: budgeting.Status{
		Budget: b, Limit: b.Limit, Spent: money.Zero(b.Limit.Currency), Remaining: b.Limit,
	}})))
	byTestID(f, "button", "budget-limit-btn-"+b.ID).Click()
	f.Stabilize()
	byTestID(f, "input", "budget-limit-input-"+b.ID).Input("175.25")
	f.Stabilize()
	var form *render.QueryNode
	for _, node := range f.AllByTag("form") {
		if strings.Contains(node.Attr("class"), "budget-limit-editform") {
			form = node
		}
	}
	if form == nil {
		t.Fatal("inline editor form did not render")
	}
	form.Submit()
	f.Stabilize()
	if field := byTestID(f, "input", "budget-limit-input-"+b.ID); field == nil || field.Attr("data-cf-value") != "175.25" {
		t.Fatalf("failed write discarded the draft: field=%v", field)
	}
	notice := byTestID(f, "div", "budget-write-notice")
	if notice == nil || notice.Text() == "" || notice.Attr("data-error") != strconv.FormatBool(true) {
		t.Fatalf("failed limit write did not post an error notice: %v", notice)
	}
	if got := app.Budgets()[0].Limit.Amount; got != b.Limit.Amount {
		t.Fatalf("rejected write changed stored limit to %d", got)
	}
}

func budq5Notice() ui.Node {
	notice := uistate.UseNotice()
	return Div(Attr("data-testid", "budget-write-notice"), Attr("data-error", strconv.FormatBool(notice.Get().Err)), notice.Get().Text)
}

func TestBUDQ5BudgetPeriodKeyStartUsesPayCycleAnchor(t *testing.T) {
	anchor := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)
	viewed := time.Date(2026, time.October, 10, 12, 0, 0, 0, time.UTC)
	got := budgetPeriodKeyStart(domain.PeriodBiweekly, viewed, anchor)
	want := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("anchored boost period start = %s, want %s", got, want)
	}
}

func TestBUDQ5GoalFundedMemoUsesRevisionAndViewedPeriod(t *testing.T) {
	app := guardTestApp(t)
	start := time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 14)
	calls := 0
	calculate := func() goalfunded.Split {
		calls++
		return goalfunded.Split{FundedMinor: int64(calls)}
	}
	first := memoGoalFunded(app, 71, "budq5-goal", start, end, calculate)
	second := memoGoalFunded(app, 71, "budq5-goal", start, end, calculate)
	if calls != 1 || first.FundedMinor != second.FundedMinor {
		t.Fatalf("same revision/period was recomputed: calls=%d, values=%+v/%+v", calls, first, second)
	}
	_ = memoGoalFunded(app, 72, "budq5-goal", start, end, calculate)
	if calls != 2 {
		t.Fatalf("new data revision reused stale result: calls=%d", calls)
	}
}

func TestBUDQ5RolloverTooltipShowsChainEvidence(t *testing.T) {
	app := guardTestApp(t)
	b := quickFillFixtureBudget()
	b.Rollover = true
	if err := app.PutBudget(b); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	text := budgetRolloverChainText(b, time.Now(), 811)
	for _, want := range []string{"base", "carry-in", "effective cap", "spent", "carry-out"} {
		if !strings.Contains(text, want) {
			t.Errorf("rollover chain is missing %q: %s", want, text)
		}
	}
}

func TestBUDQ5PeriodMemoSeparatesBudgetAndRevision(t *testing.T) {
	app := guardTestApp(t)
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	base := periodMemoKey(app, 1, "budget-a", start, end)
	if base == periodMemoKey(app, 2, "budget-a", start, end) || base == periodMemoKey(app, 1, "budget-b", start, end) {
		t.Fatal("memo key mixed data revisions or budget scopes")
	}
	otherApp := &appstate.App{}
	if base == periodMemoKey(otherApp, 1, "budget-a", start, end) {
		t.Fatal("memo key mixed app scopes")
	}
}
