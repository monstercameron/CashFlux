// SPDX-License-Identifier: MIT

// Package navorder is the pure ordering model for the user-customizable sidebar:
// a saved sequence of nav ids (paths) plus the operations to reorder it and to
// apply it to the live nav list. It has no platform dependencies, so it unit-
// tests on native Go; the wasm UI persists the sequence and wires drag-reorder.
package navorder

// indexOf returns the position of id in order, or -1.
func indexOf(order []string, id string) int {
	for i, v := range order {
		if v == id {
			return i
		}
	}
	return -1
}

// Move returns a copy of order with id relocated to toIndex (clamped to the
// valid range), preserving the relative order of the others — the reorder a
// drag-and-drop produces. An unknown id returns an unchanged copy. The input is
// not modified.
func Move(order []string, id string, toIndex int) []string {
	from := indexOf(order, id)
	out := append([]string(nil), order...)
	if from < 0 {
		return out
	}
	if toIndex < 0 {
		toIndex = 0
	}
	if toIndex >= len(out) {
		toIndex = len(out) - 1
	}
	if toIndex == from {
		return out
	}
	moved := out[from]
	out = append(out[:from], out[from+1:]...)
	out = append(out, "")
	copy(out[toIndex+1:], out[toIndex:])
	out[toIndex] = moved
	return out
}

// Apply orders current to follow saved: ids present in saved come first in
// saved's order, then any remaining current ids in their original order (so a
// newly-added screen appends rather than disappearing). ids in saved but absent
// from current are dropped (a removed/hidden screen). The input is not modified.
// Deterministic: same saved + current → same result.
func Apply(saved, current []string) []string {
	inCurrent := make(map[string]bool, len(current))
	for _, id := range current {
		inCurrent[id] = true
	}
	used := make(map[string]bool, len(current))
	out := make([]string, 0, len(current))
	for _, id := range saved {
		if inCurrent[id] && !used[id] {
			out = append(out, id)
			used[id] = true
		}
	}
	for _, id := range current {
		if !used[id] {
			out = append(out, id)
			used[id] = true
		}
	}
	return out
}
