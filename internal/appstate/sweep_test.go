// SPDX-License-Identifier: MIT

// Tests for the pure helpers in sweep.go (sweepAmount and sweepDue).
// No build tag — runs on native Go without syscall/js.
package appstate

import "testing"

func TestSweepAmount(t *testing.T) {
	tests := []struct {
		name   string
		liquid int64
		buffer int64
		want   int64
	}{
		{name: "surplus above buffer", liquid: 1000, buffer: 200, want: 800},
		{name: "exactly at buffer", liquid: 500, buffer: 500, want: 0},
		{name: "below buffer", liquid: 100, buffer: 500, want: 0},
		{name: "zero buffer sweeps all", liquid: 1500, buffer: 0, want: 1500},
		{name: "zero liquid and zero buffer", liquid: 0, buffer: 0, want: 0},
		{name: "negative liquid (overdraft)", liquid: -50, buffer: 0, want: 0},
		{name: "large surplus", liquid: 1_000_000_00, buffer: 50_000_00, want: 950_000_00},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := sweepAmount(tc.liquid, tc.buffer)
			if got != tc.want {
				t.Errorf("sweepAmount(%d, %d) = %d; want %d", tc.liquid, tc.buffer, got, tc.want)
			}
		})
	}
}

func TestSweepDue(t *testing.T) {
	tests := []struct {
		name       string
		lastPeriod string
		nowKey     string
		want       bool
	}{
		{name: "never swept (empty lastPeriod)", lastPeriod: "", nowKey: "2026-06", want: true},
		{name: "swept in a prior month", lastPeriod: "2026-05", nowKey: "2026-06", want: true},
		{name: "already swept this month", lastPeriod: "2026-06", nowKey: "2026-06", want: false},
		{name: "same strings", lastPeriod: "2025-01", nowKey: "2025-01", want: false},
		{name: "different year", lastPeriod: "2025-12", nowKey: "2026-01", want: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := sweepDue(tc.lastPeriod, tc.nowKey)
			if got != tc.want {
				t.Errorf("sweepDue(%q, %q) = %v; want %v", tc.lastPeriod, tc.nowKey, got, tc.want)
			}
		})
	}
}
