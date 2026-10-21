// SPDX-License-Identifier: MIT

// Package settingsinput contains native validation for numeric settings inputs.
package settingsinput

import (
	"math"
	"strconv"
	"strings"
)

// ParseFXRate parses a positive finite FX rate. Invalid, zero, or negative input
// returns zero, which is the Settings UI's existing signal to clear that rate.
func ParseFXRate(raw string) float64 {
	rate, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return NormalizeFXRate(rate)
}

// NormalizeFXRate returns a positive finite rate unchanged and zero otherwise.
func NormalizeFXRate(rate float64) float64 {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0
	}
	return rate
}

// ParseIdleCashBenchmarkAPR parses a finite, nonnegative benchmark percentage.
// Invalid, negative, or non-finite input returns zero, disabling the benchmark.
func ParseIdleCashBenchmarkAPR(raw string) float64 {
	apr, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || apr < 0 || math.IsNaN(apr) || math.IsInf(apr, 0) {
		return 0
	}
	return apr
}
