// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"testing"
	"time"
)

func monthlyDates(start time.Time, n int) []time.Time {
	return stepSeries(start, n, func(t time.Time) time.Time { return t.AddDate(0, 1, 0) })
}

func TestAnalyzeCostFixed(t *testing.T) {
	amounts := []int64{7500, 7500, 7500, 7500}
	got := analyzeCost(amounts, monthlyDates(d(2026, 1, 9), 4))
	if !got.coherent || got.model.Kind != AmountFixed {
		t.Fatalf("kind = %v coherent=%v, want fixed/true", got.model.Kind, got.coherent)
	}
	if got.model.Typical != 7500 {
		t.Errorf("typical = %d, want 7500", got.model.Typical)
	}
	if got.stability != 1 {
		t.Errorf("stability = %.2f, want 1", got.stability)
	}
}

func TestAnalyzeCostBanded(t *testing.T) {
	amounts := []int64{4200, 4550, 4100, 4380, 4260}
	got := analyzeCost(amounts, monthlyDates(d(2026, 1, 9), 5))
	if !got.coherent || got.model.Kind != AmountBanded {
		t.Fatalf("kind = %v coherent=%v, want banded/true", got.model.Kind, got.coherent)
	}
	if got.model.Typical < 4100 || got.model.Typical > 4550 {
		t.Errorf("typical %d outside observed range", got.model.Typical)
	}
}

// TestAnalyzeCostStepped confirms one durable price change is a single stepped
// model carrying the from→to creep signal, not a split.
func TestAnalyzeCostStepped(t *testing.T) {
	amounts := []int64{1310, 1310, 1310, 1440, 1440, 1440}
	dates := monthlyDates(d(2026, 1, 9), 6)
	got := analyzeCost(amounts, dates)
	if !got.coherent || got.model.Kind != AmountStepped {
		t.Fatalf("kind = %v coherent=%v, want stepped/true", got.model.Kind, got.coherent)
	}
	if got.model.Step == nil {
		t.Fatal("stepped model missing PriceStep")
	}
	if got.model.Step.FromMinor != 1310 || got.model.Step.ToMinor != 1440 {
		t.Errorf("step = %d→%d, want 1310→1440", got.model.Step.FromMinor, got.model.Step.ToMinor)
	}
	if !got.model.Step.At.Equal(dates[3]) {
		t.Errorf("step date = %v, want %v", got.model.Step.At, dates[3])
	}
	if got.model.Typical != 1440 {
		t.Errorf("typical = %d, want current level 1440", got.model.Typical)
	}
}

// TestAnalyzeCostNoise confirms random amounts have no coherent central value and
// are rejected (the Venmo-payment case).
func TestAnalyzeCostNoise(t *testing.T) {
	amounts := []int64{2500, 800, 15000, 300, 4200, 9100}
	got := analyzeCost(amounts, monthlyDates(d(2026, 1, 1), 6))
	if got.coherent {
		t.Errorf("random amounts should be incoherent noise, got %+v", got.model)
	}
}

// TestSplitAmounts confirms a same-signature cluster holding two concurrent
// subscriptions at distinct levels splits into two groups, while a temporal price
// step does not.
func TestSplitAmounts(t *testing.T) {
	// Two interleaved levels ($30 membership + $10 locker), monthly.
	var two []Txn
	for i := 0; i < 4; i++ {
		day := d(2026, time.Month(1+i), 3)
		two = append(two, Txn{ID: "hi" + itoa(i), Date: day, AmountMinor: 3000, Direction: Out})
		two = append(two, Txn{ID: "lo" + itoa(i), Date: day, AmountMinor: 1000, Direction: Out})
	}
	// analyzeCost/splitAmounts wants chronological order.
	sortByDate(two)
	parts := splitAmounts(two, 3)
	if parts == nil || len(parts) != 2 {
		t.Fatalf("interleaved two-level cluster should split into 2, got %v", parts)
	}

	// A pure temporal step must NOT split.
	var step []Txn
	amts := []int64{1310, 1310, 1310, 1440, 1440, 1440}
	for i, a := range amts {
		step = append(step, Txn{ID: itoa(i), Date: d(2026, time.Month(1+i), 9), AmountMinor: a, Direction: Out})
	}
	if parts := splitAmounts(step, 3); parts != nil {
		t.Errorf("temporal step should not split, got %v", parts)
	}
}

func itoa(i int) string {
	return string(rune('a' + i))
}

func sortByDate(txns []Txn) {
	for i := 1; i < len(txns); i++ {
		for j := i; j > 0 && txns[j].Date.Before(txns[j-1].Date); j-- {
			txns[j], txns[j-1] = txns[j-1], txns[j]
		}
	}
}
