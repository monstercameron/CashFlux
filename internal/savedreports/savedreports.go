// SPDX-License-Identifier: MIT

// Package savedreports manages named report configurations: a report view's
// period window (resolution + anchors) and its report-local scope, saved under
// a user-chosen name so a curated view ("Q2 · Priya's accounts") reopens in one
// click. List semantics only — persistence is the caller's (KV JSON).
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package savedreports

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/scope"
)

// Max is the saved-view cap: enough for a power user's shelf, small enough to
// stay a picker rather than a database.
const Max = 20

// Saved is one named report configuration.
type Saved struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Res  string    `json:"res"` // period.Resolution
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// Scope is the report-local scope (empty = follow the app lens).
	Scope scope.ReportScope `json:"scope"`
}

// Add appends s to the list, replacing any existing entry with the same name
// (case-insensitive trim — saving "June review" twice updates it rather than
// growing a twin), and caps the list at Max by dropping the OLDEST entries.
// Returns a new slice; the input is never mutated.
func Add(list []Saved, s Saved) []Saved {
	key := strings.ToLower(strings.TrimSpace(s.Name))
	out := make([]Saved, 0, len(list)+1)
	for _, e := range list {
		if strings.ToLower(strings.TrimSpace(e.Name)) != key {
			out = append(out, e)
		}
	}
	out = append(out, s)
	if len(out) > Max {
		out = out[len(out)-Max:]
	}
	return out
}

// Remove deletes the entry with the given id. Returns a new slice.
func Remove(list []Saved, id string) []Saved {
	out := make([]Saved, 0, len(list))
	for _, e := range list {
		if e.ID != id {
			out = append(out, e)
		}
	}
	return out
}

// ByID finds a saved view by id.
func ByID(list []Saved, id string) (Saved, bool) {
	for _, e := range list {
		if e.ID == id {
			return e, true
		}
	}
	return Saved{}, false
}
