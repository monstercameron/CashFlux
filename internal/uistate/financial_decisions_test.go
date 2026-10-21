// SPDX-License-Identifier: MIT

package uistate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestFinancialDecisionParsersPreserveCanonicalDefaultsAndMigrations(t *testing.T) {
	subs, err := ParseSubsDetectPrefs(`{"minOccurrences":1,"ignoredCategoryIds":["removed-category"],"ignoredAccountTypes":["investment"]}`)
	if err != nil || subs.MinOccurrences != 1 || subs.MinOccurrencesOrDefault() != 1 || !subs.HasIgnoredCategory("removed-category") || !subs.HasIgnoredAccountType("investment") {
		t.Fatal("stored criteria or existing sensitivity semantics changed", subs, err)
	}
	if subs, err := ParseSubsDetectPrefs(`{"minOccurrences":20}`); err != nil || subs.MinOccurrencesOrDefault() != 6 || subs.MinOccurrences != 20 {
		t.Fatal(subs, err)
	}
	settings, err := ParseSmartSettings(`{"version":0,"explicitOff":{"SMART-T6":true},"results":{"SMART-T6":"legacy retained before migration"}}`)
	if err != nil || settings.Version != smart.CurrentSettingsVersion || settings.IsEnabled("SMART-U1") || !settings.ExplicitOff["SMART-U1"] || settings.Results["SMART-T6"] != "" {
		t.Fatal("canonical migration changed", settings, err)
	}
	defaults, err := ParseSmartSettings("")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range smart.Catalog() {
		if defaults.IsEnabled(f.Code) != (f.Tier == smart.TierFree) {
			t.Fatal("default opt-in changed", f.Code)
		}
	}
	reports, err := ParseReportsConfig(`{"view":"removed","yoy":true,"rollup":true}`)
	if err != nil || reports.View != "overview" || !reports.YoY || !reports.Rollup {
		t.Fatal(reports, err)
	}
	if nw, err := ParseNetWorthConfig(`{"trendMonths":0}`); err != nil || nw.TrendMonths != 0 {
		t.Fatal("all-time window changed", nw, err)
	}
	if nw, err := ParseNetWorthConfig(`{"trendMonths":999}`); err != nil || nw.TrendMonths != 6 {
		t.Fatal(nw, err)
	}
	cfg, err := ParseWidgetConfigs(`{"savings":{"formula":"income - expense","target":"150"},"removed":{"custom":"retain"}}`)
	if err != nil || cfg.For("savings")["formula"] != "income - expense" || cfg.For("removed")["custom"] != "retain" {
		t.Fatal(cfg, err)
	}
	next := cfg.WithField("savings", "target", "20")
	if next.For("savings")["target"] != "20" || cfg.For("savings")["target"] != "150" {
		t.Fatal("shared raw overrides mutated", cfg, next)
	}
	if threshold, err := ParseLearnThreshold(""); err != nil || threshold != learntally.DefaultMinCount {
		t.Fatal(threshold, err)
	}
	if threshold, err := ParseLearnThreshold("5"); err != nil || threshold != 5 {
		t.Fatal(threshold, err)
	}
}

func TestFinancialDecisionParsersKeepCorruptSourcesUnavailable(t *testing.T) {
	parsers := map[string]func(string) error{
		"subs":     func(s string) error { _, e := ParseSubsDetectPrefs(s); return e },
		"smart":    func(s string) error { _, e := ParseSmartSettings(s); return e },
		"reports":  func(s string) error { _, e := ParseReportsConfig(s); return e },
		"networth": func(s string) error { _, e := ParseNetWorthConfig(s); return e },
		"widgets":  func(s string) error { _, e := ParseWidgetConfigs(s); return e },
	}
	for name, parse := range parsers {
		for _, raw := range []string{"{bad", " ", "null", "[]", `"{}"`, "1"} {
			if err := parse(raw); err == nil {
				t.Errorf("%s accepted %q", name, raw)
			}
		}
	}
	for _, raw := range []string{"0", "-1", "1.5", "NaN", " ", "999999999999999999999999999"} {
		if _, err := ParseLearnThreshold(raw); err == nil {
			t.Errorf("threshold accepted %q", raw)
		}
	}
}
