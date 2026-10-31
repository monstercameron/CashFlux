// SPDX-License-Identifier: MIT

package idlecash

import (
	"math"
	"testing"
)

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

func TestEvaluateUsesExactDecimalBenchmarkAndSaturates(t *testing.T) {
	tests := []struct {
		name       string
		idle       int64
		bench      float64
		wantAnnual int64
	}{
		{"4.1 percent is exact, not its binary neighbour", 100_000, 4.1, 4_100},
		{"1.1 percent of 1000000", 1_000_000, 1.1, 11_000},
		{"fractions truncate toward zero", 999, 4.5, 44},
		{"tiny benchmark", 100, 0.001, 0},
		{"large idle keeps full precision", 9_007_199_254_740_993, 100, 9_007_199_254_740_993},
		{"overflowing product saturates", math.MaxInt64, 1000, math.MaxInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(Inputs{LiquidMinor: tt.idle, BenchmarkAPRPercent: tt.bench, ThresholdMinor: -1})
			if got.ForgoneAnnualMinor != tt.wantAnnual {
				t.Fatalf("ForgoneAnnualMinor = %d, want %d", got.ForgoneAnnualMinor, tt.wantAnnual)
			}
		})
	}
}

func TestEvaluateRejectsNonFiniteBenchmark(t *testing.T) {
	for _, bench := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -3} {
		got := Evaluate(Inputs{LiquidMinor: 1_000_000, BenchmarkAPRPercent: bench})
		if got.Flag || got.ForgoneAnnualMinor != 0 || got.BenchmarkAPRPercent != 0 {
			t.Fatalf("benchmark %v produced %+v, want a disabled figure", bench, got)
		}
	}
}

func TestEvaluateThresholdContract(t *testing.T) {
	tests := []struct {
		name      string
		liquid    int64
		committed int64
		threshold int64
		wantFlag  bool
	}{
		{"zero threshold uses the default", 50_000, 0, 0, true},
		{"just below the default", 49_999, 0, 0, false},
		{"negative threshold flags any positive idle", 1, 0, -1, true},
		{"negative threshold never flags zero idle", 100, 100, -1, false},
		{"negative threshold never flags overcommitted cash", 100, 500, -1, false},
		{"explicit threshold is inclusive", 1_000, 0, 1_000, true},
		{"explicit threshold excludes below", 999, 0, 1_000, false},
		{"huge negative liquid does not wrap", math.MinInt64, 1, -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(Inputs{LiquidMinor: tt.liquid, CommittedMinor: tt.committed, BenchmarkAPRPercent: 4, ThresholdMinor: tt.threshold})
			if got.Flag != tt.wantFlag {
				t.Fatalf("Flag = %v, want %v (idle %d)", got.Flag, tt.wantFlag, got.IdleMinor)
			}
		})
	}
}
