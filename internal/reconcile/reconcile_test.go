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
