package rules

import "testing"

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
