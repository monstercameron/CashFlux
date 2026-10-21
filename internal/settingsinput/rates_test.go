// SPDX-License-Identifier: MIT

package settingsinput

import (
	"math"
	"testing"
)

func TestParseFXRate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{name: "trimmed positive", in: " 1.25 ", want: 1.25},
		{name: "zero clears", in: "0", want: 0},
		{name: "negative clears", in: "-2", want: 0},
		{name: "invalid clears", in: "one", want: 0},
		{name: "blank clears", in: "  ", want: 0},
		{name: "nan clears", in: "NaN", want: 0},
		{name: "infinity clears", in: "+Inf", want: 0},
		{name: "overflow clears", in: "1e999", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseFXRate(tt.in); got != tt.want {
				t.Fatalf("ParseFXRate(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeFXRate(t *testing.T) {
	for _, in := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := NormalizeFXRate(in); got != 0 {
			t.Errorf("NormalizeFXRate(%v) = %v, want 0", in, got)
		}
	}
	if got := NormalizeFXRate(0.75); got != 0.75 {
		t.Fatalf("NormalizeFXRate(0.75) = %v, want 0.75", got)
	}
}

func TestParseIdleCashBenchmarkAPR(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{name: "trimmed positive", in: " 4.375 ", want: 4.375},
		{name: "zero disables", in: "0", want: 0},
		{name: "negative disables", in: "-1", want: 0},
		{name: "invalid disables", in: "four", want: 0},
		{name: "blank disables", in: "", want: 0},
		{name: "nan disables", in: "NaN", want: 0},
		{name: "infinity disables", in: "Inf", want: 0},
		{name: "overflow disables", in: "1e999", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseIdleCashBenchmarkAPR(tt.in); got != tt.want {
				t.Fatalf("ParseIdleCashBenchmarkAPR(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
