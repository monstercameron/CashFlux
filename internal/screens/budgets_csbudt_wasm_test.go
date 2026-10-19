// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/untracked"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestCSBUDTTrackingFooterOwnsStableActions(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	cancelled, saved := 0, 0
	props := budgetTrackingFooterProps{Tags: 1, Cancel: func() { cancelled++ }, Save: func() { saved++ }}
	f.Render(ui.CreateElement(budgetTrackingFooter, props))
	if got := byTestID(f, "button", "budgetcats-save"); got == nil || isDisabled(got) {
		t.Fatal("fresh tag-only selection must enable Save")
	}
	byTestID(f, "button", "budgetcats-cancel").Click()
	f.Stabilize()
	props.Tags = 2
	f.Render(ui.CreateElement(budgetTrackingFooter, props))
	if isDisabled(byTestID(f, "button", "budgetcats-save")) {
		t.Fatalf("tag-only selection cannot save: attr=%q property=%#v text=%q", byTestID(f, "button", "budgetcats-save").Attr("disabled"), byTestID(f, "button", "budgetcats-save").Property("disabled"), f.Text())
	}
	byTestID(f, "button", "budgetcats-save").Click()
	f.Stabilize()
	if cancelled != 1 || saved != 1 {
		t.Fatal(cancelled, saved)
	}
	props.Tags = 0
	f.Render(ui.CreateElement(budgetTrackingFooter, props))
	if !isDisabled(byTestID(f, "button", "budgetcats-save")) {
		t.Fatal("empty tracking selection must disable Save")
	}
}

func TestCSBUDTTrackingContentKeepsPickerContracts(t *testing.T) {
	pickerTestApp(t)
	f := render.New(t)
	defer f.Cleanup()
	category, tag := "", ""
	f.Render(ui.CreateElement(budgetTrackingContent, budgetTrackingContentProps{
		Budget: domain.Budget{ID: "test"}, Label: "October 2026", Picked: map[string]bool{}, Tags: map[string]bool{},
		Meta:   budgetTrackMetaResult{DistinctTags: []string{"holiday"}},
		Toggle: func(id string) { category = id }, ToggleTag: func(id string) { tag = id }, Cancel: func() {}, Save: func() {},
	}))
	if hint := byTestID(f, "span", "budgetcats-metahint"); hint == nil || !strings.Contains(hint.Text(), "October 2026") {
		t.Fatal(hint)
	}
	byTestID(f, "input", "budgetcat-pick-c-auto").Dispatch("onchange", render.Event{})
	byTestID(f, "input", "budgettag-pick-holiday").Dispatch("onchange", render.Event{})
	f.Stabilize()
	if category != "c-auto" || tag != "holiday" {
		t.Fatal(category, tag)
	}
}

func TestCSBUDTTrackingUnavailableOwnsClose(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	closed := false
	f.Render(ui.CreateElement(budgetTrackingUnavailable, budgetTrackingUnavailableProps{Close: func() { closed = true }}))
	buttons := f.AllByTag("button")
	if len(buttons) != 1 || buttons[0].Text() != "Close" {
		t.Fatal(f.Text())
	}
	buttons[0].Click()
	f.Stabilize()
	if !closed {
		t.Fatal("close was not delivered")
	}
}

func TestCSBUDTUntrackedContentKeepsConsequencesAndActions(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	closed, applied := 0, 0
	changed := ""
	props := trackUntrackedContentProps{
		Rows: []trackRow{{Cand: untracked.Candidate{CategoryID: "food", Name: "Food"}, Include: true, Amount: 100}},
		Change: func(id string, next trackRow) {
			if !next.Include {
				changed = id
			}
		},
		Base: "USD", Method: budgeting.MethodZeroBased, Preview: budgetview.TrackPreview{
			Effect: untracked.Effect{Categories: 1, TrackedMinor: 100, AssignedDeltaMinor: 100, ToAssignBeforeMinor: 50, ToAssignAfterMinor: -50}, RiskNames: []string{"Travel"},
		}, Close: func() { closed++ }, Apply: func() { applied++ }}
	f.Render(ui.CreateElement(trackUntrackedContent, props))
	for _, id := range []string{"track-summary", "track-toassign", "track-risk"} {
		if byTestID(f, "p", id) == nil {
			t.Fatal(id, f.Text())
		}
	}
	byTestID(f, "input", "track-include-food").Dispatch("onchange", render.Event{Checked: false})
	if changed != "food" {
		t.Fatal("row callback missing", changed)
	}
	byTestID(f, "button", "track-cancel").Click()
	byTestID(f, "button", "track-apply").Click()
	f.Stabilize()
	if closed != 1 || applied != 1 {
		t.Fatal(closed, applied)
	}
	props.Method = budgeting.MethodSimple
	props.Preview = budgetview.TrackPreview{}
	props.Rows = nil
	f.Render(ui.CreateElement(trackUntrackedContent, props))
	if byTestID(f, "p", "track-toassign") != nil || byTestID(f, "p", "track-risk") != nil || !strings.HasSuffix(byTestID(f, "button", "track-apply").Text(), "Track 0") {
		t.Fatal(f.Text())
	}
}

func TestCSBUDTAnnualHeaderOwnsToggle(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	clicked := 0
	props := annualGridHeaderProps{Year: 2026, Rates: currency.Rates{Base: "USD"}, Toggle: func() { clicked++ }}
	for _, open := range []bool{false, true, false} {
		props.Open = open
		f.Render(ui.CreateElement(annualGridHeader, props))
		button := byTestID(f, "button", "budget-annualgrid-toggle")
		if button == nil || button.Attr("aria-expanded") != ariaBool(open) || !strings.Contains(button.Text(), "2026 plan, $0.00") {
			t.Fatal(f.Text())
		}
		button.Click()
		f.Stabilize()
	}
	if clicked != 3 {
		t.Fatal(clicked)
	}
}

func TestCSBUDTAnnualYearControlsOwnNavigation(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	previous, next := 0, 0
	f.Render(ui.CreateElement(annualGridYearControls, annualGridYearControlsProps{Year: 2026, Previous: func() { previous++ }, Next: func() { next++ }}))
	byTestID(f, "button", "budget-annualgrid-prev").Click()
	byTestID(f, "button", "budget-annualgrid-next").Click()
	f.Stabilize()
	if previous != 1 || next != 1 || !strings.Contains(f.Text(), "2026") {
		t.Fatal(previous, next, f.Text())
	}
}

func TestCSBUDTAnnualScenarioControlsKeepHooksAcrossToggle(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	toggle, less, more, reset := 0, 0, 0, 0
	props := annualGridScenarioControlsProps{Currency: "USD", Toggle: func() { toggle++ }, Less: func() { less++ }, More: func() { more++ }, Reset: func() { reset++ }}
	f.Render(ui.CreateElement(annualGridScenarioControls, props))
	byTestID(f, "button", "budget-annualgrid-scenario-toggle").Click()
	f.Stabilize()
	props.On = true
	props.Delta = -10000
	props.UnderCount = 1
	f.Render(ui.CreateElement(annualGridScenarioControls, props))
	if got := byTestID(f, "span", "budget-annualgrid-scenario-delta"); got == nil || got.Text() != "\u2212$100.00" {
		t.Fatal(got)
	}
	for _, id := range []string{"less", "more", "reset"} {
		byTestID(f, "button", "budget-annualgrid-scenario-"+id).Click()
	}
	f.Stabilize()
	props.On = false
	f.Render(ui.CreateElement(annualGridScenarioControls, props))
	if toggle != 1 || less != 1 || more != 1 || reset != 1 {
		t.Fatal(toggle, less, more, reset)
	}
}

func TestCSBUDTAnnualWindowControlsKeepSelectedWindow(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	selected := ""
	for _, win := range []string{annualWinAll, annualWinH1, annualWinH2} {
		f.Render(ui.CreateElement(annualGridWindowControls, annualGridWindowControlsProps{Window: win, Select: func(value string) { selected = value }}))
		button := byTestID(f, "button", "annualgrid-win-"+win)
		if button == nil || button.Attr("aria-pressed") != "true" {
			t.Fatal(win)
		}
		button.Click()
		f.Stabilize()
		if selected != win {
			t.Fatal(selected, win)
		}
	}
}

func TestCSBUDTAnnualGridResliceInvalidatesBuild(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	defer f.Cleanup()
	budgets := make([]domain.Budget, 2)
	for i, id := range []string{"first", "second"} {
		budgets[i] = domain.Budget{ID: id, Name: id, CategoryID: "c", Period: domain.PeriodMonthly, Limit: money.New(100, "USD")}
	}
	props := budgetAnnualGridProps{Budgets: budgets[:1], Rates: currency.Rates{Base: "USD"}, Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	f.Render(ui.CreateElement(BudgetAnnualGrid, props))
	byTestID(f, "button", "budget-annualgrid-toggle").Click()
	f.Stabilize()
	if byTestID(f, "button", "annualgrid-cell-first-2026-01-01") == nil {
		t.Fatal("first row missing")
	}
	props.Budgets = budgets[:2]
	f.Render(ui.CreateElement(BudgetAnnualGrid, props))
	if byTestID(f, "button", "annualgrid-cell-second-2026-01-01") == nil {
		t.Fatal("same-base-pointer reslice kept stale grid")
	}
}

func TestCSBUDTMarkerIntentCancelsOnLeave(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(budgetGoalGlyph, budgetGoalGlyphProps{BudgetID: "csbudt", Kind: "goal", Icon: icon.FileText, Title: "Goal", Text: "Funded by a goal"}))
	button := byTestID(f, "button", "budget-glyph-goal-csbudt")
	button.Dispatch("onmouseenter", render.Event{})
	f.Stabilize()
	if byTestID(f, "span", "budget-glyph-spin-goal-csbudt") == nil {
		t.Fatal("hover intent has no pending feedback")
	}
	byTestID(f, "button", "budget-glyph-goal-csbudt").Dispatch("onmouseleave", render.Event{})
	f.Stabilize()
	if byTestID(f, "span", "budget-glyph-spin-goal-csbudt") != nil {
		t.Fatal("leaving did not cancel intent")
	}
	f.Cleanup()
}

func TestCSBUDTTrackingBodySavesLiveCategoryAndCustomTagDraft(t *testing.T) {
	app := pickerTestApp(t)
	b := domain.Budget{ID: "csbudt-save", Name: "Bills", CategoryID: "c-auto", Period: domain.PeriodMonthly, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(100, "USD")}
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	atom := state.NewGlobalAtom("budgets:catsEdit", "")
	previous := atom.Get()
	atom.Set(b.ID)
	t.Cleanup(func() { atom.Set(previous) })
	f.Render(ui.CreateElement(BudgetCategoriesBody, struct{}{}))
	category := byTestID(f, "input", "budgetcat-pick-c-zoo")
	if category == nil {
		t.Fatalf("tracking category missing: %q", f.Text())
	}
	category.Dispatch("onchange", render.Event{Checked: true})
	f.Stabilize()
	search := byTestID(f, "input", "budgettags-search")
	if search == nil {
		t.Fatalf("tag search missing: %q", f.Text())
	}
	search.Input("Holiday")
	f.Stabilize()
	add := byTestID(f, "button", "budgettag-add")
	if add == nil {
		t.Fatalf("custom tag action missing: %q", f.Text())
	}
	add.Click()
	f.Stabilize()
	byTestID(f, "button", "budgetcats-save").Click()
	f.Stabilize()
	got, found := budgetview.FindBudget(app.Budgets(), b.ID)
	if !found || len(got.CategoryIDs) != 2 || got.CategoryIDs[0] != "c-auto" || got.CategoryIDs[1] != "c-zoo" || len(got.TrackedTags) != 1 || got.TrackedTags[0] != "holiday" || atom.Get() != "" {
		t.Fatalf("saved budget=%+v modal=%q", got, atom.Get())
	}
}
