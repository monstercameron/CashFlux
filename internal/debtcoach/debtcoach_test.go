// SPDX-License-Identifier: MIT

package debtcoach

import (
	"reflect"
	"testing"
)

// card is a revolving credit-card line; loan is an installment debt (no limit).
func card(name string, bal, limit, min int64, apr float64) DebtLine {
	return DebtLine{Name: name, Balance: bal, Limit: limit, MinPayment: min, AprPercent: apr, Revolving: true}
}
func loan(name string, bal, min int64, apr float64) DebtLine {
	return DebtLine{Name: name, Balance: bal, MinPayment: min, AprPercent: apr}
}

// kinds pulls the ordered list of alert kinds Evaluate returned.
func kinds(alerts []Alert) []string {
	out := make([]string, len(alerts))
	for i, a := range alerts {
		out[i] = a.Kind
	}
	return out
}

func TestEvaluate(t *testing.T) {
	// Standard config bands (the app defaults): warn at 30%, high at 75%.
	const warn, high = 30, 75

	tests := []struct {
		name string
		in   Input
		want []string // expected kinds, in order
	}{
		{
			name: "no debts is quiet",
			in:   Input{WarnUtilPct: warn, HighUtilPct: high},
			want: nil,
		},
		{
			name: "healthy card and loan raise nothing",
			in: Input{
				Debts:            []DebtLine{card("Visa", 50000, 500000, 5000, 14), loan("Car", 800000, 25000, 5)},
				Assets:           2000000,
				Liabilities:      850000,
				MinPaymentsTotal: 30000,
				// 10% utilization, low interest share.
				CreditUtilPct:        10,
				MonthlyInterestTotal: 3500,
				MinOnlyMonths:        36,
				MinOnlyOK:            true,
				WarnUtilPct:          warn, HighUtilPct: high,
			},
			want: nil,
		},
		{
			name: "over-limit card is critical",
			in: Input{
				Debts:         []DebtLine{card("Visa", 520000, 500000, 5000, 19.99)},
				CreditUtilPct: 104, MinPaymentsTotal: 5000, MonthlyInterestTotal: 8600,
				WarnUtilPct: warn, HighUtilPct: high,
			},
			// over-limit (crit) + utilization-high (crit, 104≥75) + interest-heavy
			// (86% of the min is interest). high-apr is suppressed: Visa is the
			// over-limit subject already.
			want: []string{"over-limit", "utilization-high", "interest-heavy"},
		},
		{
			name: "minimum below interest never clears",
			in: Input{
				// $5,000 at 24% => $100/mo interest; a $80 minimum can't keep up.
				Debts:            []DebtLine{loan("Store Card", 500000, 8000, 24)},
				MinPaymentsTotal: 8000, MonthlyInterestTotal: 10000,
				WarnUtilPct: warn, HighUtilPct: high,
			},
			// underwater (crit) + interest-heavy. high-apr does not fire: 24% APR is
			// below the 25% high-interest bar.
			want: []string{"min-underwater", "interest-heavy"},
		},
		{
			name: "no recorded minimum is a data-quality alert, not underwater",
			in: Input{
				// A card owing money at a real rate but with no minimum on file.
				Debts:       []DebtLine{{Name: "Travel Card", Balance: 53500, AprPercent: 19.9, MinPayment: 0, Currency: "EUR"}},
				WarnUtilPct: warn, HighUtilPct: high,
			},
			want: []string{"min-missing"},
		},
		{
			name: "high APR on an otherwise-fine debt",
			in: Input{
				Debts:            []DebtLine{loan("Payday", 100000, 40000, 30)},
				MinPaymentsTotal: 40000, MonthlyInterestTotal: 2500,
				CreditUtilPct: 0,
				WarnUtilPct:   warn, HighUtilPct: high,
			},
			// APR 30 ≥ 25 and the minimum easily covers interest, so only high-apr.
			want: []string{"high-apr"},
		},
		{
			name: "warn-band utilization only",
			in: Input{
				Debts:         []DebtLine{card("Visa", 200000, 500000, 5000, 18)},
				CreditUtilPct: 40, MinPaymentsTotal: 5000, MonthlyInterestTotal: 3000,
				WarnUtilPct: warn, HighUtilPct: high,
			},
			// 40% => warn band (not high). interest-heavy: 3000/5000 = 60% ≥ 50%.
			want: []string{"utilization-warn", "interest-heavy"},
		},
		{
			name: "owe more than you own",
			in: Input{
				Debts:            []DebtLine{loan("Car", 3000000, 60000, 6)},
				Assets:           1000000,
				Liabilities:      3000000,
				MinPaymentsTotal: 60000, MonthlyInterestTotal: 15000,
				WarnUtilPct: warn, HighUtilPct: high,
			},
			// debt-over-assets (watch) + interest-heavy (25%? 15000/60000=25% <50 so no).
			want: []string{"debt-over-assets"},
		},
		{
			name: "slow but viable minimums nudge",
			in: Input{
				Debts:            []DebtLine{loan("Consolidation", 4000000, 45000, 8)},
				MinPaymentsTotal: 45000, MonthlyInterestTotal: 26667,
				MinOnlyMonths: 180, MinOnlyOK: true,
				WarnUtilPct: warn, HighUtilPct: high,
			},
			// interest-heavy (26667/45000 = 59%) + slow-payoff (180 ≥ 120).
			want: []string{"interest-heavy", "slow-payoff"},
		},
		{
			name: "slow-payoff suppressed when a debt is underwater",
			in: Input{
				Debts:            []DebtLine{loan("Store Card", 500000, 8000, 24)},
				MinPaymentsTotal: 8000, MonthlyInterestTotal: 10000,
				MinOnlyMonths: 999, MinOnlyOK: false,
				WarnUtilPct: warn, HighUtilPct: high,
			},
			want: []string{"min-underwater", "interest-heavy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kinds(Evaluate(tt.in))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("kinds = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSeverityOrdering confirms alerts come back most-urgent-first regardless of
// the order the rules run.
func TestSeverityOrdering(t *testing.T) {
	in := Input{
		Debts: []DebtLine{
			card("Visa", 520000, 500000, 5000, 19.99), // over-limit -> critical
			loan("Payday", 100000, 40000, 30),         // high-apr -> watch
		},
		CreditUtilPct:    40, // warn -> watch
		MinPaymentsTotal: 45000, MonthlyInterestTotal: 1000,
		WarnUtilPct: 30, HighUtilPct: 75,
	}
	got := Evaluate(in)
	if len(got) == 0 {
		t.Fatal("expected alerts")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Severity < got[i].Severity {
			t.Errorf("alert %d (%s, sev %d) less urgent than %d (%s, sev %d) but came first",
				i-1, got[i-1].Kind, got[i-1].Severity, i, got[i].Kind, got[i].Severity)
		}
	}
	// The first alert must be the critical over-limit one.
	if got[0].Kind != "over-limit" {
		t.Errorf("first alert = %q, want over-limit", got[0].Kind)
	}
}

// TestWorstOfKindReportsCount verifies the worst offender is named and the rest counted.
func TestWorstOfKindReportsCount(t *testing.T) {
	in := Input{
		Debts: []DebtLine{
			card("Small", 110000, 100000, 3000, 22), // over by 10k
			card("Big", 260000, 200000, 5000, 25),   // over by 60k -> the worst
		},
		WarnUtilPct: 30, HighUtilPct: 75,
	}
	got := Evaluate(in)
	var overLimit *Alert
	for i := range got {
		if got[i].Kind == "over-limit" {
			overLimit = &got[i]
		}
	}
	if overLimit == nil {
		t.Fatal("expected an over-limit alert")
	}
	if overLimit.Subject != "Big" {
		t.Errorf("over-limit subject = %q, want Big (the larger balance)", overLimit.Subject)
	}
	if overLimit.Count != 2 {
		t.Errorf("over-limit count = %d, want 2", overLimit.Count)
	}
}
