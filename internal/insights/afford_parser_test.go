package insights

import (
	"testing"
)

func TestParseAffordQuery(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		amount  int64  // minor units
		months  int    // MonthsAhead — exact only when wantMonths is true
		label   string // TargetLabel prefix/exact (checked when wantLabel is true)

		wantMonths bool // whether to check months exactly
		wantLabel  bool // whether to check TargetLabel exactly
	}{
		// Amounts
		{
			name:   "plain dollar amount",
			input:  "Can I afford $500?",
			amount: 50000, months: 0, wantMonths: true, wantLabel: true, label: "",
		},
		{
			name:   "comma-formatted amount",
			input:  "Can I afford $1,200",
			amount: 120000, months: 0, wantMonths: true, wantLabel: true, label: "",
		},
		{
			name:   "we afford",
			input:  "Can we afford $2,000 in 3 months?",
			amount: 200000, months: 3, wantMonths: true, wantLabel: true, label: "3 months",
		},
		// "in N months" variants
		{
			name:   "in 6 months",
			input:  "Can I afford $3,500 in 6 months",
			amount: 350000, months: 6, wantMonths: true, wantLabel: true, label: "6 months",
		},
		{
			name:   "in 1 month singular",
			input:  "Can I afford $100 in 1 month",
			amount: 10000, months: 1, wantMonths: true, wantLabel: true, label: "1 month",
		},
		// "by <month>" variants
		{
			name:   "by short month name",
			input:  "Can I afford $800 by Dec",
			amount: 80000, wantLabel: true, label: "December",
		},
		{
			name:   "by full month name",
			input:  "Can I afford $800 by December",
			amount: 80000, wantLabel: true, label: "December",
		},
		{
			name:   "by month and year",
			input:  "Can I afford $1,500 by Dec 2027",
			amount: 150000, wantLabel: true, label: "December 2027",
		},
		// Case-insensitivity
		{
			name:   "case insensitive",
			input:  "CAN I AFFORD $200 IN 2 MONTHS",
			amount: 20000, months: 2, wantMonths: true,
		},
		// Non-affordability questions → nil
		{
			name:    "unrelated question",
			input:   "Where did my money go last month?",
			wantNil: true,
		},
		{
			name:    "no dollar sign",
			input:   "Can I afford 500 dollars?",
			wantNil: true,
		},
		{
			name:    "missing can",
			input:   "I want to afford $500",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseAffordQuery(tc.input)
			if tc.wantNil {
				if ok || got != nil {
					t.Errorf("ParseAffordQuery(%q) = (%v, %v), want (nil, false)", tc.input, got, ok)
				}
				return
			}
			if !ok || got == nil {
				t.Fatalf("ParseAffordQuery(%q) returned nil/false, want a result", tc.input)
			}
			if got.Amount != tc.amount {
				t.Errorf("Amount = %d, want %d", got.Amount, tc.amount)
			}
			if tc.wantMonths && got.MonthsAhead != tc.months {
				t.Errorf("MonthsAhead = %d, want %d", got.MonthsAhead, tc.months)
			}
			if tc.wantLabel && got.TargetLabel != tc.label {
				t.Errorf("TargetLabel = %q, want %q", got.TargetLabel, tc.label)
			}
		})
	}
}
