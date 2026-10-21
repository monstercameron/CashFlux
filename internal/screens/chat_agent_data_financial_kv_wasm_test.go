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

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/benchseries"
	"github.com/monstercameron/CashFlux/internal/reportsnap"
	"github.com/monstercameron/CashFlux/internal/savedreports"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

type assistantFinancialFixture struct {
	name, key, raw string
	settings       bool
}

var assistantFinancialFixtures = []assistantFinancialFixture{
	{"budget_sweep_config", "cashflux:budgets:sweepconfig", `{"enabled":true,"budgetIds":["removed-budget"],"targetGoalId":"archived-goal"}`, false},
	{"goal_roundup_config", "cashflux:goals:roundupconfig", `{"enabled":true,"cadence":"monthly","accountIds":["removed-account"],"lastSweepStamp":"2026-10-01T00:00:00Z"}`, false},
	{"occurrence_paid", "cashflux:occurrences:paid", `{"recurring:removed|2026-10-20":1792454400}`, false},
	{"standing_instructions", "cashflux:standing-instructions", `{"instructions":[{"id":"instruction","kind":"keep_liquid","amountMinor":9223372036854775807,"note":"Historical decision"},{"id":"unsupported","kind":"legacy-kind","subject":"removed"}]}`, true},
	{"transaction_templates", "cashflux:txntemplates", `{"items":[{"id":"template","name":"Saved bill","accountId":"removed-account","currency":"JPY","amountMinor":9223372036854775807,"direction":"expense"}]}`, false},
	{"tax_assumptions", "cashflux:tax-plan", `{"effectiveRatePct":22.5,"priorYearTaxMinor":9223372036854775807,"paidToDateMinor":50}`, true},
	{"retirement_assumptions", "cashflux:retirement-plan", `{"years":20,"contribution":9223372036854775807,"spend":12345,"returnPct":6.5}`, true},
	{"inflation_assumption", "cashflux:inflation-pct", `4.125`, true},
	{"notification_routes", "cashflux:notify:routes", `[{"prefix":"same","route":"/first"},{"prefix":"same","route":"/second"}]`, false},
	{"notification_rule_config", "cashflux:notify:ruleconfig", `{"default-budget":false}`, true},
	{"portfolio_targets", "cashflux:rebalance-targets", `[{"AssetClass":"equity","Pct":70},{"AssetClass":"equity","Pct":30}]`, true},
	{"benchmark_series", "cashflux:benchmark-series", `{"name":"User reference","points":[{"d":"2026-10-20T00:00:00Z","v":1234.5}]}`, true},
	{"health_history", "cashflux:health:trend", `[{"month":"2026-09","score":88,"band":"historical-band"}]`, false},
	{"saved_reports", "cashflux:saved-reports", `[{"id":"report","name":"Historical view","res":"month","from":"2026-09-01T00:00:00Z","to":"2026-10-01T00:00:00Z","scope":{"accountIds":["removed-account"]}}]`, false},
	{"report_snapshots", "cashflux:report-snapshots", `[{"id":"snapshot","takenAt":"2026-10-20T00:00:00Z","periodLabel":"Historical period","base":"JPY","income":9223372036854775807,"expense":1,"categories":[{"label":"Stored category","amount":9223372036854775807}]}]`, false},
	{"debt_config", "cashflux:debt:config", `{"warnUtilizationPct":40,"highUtilizationPct":80,"defaultExtraMinor":9223372036854775807}`, false},
}

func assistantSeedFinancialKV(t *testing.T, app *appstate.App) {
	t.Helper()
	for _, f := range assistantFinancialFixtures {
		assistantSetFinancialRaw(t, app, f, f.raw)
	}
}

func assistantSetFinancialRaw(t *testing.T, app *appstate.App, f assistantFinancialFixture, raw string) {
	t.Helper()
	var err error
	if f.settings {
		err = app.Store().SetSettingKV(f.key, raw)
	} else {
		err = app.Store().SetKV(f.key, raw)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func assistantFinancialPage(t *testing.T, tool chatTool, name string) assistantdata.Page {
	t.Helper()
	args, _ := json.Marshal(assistantdata.Query{Collection: name, Limit: 50})
	out := tool.run(args)
	var page assistantdata.Page
	if err := json.Unmarshal([]byte(out), &page); err != nil || page.Collection != name {
		t.Fatal(name, out, err)
	}
	return page
}

func TestAssistantDataFinancialKVExposesAllTypedSourcesAndExactUnits(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	assistantSeedFinancialKV(t, app)
	query := assistantDataTool(t, app, "query_data")
	for _, f := range assistantFinancialFixtures {
		before := app.Rev()
		page := assistantFinancialPage(t, query, f.name)
		if page.Total < 1 || len(page.Records) < 1 || page.Notes == "" || app.Rev() != before {
			t.Fatal("source unavailable, missing provenance, or read mutated data", f.name, page)
		}
		var raw string
		if f.settings {
			raw, _, _ = app.Store().GetSettingKV(f.key)
		} else {
			raw, _, _ = app.Store().GetKV(f.key)
		}
		if raw != f.raw {
			t.Fatal("read rewrote stored history/config", f.name, raw)
		}
	}
	for _, tc := range []struct{ name, text string }{
		{"tax_assumptions", "percentage"}, {"tax_assumptions", "baseCurrency minor units"},
		{"retirement_assumptions", "ANNUAL amounts"}, {"retirement_assumptions", "inflationStored"},
		{"inflation_assumption", "percentage points"}, {"portfolio_targets", "not money"},
		{"benchmark_series", "NOT currency or minor units"}, {"report_snapshots", "each record's base currency"},
		{"occurrence_paid", "not the due date"}, {"standing_instructions", "constrain advice"},
		{"notification_rule_config", "default-bill-due uses days"},
	} {
		if p := assistantFinancialPage(t, query, tc.name); !strings.Contains(p.Notes, tc.text) {
			t.Fatal("unit/provenance absent", tc.name, p.Notes)
		}
	}
	retirement := assistantFinancialPage(t, query, "retirement_assumptions")
	if data := string(retirement.Records[0].Data); !strings.Contains(data, `"contribution":9223372036854775807`) || !strings.Contains(data, `"inflationPct":4.125`) || !strings.Contains(data, `"inflationStored":true`) {
		t.Fatal("exact money or shared inflation lost", data)
	}
	instructions := assistantFinancialPage(t, query, "standing_instructions")
	if instructions.Total != 2 {
		t.Fatal("reader silently filtered unsupported retained instruction", instructions)
	}
	routes := assistantFinancialPage(t, query, "notification_routes")
	if data := string(routes.Records[0].Data); strings.Index(data, "/first") > strings.Index(data, "/second") {
		t.Fatal("first-match order changed", data)
	}
	targets := assistantFinancialPage(t, query, "portfolio_targets")
	if strings.Count(string(targets.Records[0].Data), `"AssetClass":"equity"`) != 2 {
		t.Fatal("raw target duplicates/field names lost", targets)
	}
}

func TestAssistantDataFinancialKVRetainsAllHistoryAndLargeReferenceDetails(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	health := make([]uistate.HealthSnapshot, 15)
	reports := make([]savedreports.Saved, 25)
	snapshots := make([]reportsnap.Snapshot, 30)
	for i := range health {
		health[i] = uistate.HealthSnapshot{Month: now.AddDate(0, -i, 0).Format("2006-01"), Score: i, Band: "original-label"}
	}
	for i := range reports {
		reports[i] = savedreports.Saved{ID: fmt.Sprintf("saved-%03d", i), Name: "Repeated retained name", From: now, To: now}
	}
	for i := range snapshots {
		snapshots[i] = reportsnap.Snapshot{ID: fmt.Sprintf("snap-%03d", i), TakenAt: now, Base: "JPY", Income: math.MaxInt64, Expense: math.MinInt64}
	}
	assistantSetJSONKV(t, app, "cashflux:health:trend", health)
	assistantSetJSONKV(t, app, "cashflux:saved-reports", reports)
	assistantSetJSONKV(t, app, "cashflux:report-snapshots", snapshots)
	series := benchseries.Series{Name: "Complete stored reference", Points: make([]benchseries.Point, benchseries.MaxPoints+1)}
	for i := range series.Points {
		series.Points[i] = benchseries.Point{Date: now, Value: float64(i)}
	}
	encoded, _ := json.Marshal(series)
	if err := app.Store().SetSettingKV("cashflux:benchmark-series", string(encoded)); err != nil {
		t.Fatal(err)
	}
	query := assistantDataTool(t, app, "query_data")
	for _, tc := range []struct {
		name  string
		count int
	}{{"health_history", 15}, {"saved_reports", 25}, {"report_snapshots", 30}} {
		page := assistantFinancialPage(t, query, tc.name)
		if page.Total != tc.count || len(page.Records) != tc.count {
			t.Fatal("stored history was recapped/deduplicated", tc, page)
		}
	}
	page := assistantFinancialPage(t, query, "benchmark_series")
	if page.Total != 1 || !page.Records[0].DetailRequired {
		t.Fatal("large reference missing explicit detail continuation", page)
	}
	read := assistantDataTool(t, app, "read_record")
	args, _ := json.Marshal(assistantdata.DetailRequest{Collection: "benchmark_series", ID: "household", Field: "config.points.20000.v", Snapshot: page.Snapshot})
	out := read.run(args)
	var detail assistantdata.Detail
	if err := json.Unmarshal([]byte(out), &detail); err != nil || detail.JSONText != "20000" {
		t.Fatal("last retained observation beyond importer cap unreachable", out, err)
	}
	stored, _, _ := app.Store().GetSettingKV("cashflux:benchmark-series")
	if stored != string(encoded) {
		t.Fatal("reading mutated reference history")
	}
}

func TestAssistantDataFinancialKVUsesFreshBoundSourcesAndShowsEveryCorruption(t *testing.T) {
	first := primaryAccountsGoalsApp(t)
	assistantSeedFinancialKV(t, first)
	firstQuery := assistantDataTool(t, first, "query_data")
	second := primaryAccountsGoalsApp(t) // Default now points at a different workspace.
	if err := second.Store().SetSettingKV("cashflux:tax-plan", `{"effectiveRatePct":99}`); err != nil {
		t.Fatal(err)
	}
	if p := assistantFinancialPage(t, firstQuery, "tax_assumptions"); !strings.Contains(string(p.Records[0].Data), `"effectiveRatePct":22.5`) {
		t.Fatal("global workspace leaked", p)
	}
	before := assistantFinancialPage(t, firstQuery, "retirement_assumptions")
	if err := first.Store().SetSettingKV("cashflux:inflation-pct", "3.75"); err != nil {
		t.Fatal(err)
	}
	after := assistantFinancialPage(t, firstQuery, "retirement_assumptions")
	if before.Snapshot == after.Snapshot || !strings.Contains(string(after.Records[0].Data), `"inflationPct":3.75`) {
		t.Fatal("cross-source assumption stale", after)
	}
	// Every whitelisted source must propagate parse failure, without migration,
	// fallback writes or a plausible empty result. The other workspace stays valid.
	for _, f := range assistantFinancialFixtures {
		assistantSetFinancialRaw(t, first, f, "{broken")
		args, _ := json.Marshal(assistantdata.Query{Collection: f.name})
		rev := first.Rev()
		out := firstQuery.run(args)
		if !strings.Contains(out, `"error"`) || strings.Contains(out, `"total":0`) || first.Rev() != rev {
			t.Fatal("corrupt source became a default/empty or mutated", f.name, out)
		}
		assistantSetFinancialRaw(t, first, f, f.raw)
	}
	// Settings and dataset KV are distinct stores, even for identical key text.
	if err := first.Store().SetKV("cashflux:tax-plan", `{"effectiveRatePct":1}`); err != nil {
		t.Fatal(err)
	}
	if p := assistantFinancialPage(t, firstQuery, "tax_assumptions"); !strings.Contains(string(p.Records[0].Data), `"effectiveRatePct":22.5`) {
		t.Fatal("wrong namespace read", p)
	}
	if err := first.Store().SetSettingKV("cashflux:websearch-key", "opaque-search-provider-value"); err != nil {
		t.Fatal(err)
	}
	if err := first.Store().SetSettingKV("cashflux:standing-instructions", `{"instructions":[{"id":"copied","kind":"keep_liquid","note":"copied opaque-search-provider-value"}]}`); err != nil {
		t.Fatal(err)
	}
	p := assistantFinancialPage(t, firstQuery, "standing_instructions")
	if strings.Contains(string(p.Records[0].Data), "opaque-search-provider-value") || !strings.Contains(string(p.Records[0].Data), "REDACTED CREDENTIAL") {
		t.Fatal("known search credential in a note leaked", p)
	}
}

func TestAssistantDataFinancialKVSharedGettersPreserveLegacyDefaults(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	assistantSeedFinancialKV(t, app)
	if c := uistate.DebtConfigGet(); c.WarnUtilizationPct != 40 || c.DefaultExtraMinor != math.MaxInt64 || !c.ExcludeMortgage {
		t.Fatal(c)
	}
	if c := uistate.RoundUpConfigGet(); c.Cadence != "monthly" || !c.Enabled {
		t.Fatal(c)
	}
	if c := uistate.SweepConfigGet(); !c.Enabled || c.TargetGoalID != "archived-goal" {
		t.Fatal(c)
	}
	if p := uistate.LoadRetirementPlan(); p.InflationPct != 4.125 || p.AnnualContributionMinor != math.MaxInt64 {
		t.Fatal(p)
	}
	if p := uistate.LoadTaxPlan(); p.EffectiveRatePct != 22.5 || p.PriorYearTaxMinor != math.MaxInt64 {
		t.Fatal(p)
	}
	if rs := uistate.NotifyRoutes(); len(rs) != 2 || rs[0].Route != "/first" {
		t.Fatal(rs)
	}
	for _, f := range assistantFinancialFixtures {
		assistantSetFinancialRaw(t, app, f, "{broken")
	}
	if p := uistate.LoadRetirementPlan(); p.ReturnPct != 7 || p.YearsToRetirement != 0 {
		t.Fatal("legacy retirement fallback changed", p)
	}
	if p := uistate.LoadTaxPlan(); p.EffectiveRatePct != 0 || p.PriorYearTaxMinor != 0 {
		t.Fatal("legacy tax fallback changed", p)
	}
	if c := uistate.DebtConfigGet(); c.WarnUtilizationPct != 30 || c.HighUtilizationPct != 75 {
		t.Fatal("legacy debt fallback changed", c)
	}
	if c := uistate.RoundUpConfigGet(); c.Cadence != "weekly" || c.Enabled {
		t.Fatal("legacy round-up fallback changed", c)
	}
	if err := app.Store().SetKV("cashflux:goals:roundupconfig", "null"); err != nil {
		t.Fatal(err)
	}
	if c := uistate.RoundUpConfigGet(); c.Cadence != "" {
		t.Fatal("legacy null round-up payload changed", c)
	}
	query := assistantDataTool(t, app, "query_data")
	if out := query.run(json.RawMessage(`{"collection":"goal_roundup_config"}`)); !strings.Contains(out, `"error"`) {
		t.Fatal("strict tool silently accepted null config", out)
	}
}
