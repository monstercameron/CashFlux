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
