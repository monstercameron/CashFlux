package rules

import "testing"

func TestLegacyRuleMigrationPreservesAmountAndPhraseBehavior(t *testing.T) {
	legacy := []Rule{
		{ID: "spend", Conditions: []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpLt, Value: "-5000"}}},
		{ID: "refund", Conditions: []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "5000"}}},
		{ID: "cross-zero", Conditions: []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "-5000"}}},
		{ID: "phrase", Match: "netflix"},
	}
	migrated, changed := MigrateLegacyRuleSemantics(legacy)
	if !changed {
		t.Fatal("legacy rules did not migrate")
	}
	for _, tc := range []struct {
		id     string
		amount int64
		want   bool
	}{
		{"spend", -8_000, true}, {"spend", -1_000, false}, {"spend", 8_000, false},
		{"refund", 8_000, true}, {"refund", -8_000, false},
		{"cross-zero", -4_000, true}, {"cross-zero", -6_000, false}, {"cross-zero", 9_000, true},
	} {
		var candidate Rule
		for _, r := range migrated {
			if r.ID == tc.id {
				candidate = r
			}
		}
		got := MatchConditions(candidate.Conditions, "", "", tc.amount, "", TxnDate{})
		if got != tc.want {
			t.Errorf("%s amount %d matched %v, want %v", tc.id, tc.amount, got, tc.want)
		}
	}
	if got := FirstMatchFull(migrated[3:], "POS1234NETFLIX", "", 0, "", TxnDate{}); got == nil || got.ID != "phrase" {
		t.Fatalf("migrated phrase rule lost its old substring match: %+v", got)
	}
	again, changed := MigrateLegacyRuleSemantics(migrated)
	if changed {
		t.Fatal("migration was not idempotent")
	}
	if len(again[0].Conditions) != 2 || again[2].Conditions[0].Field != ConditionFieldLegacySignedAmount || !again[3].LegacySubstring {
		t.Fatalf("second migration changed saved semantics: %+v", again)
	}
}

func TestNewWordMatchingHandlesBoundariesAndCJK(t *testing.T) {
	for _, tc := range []struct {
		text, pattern string
		want          bool
	}{
		{"Rental car SFO", "rent", false},
		{"CHASE CREDIT CRD AUTOPAY", "auto", false},
		{"上海星巴克咖啡", "星巴克", true},
		{"渋谷のセブンイレブン", "セブンイレブン", true},
		{"AMAZON.COM", ".com", true},
		{"STORE#47", "#47", true},
		{"POS1234NETFLIX", "netflix", false},
	} {
		if got := matchesRule(tc.text, tc.pattern); got != tc.want {
			t.Errorf("matchesRule(%q, %q) = %v, want %v", tc.text, tc.pattern, got, tc.want)
		}
	}
}
