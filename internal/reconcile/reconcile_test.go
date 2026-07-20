// SPDX-License-Identifier: MIT

package reconcile

import "testing"

func TestPreviewDelta(t *testing.T) {
	tests := []struct {
		name            string
		currentMinor    int64
		targetMinor     int64
		wantAdj         int64
		wantNeedsAdjust bool
	}{
		{
			name:            "no change — no adjustment needed",
			currentMinor:    71000,
			targetMinor:     71000,
			wantAdj:         0,
			wantNeedsAdjust: false,
		},
		{
			name:            "bank higher than ledger — positive adjustment",
			currentMinor:    71000,
			targetMinor:     111500,
			wantAdj:         40500,
			wantNeedsAdjust: true,
		},
		{
			name:            "bank lower than ledger — negative adjustment (credit)",
			currentMinor:    111500,
			targetMinor:     71000,
			wantAdj:         -40500,
			wantNeedsAdjust: true,
		},
		{
			name:            "zero current — full target becomes adjustment",
			currentMinor:    0,
			targetMinor:     50000,
			wantAdj:         50000,
			wantNeedsAdjust: true,
		},
		{
			name:            "both zero — no adjustment",
			currentMinor:    0,
			targetMinor:     0,
			wantAdj:         0,
			wantNeedsAdjust: false,
		},
		{
			name:            "L57 example: $710 → $1,115 = +$405",
			currentMinor:    71000,
			targetMinor:     111500,
			wantAdj:         40500,
			wantNeedsAdjust: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PreviewDelta(tc.currentMinor, tc.targetMinor)
			if got.CurrentMinor != tc.currentMinor {
				t.Errorf("CurrentMinor = %d; want %d", got.CurrentMinor, tc.currentMinor)
			}
			if got.TargetMinor != tc.targetMinor {
				t.Errorf("TargetMinor = %d; want %d", got.TargetMinor, tc.targetMinor)
			}
			if got.AdjustmentMinor != tc.wantAdj {
				t.Errorf("AdjustmentMinor = %d; want %d", got.AdjustmentMinor, tc.wantAdj)
			}
			if got.NeedsAdjustment != tc.wantNeedsAdjust {
				t.Errorf("NeedsAdjustment = %v; want %v", got.NeedsAdjustment, tc.wantNeedsAdjust)
			}
		})
	}
}

func TestPreviewBulkClear(t *testing.T) {
	tests := []struct {
		name             string
		clearedMinor     int64
		statementMinor   int64
		unclearedAmounts []int64
		wantCount        int
		wantSum          int64
		wantProjected    int64
		wantDifference   int64
		wantReconciled   bool
	}{
		{
			name:           "clearing all closes the gap exactly",
			clearedMinor:   71000,
			statementMinor: 111500,
			// two outstanding deposits totalling the +$405 gap.
			unclearedAmounts: []int64{30000, 10500},
			wantCount:        2,
			wantSum:          40500,
			wantProjected:    111500,
			wantDifference:   0,
			wantReconciled:   true,
		},
		{
			name:             "clearing all still leaves a difference",
			clearedMinor:     71000,
			statementMinor:   120000,
			unclearedAmounts: []int64{30000, 10500},
			wantCount:        2,
			wantSum:          40500,
			wantProjected:    111500,
			wantDifference:   8500,
			wantReconciled:   false,
		},
		{
			name:             "nothing uncleared — already matches",
			clearedMinor:     150000,
			statementMinor:   150000,
			unclearedAmounts: nil,
			wantCount:        0,
			wantSum:          0,
			wantProjected:    150000,
			wantDifference:   0,
			wantReconciled:   true,
		},
		{
			name:             "outstanding withdrawals reduce the cleared balance",
			clearedMinor:     200000,
			statementMinor:   185000,
			unclearedAmounts: []int64{-10000, -5000},
			wantCount:        2,
			wantSum:          -15000,
			wantProjected:    185000,
			wantDifference:   0,
			wantReconciled:   true,
		},
		{
			name:             "mixed signs net to the statement",
			clearedMinor:     50000,
			statementMinor:   62500,
			unclearedAmounts: []int64{20000, -7500},
			wantCount:        2,
			wantSum:          12500,
			wantProjected:    62500,
			wantDifference:   0,
			wantReconciled:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PreviewBulkClear(tc.clearedMinor, tc.statementMinor, tc.unclearedAmounts)
			if got.Count != tc.wantCount {
				t.Errorf("Count = %d; want %d", got.Count, tc.wantCount)
			}
			if got.SumMinor != tc.wantSum {
				t.Errorf("SumMinor = %d; want %d", got.SumMinor, tc.wantSum)
			}
			if got.ProjectedClearedMinor != tc.wantProjected {
				t.Errorf("ProjectedClearedMinor = %d; want %d", got.ProjectedClearedMinor, tc.wantProjected)
			}
			if got.Result.DifferenceMinor != tc.wantDifference {
				t.Errorf("Result.DifferenceMinor = %d; want %d", got.Result.DifferenceMinor, tc.wantDifference)
			}
			if got.Result.Reconciled != tc.wantReconciled {
				t.Errorf("Result.Reconciled = %v; want %v", got.Result.Reconciled, tc.wantReconciled)
			}
		})
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name           string
		clearedMinor   int64
		statementMinor int64
		wantDiff       int64
		wantReconciled bool
	}{
		{
			name:           "exact match — reconciled",
			clearedMinor:   150000,
			statementMinor: 150000,
			wantDiff:       0,
			wantReconciled: true,
		},
		{
			name:           "statement higher than cleared — positive diff",
			clearedMinor:   100000,
			statementMinor: 125050,
			wantDiff:       25050,
			wantReconciled: false,
		},
		{
			name:           "statement lower than cleared — negative diff",
			clearedMinor:   200000,
			statementMinor: 195000,
			wantDiff:       -5000,
			wantReconciled: false,
		},
		{
			name:           "empty cleared balance (zero) — diff equals statement",
			clearedMinor:   0,
			statementMinor: 50000,
			wantDiff:       50000,
			wantReconciled: false,
		},
		{
			name:           "both zero — reconciled",
			clearedMinor:   0,
			statementMinor: 0,
			wantDiff:       0,
			wantReconciled: true,
		},
		{
			name:           "opening-balance offset — statement matches after marking all clear",
			clearedMinor:   300025,
			statementMinor: 300025,
			wantDiff:       0,
			wantReconciled: true,
		},
		{
			name:           "opening-balance offset — one transaction still uncleared",
			clearedMinor:   300025,
			statementMinor: 312550,
			wantDiff:       12525,
			wantReconciled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Diff(tc.clearedMinor, tc.statementMinor)
			if got.DifferenceMinor != tc.wantDiff {
				t.Errorf("DifferenceMinor = %d; want %d", got.DifferenceMinor, tc.wantDiff)
			}
			if got.Reconciled != tc.wantReconciled {
				t.Errorf("Reconciled = %v; want %v", got.Reconciled, tc.wantReconciled)
			}
		})
	}
}
