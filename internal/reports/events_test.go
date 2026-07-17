// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestEventsIn(t *testing.T) {
	d := func(m, day int) time.Time { return time.Date(2026, time.Month(m), day, 0, 0, 0, 0, time.UTC) }
	events := []domain.Event{
		{ID: "before", Start: d(1, 1), End: d(2, 1)},  // ends before the window
		{ID: "spans", Start: d(2, 15), End: d(8, 1)},  // spans the whole window
		{ID: "inside", Start: d(4, 5), End: d(4, 12)}, // fully inside
		{ID: "tail", Start: d(6, 20), End: d(7, 10)},  // starts inside, ends after
		{ID: "after", Start: d(7, 1), End: d(8, 1)},   // starts at the window's end
		{ID: "open", Start: d(3, 1)},                  // open-ended
		{ID: "openafter", Start: d(9, 1)},             // open-ended, starts after
		{ID: "abut", Start: d(2, 1), End: d(3, 1)},    // ends exactly at window start
	}
	got := EventsIn(events, d(3, 1), d(7, 1))
	want := []string{"spans", "inside", "tail", "open"}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("got[%d] = %s, want %s", i, got[i].ID, id)
		}
	}
}
