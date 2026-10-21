// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

type assistantFormulaTestPage struct {
	assistantFormulaContext
	Snapshot      string                   `json:"snapshot"`
	Total         int                      `json:"total"`
	FilteredTotal int                      `json:"filtered_total"`
	NextOffset    *int                     `json:"next_offset"`
	Metrics       []assistantFormulaMetric `json:"metrics"`
	Error         string                   `json:"error"`
	Restart       bool                     `json:"restart_required"`
}

func assistantFormulaTestClock(t *testing.T) time.Time {
	t.Helper()
	now := time.Date(2026, 10, 20, 12, 0, 15, 0, time.UTC)
	previous := assistantFormulaNow
	assistantFormulaNow = func() time.Time { return now }
	assistantFormulaCache.Clear()
	t.Cleanup(func() { assistantFormulaNow = previous; assistantFormulaCache.Clear() })
	return now
}

func assistantFormulaTestTool(t *testing.T, app *appstate.App, name string) chatTool {
	t.Helper()
	for _, tool := range agToolsFormula(app, "USD", currency.Rates{Base: "USD"}) {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatal("missing formula tool " + name)
	return chatTool{}
}

func assistantFormulaTestReadPage(t *testing.T, app *appstate.App, args any) assistantFormulaTestPage {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	var page assistantFormulaTestPage
	out := assistantFormulaTestTool(t, app, "list_formula_metrics").run(raw)
	if err := json.Unmarshal([]byte(out), &page); err != nil || page.Error != "" {
		t.Fatalf("formula page failed: %s (%v)", out, err)
	}
	return page
}

func TestAssistantFormulaCatalogRegistrationGroupsAndStrictArguments(t *testing.T) {
	assistantFormulaTestClock(t)
	app := primaryTodoNotificationsApp(t)
	count := 0
	for _, tool := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull) {
		if tool.spec.Function.Name == "list_formula_metrics" || tool.spec.Function.Name == "evaluate_formula" {
			count++
			if tool.mutates || !strings.Contains(string(tool.spec.Function.Parameters), `"snapshot"`) {
				t.Fatal("registered formula tool is not the snapshot-aware read handler")
			}
		}
	}
	if count != 2 {
		t.Fatalf("registered formula tools = %d, want 2", count)
	}
	page := assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Group: "Counts", IncludeValues: true})
	if len(page.Metrics) == 0 {
		t.Fatal("declared Counts group was not reachable")
	}
	for _, row := range page.Metrics {
		if row.Group != "Counts" || row.Unit != "count" || !row.Available || row.Value == nil {
			t.Fatalf("group/unit/known-zero metadata disagrees: %+v", row)
		}
	}
	tool := assistantFormulaTestTool(t, app, "list_formula_metrics")
	for _, raw := range []string{`null`, `[]`, `{"unknown":true}`, `{"filter":null}`, `{"Limit":1}`, `{"limit":101}`, `{"offset":-1}`, `{} {}`} {
		var result assistantFormulaTestPage
		out := tool.run(json.RawMessage(raw))
		if err := json.Unmarshal([]byte(out), &result); err != nil || result.Error == "" || len(result.Metrics) != 0 {
			t.Fatalf("invalid formula arguments were accepted: %s", out)
		}
	}
}

func TestAssistantFormulaCatalogPagesReachEveryNameAcrossRebuiltHelpers(t *testing.T) {
	now := assistantFormulaTestClock(t)
	app := primaryTodoNotificationsApp(t)
	for i := range 165 {
		// Valid imported definitions avoid testing the unrelated Studio editor.
		if err := app.Store().PutMolecule(domain.Molecule{Name: fmt.Sprintf("zz_metric_%03d", i), Formula: fmt.Sprintf("income + %d", i), Doc: "A custom catalog molecule"}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	offset, snapshot, last := 0, "", ""
	total := 0
	for {
		page := assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Offset: offset, Limit: 37, Snapshot: snapshot})
		if snapshot == "" {
			snapshot, total = page.Snapshot, page.Total
		}
		if page.Snapshot != snapshot || page.Total != total || page.FilteredTotal != total || !page.ObservedAt.Equal(now) ||
			page.Scope != "whole household" || page.BaseCurrency != "USD" || page.PeriodStart.Day() != 1 || page.PeriodBefore.Month() != time.November {
			t.Fatalf("paging lost its fixed context: %+v", page)
		}
		for _, row := range page.Metrics {
			if seen[row.Name] || row.Name <= last || row.Value != nil || row.Unit == "" || row.Doc == "" {
				t.Fatalf("unstable/duplicate/incomplete metadata or default value leak: %+v", row)
			}
			seen[row.Name], last = true, row.Name
		}
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	if len(seen) != total || total <= 140 {
		t.Fatalf("catalog inaccessible tail: read %d of %d", len(seen), total)
	}
	for i := range 165 {
		if !seen[fmt.Sprintf("zz_metric_%03d", i)] {
			t.Fatalf("custom molecule %d not discoverable", i)
		}
	}
	page := assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Filter: "zz_metric_164", MoleculesOnly: true, IncludeValues: true})
	if len(page.Metrics) != 1 || page.Metrics[0].Value == nil || *page.Metrics[0].Value != 164 || page.Metrics[0].Kind != "molecule" || page.Metrics[0].Formula != "income + 164" {
		t.Fatalf("custom metadata/value mismatch: %+v", page.Metrics)
	}
	var evaluated struct {
		Value      float64                  `json:"value"`
		Available  bool                     `json:"available"`
		Derivation struct{ Formula string } `json:"derivation"`
	}
	raw, _ := json.Marshal(map[string]string{"expression": "zz_metric_164", "snapshot": page.Snapshot})
	out := assistantFormulaTestTool(t, app, "evaluate_formula").run(raw)
	if err := json.Unmarshal([]byte(out), &evaluated); err != nil || !evaluated.Available || evaluated.Derivation.Formula != "income + 164" {
		t.Fatalf("canonical explanation/evaluation failed: %s (%v)", out, err)
	}
	want, err := formula.Eval("zz_metric_164", formula.Env{Vars: liveEngineVarsAt(app, now)})
	if err != nil || evaluated.Value != want.(float64) {
		t.Fatalf("tool differs from canonical engine/evaluator: %s, canonical=%v (%v)", out, want, err)
	}
}

func TestAssistantFormulaSnapshotsRejectChangedExpiredAndEvictedContinuations(t *testing.T) {
	now := assistantFormulaTestClock(t)
	app := primaryTodoNotificationsApp(t)
	tool := assistantFormulaTestTool(t, app, "list_formula_metrics")
	first := assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Limit: 1})
	checkRestart := func(args assistantFormulaPageArgs, want string) {
		t.Helper()
		raw, _ := json.Marshal(args)
		var result assistantFormulaTestPage
		out := tool.run(raw)
		if err := json.Unmarshal([]byte(out), &result); err != nil || !result.Restart || !strings.Contains(result.Error, want) || len(result.Metrics) != 0 {
			t.Fatalf("stale continuation did not clearly restart (%s): %s", want, out)
		}
	}
	checkRestart(assistantFormulaPageArgs{Offset: 1}, "requires snapshot")
	checkRestart(assistantFormulaPageArgs{Offset: 1, Snapshot: first.Snapshot, Filter: "income"}, "selection changed")
	if err := app.PutTask(assistantOpenTask("new", "New task")); err != nil {
		t.Fatal(err)
	}
	checkRestart(assistantFormulaPageArgs{Offset: 1, Snapshot: first.Snapshot}, "inputs changed")
	fresh := assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Limit: 1})
	assistantFormulaCache.Clear() // Simulate eviction by the bounded cache.
	checkRestart(assistantFormulaPageArgs{Offset: 1, Snapshot: fresh.Snapshot}, "evicted")
	fresh = assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Limit: 1})
	assistantFormulaNow = func() time.Time { return now.Add(6 * time.Minute) }
	checkRestart(assistantFormulaPageArgs{Offset: 1, Snapshot: fresh.Snapshot}, "expired")
}

func TestAssistantFormulaCacheIsolatesAppsAndKeepsBoundedSnapshots(t *testing.T) {
	assistantFormulaTestClock(t)
	firstApp, otherApp := primaryTodoNotificationsApp(t), primaryTodoNotificationsApp(t)
	// The revision is global, so both Apps have exactly the same current value.
	if firstApp.Rev() != otherApp.Rev() {
		t.Fatal("fixture needs equal revisions")
	}
	appstate.Default = firstApp
	first := assistantFormulaTestReadPage(t, firstApp, assistantFormulaPageArgs{Limit: 1})
	appstate.Default = otherApp
	other := assistantFormulaTestReadPage(t, otherApp, assistantFormulaPageArgs{Limit: 1})
	if first.Snapshot == other.Snapshot || assistantFormulaCache.Len() > 2 {
		t.Fatal("formula snapshot leaked across apps or exceeded its retention bound")
	}
	raw, _ := json.Marshal(assistantFormulaPageArgs{Offset: 1, Snapshot: first.Snapshot})
	if out := assistantFormulaTestTool(t, otherApp, "list_formula_metrics").run(raw); !strings.Contains(out, "inputs changed") {
		t.Fatalf("another app accepted the first app's token: %s", out)
	}
}

func TestAssistantFormulaUnavailableAndNonfiniteNeverBecomeZero(t *testing.T) {
	when := assistantFormulaTestClock(t)
	app := primaryTodoNotificationsApp(t)
	if err := app.Store().PutMolecule(domain.Molecule{Name: "unresolved_custom", Formula: "unknown_input + 1", Doc: "Legacy unavailable molecule"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(domain.Account{ID: "cash", Name: "Cash", Currency: "USD", Class: domain.ClassAsset,
		Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	// Imported numeric custom values can be individually finite but overflow
	// when the canonical engine sums them. The tool must not serialize Inf.
	if err := app.Store().PutCustomFieldDef(customfields.Def{ID: "large", EntityType: "transaction", Key: "large", Label: "Large", Type: customfields.TypeNumber}); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if err := app.PutTransaction(domain.Transaction{ID: fmt.Sprint(i), AccountID: "cash", Desc: "Small expense", Date: when,
			Amount: money.New(-1, "USD"), Custom: map[string]any{"large": math.MaxFloat64}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, filter := range []string{"unresolved_custom", "cf_txn_large"} {
		page := assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Filter: filter, IncludeValues: true})
		if len(page.Metrics) != 1 || page.Metrics[0].Available || page.Metrics[0].Value != nil {
			t.Fatalf("unavailable/nonfinite metric implied a value: %+v", page.Metrics)
		}
	}
	tool := assistantFormulaTestTool(t, app, "evaluate_formula")
	for _, expr := range []string{"unresolved_custom", "not_a_metric", "safediv(cf_txn_large, 0, 0)", "1 / 0"} {
		raw, _ := json.Marshal(map[string]string{"expression": expr})
		var result struct {
			Error     string   `json:"error"`
			Value     *float64 `json:"value"`
			Available bool     `json:"available"`
		}
		out := tool.run(raw)
		if err := json.Unmarshal([]byte(out), &result); err != nil || result.Error == "" || result.Available || result.Value != nil {
			t.Fatalf("unavailable evaluation became confident zero: %s (%v)", out, err)
		}
	}
	rows, _ := assistantFormulaCatalog(nil, nil, map[string]float64{"nan": math.NaN(), "inf": math.Inf(1), "zero": 0})
	for _, row := range rows {
		if row.Name == "zero" {
			if !row.Available || row.Value == nil || *row.Value != 0 {
				t.Fatal("known zero lost its distinct availability")
			}
		} else if row.Available || row.Value != nil {
			t.Fatalf("nonfinite value was exposed: %+v", row)
		}
	}
}

func TestAssistantFormulaFreshMetadataReadRejectsClosedStoreAfterWarmCache(t *testing.T) {
	assistantFormulaTestClock(t)
	app := primaryTodoNotificationsApp(t)
	tools := agToolsFormula(app, "USD", currency.Rates{Base: "USD"})
	assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{})
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		args := json.RawMessage(`{}`)
		if tool.spec.Function.Name == "evaluate_formula" {
			args = json.RawMessage(`{"expression":"income"}`)
		}
		out := tool.run(args)
		if !strings.Contains(out, "read formula settings") || strings.Contains(out, `"available":true`) || strings.Contains(out, `"value":0`) {
			t.Fatalf("warm cache masked fresh Store metadata failure: %s", out)
		}
	}
}

func TestAssistantFormulaReadsBoundFinancialConfigsAndPoolCatalog(t *testing.T) {
	when := assistantFormulaTestClock(t)
	first, other := primaryTodoNotificationsApp(t), primaryTodoNotificationsApp(t)
	apps := []*appstate.App{first, other}
	for i, app := range apps {
		multiple := int64(i + 1)
		values := []struct {
			key   string
			value any
		}{
			{"cashflux:allocate:config", uistate.AllocConfig{AmountMinor: multiple * 10000}},
			{"cashflux:planning:config", uistate.PlanningConfig{RunwayBufferMinor: multiple * 11000, RunwayDays: 30 + i, ForecastMonths: 7 + i}},
			{"cashflux:bills:smartconfig", uistate.BillsSmartConfig{PayFrequency: []string{"weekly", "monthly"}[i], MinKeepMinor: multiple * 1000}},
			{"cashflux:invest:pools", []uistate.InvestPool{{ID: "pool", Name: []string{"Bound one", "Bound two"}[i]}}},
		}
		for _, value := range values {
			raw, _ := json.Marshal(value.value)
			if err := app.SetKV(value.key, string(raw)); err != nil {
				t.Fatal(err)
			}
		}
		preferences := prefs.Default()
		preferences.IdleCashBenchmarkAPR = 3.5 + float64(i)
		preferences.WeekStart = []prefs.WeekStart{prefs.WeekMonday, prefs.WeekSaturday}[i]
		preferences.PayCycleAnchor = "2026-10-02"
		raw, _ := json.Marshal(preferences)
		if err := app.SetSettingKV("cashflux:prefs", string(raw)); err != nil {
			t.Fatal(err)
		}
		settings := smart.EnableFreeOnly(smart.Settings{})
		if i == 0 {
			settings = settings.SetEnabled("SMART-A1", false)
		}
		raw, _ = json.Marshal(settings)
		if err := app.SetSettingKV("cashflux:smart-settings", string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if first.Rev() != other.Rev() {
		t.Fatal("fixture needs equal global store revisions")
	}
	appstate.Default = other // Deliberately keep the other dataset active throughout.
	for i, app := range apps {
		inputs, err := assistantFormulaInputs(app, when)
		if err != nil || inputs.WeekStart != []time.Weekday{time.Monday, time.Saturday}[i] ||
			inputs.BillsConfig.PayFrequency != []string{"weekly", "monthly"}[i] || inputs.BillsConfig.MinKeepMinor != int64(i+1)*1000 {
			t.Fatalf("bound input configuration mismatch: %+v (%v)", inputs, err)
		}
		for expr, want := range map[string]float64{"alloc_amount": float64(i+1) * 100, "runway_buffer": float64(i+1) * 110,
			"forecast_horizon": float64(7 + i), "idle_cash_benchmark": 3.5 + float64(i)} {
			raw, _ := json.Marshal(map[string]string{"expression": expr})
			var result struct {
				Value     float64 `json:"value"`
				Available bool    `json:"available"`
				Error     string  `json:"error"`
			}
			out := assistantFormulaTestTool(t, app, "evaluate_formula").run(raw)
			if err := json.Unmarshal([]byte(out), &result); err != nil || !result.Available || result.Value != want || result.Error != "" {
				t.Fatalf("App %d formula %s read another dataset's config: %s", i, expr, out)
			}
		}
		page := assistantFormulaTestReadPage(t, app, assistantFormulaPageArgs{Group: "Pools", IncludeValues: true})
		if len(page.Metrics) != 1 || !strings.Contains(page.Metrics[0].Label, []string{"Bound one", "Bound two"}[i]) {
			t.Fatalf("pool metadata leaked active dataset instead of bound dataset: %+v", page.Metrics)
		}
	}
	firstInputs, _ := assistantFormulaInputs(first, when)
	otherInputs, _ := assistantFormulaInputs(other, when)
	if firstInputs.Smart.FreeOn+1 != otherInputs.Smart.FreeOn {
		t.Fatal("Smart settings were read from the active dataset instead of bound settings KV")
	}
}

func TestAssistantFormulaCorruptedConfigsNeverFallBackAfterWarmCache(t *testing.T) {
	assistantFormulaTestClock(t)
	for _, tc := range []struct {
		key, value string
		settings   bool
	}{
		{"cashflux:prefs", "null", true},
		{"cashflux:prefs", "[]", true},
		{"cashflux:allocate:config", `{"amountMinor":"bad"}`, false},
		{"cashflux:planning:config", `{"forecastMonths":"bad"}`, false},
		{"cashflux:bills:smartconfig", `[]`, false},
		{"cashflux:invest:pools", `[null]`, false},
		{"cashflux:smart-settings", `null`, true},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			app := primaryTodoNotificationsApp(t)
			tool := assistantFormulaTestTool(t, app, "evaluate_formula")
			if out := tool.run(json.RawMessage(`{"expression":"income"}`)); !strings.Contains(out, `"available":true`) {
				t.Fatalf("healthy baseline failed: %s", out)
			}
			var err error
			if tc.settings {
				err = app.SetSettingKV(tc.key, tc.value)
			} else {
				err = app.SetKV(tc.key, tc.value)
			}
			if err != nil {
				t.Fatal(err)
			}
			out := tool.run(json.RawMessage(`{"expression":"income"}`))
			if !strings.Contains(out, `"available":false`) || !strings.Contains(out, `"error"`) || strings.Contains(out, `"value":0`) {
				t.Fatalf("corrupted config silently became defaults or zero: %s", out)
			}
		})
	}
}
