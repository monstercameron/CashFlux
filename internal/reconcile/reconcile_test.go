package reconcile

import "testing"

func TestDiff(t *testing.T) {
	tests := []struct {
		name            string
		clearedMinor    int64
		statementMinor  int64
		wantDiff        int64
		wantReconciled  bool
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
