// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBR3RowBarHasOwnGridAreaAndStatusIsQuiet(t *testing.T) {
	guardTestApp(t)
	b := quickFillFixtureBudget()
	b.Name = "Groceries"
	b.Limit = money.New(20000, "USD")
	status := budgeting.Status{Budget: b, Limit: b.Limit, Spent: money.New(10000, "USD"), Remaining: money.New(10000, "USD"), Percent: 50, State: budgeting.StateOK}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, budgetRowProps{Status: status, Compact: true, PaceMarkerPct: 45, OnDelete: func(string) {}}))
	bar, figures := byClass(f, "budget-crow-bar"), byClass(f, "budget-crow-figures")
	if bar == nil || figures == nil || bar == figures || hasClassChild(figures, "budget-crow-bar") {
		t.Fatal("compact progress bar must have a separate grid cell from the figures")
	}
	if byClass(f, "budget-crow-chip") != nil {
		t.Fatal("On track should not render a status chip")
	}
	if byTestID(f, "div", "budget-pace-tick-"+b.ID) == nil {
		t.Fatal("live row should show its pace tick on the bar")
	}
}

func hasClassChild(node *render.QueryNode, class string) bool {
	for _, child := range node.Children() {
		if strings.Contains(" "+child.Attr("class")+" ", " "+class+" ") {
			return true
		}
	}
	return false
}

func TestBR3RowProjectionUsesEndedAndCommittedCopy(t *testing.T) {
	ended := budgeting.Status{Budget: domain.Budget{ID: "ended"}, PeriodStatus: budgeting.PeriodEnded}
	ended.Projection.EndedOverBy = money.New(24480, "USD")
	f := render.New(t)
	f.Render(budgetRowProjectionLine(ended, budgetRowProps{}))
	if got := strings.Join(strings.Fields(f.Text()), " "); got != "Ended $244.80 over" {
		t.Fatalf("ended projection = %q", got)
	}
	live := budgeting.Status{Budget: domain.Budget{ID: "live"}, PeriodStatus: budgeting.PeriodActive,
		Committed: budgeting.CommittedResult{Required: money.New(110000, "USD")}, FreeAfterCommitted: money.New(20000, "USD")}
	live.Projection.CommittedDominates = true
	live.Projection.Committed = money.New(110000, "USD")
	f.Rerender(budgetRowProjectionLine(live, budgetRowProps{}))
	if got := strings.Join(strings.Fields(f.Text()), " "); got != "$200.00 free after $1,100.00 in committed bills" {
		t.Fatalf("committed projection = %q", got)
	}
	failedCommitment := budgeting.Status{Budget: domain.Budget{ID: "commitment-error"}, CommitmentError: &budgeting.BudgetEvaluationError{Err: errors.New("offline")}}
	failedCommitment.Projection.CommittedDominates = true
	failedCommitment.Projection.Committed = money.New(110000, "USD")
	f.Rerender(budgetRowProjectionLine(failedCommitment, budgetRowProps{}))
	if got := strings.Join(strings.Fields(f.Text()), " "); got != "On track — $1,100.00 of this is bills" {
		t.Fatalf("committed-dominates projection = %q", got)
	}
}

func TestBR3ViewWiresPerBudgetRestPerDay(t *testing.T) {
	b := domain.Budget{ID: "daily-rest", Name: "Groceries", Limit: money.New(30000, "USD"), Period: domain.PeriodMonthly}
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	builder := newBudgetViewBuilder(budgeting.ViewInput{Base: "USD"}, false)
	builder.addBudget(budgeting.ViewBudget{
		Valid: true, Budget: b, Status: budgeting.Status{Budget: b, Limit: b.Limit},
		Start: start, End: start.AddDate(0, 1, 0), RestPerDay: 1250,
	})
	if got, want := builder.view.ProratedRest[b.ID], fmtMoney(money.New(1250, "USD")); got != want {
		t.Fatalf("per-budget daily rest = %q, want %q", got, want)
	}
}

func TestBR3CadenceSuffixUsesShortUserFacingNouns(t *testing.T) {
	tests := []struct {
		period domain.Period
		want   string
	}{{domain.PeriodWeekly, "/ wk"}, {domain.PeriodQuarterly, "/ qtr"}, {domain.PeriodYearly, "/ yr"}}
	for _, tc := range tests {
		if got := budgetCadenceSuffix(tc.period); got != tc.want {
			t.Errorf("%s cadence = %q, want %q", tc.period, got, tc.want)
		}
	}
}

func TestBR3IndividualBudgetShowsOwnerInitial(t *testing.T) {
	f := render.New(t)
	f.Render(budgetOwnerInitial("member-a", []domain.Member{{ID: "member-a", Name: "Mira Chen"}}))
	var owner *render.QueryNode
	for _, node := range f.AllByTag("span") {
		if strings.Contains(node.Attr("class"), "budget-owner-initial") {
			owner = node
			break
		}
	}
	if owner == nil || owner.Attr("aria-label") != "Mira Chen" {
		t.Fatalf("owner marker = %v, want the owner's initial with an accessible name", owner)
	}
}

func byClass(f *render.Fixture, class string) *render.QueryNode {
	for _, node := range f.AllByTag("div") {
		if strings.Contains(" "+node.Attr("class")+" ", " "+class+" ") {
			return node
		}
	}
	return nil
}
