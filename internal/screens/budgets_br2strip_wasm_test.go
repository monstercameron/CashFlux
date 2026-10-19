// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBR2StripShowsUsableSubtotalAndDeduplicatedIssue(t *testing.T) {
	problem := &budgeting.BudgetEvaluationError{BudgetID: "bad", Operation: "evaluate", Err: money.ErrCurrencyMismatch}
	view := budgetView{Base: "USD", Statuses: []budgeting.Status{
		{Budget: domain.Budget{ID: "ok", Name: "Food", Period: domain.PeriodMonthly, Limit: money.New(7000, "USD")}, Limit: money.New(9000, "USD")},
		{Budget: domain.Budget{ID: "bad", Name: "Trip", Period: domain.PeriodMonthly}, Error: problem,
			Issues: []budgeting.ViewIssue{{BudgetID: "bad", Operation: "evaluate", Message: "Trip needs a currency rate."}}},
	}}
	plan := budgetSummaryPlan(view, bud2TestApp(t), period.NewWindow(period.Month, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Monday))
	if plan.normalizedBudgetTotal != 7000 || plan.excluded != 1 || len(plan.issues) != 1 {
		t.Fatalf("summary plan = total %d excluded %d issues %d; want 7000, 1, 1", plan.normalizedBudgetTotal, plan.excluded, len(plan.issues))
	}
	strip := render.New(t)
	view.TotalSpent = 2500
	strip.Render(budgetSummaryStatus(budgetSummaryStatusProps{
		v:                     view,
		vw:                    period.NewWindow(period.Month, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Monday),
		normalizedBudgetTotal: plan.normalizedBudgetTotal, normalizationOK: true,
		excluded: plan.excluded, issues: plan.issues, attnCount: 2,
	}))
	figure := byTestID(strip, "span", "budgets-status-figure")
	if figure == nil || figure.Text() != "$25.00" {
		t.Fatalf("spent headline figure = %v, want $25.00", figure)
	}
	sentence := byTestID(strip, "span", "budgets-status-sentence")
	if sentence == nil || !strings.Contains(sentence.Text(), "spent of $70.00 monthly plan") || !strings.Contains(sentence.Text(), "excludes 1 budget") {
		t.Fatalf("status sentence = %v, want monthly plan and one-budget exclusion", sentence)
	}
	if meter := byTestID(strip, "div", "budgets-status-meter"); meter == nil || meter.Attr("role") != "meter" {
		t.Fatalf("headline meter = %v, want accessible meter", meter)
	}
	if attention := byTestID(strip, "button", "budgets-hero-attn"); attention == nil || attention.Text() != "2 budgets need attention" {
		t.Fatalf("attention line = %v, want the new attention headline", attention)
	}
	issues := byTestID(strip, "p", "budgets-status-issues")
	if issues == nil || strings.Count(issues.Text(), "Trip needs a currency rate.") != 1 {
		t.Fatalf("issue line = %v, want one deduplicated issue", issues)
	}
	if strings.Count(strip.Text(), "excludes 1 budget") != 1 {
		t.Fatalf("exclusion copy appears %d times, want once: %q", strings.Count(strip.Text(), "excludes 1 budget"), strip.Text())
	}
}

func TestBR2FailedBudgetStillRendersRepairAndDeleteControls(t *testing.T) {
	bud2TestApp(t)
	status := budgeting.Status{Budget: domain.Budget{ID: "bad", Name: "Splurges & getaways"},
		Error: &budgeting.BudgetEvaluationError{BudgetID: "bad", Operation: "evaluate", Err: money.ErrCurrencyMismatch}}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, budgetRowProps{Status: status, OnDelete: func(string) {}}))
	row := byTestID(f, "div", "budget-card-bad")
	if row == nil || !strings.Contains(row.Text(), "Splurges & getaways") || !strings.Contains(row.Text(), "Can't calculate · Fix") {
		t.Fatalf("failed budget row did not preserve its name and repair cue: %v", row)
	}
	if strings.Contains(row.Text(), "$0.00") {
		t.Fatalf("failed budget row fabricated zero figures: %q", row.Text())
	}
	if byTestID(f, "button", "budget-fix-bad") == nil || byTestID(f, "button", "delete-budget-btn-bad") == nil {
		t.Fatal("failed budget row lost its Fix or delete action")
	}
}

func TestBR2RolloverTableUsesBoundedSignedStatus(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	prior := make([]budgeting.RolloverPeriodState, 3)
	for i := range prior {
		prior[i] = budgeting.RolloverPeriodState{Start: start.AddDate(0, i, 0), EffectiveCap: money.New(20000, "USD"),
			CarryIn: money.New(-3730, "USD"), Spend: money.New(20000, "USD"), CarryOut: money.New(-3730, "USD")}
	}
	view := budgeting.RolloverView{Prior: prior, Current: budgeting.RolloverPeriodState{Start: start.AddDate(0, 3, 0), EffectiveCap: money.New(20000, "USD"),
		CarryIn: money.New(-3730, "USD"), Spend: money.New(10000, "USD"), CarryOut: money.New(6270, "USD")}}
	f := render.New(t)
	f.Render(budgetRolloverTable(view, "food"))
	table := byTestID(f, "table", "budget-rollover-table-food")
	if table == nil || len(f.AllByTag("tr")) != 5 {
		t.Fatalf("rollover table rows = %d, want 1 header + 4 periods", len(f.AllByTag("tr")))
	}
	if !strings.Contains(table.Text(), "−$37.30") || !strings.Contains(table.Text(), "Aug 2026") {
		t.Fatalf("rollover table lacks signed carry or localized month: %q", table.Text())
	}
}

func TestBR2ProjectionCommitmentsAndPlannedChip(t *testing.T) {
	status := budgeting.Status{Budget: domain.Budget{ID: "future", Name: "Travel", Period: domain.PeriodMonthly},
		PeriodStatus: budgeting.PeriodPlanned, Projection: budgeting.Pace{OverBy: money.New(57600, "USD")},
		Committed: budgeting.CommittedResult{Required: money.New(3800, "USD")}, FreeAfterCommitted: money.New(199, "USD")}
	props := budgetRowProps{Status: status, Anchor: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), PaceOver: "$576.00"}
	bar := budgetRowBarValues(status, props, false)
	if !strings.Contains(bar.label, "Starts") {
		t.Fatalf("planned period chip = %q", bar.label)
	}
	f := render.New(t)
	var noop ui.Handler
	result := budgetRowPaceNotes(false, props, status, prefs.Prefs{}, noop)
	unknown := status
	unknown.Budget.ID = "unknown"
	unknown.CommitmentError = &budgeting.BudgetEvaluationError{BudgetID: "unknown", Operation: "commitments", Err: money.ErrCurrencyMismatch}
	unknownLine := budgetRowPaceNotes(false, props, unknown, prefs.Prefs{}, noop).paceLine
	f.Render(Fragment(result.paceLine, unknownLine))
	if projection := byTestID(f, "span", "budget-projection-future"); projection == nil || projection.Text() != "On pace to overspend by $576.00" {
		t.Fatalf("projection copy = %v", projection)
	}
	if free := byTestID(f, "span", "budget-free-after-committed-future"); free == nil || free.Text() != "$1.99 free after $38.00 in bills due this month" {
		t.Fatalf("free-after-commitments copy = %v", free)
	}
	if byTestID(f, "span", "budget-free-after-committed-unknown") != nil {
		t.Fatal("unknown commitments were formatted as a zero or known free amount")
	}
}

func TestBR2AnnualPlanNormalizesRatesAndIsViewIndependent(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "monthly", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
		{ID: "weekly", Period: domain.PeriodWeekly, Limit: money.New(1000, "EUR")},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}
	for _, viewedMonth := range []time.Month{time.October, time.November} {
		_ = viewedMonth
		if got := annualPlanTotalLabel(budgets, rates); got != "$1,772.00" {
			t.Fatalf("annual total label = %q, want rate-normalized stable total", got)
		}
	}
}
