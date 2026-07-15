// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestEventCRUDAndRoundTrip(t *testing.T) {
	s := newStore(t)
	e := domain.Event{
		ID:        "ev1",
		Name:      "Portugal trip",
		Start:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
		Note:      "two weeks in Lisbon + Porto",
		Icon:      "plane",
		CreatedAt: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
	}
	if err := s.PutEvent(e); err != nil {
		t.Fatalf("PutEvent: %v", err)
	}
	got, err := s.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Portugal trip" || got[0].Icon != "plane" {
		t.Fatalf("events = %+v", got)
	}

	// Export/import lossless.
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Events) != 1 {
		t.Fatalf("snapshot events = %d, want 1", len(snap.Events))
	}
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	g2, _ := s2.ListEvents()
	if len(g2) != 1 || !g2[0].End.Equal(e.End) || g2[0].Note != e.Note {
		t.Fatalf("round-trip mismatch: %+v", g2)
	}

	if ok, err := s.DeleteEvent("ev1"); err != nil || !ok {
		t.Fatalf("DeleteEvent: ok=%v err=%v", ok, err)
	}
	if g3, _ := s.ListEvents(); len(g3) != 0 {
		t.Fatalf("event not deleted: %+v", g3)
	}
}
