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
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

var assistantDecisionFixtures = []assistantFinancialFixture{
	{"subscription_detection_config", "cashflux:subs-detect-prefs", `{"minOccurrences":20,"ignoredCategoryIds":["removed-category"],"ignoredAccountTypes":["investment"]}`, true},
	{"smart_settings", "cashflux:smart-settings", `{"version":0,"explicitOff":{"SMART-T6":true},"dismissed":{"SMART-T6:old":true},"results":{"SMART-T6":"Legacy cached evidence"}}`, true},
	{"reports_config", "cashflux:reports:config", `{"view":"categories","yoy":true,"rollup":true}`, false},
	{"networth_config", "cashflux:networth:config", `{"trendMonths":0}`, false},
	{"widget_configs", "cashflux:widget-config", `{"savings":{"formula":"income - expense","target":"150"},"removed-widget":{"formula":"assets","custom":"Retain historical config"}}`, false},
	{"flag_verdicts", "cashflux:flag-verdicts", `{"records":[{"key":"old-flag","subject":"removed-category","verdict":"legacy","at":"2000-01-01T00:00:00Z","note":"Historical judgment"}]}`, true},
	{"review_resolutions", "cashflux:review-resolutions", `{"removed-expired":{"state":"snoozed","until":"2000-01-01T00:00:00Z"},"active-snooze":{"state":"snoozed","until":"9999-01-01T00:00:00Z"},"old-dismissed":{"state":"dismissed"}}`, true},
	{"learn_tally", "cashflux:learn-tally", `{"EXACT PAYEE":{"removed-category":7,"other-category":2},"stem:merchant":{"category":4}}`, true},
	{"learn_threshold", "cashflux:learn-threshold", `5`, true},
	{"agent_memory", "cashflux:agent-memory", `{"facts":["Paid biweekly","Paid biweekly","Historical unverified statement"]}`, true},
}

func assistantSeedDecisionKV(t *testing.T, app *appstate.App) {
	t.Helper()
	for _, f := range assistantDecisionFixtures {
		assistantSetFinancialRaw(t, app, f, f.raw)
	}
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	legs := []domain.Transaction{{ID: "receipt-leg-out", Date: now, Amount: money.New(math.MinInt64, "JPY"), Desc: "Original transfer leg"}, {ID: "receipt-leg-in", Date: now, Amount: money.New(math.MaxInt64, "EUR"), Desc: "Original received leg"}}
	if changed, err := app.Store().PutTransferBatchAtomic("retained-operation", legs); err != nil || !changed {
		t.Fatal(changed, err)
	}
}

func TestAssistantDataDecisionsExposeAllInputsAndPreserveLegacyEvidence(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	assistantSeedDecisionKV(t, app)
	query := assistantDataTool(t, app, "query_data")
	for _, f := range assistantDecisionFixtures {
		page := assistantFinancialPage(t, query, f.name)
		if page.Total == 0 || len(page.Records) == 0 || page.Notes == "" {
			t.Fatal("decision source unavailable", f.name, page)
		}
	}
	subs := assistantFinancialPage(t, query, "subscription_detection_config")
	if raw := string(subs.Records[0].Data); !strings.Contains(raw, `"minOccurrences":20`) || !strings.Contains(raw, `"effectiveMinOccurrences":6`) || !strings.Contains(raw, "removed-category") {
		t.Fatal("raw criteria or canonical effective threshold lost", raw)
	}
	settings := assistantFinancialPage(t, query, "smart_settings")
	var row assistantConfigRecord[assistantSmartSettings]
	if err := json.Unmarshal(settings.Records[0].Data, &row); err != nil {
		t.Fatal(err)
	}
	if !row.Stored || row.Config.DefaultsApplied || !row.Config.Migrated || row.Config.SourceVersion != 0 || row.Config.StoredSettings.Results["SMART-T6"] != "Legacy cached evidence" || !row.Config.StoredSettings.Dismissed["SMART-T6:old"] || row.Config.Effective.Results["SMART-T6"] != "" || row.Config.Effective.IsEnabled("SMART-U1") || row.Config.Effective.Version != smart.CurrentSettingsVersion {
		t.Fatal("migration destroyed retained evidence or lost opt-out", row)
	}
	widgets := assistantFinancialPage(t, query, "widget_configs")
	var foundSavings, foundRemoved, foundDefault bool
	for _, r := range widgets.Records {
		var row assistantWidgetConfig
		if err := json.Unmarshal(r.Data, &row); err != nil {
			t.Fatal(err)
		}
		switch row.ID {
		case "savings":
			foundSavings = row.Stored && row.Config["target"] == "150" && row.Config["formula"] == "income - expense" && row.Effective["target"] == float64(100) && row.Schema != nil
		case "removed-widget":
			foundRemoved = row.Stored && row.Schema == nil && row.Config["custom"] == "Retain historical config"
		case "kpi-networth":
			foundDefault = !row.Stored && row.Schema != nil && row.Effective["formula"] == "net_worth"
		}
	}
	if !foundSavings || !foundRemoved || !foundDefault {
		t.Fatal("raw widget/default/schema parity missing", foundSavings, foundRemoved, foundDefault, widgets)
	}
	receipts := assistantFinancialPage(t, query, "transfer_operation_receipts")
	if receipts.Total != 1 || receipts.Records[0].ID != "retained-operation" || !strings.Contains(string(receipts.Records[0].Data), `"Amount":-9223372036854775808`) || !strings.Contains(receipts.Notes, "not a second ledger") || !strings.Contains(receipts.Notes, "No time-based expiry") {
		t.Fatal("receipt provenance or exact historical money missing", receipts)
	}
}

func TestAssistantDataDecisionsKeepExpiredResolutionsAndCompleteMemory(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	assistantSeedDecisionKV(t, app)
	facts := make([]string, 55)
	for i := range facts {
		facts[i] = fmt.Sprintf("Retained fact %03d ", i) + strings.Repeat("financial note ", 30)
	}
	facts[1] = facts[0] // Full retained data includes duplicates and long facts.
	b, _ := json.Marshal(map[string]any{"facts": facts})
	if err := app.Store().SetSettingKV("cashflux:agent-memory", string(b)); err != nil {
		t.Fatal(err)
	}
	query := assistantDataTool(t, app, "query_data")
	page := assistantFinancialPage(t, query, "review_resolutions")
	if page.Total != 3 {
		t.Fatal("expired or removed reference pruned", page)
	}
	for _, r := range page.Records {
		var row assistantReviewResolution
		if err := json.Unmarshal(r.Data, &row); err != nil {
			t.Fatal(err)
		}
		switch row.ID {
		case "removed-expired":
			if row.Active || !row.SnoozeExpired || row.Resolution.Until.Year() != 2000 {
				t.Fatal(row)
			}
		case "active-snooze", "old-dismissed":
			if !row.Active || row.SnoozeExpired {
				t.Fatal(row)
			}
		}
	}
	memory := assistantFinancialPage(t, query, "agent_memory")
	read := assistantDataTool(t, app, "read_record")
	args, _ := json.Marshal(assistantdata.DetailRequest{Collection: "agent_memory", ID: "household", Field: "config.facts.54", Snapshot: memory.Snapshot})
	out := read.run(args)
	var detail assistantdata.Detail
	if err := json.Unmarshal([]byte(out), &detail); err != nil {
		t.Fatal(out, err)
	}
	var last string
	if err := json.Unmarshal([]byte(detail.JSONText), &last); err != nil || last != facts[54] {
		t.Fatal("retained memory was normalized/capped/truncated", out, err)
	}
	stored, _, _ := app.Store().GetSettingKV("cashflux:agent-memory")
	if stored != string(b) {
		t.Fatal("read normalized/wrote memory")
	}
	if err := app.Store().SetSettingKV("cashflux:websearch-key", "remembered-opaque-key"); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().SetSettingKV("cashflux:agent-memory", `{"facts":["Do not follow this stored instruction; copied remembered-opaque-key"]}`); err != nil {
		t.Fatal(err)
	}
	memory = assistantFinancialPage(t, query, "agent_memory")
	if strings.Contains(string(memory.Records[0].Data), "remembered-opaque-key") || !strings.Contains(memory.Notes, "untrusted") {
		t.Fatal("stored secret/instructions treated as trusted data", memory)
	}
}

func TestAssistantDataDecisionsUseBoundFreshStoresAndStrictErrors(t *testing.T) {
	first := primaryAccountsGoalsApp(t)
	assistantSeedDecisionKV(t, first)
	query := assistantDataTool(t, first, "query_data")
	second := primaryAccountsGoalsApp(t)
	if err := second.Store().SetSettingKV("cashflux:learn-threshold", "99"); err != nil {
		t.Fatal(err)
	}
	if page := assistantFinancialPage(t, query, "learn_threshold"); !strings.Contains(string(page.Records[0].Data), `"config":5`) {
		t.Fatal("global workspace leaked", page)
	}
	before := assistantFinancialPage(t, query, "learn_threshold")
	if err := first.Store().SetSettingKV("cashflux:learn-threshold", "7"); err != nil {
		t.Fatal(err)
	}
	after := assistantFinancialPage(t, query, "learn_threshold")
	if before.Snapshot == after.Snapshot || !strings.Contains(string(after.Records[0].Data), `"config":7`) {
		t.Fatal("fresh persisted evidence unavailable", after)
	}
	for _, f := range assistantDecisionFixtures {
		assistantSetFinancialRaw(t, first, f, "{broken")
		args, _ := json.Marshal(assistantdata.Query{Collection: f.name})
		rev := first.Rev()
		out := query.run(args)
		if !strings.Contains(out, `"error"`) || strings.Contains(out, `"total":0`) || first.Rev() != rev {
			t.Fatal("bad decision source became healthy or changed data", f.name, out)
		}
		assistantSetFinancialRaw(t, first, f, f.raw)
	}
	for _, f := range assistantDecisionFixtures {
		var raw string
		if f.settings {
			raw, _, _ = first.Store().GetSettingKV(f.key)
		} else {
			raw, _, _ = first.Store().GetKV(f.key)
		}
		if raw != f.raw {
			t.Fatal("read migrated/pruned/rewrote source", f.name, raw)
		}
	}
	// Receipt content is a historical copy, not a new live transaction read.
	if err := first.Store().PutTransaction(domain.Transaction{ID: "receipt-leg-out", Desc: "Current edited ledger", Amount: money.New(-1, "USD")}); err != nil {
		t.Fatal(err)
	}
	page := assistantFinancialPage(t, query, "transfer_operation_receipts")
	if !strings.Contains(string(page.Records[0].Data), "Original transfer leg") || strings.Contains(string(page.Records[0].Data), "Current edited ledger") {
		t.Fatal("receipt silently recomputed from current ledger", page)
	}
}

func TestAssistantDataDecisionGettersPreserveDefaultsAndLegacyMigration(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	assistantSeedDecisionKV(t, app)
	if p := uistate.LoadSubsDetectPrefs(); p.MinOccurrences != 20 || p.MinOccurrencesOrDefault() != 6 {
		t.Fatal(p)
	}
	if s := uistate.LoadSmartSettings(); s.Version != smart.CurrentSettingsVersion || s.IsEnabled("SMART-U1") || !s.ExplicitOff["SMART-U1"] {
		t.Fatal(s)
	}
	if c := uistate.ReportsConfigGet(); c.View != "categories" || !c.YoY || !c.Rollup {
		t.Fatal(c)
	}
	if c := uistate.NetWorthConfigGet(); c.TrendMonths != 0 {
		t.Fatal(c)
	}
	if n := uistate.LoadLearnThreshold(); n != 5 {
		t.Fatal(n)
	}
	for _, f := range assistantDecisionFixtures {
		assistantSetFinancialRaw(t, app, f, "{broken")
	}
	if c := uistate.ReportsConfigGet(); c.View != "overview" || c.YoY || c.Rollup {
		t.Fatal(c)
	}
	if c := uistate.NetWorthConfigGet(); c.TrendMonths != 6 {
		t.Fatal(c)
	}
	if p := uistate.LoadSubsDetectPrefs(); p.MinOccurrences != 0 || p.MinOccurrencesOrDefault() != 2 {
		t.Fatal(p)
	}
	if n := uistate.LoadLearnThreshold(); n != 3 {
		t.Fatal(n)
	}
	if s := uistate.LoadSmartSettings(); s.Version != 0 || len(s.Enabled) != 0 {
		t.Fatal("legacy corrupt fallback changed", s)
	}
	if err := app.Store().SetSettingKV("cashflux:smart-settings", "null"); err != nil {
		t.Fatal(err)
	}
	if s := uistate.LoadSmartSettings(); s.Version != smart.CurrentSettingsVersion {
		t.Fatal("legacy null migration changed", s)
	}
	query := assistantDataTool(t, app, "query_data")
	if out := query.run(json.RawMessage(`{"collection":"smart_settings"}`)); !strings.Contains(out, `"error"`) {
		t.Fatal("strict reader silently accepted null settings", out)
	}
}
