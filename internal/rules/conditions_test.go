// SPDX-License-Identifier: MIT

package rules

import (
	"testing"
	"time"
)

func TestConditionMatches(t *testing.T) {
	tx := TxnView{Text: "Large Coffee at Starbucks", AccountID: "a1", Amount: 3000}

	tests := []struct {
		name string
		cond Condition
		view TxnView
		want bool
	}{
		{"empty condition matches everything", Condition{}, tx, true},
		{"AND keywords all present", Condition{AllKeywords: []string{"coffee", "starbucks"}}, tx, true},
		{"AND keywords one missing fails", Condition{AllKeywords: []string{"coffee", "tea"}}, tx, false},
		{"AND is case-insensitive", Condition{AllKeywords: []string{"LARGE"}}, tx, true},
		{"OR keywords one present", Condition{AnyKeywords: []string{"tea", "coffee"}}, tx, true},
		{"OR keywords none present fails", Condition{AnyKeywords: []string{"tea", "juice"}}, tx, false},
		{"account scope match", Condition{AccountID: "a1"}, tx, true},
		{"account scope mismatch fails", Condition{AccountID: "a2"}, tx, false},
		{"amount within range", Condition{MinAmount: 1000, MaxAmount: 5000}, tx, true},
		{"amount below min fails", Condition{MinAmount: 4000}, tx, false},
		{"amount above max fails", Condition{MaxAmount: 2000}, tx, false},
		{"min only, at/above passes", Condition{MinAmount: 3000}, tx, true},
		{"combo AND + amount + account", Condition{AllKeywords: []string{"coffee"}, MinAmount: 1000, AccountID: "a1"}, tx, true},
		{"combo fails on one part", Condition{AllKeywords: []string{"coffee"}, MaxAmount: 1000}, tx, false},
		{"blank keywords are ignored", Condition{AllKeywords: []string{"  ", ""}, AnyKeywords: []string{""}}, tx, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cond.Matches(tc.view); got != tc.want {
				t.Errorf("Matches = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMatchConditions covers the C105 structured RuleCondition matching logic.
func TestMatchConditions(t *testing.T) {
	may15 := NewTxnDate(time.Date(2026, time.May, 15, 10, 0, 0, 0, time.UTC))
	noDate := TxnDate{}

	tests := []struct {
		name        string
		conditions  []RuleCondition
		payee       string
		desc        string
		amountMinor int64
		accountID   string
		txnDate     TxnDate
		want        bool
	}{
		// 1. Empty conditions: matches everything (legacy fallback guard).
		{
			name: "empty conditions always match",
			want: true,
		},
		// 2. Legacy substring rule (no conditions) — tested via FirstMatchFull below, but
		//    validate MatchConditions(nil, ...) too.
		{
			name:       "nil conditions always match",
			conditions: nil,
			want:       true,
		},
		// 3. Payee contains.
		{
			name:       "payee contains match",
			conditions: []RuleCondition{{Field: ConditionFieldPayee, Op: ConditionOpContains, Value: "starbucks"}},
			payee:      "Starbucks NYC",
			want:       true,
		},
		{
			name:       "payee contains no match",
			conditions: []RuleCondition{{Field: ConditionFieldPayee, Op: ConditionOpContains, Value: "uber"}},
			payee:      "Starbucks NYC",
			want:       false,
		},
		// 4. Description equals (case-insensitive).
		{
			name:       "description equals match",
			conditions: []RuleCondition{{Field: ConditionFieldDescription, Op: ConditionOpEquals, Value: "morning coffee"}},
			desc:       "Morning Coffee",
			want:       true,
		},
		// 5. Amount > condition (greater than in minor units).
		{
			name:        "amount > passes",
			conditions:  []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "5000"}},
			amountMinor: 7500,
			want:        true,
		},
		{
			name:        "amount > fails",
			conditions:  []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "5000"}},
			amountMinor: 3000,
			want:        false,
		},
		// 6. Amount < condition.
		{
			name:        "amount < passes",
			conditions:  []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpLt, Value: "-1000"}},
			amountMinor: -2500,
			want:        true,
		},
		{
			name:        "amount < fails",
			conditions:  []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpLt, Value: "-1000"}},
			amountMinor: -500,
			want:        false,
		},
		// 7. Account is / is-not.
		{
			name:       "account is match",
			conditions: []RuleCondition{{Field: ConditionFieldAccount, Op: ConditionOpIs, Value: "acc-123"}},
			accountID:  "acc-123",
			want:       true,
		},
		{
			name:       "account is mismatch",
			conditions: []RuleCondition{{Field: ConditionFieldAccount, Op: ConditionOpIs, Value: "acc-123"}},
			accountID:  "acc-999",
			want:       false,
		},
		{
			name:       "account is-not match",
			conditions: []RuleCondition{{Field: ConditionFieldAccount, Op: ConditionOpIsNot, Value: "acc-123"}},
			accountID:  "acc-999",
			want:       true,
		},
		{
			name:       "account is-not fails when equal",
			conditions: []RuleCondition{{Field: ConditionFieldAccount, Op: ConditionOpIsNot, Value: "acc-123"}},
			accountID:  "acc-123",
			want:       false,
		},
		// 8. Multiple AND conditions: payee contains + amount >.
		{
			name: "AND: payee contains + amount > both match",
			conditions: []RuleCondition{
				{Field: ConditionFieldPayee, Op: ConditionOpContains, Value: "starbucks"},
				{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "1000"},
			},
			payee:       "Starbucks NYC",
			amountMinor: 1500,
			want:        true,
		},
		{
			name: "AND: payee matches but amount fails",
			conditions: []RuleCondition{
				{Field: ConditionFieldPayee, Op: ConditionOpContains, Value: "starbucks"},
				{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "5000"},
			},
			payee:       "Starbucks NYC",
			amountMinor: 1500,
			want:        false,
		},
		// 9. Date in-month.
		{
			name:       "in-month match",
			conditions: []RuleCondition{{Field: ConditionFieldDate, Op: ConditionOpInMonth, Value: "2026-05"}},
			txnDate:    may15,
			want:       true,
		},
		{
			name:       "in-month wrong month fails",
			conditions: []RuleCondition{{Field: ConditionFieldDate, Op: ConditionOpInMonth, Value: "2026-06"}},
			txnDate:    may15,
			want:       false,
		},
		{
			name:       "date condition with zero date fails",
			conditions: []RuleCondition{{Field: ConditionFieldDate, Op: ConditionOpInMonth, Value: "2026-05"}},
			txnDate:    noDate,
			want:       false,
		},
		// 10. Date on (exact day).
		{
			name:       "date on match",
			conditions: []RuleCondition{{Field: ConditionFieldDate, Op: ConditionOpOn, Value: "2026-05-15"}},
			txnDate:    may15,
			want:       true,
		},
		{
			name:       "date on wrong day fails",
			conditions: []RuleCondition{{Field: ConditionFieldDate, Op: ConditionOpOn, Value: "2026-05-14"}},
			txnDate:    may15,
			want:       false,
		},
		// 11. Rule with conditions does NOT match when one condition fails (AND).
		{
			name: "three conditions: last fails, whole rule fails",
			conditions: []RuleCondition{
				{Field: ConditionFieldPayee, Op: ConditionOpContains, Value: "uber"},
				{Field: ConditionFieldAccount, Op: ConditionOpIs, Value: "acc-1"},
				{Field: ConditionFieldAmount, Op: ConditionOpLt, Value: "0"},
			},
			payee:       "Uber Eats",
			accountID:   "acc-1",
			amountMinor: 500, // positive, so LT 0 fails
			want:        false,
		},
		// 12. Unparseable amount value: safe non-match.
		{
			name:        "unparseable amount returns false, not panic",
			conditions:  []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpGt, Value: "not-a-number"}},
			amountMinor: 1000,
			want:        false,
		},
		// 13. Amount == (exact).
		{
			name:        "amount == match",
			conditions:  []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpEq, Value: "2500"}},
			amountMinor: 2500,
			want:        true,
		},
		{
			name:        "amount == mismatch",
			conditions:  []RuleCondition{{Field: ConditionFieldAmount, Op: ConditionOpEq, Value: "2500"}},
			amountMinor: 2501,
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchConditions(tc.conditions, tc.payee, tc.desc, tc.amountMinor, tc.accountID, tc.txnDate)
			if got != tc.want {
				t.Errorf("MatchConditions = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFirstMatchFull verifies that FirstMatchFull selects condition-bearing rules
// in preference to the legacy Match path and falls back correctly.
func TestFirstMatchFull(t *testing.T) {
	noDate := TxnDate{}

	// Rule set: a condition-bearing rule and a legacy Match rule.
	rs := []Rule{
		{
			ID: "cond-rule",
			Conditions: []RuleCondition{
				{Field: ConditionFieldPayee, Op: ConditionOpContains, Value: "amazon"},
			},
			SetCategoryID: "shopping",
		},
		{
			ID:            "match-rule",
			Match:         "grocery",
			SetCategoryID: "food",
		},
	}

	t.Run("condition-bearing rule fires when conditions match", func(t *testing.T) {
		r := FirstMatchFull(rs, "Amazon Prime", "", 0, "", noDate)
		if r == nil || r.ID != "cond-rule" {
			t.Fatalf("expected cond-rule, got %v", r)
		}
	})

	t.Run("legacy match rule fires when no condition rule matches", func(t *testing.T) {
		r := FirstMatchFull(rs, "Local Grocery Store", "weekly shop", 0, "", noDate)
		if r == nil || r.ID != "match-rule" {
			t.Fatalf("expected match-rule, got %v", r)
		}
	})

	t.Run("returns nil when no rule matches", func(t *testing.T) {
		r := FirstMatchFull(rs, "Dentist visit", "check-up", 0, "", noDate)
		if r != nil {
			t.Fatalf("expected nil, got %v", r)
		}
	})

	t.Run("legacy FirstMatch skips condition-bearing rules", func(t *testing.T) {
		// "amazon" is a condition field, not a Match string.
		// FirstMatch (old path) must not evaluate conditions.
		r := FirstMatch(rs, "Amazon Prime")
		if r != nil {
			t.Fatalf("FirstMatch should skip condition-bearing rules, got %v", r)
		}
	})
}

// TestPayeeConditionMatchesCleanedForm locks the commercial-parity contract:
// a payee condition written against the CLEANED merchant name matches the raw
// processor descriptor, and one written against the raw form still matches.
func TestPayeeConditionMatchesCleanedForm(t *testing.T) {
	raw := "SQ *BLUE BOTTLE COFFEE #47"
	tests := []struct {
		name  string
		op    ConditionOp
		value string
		want  bool
	}{
		{"contains cleaned name", ConditionOpContains, "Blue Bottle Coffee", true},
		{"contains raw fragment", ConditionOpContains, "SQ *BLUE", true},
		{"equals cleaned name", ConditionOpEquals, "Blue Bottle Coffee", true},
		{"no match", ConditionOpContains, "Chipotle", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conds := []RuleCondition{{Field: ConditionFieldPayee, Op: tc.op, Value: tc.value}}
			got := MatchConditions(conds, raw, "", -1200, "acct1", TxnDate{})
			if got != tc.want {
				t.Fatalf("MatchConditions(%q vs %q %s) = %v, want %v", raw, tc.value, tc.op, got, tc.want)
			}
		})
	}
}

// TestLegacyMatchHitsCleanedPayee locks the same contract for the legacy
// substring Match: a rule whose Match is the tidy merchant name catches the
// raw descriptor.
func TestLegacyMatchHitsCleanedPayee(t *testing.T) {
	r := Rule{Match: "blue bottle coffee", SetCategoryID: "cat-coffee"}
	got := FirstMatchFull([]Rule{r}, "SQ *BLUE BOTTLE COFFEE #47", "", -1200, "acct1", TxnDate{})
	if got == nil {
		t.Fatal("legacy Match against the cleaned merchant name did not match the raw descriptor")
	}
	if none := FirstMatchFull([]Rule{{Match: "chipotle"}}, "SQ *BLUE BOTTLE COFFEE #47", "", -1200, "acct1", TxnDate{}); none != nil {
		t.Fatal("unrelated rule matched")
	}
}
