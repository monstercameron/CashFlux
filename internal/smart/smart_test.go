// SPDX-License-Identifier: MIT

package smart

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPageValidAndLabel(t *testing.T) {
	for _, p := range Pages() {
		if !p.Valid() {
			t.Errorf("page %q should be valid", p)
		}
		if p.Label() == "" {
			t.Errorf("page %q has no label", p)
		}
	}
	if Page("nope").Valid() {
		t.Errorf("unknown page reported valid")
	}
	if got := Page("nope").Label(); got != "nope" {
		t.Errorf("unknown label = %q, want passthrough", got)
	}
}

func TestTierLabel(t *testing.T) {
	if !TierFree.Valid() || !TierAI.Valid() || Tier("x").Valid() {
		t.Errorf("tier validity wrong")
	}
	if TierFree.Label() != "Free" || TierAI.Label() != "AI" {
		t.Errorf("tier labels wrong")
	}
}

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{
		SeverityInfo: "info", SeverityNudge: "nudge", SeverityWarn: "warn", SeverityAlert: "alert",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("severity %d = %q, want %q", sev, got, want)
		}
	}
}

func TestInsightBuilders(t *testing.T) {
	m := money.New(-1234, "USD")
	i := Insight{Feature: "SMART-A1", Page: PageAccounts, Key: "k", Title: "t"}.
		WithAmount(m).
		WithAction(Action{Kind: ActionCreateTask, Label: "Add a to-do", TaskTitle: "Check it"})
	if !i.HasAmount || i.Amount != m {
		t.Errorf("WithAmount did not set amount")
	}
	if i.Action == nil || i.Action.Kind != ActionCreateTask {
		t.Errorf("WithAction did not set action")
	}
	// Builders must not mutate the original.
	orig := Insight{Feature: "x"}
	_ = orig.WithAmount(m)
	if orig.HasAmount {
		t.Errorf("WithAmount mutated the receiver")
	}
}

func TestCatalogIntegrity(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Catalog() {
		if f.Code == "" || f.Title == "" || f.Summary == "" {
			t.Errorf("feature %+v has empty required field", f)
		}
		if seen[f.Code] {
			t.Errorf("duplicate feature code %q", f.Code)
		}
		seen[f.Code] = true
		if !f.Page.Valid() {
			t.Errorf("feature %q has invalid page %q", f.Code, f.Page)
		}
		if !f.Tier.Valid() {
			t.Errorf("feature %q has invalid tier %q", f.Code, f.Tier)
		}
		if f.Tier == TierAI {
			if f.TypicalInputTokens <= 0 || f.TypicalOutputTokens <= 0 {
				t.Errorf("AI feature %q must carry a token footprint for cost preview", f.Code)
			}
		} else {
			if f.RuleCore {
				t.Errorf("Free feature %q must not set RuleCore", f.Code)
			}
			if f.TypicalInputTokens != 0 || f.TypicalOutputTokens != 0 {
				t.Errorf("Free feature %q must not carry a token footprint", f.Code)
			}
		}
		// Round-trip lookup.
		got, ok := ByCode(f.Code)
		if !ok || got.Code != f.Code {
			t.Errorf("ByCode(%q) failed", f.Code)
		}
	}
	if _, ok := ByCode("SMART-NOPE"); ok {
		t.Errorf("ByCode found a non-existent feature")
	}
}

func TestCatalogCoversAllPages(t *testing.T) {
	for _, p := range Pages() {
		if len(FeaturesForPage(p)) == 0 {
			t.Errorf("page %q has no features", p)
		}
	}
}

func TestCounts(t *testing.T) {
	free, aiCount := Counts()
	if free == 0 || aiCount == 0 {
		t.Errorf("expected both free and AI features, got free=%d ai=%d", free, aiCount)
	}
	if free+aiCount != len(catalog) {
		t.Errorf("counts %d+%d != catalog size %d", free, aiCount, len(catalog))
	}
	// The series is deliberately rule-heavy.
	if free <= aiCount {
		t.Errorf("expected more Free than AI features (rule-first), got free=%d ai=%d", free, aiCount)
	}
}

func TestEstimateCost(t *testing.T) {
	// A Free feature is always zero-cost.
	a1, _ := ByCode("SMART-A1")
	if c := a1.EstimateCost(false); !c.Free() || c.Cents != 0 {
		t.Errorf("free feature cost = %+v, want free/zero", c)
	}
	// An AI feature has a non-negative cent estimate and names its model.
	t1, _ := ByCode("SMART-T1")
	c := t1.EstimateCost(false)
	if c.Free() || c.Model == "" {
		t.Errorf("AI feature cost = %+v, want AI tier + model", c)
	}
	// Escalated estimate should be at least the default (stronger model).
	if esc := t1.EstimateCost(true); esc.Cents < c.Cents {
		t.Errorf("escalated cost %d should be >= default %d", esc.Cents, c.Cents)
	}
}

func TestFormatCents(t *testing.T) {
	cases := map[int64]string{0: "<1¢", -5: "<1¢", 1: "1¢", 42: "42¢", 99: "99¢", 100: "$1", 150: "$1.50", 205: "$2.05", 1234: "$12.34"}
	for cents, want := range cases {
		if got := FormatCents(cents); got != want {
			t.Errorf("FormatCents(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestSettingsEnable(t *testing.T) {
	var s Settings
	if s.IsEnabled("SMART-A1") {
		t.Errorf("default should be opt-out")
	}
	s = s.SetEnabled("SMART-A1", true)
	if !s.IsEnabled("SMART-A1") {
		t.Errorf("enable failed")
	}
	s = s.SetEnabled("SMART-A1", false)
	if s.IsEnabled("SMART-A1") {
		t.Errorf("disable failed")
	}
	// Unknown code is a no-op.
	s2 := s.SetEnabled("SMART-NOPE", true)
	if s2.IsEnabled("SMART-NOPE") {
		t.Errorf("enabled an unknown feature")
	}
}

func TestSettingsDismiss(t *testing.T) {
	var s Settings
	s = s.Dismiss("key-1")
	if !s.IsDismissed("key-1") {
		t.Errorf("dismiss failed")
	}
	s = s.Restore("key-1")
	if s.IsDismissed("key-1") {
		t.Errorf("restore failed")
	}
	if s.Dismiss("").IsDismissed("") {
		t.Errorf("empty key should not dismiss")
	}
}

func TestSettingsActive(t *testing.T) {
	s := Settings{}.SetEnabled("SMART-A1", true).SetEnabled("SMART-A2", true).Dismiss("dead")
	in := []Insight{
		{Feature: "SMART-A1", Key: "live"},
		{Feature: "SMART-A1", Key: "dead"}, // dismissed
		{Feature: "SMART-A8", Key: "off"},  // feature not enabled
		{Feature: "SMART-A2", Key: "live2"},
	}
	got := s.Active(in)
	if len(got) != 2 {
		t.Fatalf("Active returned %d, want 2: %+v", len(got), got)
	}
	for _, ins := range got {
		if ins.Key == "dead" || ins.Key == "off" {
			t.Errorf("Active leaked %q", ins.Key)
		}
	}
	// Must not have mutated the input slice.
	if len(in) != 4 {
		t.Errorf("Active mutated input")
	}
}

func TestSettingsEnabledHelpers(t *testing.T) {
	s := Settings{}.SetEnabled("SMART-A2", true).SetEnabled("SMART-A1", true).SetEnabled("SMART-T1", true)
	codes := s.EnabledCodes()
	// Catalog order: A1 before A2 before T1.
	if len(codes) != 3 || codes[0] != "SMART-A1" || codes[1] != "SMART-A2" || codes[2] != "SMART-T1" {
		t.Errorf("EnabledCodes order wrong: %v", codes)
	}
	acct := s.EnabledFeaturesForPage(PageAccounts)
	if len(acct) != 2 {
		t.Errorf("EnabledFeaturesForPage(accounts) = %d, want 2", len(acct))
	}
	if !s.AnyAIEnabled() {
		t.Errorf("SMART-T1 is AI; AnyAIEnabled should be true")
	}
	if (Settings{}).SetEnabled("SMART-A1", true).AnyAIEnabled() {
		t.Errorf("only a Free feature enabled; AnyAIEnabled should be false")
	}
}

func TestSortInsights(t *testing.T) {
	in := []Insight{
		{Feature: "SMART-A2", Key: "b", Severity: SeverityInfo},
		{Feature: "SMART-A1", Key: "a", Severity: SeverityAlert},
		{Feature: "SMART-A1", Key: "c", Severity: SeverityInfo},
	}
	SortInsights(in)
	if in[0].Severity != SeverityAlert {
		t.Errorf("highest severity should sort first, got %+v", in[0])
	}
	if in[1].Feature != "SMART-A1" || in[2].Feature != "SMART-A2" {
		t.Errorf("ties should sort by feature code: %+v", in)
	}
}
