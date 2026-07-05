// SPDX-License-Identifier: MIT

package rules

import (
	"testing"
	"time"
)

// fullCtxs is a small fixture: two big spends, one small, one on a specific account.
func fullCtxs() []TxnCtx {
	d := NewTxnDate(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	return []TxnCtx{
		{Payee: "Big Store", Desc: "tv", AmountMinor: -25000, AccountID: "a1", Date: d},
		{Payee: "Big Store", Desc: "sofa", AmountMinor: -90000, AccountID: "a2", Date: d},
		{Payee: "Corner Cafe", Desc: "coffee", AmountMinor: -450, AccountID: "a1", Date: d},
		{Payee: "Acme Payroll", Desc: "salary", AmountMinor: 400000, AccountID: "a1", Date: d},
	}
}

func TestMatchCountFullConditions(t *testing.T) {
	// amount < -10000 (big outflows) — a condition rule the legacy MatchCount
	// reads as 0 because its Match phrase is empty/ignored.
	r := Rule{
		ID:            "big",
		SetCategoryID: "shopping",
		Conditions:    []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpLt, Value: "-10000"}},
	}
	if got := r.MatchCountFull(fullCtxs()); got != 2 {
		t.Fatalf("MatchCountFull = %d, want 2", got)
	}
	if got := r.MatchCount([]string{"Big Store tv", "Big Store sofa"}); got != 0 {
		t.Fatalf("legacy MatchCount for a conditions rule = %d, want 0 (the honest-count gap MatchCountFull closes)", got)
	}
}

func TestMatchCountFullLegacyPhrase(t *testing.T) {
	r := Rule{ID: "cafe", Match: "cafe", SetCategoryID: "dining"}
	if got := r.MatchCountFull(fullCtxs()); got != 1 {
		t.Fatalf("MatchCountFull (phrase rule) = %d, want 1", got)
	}
}

func TestCoveredFullMixedRules(t *testing.T) {
	rs := []Rule{
		{ID: "big", SetCategoryID: "shopping", Conditions: []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpLt, Value: "-10000"}}},
		{ID: "cafe", Match: "cafe", SetCategoryID: "dining"},
	}
	// Big Store ×2 (condition) + Corner Cafe (phrase) = 3 of 4 covered.
	if got := CoveredFull(rs, fullCtxs()); got != 3 {
		t.Fatalf("CoveredFull = %d, want 3", got)
	}
	// Legacy Covered sees only the phrase rule.
	texts := []string{"Big Store tv", "Big Store sofa", "Corner Cafe coffee", "Acme Payroll salary"}
	if got := Covered(rs, texts); got != 1 {
		t.Fatalf("legacy Covered = %d, want 1 (the coverage gap CoveredFull closes)", got)
	}
}
