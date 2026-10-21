// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/untracked"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestUntrackedBC4INTMixedApplyFailureKeepsEveryBudgetAndDraft(t *testing.T) {
	a := guardTestApp(t)
	ref := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	if err := a.PutBudgetAt(domain.Budget{ID: "existing", Name: "Transport", CategoryID: "c-transport", Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(10000, "USD")}, ref); err != nil {
		t.Fatal(err)
	}
	before := a.Budgets()
	candidates := []untracked.Candidate{
		{CategoryID: "raise", Name: "Raise", SuggestMinor: 1000, Period: domain.PeriodMonthly},
		{CategoryID: "create", Name: "", SuggestMinor: 1000, Period: domain.PeriodMonthly}, // invalid create after a valid raise
	}
	rows := putTrackRow("", "raise", trackRow{Include: true, Amount: 1000, BudgetID: "existing", Raise: true})
	cleared, closed := false, false
	err := applyTrackUntracked(a, candidates, "USD", currency.Rates{Base: "USD"}, "", ref,
		func() string { return rows }, func(string) { cleared = true }, func() { closed = true })
	if err == nil || cleared || closed || !reflect.DeepEqual(before, a.Budgets()) {
		t.Fatalf("partial apply: err=%v cleared=%v closed=%v budgets=%+v", err, cleared, closed, a.Budgets())
	}
}

func TestSweepBC4INTHoldRespectsEligibilityAndMemberLens(t *testing.T) {
	a := guardTestApp(t)
	if err := a.PutBudgetAt(domain.Budget{ID: "unpriced", Name: "Unpriced", CategoryID: "c-transport", Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(10000, "EUR")}, time.Now().AddDate(0, -2, 0)); err != nil {
		t.Fatal(err)
	}
	cfg := uistate.SweepConfig{Enabled: true, BudgetIDs: []string{"unpriced"}}
	uistate.SetSweepConfig(cfg)
	_, _, month := closedMonthRange(time.Now())
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(budgetsSweepCard, struct{}{}))
	previousScope := state.NewGlobalAtom("household:activeScope", scope.ReportScope{}).Get()
	t.Cleanup(func() { uistate.SetActiveScope(previousScope) })
	uistate.SetActiveScope(scope.ReportScope{})
	f.Stabilize()
	if byTestID(f, "div", "sweep-hold") == nil {
		t.Fatal("missing-rate sweep did not show its hold")
	}
	uistate.MarkSweepPromptHandled(month)
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "div", "sweep-hold") != nil {
		t.Fatal("handled household prompt still shows an FX hold")
	}
	for _, member := range []string{"alice", "bob"} {
		uistate.SetActiveScope(scope.ReportScope{Owners: []string{member}})
		uistate.BumpDataRevision()
		f.Stabilize()
		if byTestID(f, "div", "sweep-hold") == nil {
			t.Fatalf("household or another member suppressed %s's hold", member)
		}
		markSweepPromptHandledForMember(month, member)
		uistate.BumpDataRevision()
		f.Stabilize()
		if byTestID(f, "div", "sweep-hold") != nil {
			t.Fatalf("handled %s prompt still shows an FX hold", member)
		}
	}
	for _, member := range []string{"", "alice", "bob"} {
		if sweepPromptDueForMember(month, member, cfg) {
			t.Fatalf("handling another lens forgot %q's month", member)
		}
	}
	cfg.Enabled = false
	uistate.SetSweepConfig(cfg)
	uistate.SetActiveScope(scope.ReportScope{Owners: []string{"carol"}})
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "div", "sweep-hold") != nil {
		t.Fatal("disabled prompt still shows an FX hold")
	}
}

func TestSweepBC4INTDismissalDoesNotHideOtherMember(t *testing.T) {
	a := guardTestApp(t)
	if err := a.PutBudgetAt(domain.Budget{ID: "leftover", Name: "Food", CategoryID: "c-transport", Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(10000, "USD")}, time.Now().AddDate(0, -2, 0)); err != nil {
		t.Fatal(err)
	}
	uistate.SetSweepConfig(uistate.SweepConfig{Enabled: true, BudgetIDs: []string{"leftover"}})
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(budgetsSweepCard, struct{}{}))
	previousScope := state.NewGlobalAtom("household:activeScope", scope.ReportScope{}).Get()
	t.Cleanup(func() { uistate.SetActiveScope(previousScope) })
	uistate.SetActiveScope(scope.ReportScope{Owners: []string{"alice"}})
	uistate.BumpDataRevision()
	f.Stabilize()
	dismiss := byTestID(f, "button", "sweep-dismiss")
	if dismiss == nil {
		t.Fatal("missing dismiss action")
	}
	dismiss.Click()
	f.Stabilize()
	if byTestID(f, "div", "sweep-card") != nil {
		t.Fatal("dismissed card remains visible")
	}
	uistate.SetActiveScope(scope.ReportScope{Owners: []string{"bob"}})
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "div", "sweep-card") == nil {
		t.Fatal("local dismissal leaked into another member lens")
	}
}

func TestBudgetBC4INTCadenceChangeExplainsFreshStart(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	var cadence ui.State[string]
	f.Render(ui.CreateElement(func() ui.Node {
		cadence = ui.UseState(string(domain.PeriodMonthly))
		name, limit := ui.UseState("Food"), ui.UseState("100")
		return ui.CreateElement(budgetEditBasics, budgetEditBasicsProps{budgetEditFieldsProps: budgetEditFieldsProps{
			periodS: cadence, nameS: name, limitS: limit,
			b: domain.Budget{ID: "existing", Period: domain.PeriodMonthly},
		}})
	}))
	if byTestID(f, "p", "budget-period-hint") != nil {
		t.Fatal("unchanged cadence shows a reset warning")
	}
	cadence.Set(string(domain.PeriodWeekly))
	f.Stabilize()
	hint := byTestID(f, "p", "budget-period-hint")
	if hint == nil || !strings.Contains(hint.Text(), "starts fresh from this period") || !strings.Contains(hint.Text(), "Old top-ups and carry do not move over") || !strings.Contains(hint.Text(), "notes stay") {
		t.Fatalf("cadence copy=%v", hint)
	}
	cadence.Set(string(domain.PeriodMonthly))
	f.Stabilize()
	if byTestID(f, "p", "budget-period-hint") != nil {
		t.Fatal("reverted cadence still shows a reset warning")
	}
}
