package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/rules"
)

func TestLoadMigratesLegacyRuleSemanticsOnce(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	input := Dataset{SchemaVersion: 1, Rules: []rules.Rule{
		{ID: "spend", Conditions: []rules.RuleCondition{{Field: rules.ConditionFieldAmount, Op: rules.ConditionOpLt, Value: "-5000"}}},
		{ID: "merchant", Match: "netflix"},
	}}
	if err := s.Load(input); err != nil {
		t.Fatalf("Load legacy dataset: %v", err)
	}
	first, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after migration: %v", err)
	}
	if first.SettingsKV[ruleSemanticsMigrationKey] != "1" {
		t.Fatalf("migration marker = %q", first.SettingsKV[ruleSemanticsMigrationKey])
	}
	byID := map[string]rules.Rule{}
	for _, r := range first.Rules {
		byID[r.ID] = r
	}
	if len(byID["spend"].Conditions) != 2 || byID["spend"].Conditions[0].Value != "5000" || byID["spend"].Conditions[1].Value != "outflow" {
		t.Fatalf("signed amount condition was not migrated: %+v", byID["spend"].Conditions)
	}
	if got := rules.FirstMatchFull([]rules.Rule{byID["merchant"]}, "POS1234NETFLIX", "", 0, "", rules.TxnDate{}); got != nil {
		t.Fatalf("migrated rule matched inside a word: %+v", got)
	}
	if got := rules.FirstMatchFull([]rules.Rule{byID["merchant"]}, "NETFLIX.COM", "", 0, "", rules.TxnDate{}); got == nil {
		t.Fatal("migrated rule did not match at a word start")
	}
	if err := s.Load(first); err != nil {
		t.Fatalf("Load migrated dataset: %v", err)
	}
	second, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after second load: %v", err)
	}
	for _, r := range second.Rules {
		if r.ID == "spend" && len(r.Conditions) != 2 {
			t.Fatalf("second load duplicated migration conditions: %+v", r.Conditions)
		}
	}
}

func TestLoadRecognizesPriorCombinedRuleMigrationMarker(t *testing.T) {
	input := Dataset{
		SchemaVersion: 1,
		SettingsKV:    map[string]string{legacyRuleSemanticsMigrationKey: "1"},
		Rules: []rules.Rule{{ID: "spend", Conditions: []rules.RuleCondition{
			{Field: rules.ConditionFieldAmount, Op: rules.ConditionOpGt, Value: "5000"},
			{Field: rules.ConditionFieldFlow, Op: rules.ConditionOpEquals, Value: "outflow"},
		}}},
	}
	got := migrateLegacyRuleSemantics(input)
	if got.SettingsKV[ruleSemanticsMigrationKey] != "1" {
		t.Fatalf("signed amount migration marker = %q", got.SettingsKV[ruleSemanticsMigrationKey])
	}
	if len(got.Rules[0].Conditions) != 2 {
		t.Fatalf("prior migration was applied again: %+v", got.Rules[0].Conditions)
	}
}
