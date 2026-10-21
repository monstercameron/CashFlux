// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func reportRenderCounters(t *testing.T) (map[string]int, map[string]int) {
	t.Helper()
	models, sections := map[string]int{}, map[string]int{}
	oldModel, oldSection := reportModelBuildObserver, reportSectionBuildObserver
	reportModelBuildObserver = func(name string) { models[name]++ }
	reportSectionBuildObserver = func(name string) { sections[name]++ }
	t.Cleanup(func() { reportModelBuildObserver, reportSectionBuildObserver = oldModel, oldSection })
	return models, sections
}
func copyReportRenderCounts(in map[string]int) map[string]int {
	out := map[string]int{}
	for name, n := range in {
		out[name] = n
	}
	return out
}

func TestReportsPostureRendersPreserveRevisionModelsAndFullSections(t *testing.T) {
	stubBudgetRowBrowser(t)
	models, sections := reportRenderCounters(t)
	oldClock := reportNow
	clock := agrDay.Add(12 * time.Hour)
	reportNow = func() time.Time { return clock }
	defer func() { reportNow = oldClock }()
	controls := &reportRenderControls{}
	f, app := renderLazyReport(t, controls)
	revision := app.Rev()
	initialModels := copyReportRenderCounts(models)
	if models["series"] != 1 || models["activity"] != 1 {
		t.Fatalf("initial models=%v", models)
	}
	primaryClick(t, f, "rpta-mode-full")
	if app.Rev() != revision || !reflect.DeepEqual(models, initialModels) {
		t.Fatalf("Full mode wrote data or rebuilt models: revision=%d->%d builds=%v->%v", revision, app.Rev(), initialModels, models)
	}
	fullSections := copyReportRenderCounts(sections)
	for _, name := range []string{"moneyflow", "motion", "categories", "where", "appendix"} {
		if fullSections[name] == 0 {
			t.Fatalf("Full never built %s", name)
		}
	}
	for _, id := range []string{"reports-export-toggle", "reports-export-toggle", "reports-scope-toggle", "reports-scope-toggle"} {
		clock = clock.Add(time.Minute)
		primaryClick(t, f, id)
		if app.Rev() != revision || !reflect.DeepEqual(models, initialModels) || !reflect.DeepEqual(sections, fullSections) {
			t.Fatalf("%s wrote/rebuilt unchanged report: revision=%d->%d models=%v->%v sections=%v->%v", id, revision, app.Rev(), initialModels, models, fullSections, sections)
		}
	}
	primaryClick(t, f, "rpta-mode-summary")
	if app.Rev() != revision || !reflect.DeepEqual(models, initialModels) || !reflect.DeepEqual(sections, fullSections) {
		t.Fatalf("Summary transition wrote/rebuilt report: models=%v sections=%v", models, sections)
	}
	primaryClick(t, f, "rpta-mode-full")
	if app.Rev() != revision || !reflect.DeepEqual(models, initialModels) {
		t.Fatal("Full remount rescanned unchanged financial data")
	}
	for name, n := range fullSections {
		if sections[name] != n+1 {
			t.Fatalf("Full section %s did not remount exactly once: %d->%d", name, n, sections[name])
		}
	}
	if got := uistate.ReportsConfigGet(); got.View != "overview" || got.Rollup || got.YoY {
		t.Fatalf("opening report changed effective posture: %+v", got)
	}
	// Explicit posture persistence keeps the existing valid legacy View.
	uistate.SetReportsConfig(uistate.ReportsConfig{View: "categories"})
	primaryClick(t, f, "reports-rollup-toggle")
	if got := uistate.ReportsConfigGet(); got.View != "categories" || !got.Rollup {
		t.Fatalf("rollup replaced valid stored View: %+v", got)
	}
}

func TestReportsModelsInvalidateForPeriodScopeDataAndDate(t *testing.T) {
	stubBudgetRowBrowser(t)
	models, sections := reportRenderCounters(t)
	oldClock := reportNow
	clock := agrDay.Add(12 * time.Hour)
	reportNow = func() time.Time { return clock }
	defer func() { reportNow = oldClock }()
	controls := &reportRenderControls{}
	f, app := renderLazyReport(t, controls)
	primaryClick(t, f, "rpta-mode-full")
	before := models["series"]
	selected := controls.Window.Get().Shift(1)
	uistate.SetPeriod(controls.Window, selected)
	f.Stabilize()
	if models["series"] <= before {
		t.Fatal("period changed without rebuilding series")
	}
	restored, ok := uistate.LoadPeriodWindow(clock)
	if !ok || !restored.From.Equal(selected.From) || !restored.To.Equal(selected.To) {
		t.Fatalf("explicit SetPeriod failed to persist after render write removal: %+v %t", restored, ok)
	}
	before = models["series"]
	uistate.SetReportScope(scope.ReportScope{AccountIDs: []string{"a-check"}})
	f.Stabilize()
	if models["series"] <= before || byTestID(f, "div", "rpta-scope-line") == nil {
		t.Fatal("scope change retained previous series or scope disclosure")
	}
	before = models["activity"]
	if err := app.PutTransaction(domain.Transaction{ID: "render-fresh", AccountID: "a-check", CategoryID: "c-gro", Desc: "Fresh render facts", Date: agrDay, Amount: money.New(-5678, "USD")}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if models["activity"] <= before || !strings.Contains(f.Text(), "Fresh render facts") {
		t.Fatal("data change retained old activity")
	}
	before = models["activity"]
	seriesBefore, sectionsBefore, revision := models["series"], sections["motion"], app.Rev()
	clock = clock.AddDate(0, 0, 1)
	primaryClick(t, f, "reports-export-toggle")
	if app.Rev() != revision || models["activity"] != before+1 || models["series"] != seriesBefore || sections["motion"] != sectionsBefore {
		t.Fatalf("date refresh wrong: revision=%d->%d activity=%d->%d series=%d->%d motion=%d->%d", revision, app.Rev(), before, models["activity"], seriesBefore, models["series"], sectionsBefore, sections["motion"])
	}
	// Motion's label uses the existing (start,end] convention at the exact
	// cutoff. A nanosecond changes the label even when the day-keyed activity
	// snapshot remains identical; unrelated Full sections still stay reusable.
	clock = time.Date(2026, time.September, 1, 0, 0, 0, 0, agrDay.Location())
	primaryClick(t, f, "reports-export-toggle")
	if byTestID(f, "span", "rpta-inprogress") == nil {
		t.Fatal("August lost its in-progress label at its inclusive end")
	}
	modelsBefore, cutoffSections := copyReportRenderCounts(models), copyReportRenderCounts(sections)
	clock = clock.Add(time.Nanosecond)
	primaryClick(t, f, "reports-export-toggle")
	if !reflect.DeepEqual(models, modelsBefore) || app.Rev() != revision {
		t.Fatalf("subday label update rebuilt financial models or wrote data: %v->%v", modelsBefore, models)
	}
	if byTestID(f, "span", "rpta-inprogress") != nil {
		t.Fatal("completed August retained its in-progress label after the cutoff")
	}
	for name, n := range cutoffSections {
		want := n
		if name == "motion" {
			want++
		}
		if sections[name] != want {
			t.Fatalf("cutoff rebuilt %s %d->%d, want %d", name, n, sections[name], want)
		}
	}
}
