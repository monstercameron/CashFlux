// SPDX-License-Identifier: MIT

package rules

import (
	"reflect"
	"testing"
)

func TestSummarizeCoverageMatchesExistingFullFacts(t *testing.T) {
	txns := append(fullCtxs(), TxnCtx{Payee: "SQ *BLUE BOTTLE COFFEE #47", Desc: "СУПЕР", AmountMinor: -12500})
	rulesBefore := append([]Rule(nil), []Rule{
		{ID: "broad", Match: "big", SetCategoryID: "one"},
		{ID: "condition", Conditions: []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "10000"}}},
		{ID: "shadowed", Match: "big store"},
		{ID: "empty"},
		{ID: "cafe", Match: "cafe"},
		{ID: "cleaned-payee", Match: "blue bottle coffee"},
		{ID: "unicode-desc", Match: "супер"},
	}...)
	rulesBefore[1].Conditions[0].Currency = "USD"
	rulesCopy := cloneCoverageRules(rulesBefore)
	txnCopy := append([]TxnCtx(nil), txns...)

	got := SummarizeCoverage(rulesBefore, txns)
	if got.Covered != CoveredFull(rulesBefore, txns) {
		t.Fatalf("Covered = %d, want canonical %d", got.Covered, CoveredFull(rulesBefore, txns))
	}
	if len(got.Counts) != len(rulesBefore) {
		t.Fatalf("Counts length = %d, want %d", len(got.Counts), len(rulesBefore))
	}
	max := 0
	for i, r := range rulesBefore {
		want := r.MatchCountFull(txns)
		if got.Counts[i] != want {
			t.Errorf("Counts[%d] (%s) = %d, want %d", i, r.ID, got.Counts[i], want)
		}
		if want > max {
			max = want
		}
	}
	if got.MaxMatchCount != max {
		t.Errorf("MaxMatchCount = %d, want %d", got.MaxMatchCount, max)
	}
	if !reflect.DeepEqual(got.Conflicts, Conflicts(rulesBefore)) {
		t.Errorf("Conflicts = %#v, want %#v", got.Conflicts, Conflicts(rulesBefore))
	}
	if !reflect.DeepEqual(rulesBefore, rulesCopy) || !reflect.DeepEqual(txns, txnCopy) {
		t.Fatal("SummarizeCoverage mutated rules or transaction contexts")
	}
}

func TestSummarizeCoveragePreservesPrecedenceAndEmptyInputs(t *testing.T) {
	firstThenSecond := []Rule{{ID: "first", Match: "big"}, {ID: "second", Match: "sofa"}}
	got := SummarizeCoverage(firstThenSecond, fullCtxs())
	if got.Covered != 2 || !reflect.DeepEqual(got.Counts, []int{2, 1}) {
		t.Fatalf("summary = covered %d, counts %#v; want covered 2 and storage-ordered counts [2 1]", got.Covered, got.Counts)
	}
	reversed := SummarizeCoverage([]Rule{firstThenSecond[1], firstThenSecond[0]}, fullCtxs())
	if reversed.Covered != 2 || !reflect.DeepEqual(reversed.Counts, []int{1, 2}) {
		t.Fatalf("reversed summary = covered %d, counts %#v; want storage-ordered counts [1 2]", reversed.Covered, reversed.Counts)
	}

	shadowed := SummarizeCoverage([]Rule{{ID: "broad", Match: "big"}, {ID: "narrow", Match: "big store"}}, fullCtxs())
	if !reflect.DeepEqual(shadowed.Conflicts, []Conflict{{Index: 1, ShadowedBy: 0}}) {
		t.Fatalf("earlier broad rule conflict = %#v, want later index 1 shadowed by index 0", shadowed.Conflicts)
	}
	prioritized := SummarizeCoverage([]Rule{{ID: "narrow", Match: "big store"}, {ID: "broad", Match: "big"}}, fullCtxs())
	if len(prioritized.Conflicts) != 0 {
		t.Fatalf("narrow rule placed first should not shadow later broad rule: %#v", prioritized.Conflicts)
	}

	emptyTxns := SummarizeCoverage(firstThenSecond, nil)
	if emptyTxns.Covered != 0 || emptyTxns.MaxMatchCount != 0 || !reflect.DeepEqual(emptyTxns.Counts, []int{0, 0}) {
		t.Fatalf("empty transaction summary = %#v", emptyTxns)
	}
	emptyRules := SummarizeCoverage(nil, fullCtxs())
	if emptyRules.Covered != 0 || emptyRules.MaxMatchCount != 0 || len(emptyRules.Counts) != 0 || len(emptyRules.Conflicts) != 0 {
		t.Fatalf("empty rule summary = %#v", emptyRules)
	}
}

func cloneCoverageRules(in []Rule) []Rule {
	out := append([]Rule(nil), in...)
	for i := range out {
		out[i].Conditions = append([]RuleCondition(nil), in[i].Conditions...)
		out[i].SetTags = append([]string(nil), in[i].SetTags...)
	}
	return out
}

func BenchmarkCoveragePreparation(b *testing.B) {
	txns := make([]TxnCtx, 3000)
	for i := range txns {
		txns[i] = TxnCtx{Payee: "Merchant " + string(rune('A'+i%26)), Desc: "office supplies", AmountMinor: int64(-2500 - i), AccountID: "checking"}
	}
	rs := make([]Rule, 120)
	for i := range rs {
		if i%4 == 0 {
			rs[i] = Rule{ID: string(rune('a' + i%26)), Conditions: []RuleCondition{{Field: ConditionFieldAccount, Op: ConditionOpIs, Value: "checking"}}}
		} else {
			rs[i] = Rule{ID: string(rune('a' + i%26)), Match: "merchant " + string(rune('A'+i%26))}
		}
	}
	b.Run("old-repeated", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = Conflicts(rs)
			_ = CoveredFull(rs, txns)
			for _, r := range rs {
				_ = r.MatchCountFull(txns)
			}
		}
	})
	b.Run("prepared-summary", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = SummarizeCoverage(rs, txns)
		}
	})
}
