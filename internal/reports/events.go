// SPDX-License-Identifier: MIT

package reports

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// EventsIn returns the life events whose half-open range [Start, End)
// intersects the report window [from, to) — the annotations a time-scoped
// report should surface ("why does April look like that? — the Portugal
// trip"). Open-ended events intersect every window from their Start on.
// Input order is preserved.
func EventsIn(events []domain.Event, from, to time.Time) []domain.Event {
	var out []domain.Event
	for _, e := range events {
		if !e.Start.Before(to) {
			continue // starts at/after the window's end
		}
		if !e.End.IsZero() && !e.End.After(from) {
			continue // ended at/before the window's start
		}
		out = append(out, e)
	}
	return out
}
