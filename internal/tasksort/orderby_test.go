// SPDX-License-Identifier: MIT

package tasksort

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func tDue(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func sample() []domain.Task {
	return []domain.Task{
		{Title: "Zebra", Status: domain.StatusOpen, Priority: domain.PriorityLow, Due: tDue(2026, 7, 1)},
		{Title: "apple", Status: domain.StatusOpen, Priority: domain.PriorityHigh},
		{Title: "Mango", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Due: tDue(2026, 6, 1)},
		{Title: "Done thing", Status: domain.StatusDone, Priority: domain.PriorityHigh, Due: tDue(2026, 5, 1)},
	}
}

func titles(ts []domain.Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Title
	}
	return out
}

func openFirst(t *testing.T, ts []domain.Task) {
	t.Helper()
	seenDone := false
	for _, x := range ts {
		if x.Status == domain.StatusDone {
			seenDone = true
		} else if seenDone {
			t.Fatalf("an open task came after a done one: %v", titles(ts))
		}
	}
}

func TestOrderByPriority(t *testing.T) {
	got := OrderBy(sample(), ModePriority)
	openFirst(t, got)
	// Among open: High (apple) → Medium (Mango) → Low (Zebra).
	want := []string{"apple", "Mango", "Zebra", "Done thing"}
	for i := range want {
		if got[i].Title != want[i] {
			t.Fatalf("priority order = %v, want %v", titles(got), want)
		}
	}
}

func TestOrderByAZ(t *testing.T) {
	got := OrderBy(sample(), ModeAZ)
	openFirst(t, got)
	// Case-insensitive: apple, Mango, Zebra (open), then done.
	want := []string{"apple", "Mango", "Zebra", "Done thing"}
	for i := range want {
		if got[i].Title != want[i] {
			t.Fatalf("A–Z order = %v, want %v", titles(got), want)
		}
	}
}

func TestOrderByDue(t *testing.T) {
	got := OrderBy(sample(), ModeDue)
	openFirst(t, got)
	// Dated before undated, earliest first: Mango (Jun) → Zebra (Jul) → apple (none).
	want := []string{"Mango", "Zebra", "apple", "Done thing"}
	for i := range want {
		if got[i].Title != want[i] {
			t.Fatalf("Due order = %v, want %v", titles(got), want)
		}
	}
}

func TestOrderBySmartMatchesOrder(t *testing.T) {
	a := OrderBy(sample(), ModeSmart)
	b := Order(sample())
	for i := range a {
		if a[i].Title != b[i].Title {
			t.Fatalf("ModeSmart != Order: %v vs %v", titles(a), titles(b))
		}
	}
}

func TestParseModeDefaults(t *testing.T) {
	if ParseMode("nonsense") != ModeSmart {
		t.Fatal("unknown mode should default to Smart")
	}
	if ParseMode("priority") != ModePriority {
		t.Fatal("priority should parse")
	}
}

func TestOrderByDoesNotMutate(t *testing.T) {
	in := sample()
	_ = OrderBy(in, ModePriority)
	if in[0].Title != "Zebra" {
		t.Fatal("OrderBy mutated its input")
	}
}
