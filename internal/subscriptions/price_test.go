// SPDX-License-Identifier: MIT

package subscriptions

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestDetectPriceChangesIncrease(t *testing.T) {
	// Netflix: $15.99 for three months, then $17.99 for two.
	txns := []domain.Transaction{
		charge("Netflix", 1599, d(2026, time.January, 1)),
		charge("Netflix", 1599, d(2026, time.February, 1)),
		charge("Netflix", 1599, d(2026, time.March, 1)),
		charge("Netflix", 1799, d(2026, time.April, 1)),
		charge("Netflix", 1799, d(2026, time.May, 1)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1: %+v", len(changes), changes)
	}
	c := changes[0]
	if c.Name != "Netflix" || c.OldAmount != 1599 || c.NewAmount != 1799 {
		t.Errorf("change = %+v, want Netflix 1599→1799", c)
	}
	if c.Delta != 200 || !c.Increased() {
		t.Errorf("Delta = %d Increased=%v, want +200 increase", c.Delta, c.Increased())
	}
	if c.PercentChange != 13 { // 200/1599 = 12.5% → rounds to 13
		t.Errorf("PercentChange = %d, want 13", c.PercentChange)
	}
	if !c.ChangedAt.Equal(d(2026, time.April, 1)) {
		t.Errorf("ChangedAt = %s, want 2026-04-01", c.ChangedAt.Format("2006-01-02"))
	}
}

func TestDetectPriceChangesDecrease(t *testing.T) {
	txns := []domain.Transaction{
		charge("Spotify", 1099, d(2026, time.January, 5)),
		charge("Spotify", 1099, d(2026, time.February, 5)),
		charge("Spotify", 1099, d(2026, time.March, 5)),
		charge("Spotify", 999, d(2026, time.April, 5)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(changes))
	}
	c := changes[0]
	if c.Delta != -100 || c.Increased() {
		t.Errorf("Delta = %d Increased=%v, want -100 decrease", c.Delta, c.Increased())
	}
	if c.PercentChange != -9 { // -100/1099 = -9.1% → -9
		t.Errorf("PercentChange = %d, want -9", c.PercentChange)
	}
}

func TestDetectPriceChangesStablePrice(t *testing.T) {
	txns := []domain.Transaction{
		charge("Gym", 2500, d(2026, time.January, 1)),
		charge("Gym", 2500, d(2026, time.February, 1)),
		charge("Gym", 2500, d(2026, time.March, 1)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("a stable price should report no change, got %+v", changes)
	}
}

func TestDetectPriceChangesNeedsCadence(t *testing.T) {
	// Three irregular charges (no monthly/weekly/yearly cadence) → not a subscription.
	txns := []domain.Transaction{
		charge("Coffee", 500, d(2026, time.January, 1)),
		charge("Coffee", 500, d(2026, time.January, 3)),
		charge("Coffee", 700, d(2026, time.January, 4)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("irregular spacing should be ignored, got %+v", changes)
	}
}

func TestDetectPriceChangesMinCountFloor(t *testing.T) {
	// Two charges can't distinguish a change from a one-off; minCount floors at 3.
	txns := []domain.Transaction{
		charge("News", 800, d(2026, time.January, 1)),
		charge("News", 900, d(2026, time.February, 1)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 2) // asked for 2, clamped to 3
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("two charges should not yield a change, got %+v", changes)
	}
}

func TestDetectPriceChangesSortedMostRecentFirst(t *testing.T) {
	txns := []domain.Transaction{
		// Older change (Feb).
		charge("A", 100, d(2026, time.January, 1)),
		charge("A", 100, d(2026, time.February, 1)),
		charge("A", 120, d(2026, time.March, 1)),
		charge("A", 120, d(2026, time.April, 1)),
		// Newer change (May).
		charge("B", 200, d(2026, time.February, 1)),
		charge("B", 200, d(2026, time.March, 1)),
		charge("B", 200, d(2026, time.April, 1)),
		charge("B", 250, d(2026, time.May, 1)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("got %d changes, want 2", len(changes))
	}
	if changes[0].Name != "B" || changes[1].Name != "A" {
		t.Errorf("order = %q, %q; want B (May) then A (Mar)", changes[0].Name, changes[1].Name)
	}
}

// ─── C347: a price alert about variable spending is noise ────────────────────

// TestVariableSpendIsNotAPriceChange is the ticket's exact complaint: the
// price tracker reported "Date night went up 9%" by comparing two arbitrary
// evenings out.
func TestVariableSpendIsNotAPriceChange(t *testing.T) {
	txns := []domain.Transaction{
		charge("Date night", 8800, d(2026, time.January, 12)),
		charge("Date night", 9500, d(2026, time.February, 14)),
		charge("Date night", 10200, d(2026, time.March, 13)),
		charge("Date night", 9600, d(2026, time.April, 11)),
		charge("Date night", 10400, d(2026, time.May, 9)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("a wandering amount reported %d price changes (%+v) — there was never a "+
			"settled price for it to change FROM (C347)", len(changes), changes)
	}
}

// A genuine hike still reports, and carries the evidence that it was a step.
func TestRealHikeReportsItsRuns(t *testing.T) {
	txns := []domain.Transaction{
		charge("Netflix", 1549, d(2025, time.November, 11)),
		charge("Netflix", 1549, d(2025, time.December, 11)),
		charge("Netflix", 1549, d(2026, time.January, 11)),
		charge("Netflix", 1799, d(2026, time.February, 11)),
		charge("Netflix", 1799, d(2026, time.March, 11)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(changes))
	}
	c := changes[0]
	if c.OldRun != 3 || c.NewRun != 2 {
		t.Errorf("runs = old %d / new %d, want 3 / 2 — the runs ARE the evidence that "+
			"this was a step rather than a wobble", c.OldRun, c.NewRun)
	}
	if !c.Increased() || c.Delta != 250 {
		t.Errorf("change = %+v, want +250", c)
	}
}

// The new side is deliberately not held to the same bar: three charges at one
// price and one at another has genuinely changed, and making the user wait a
// second cycle would trade a real alert for a rule.
func TestASingleChargeAtANewPriceStillCounts(t *testing.T) {
	txns := []domain.Transaction{
		charge("Spotify", 1099, d(2026, time.January, 5)),
		charge("Spotify", 1099, d(2026, time.February, 5)),
		charge("Spotify", 1099, d(2026, time.March, 5)),
		charge("Spotify", 1199, d(2026, time.April, 5)),
	}
	changes, err := DetectPriceChanges(txns, usd(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changes) != 1 || changes[0].NewRun != 1 || changes[0].OldRun != 3 {
		t.Errorf("changes = %+v, want one change with old run 3 / new run 1", changes)
	}
}
