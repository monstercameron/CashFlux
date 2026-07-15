// SPDX-License-Identifier: MIT

// Package whatif provides the pure diff primitive for the what-if sandbox (AG2):
// given the engine's formula-variable surface BEFORE and AFTER a hypothetical
// change (two map[string]float64 snapshots produced by liveEngineVars over the real
// dataset and a workspace copy), it reports which variables moved and by how much.
//
// This is the explainable core of the sandbox: the diff is a vars-to-vars
// comparison, so "what if I moved to a $1,800 apartment?" becomes a concrete list
// of moved figures (net worth, runway, goal ETAs) with no black box. The dataset-
// copy machinery and the UI live elsewhere; this package only computes the delta.
//
// Pure Go, no syscall/js: unit-tested on native Go.
package whatif

import (
	"math"
	"sort"
)

// Change is one formula variable that differs between the baseline and the
// scenario. Added is true when the variable exists only in the scenario, Removed
// when only in the baseline; otherwise it changed value. Delta is After - Before
// (0 for pure add/remove where a side is absent).
type Change struct {
	Name    string
	Before  float64
	After   float64
	Delta   float64
	Added   bool
	Removed bool
}

// Diff compares two variable snapshots and returns the changed variables sorted by
// descending absolute delta (the figures the scenario moves most first), ties
// broken by name for determinism. Values within epsilon are treated as unchanged so
// float noise doesn't surface as a change; pass epsilon 0 for exact comparison.
func Diff(before, after map[string]float64, epsilon float64) []Change {
	if epsilon < 0 {
		epsilon = -epsilon
	}
	seen := make(map[string]bool, len(before)+len(after))
	var out []Change
	for name, b := range before {
		seen[name] = true
		a, ok := after[name]
		if !ok {
			out = append(out, Change{Name: name, Before: b, After: 0, Delta: -b, Removed: true})
			continue
		}
		if math.Abs(a-b) > epsilon {
			out = append(out, Change{Name: name, Before: b, After: a, Delta: a - b})
		}
	}
	for name, a := range after {
		if seen[name] {
			continue
		}
		out = append(out, Change{Name: name, Before: 0, After: a, Delta: a, Added: true})
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := math.Abs(out[i].Delta), math.Abs(out[j].Delta)
		if di != dj {
			return di > dj
		}
		return out[i].Name < out[j].Name
	})
	return out
}
