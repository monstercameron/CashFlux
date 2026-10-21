// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalhistory"
	"github.com/monstercameron/CashFlux/internal/goalinterest"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goaltrajectory"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestPrimaryGoalCompactCardsSkipDetailPreparation(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}); err != nil {
		t.Fatal(err)
	}
	a := domain.Account{ID: "bank", Name: "Savings", Type: domain.TypeSavings, Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(100000, "USD"), APY: 3, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	g := domain.Goal{ID: "detail", Name: "Details fixture", CurrentAmount: money.New(20000, "USD"), TargetAmount: money.New(100000, "USD"), MonthlyContribution: money.New(10000, "USD"), AccountID: a.ID, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Allocations:   []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(2500, "EUR")}},
		Contributions: []domain.GoalContribution{{At: now, Amount: money.New(10000, "USD")}, {At: now.AddDate(0, -1, 0), Amount: money.New(5000, "USD")}}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	for i := range 8 {
		other := g
		other.ID = fmt.Sprintf("compact-%d", i)
		other.Name = other.ID
		if err := app.PutGoal(other); err != nil {
			t.Fatal(err)
		}
	}
	counts := map[string]int{}
	preparedAt := map[string]time.Time{}
	previous := prepareGoalCardDetails
	prepareGoalCardDetails = func(goal domain.Goal, e goalsvc.Evaluation, accounts []domain.Account, rates currency.Rates, at time.Time) goalCardDetails {
		counts[goal.ID]++
		preparedAt[goal.ID] = at
		return previous(goal, e, accounts, rates, at)
	}
	defer func() { prepareGoalCardDetails = previous }()
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Goals))
	f.Stabilize()
	byTestID(f, "button", "goals-tab-goals").Click()
	f.Stabilize()
	if len(counts) != 0 {
		t.Fatalf("compact host eagerly prepared details: %v", counts)
	}
	for _, id := range []string{"goal-interest-eta-", "goal-trajectory-", "goal-hist-", "goal-figs-"} {
		if byTestID(f, "div", id+g.ID) != nil || byTestID(f, "span", id+g.ID) != nil {
			t.Fatalf("compact card mounted expanded content %s", id)
		}
	}
	byTestID(f, "button", "goal-expand-"+g.ID).Click()
	f.Stabilize()
	if counts[g.ID] == 0 || len(counts) != 1 {
		t.Fatalf("expansion prepared unrelated goals: %v", counts)
	}
	assertGoalDetailServiceParity(t, f, app, g, preparedAt[g.ID], a.APY)
	beforeCollapse := counts[g.ID]
	byTestID(f, "button", "goal-collapse-"+g.ID).Click()
	f.Stabilize()
	if counts[g.ID] != beforeCollapse || byTestID(f, "div", "goal-trajectory-"+g.ID) != nil {
		t.Fatalf("collapse prepared or retained detail content: %v", counts)
	}
	// Settings revisions should refresh compact figures without constructing the
	// interest, trajectory, history or expanded-only action content.
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 4}}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if counts[g.ID] != beforeCollapse || len(counts) != 1 {
		t.Fatalf("compact revision prepared hidden details: %v", counts)
	}
	byTestID(f, "button", "goal-expand-"+g.ID).Click()
	f.Stabilize()
	if counts[g.ID] <= beforeCollapse || len(counts) != 1 {
		t.Fatalf("re-expansion did not prepare only requested goal: %v", counts)
	}
	assertGoalDetailServiceParity(t, f, app, g, preparedAt[g.ID], a.APY)
	beforeRevision := counts[g.ID]
	g.MonthlyContribution = money.New(20000, "USD")
	g.Contributions = append(g.Contributions, domain.GoalContribution{At: now, Amount: money.New(2500, "USD")})
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if counts[g.ID] <= beforeRevision || len(counts) != 1 {
		t.Fatalf("expanded revision did not refresh only visible details: %v", counts)
	}
	assertGoalDetailServiceParity(t, f, app, g, preparedAt[g.ID], a.APY)
}

func assertGoalDetailServiceParity(t *testing.T, f *render.Fixture, app *appstate.App, g domain.Goal, now time.Time, apy float64) {
	t.Helper()
	e := goalsvc.EvaluateGoal(g, goalsvc.TaskSummary{}, goalRates(app), now)
	monthly, ok, err := e.Assignment(g, goalRates(app))
	if err != nil || !ok {
		t.Fatalf("bad fixture monthly: %v %v", ok, err)
	}
	interest := goalinterest.Project(e.Coverage.Covered.Amount, monthly.Amount, g.TargetAmount.Amount, apy)
	interestText := uistate.T("goals.interestEta", fmtMoney(monthly), fmt.Sprintf("%g%%", apy), fmtMoney(g.TargetAmount), interest.Months)
	if node := byTestID(f, "span", "goal-interest-eta-"+g.ID); node == nil || !strings.Contains(node.Text(), interestText) {
		t.Fatalf("expanded interest differs from service: %s want %s", f.Text(), interestText)
	}
	trajectory := goaltrajectory.Project(goaltrajectory.Input{CurrentMinor: e.Coverage.Covered.Amount, TargetMinor: g.TargetAmount.Amount, MonthlyMinor: monthly.Amount, Start: now})
	want := uistate.T("goaltrajectory.onTrack", fmtMoney(g.TargetAmount), trajectory.ProjectedDate.Format("January 2006"), trajectory.MonthsToGoal)
	if node := byTestID(f, "div", "goal-trajectory-eta-"+g.ID); node == nil || node.Text() != want {
		t.Fatalf("expanded trajectory differs from service: %s want %s", f.Text(), want)
	}
	history := goalhistory.Summarize(goalhistory.Series(g, monthly.Amount, contribHistoryMonths, now))
	want = uistate.T("goalhist.readBehind", history.MissedMonths, fmtMoney(money.New(history.ShortfallMinor, g.TargetAmount.Currency)))
	if node := byTestID(f, "span", "goal-hist-read-"+g.ID); node == nil || node.Text() != want {
		t.Fatalf("expanded history differs from service: %s want %s", f.Text(), want)
	}
}

func TestPrimaryGoalNonFinancialDueLabelHasNoSeparatorPrefix(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	g := domain.Goal{ID: "milestone", Name: "Milestone", Kind: domain.GoalKindMilestone, TargetDate: time.Now().AddDate(0, 1, 0), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Goals))
	f.Stabilize()
	byTestID(f, "button", "goals-tab-goals").Click()
	f.Stabilize()
	want := "by " + uistate.LoadPrefs().FormatDate(g.TargetDate)
	assertDue := func() {
		t.Helper()
		for _, node := range f.AllByTag("div") {
			if strings.Contains(node.Attr("class"), "budget-sub goal-sub") {
				if node.Text() != want || strings.HasPrefix(node.Text(), " · ") {
					t.Fatalf("due label has stray separator: %q want %q", node.Text(), want)
				}
				return
			}
		}
		t.Fatalf("due label absent: %s", f.Text())
	}
	assertDue()
	byTestID(f, "button", "goal-expand-"+g.ID).Click()
	f.Stabilize()
	assertDue()
}
