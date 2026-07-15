// SPDX-License-Identifier: MIT

package idlecash

import "testing"

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name        string
		in          Inputs
		wantIdle    int64
		wantAnnual  int64
		wantMonthly int64
		wantFlag    bool
	}{
		{
			name:        "classic idle case",
			in:          Inputs{LiquidMinor: 1_200_000, CommittedMinor: 400_000, BenchmarkAPRPercent: 4.375},
			wantIdle:    800_000,
			wantAnnual:  35_000, // 800000 * 4.375/100
			wantMonthly: 2_916,
			wantFlag:    true,
		},
		{
			name:     "committed exceeds liquid -> no idle",
			in:       Inputs{LiquidMinor: 300_000, CommittedMinor: 400_000, BenchmarkAPRPercent: 5},
			wantIdle: 0,
			wantFlag: false,
		},
		{
			name:       "idle but no benchmark -> no forgone, no flag",
			in:         Inputs{LiquidMinor: 1_000_000, CommittedMinor: 100_000, BenchmarkAPRPercent: 0},
			wantIdle:   900_000,
			wantAnnual: 0,
			wantFlag:   false,
		},
		{
			name:     "idle below threshold -> no flag",
			in:       Inputs{LiquidMinor: 440_000, CommittedMinor: 400_000, BenchmarkAPRPercent: 4},
			wantIdle: 40_000,
			wantFlag: false, // 40000 < DefaultThresholdMinor (50000)
		},
		{
			name:     "negative committed treated as zero",
			in:       Inputs{LiquidMinor: 100_000, CommittedMinor: -999, BenchmarkAPRPercent: 1, ThresholdMinor: -1},
			wantIdle: 100_000,
			wantFlag: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.in)
			if got.IdleMinor != tt.wantIdle {
				t.Errorf("IdleMinor = %d, want %d", got.IdleMinor, tt.wantIdle)
			}
			if tt.wantAnnual != 0 && got.ForgoneAnnualMinor != tt.wantAnnual {
				t.Errorf("ForgoneAnnualMinor = %d, want %d", got.ForgoneAnnualMinor, tt.wantAnnual)
			}
			if tt.wantMonthly != 0 && got.ForgoneMonthlyMinor != tt.wantMonthly {
				t.Errorf("ForgoneMonthlyMinor = %d, want %d", got.ForgoneMonthlyMinor, tt.wantMonthly)
			}
			if got.Flag != tt.wantFlag {
				t.Errorf("Flag = %v, want %v", got.Flag, tt.wantFlag)
			}
		})
	}
}
