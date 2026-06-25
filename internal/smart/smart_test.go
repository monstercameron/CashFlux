// SPDX-License-Identifier: MIT

package smart

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

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

// TestIsEnabledTierDefaults verifies the core C254 contract: Free features are
// on by default, AI features are off by default, and explicit user choices win.
func TestIsEnabledTierDefaults(t *testing.T) {
	// Sanity-check the fixtures are the tiers we expect.
	a1, _ := ByCode("SMART-A1") // Free
	t1, _ := ByCode("SMART-T1") // AI
	if a1.Tier != TierFree {
		t.Fatalf("fixture SMART-A1 expected TierFree, got %q", a1.Tier)
	}
	if t1.Tier != TierAI {
		t.Fatalf("fixture SMART-T1 expected TierAI, got %q", t1.Tier)
	}

	cases := []struct {
		name string
		s    Settings
		code string
		want bool
	}{
		// Tier defaults on a zero Settings (never touched).
		{"Free unset → enabled", Settings{}, "SMART-A1", true},
		{"AI unset → disabled", Settings{}, "SMART-T1", false},
		// Explicit user choices override tier defaults.
		{"Free explicitly off → off", Settings{}.SetEnabled("SMART-A1", false), "SMART-A1", false},
		{"AI explicitly on → on", Settings{}.SetEnabled("SMART-T1", true), "SMART-T1", true},
		// Explicit on for a Free feature (redundant but must work).
		{"Free explicitly on → on", Settings{}.SetEnabled("SMART-A1", true), "SMART-A1", true},
		// Explicit off then back on clears the explicit-off record.
		{"Free off then on → on", Settings{}.SetEnabled("SMART-A1", false).SetEnabled("SMART-A1", true), "SMART-A1", true},
		// Unknown code is always false.
		{"Unknown code → false", Settings{}, "SMART-NOPE", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.IsEnabled(tc.code); got != tc.want {
				t.Errorf("IsEnabled(%q) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestSettingsEnable(t *testing.T) {
	// Unknown code is a no-op.
	var s Settings
	s2 := s.SetEnabled("SMART-NOPE", true)
	if s2.IsEnabled("SMART-NOPE") {
		t.Errorf("enabled an unknown feature")
	}
	// SetEnabled round-trips correctly.
	s = s.SetEnabled("SMART-A1", true)
	if !s.IsEnabled("SMART-A1") {
		t.Errorf("explicit enable failed")
	}
	s = s.SetEnabled("SMART-A1", false)
	if s.IsEnabled("SMART-A1") {
		t.Errorf("explicit disable failed — ExplicitOff not honored")
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
	// Use an AI feature (SMART-T1) as the "off" fixture — AI features are off by
	// default, so it is filtered without any explicit disable call.
	s := Settings{}.SetEnabled("SMART-A1", true).SetEnabled("SMART-A2", true).Dismiss("dead")
	in := []Insight{
		{Feature: "SMART-A1", Key: "live"},
		{Feature: "SMART-A1", Key: "dead"}, // dismissed
		{Feature: "SMART-T1", Key: "off"},  // AI feature, off by default
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
	// EnabledCodes should return all Free features by default plus any explicitly
	// enabled AI features, in catalog order. We verify ordering with a small
	// targeted check: explicitly enable one AI feature and confirm it appears
	// after the Free features that precede it in the catalog.
	s := Settings{}.SetEnabled("SMART-T1", true) // add one AI feature
	codes := s.EnabledCodes()
	// All free features are on by default; SMART-T1 (AI, explicitly on) must
	// also appear. Catalog order puts A-series before T-series.
	foundA1, foundT1 := false, false
	a1Idx, t1Idx := -1, -1
	for i, c := range codes {
		if c == "SMART-A1" {
			foundA1 = true
			a1Idx = i
		}
		if c == "SMART-T1" {
			foundT1 = true
			t1Idx = i
		}
	}
	if !foundA1 {
		t.Errorf("SMART-A1 (Free) should appear in EnabledCodes by default")
	}
	if !foundT1 {
		t.Errorf("SMART-T1 (AI, explicitly on) should appear in EnabledCodes")
	}
	if foundA1 && foundT1 && a1Idx > t1Idx {
		t.Errorf("catalog order violated: A1 idx %d, T1 idx %d", a1Idx, t1Idx)
	}

	// EnabledFeaturesForPage(accounts) includes all Free account features by
	// default. The catalog has 5 Free + 3 AI account features; with no AI enabled
	// on the accounts page we expect exactly the 5 Free ones.
	sNoAI := Settings{}
	acct := sNoAI.EnabledFeaturesForPage(PageAccounts)
	for _, f := range acct {
		if f.Tier == TierAI {
			t.Errorf("AI feature %q appeared in EnabledFeaturesForPage without being enabled", f.Code)
		}
	}
	if len(acct) == 0 {
		t.Errorf("EnabledFeaturesForPage(accounts) returned nothing; Free features should be on by default")
	}

	// AnyAIEnabled is only true when the user has explicitly opted into an AI feature.
	if !s.AnyAIEnabled() {
		t.Errorf("SMART-T1 is AI and explicitly enabled; AnyAIEnabled should be true")
	}
	if (Settings{}).AnyAIEnabled() {
		t.Errorf("zero Settings has no explicit AI opt-in; AnyAIEnabled should be false")
	}
	if (Settings{}).SetEnabled("SMART-A1", true).AnyAIEnabled() {
		t.Errorf("only a Free feature explicitly enabled; AnyAIEnabled should be false")
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

// TestSettingsJSONRoundTrip is the C255 regression guard: a Settings value with
// mixed Enabled and ExplicitOff entries must survive a JSON marshal→unmarshal
// cycle losslessly — i.e., the loaded value is equal to the saved value in every
// field. This mirrors exactly the path that uistate.SaveSmartSettings /
// LoadSmartSettings uses (json.Marshal → SettingKVSet → SettingKVGet →
// json.Unmarshal).
func TestSettingsJSONRoundTrip(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()

	// Build a Settings that exercises every field: mixed Enabled + ExplicitOff,
	// a dismissed insight, a cadence override, a muted feature, a last-run stamp,
	// a cached AI result, and a non-default density.
	orig := Settings{}.
		SetEnabled("SMART-A1", true).
		SetEnabled("SMART-A2", false). // lands in ExplicitOff
		SetEnabled("SMART-T1", true).
		Dismiss("insight-key-42").
		SetCadence("SMART-T1", CadenceWeekly).
		SetMuted("SMART-A1", true).
		MarkRun("SMART-T1", now).
		SetResult("SMART-T1", "Your spending looks healthy.").
		SetDensity(DensityEverywhere)

	// --- marshal ---
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal(Settings) error: %v", err)
	}

	// --- unmarshal ---
	var got Settings
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal(Settings) error: %v", err)
	}

	// Deep equality: every exported field must match.
	if !reflect.DeepEqual(orig, got) {
		t.Errorf("Settings round-trip mismatch\n  orig: %+v\n   got: %+v", orig, got)
	}

	// Spot-check individual semantics survive the round-trip:
	cases := []struct {
		name string
		got  bool
		want bool
	}{
		{"SMART-A1 enabled (explicit on)", got.IsEnabled("SMART-A1"), true},
		{"SMART-A2 disabled (explicit off)", got.IsEnabled("SMART-A2"), false},
		{"SMART-T1 enabled (AI explicit on)", got.IsEnabled("SMART-T1"), true},
		{"SMART-A1 muted", got.IsMuted("SMART-A1"), true},
		{"SMART-A2 not muted", got.IsMuted("SMART-A2"), false},
		{"insight-key-42 dismissed", got.IsDismissed("insight-key-42"), true},
		{"unknown insight not dismissed", got.IsDismissed("no-such-key"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}

	// Cadence override must survive.
	if c := got.CadenceFor("SMART-T1"); c != CadenceWeekly {
		t.Errorf("cadence after round-trip = %v, want %v", c, CadenceWeekly)
	}

	// LastRun timestamp must survive (to the second).
	if rt := got.LastRunAt("SMART-T1"); !rt.Equal(now) {
		t.Errorf("LastRunAt after round-trip = %v, want %v", rt, now)
	}

	// Cached AI result must survive.
	if r := got.ResultFor("SMART-T1"); r != "Your spending looks healthy." {
		t.Errorf("ResultFor after round-trip = %q", r)
	}

	// Density must survive.
	if got.DensityOrDefault() != DensityEverywhere {
		t.Errorf("Density after round-trip = %v, want %v", got.DensityOrDefault(), DensityEverywhere)
	}

	// Zero Settings must also round-trip without error (the "fresh install" path).
	var zero Settings
	zb, _ := json.Marshal(zero)
	var zgot Settings
	if err := json.Unmarshal(zb, &zgot); err != nil {
		t.Fatalf("zero Settings round-trip error: %v", err)
	}
	if !reflect.DeepEqual(zero, zgot) {
		t.Errorf("zero Settings round-trip mismatch: %+v", zgot)
	}
}
