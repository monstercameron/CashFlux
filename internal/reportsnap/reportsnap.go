// SPDX-License-Identifier: MIT

// Package reportsnap models report snapshots: a report's headline aggregates
// frozen at the moment they were taken, so a month-end state stays inspectable
// after the live numbers move on (late imports, edits, recategorizations).
// List semantics only — persistence is the caller's (KV JSON).
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package reportsnap

import "time"

// Max caps the stored snapshots (two years of monthly closes).
const Max = 24

// LabelAmount is one frozen breakdown line (minor units).
type LabelAmount struct {
	Label  string `json:"label"`
	Amount int64  `json:"amount"`
}

// Snapshot is one frozen report state.
type Snapshot struct {
	ID          string        `json:"id"`
	TakenAt     time.Time     `json:"takenAt"`
	PeriodLabel string        `json:"periodLabel"`
	Base        string        `json:"base"`
	Income      int64         `json:"income"`  // minor units
	Expense     int64         `json:"expense"` // minor units
	Categories  []LabelAmount `json:"categories,omitempty"`
	Payees      []LabelAmount `json:"payees,omitempty"`
}

// Net is the snapshot's frozen income − expense.
func (s Snapshot) Net() int64 { return s.Income - s.Expense }

// Add appends s (newest last) and caps the list at Max, dropping the oldest.
// Returns a new slice; the input is never mutated.
func Add(list []Snapshot, s Snapshot) []Snapshot {
	out := make([]Snapshot, 0, len(list)+1)
	out = append(out, list...)
	out = append(out, s)
	if len(out) > Max {
		out = out[len(out)-Max:]
	}
	return out
}

// Remove deletes the snapshot with the given id. Returns a new slice.
func Remove(list []Snapshot, id string) []Snapshot {
	out := make([]Snapshot, 0, len(list))
	for _, e := range list {
		if e.ID != id {
			out = append(out, e)
		}
	}
	return out
}

// ByID finds a snapshot by id.
func ByID(list []Snapshot, id string) (Snapshot, bool) {
	for _, e := range list {
		if e.ID == id {
			return e, true
		}
	}
	return Snapshot{}, false
}

// TopN keeps the first n lines of a breakdown (callers pass rank-ordered data).
func TopN(lines []LabelAmount, n int) []LabelAmount {
	if len(lines) <= n {
		return lines
	}
	return lines[:n]
}
