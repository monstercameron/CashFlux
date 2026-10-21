// SPDX-License-Identifier: MIT

package nwchange

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCheckedDeltaAndPercentPreserveValidResults(t *testing.T) {
	c := Change{StartMinor: 100, EndMinor: 150, Known: true}
	if got, err := c.DeltaMinorChecked(); err != nil || got != 50 {
		t.Fatalf("DeltaMinorChecked = %d, %v; want 50, nil", got, err)
	}
	if got, ok, err := c.PercentChangeChecked(); err != nil || !ok || got != 50 {
		t.Fatalf("PercentChangeChecked = %d, %v, %v; want 50, true, nil", got, ok, err)
	}
	negative := Change{StartMinor: -100, EndMinor: -50, Known: true}
	if got, ok, err := negative.PercentChangeChecked(); err != nil || !ok || got != 50 {
		t.Fatalf("negative baseline percent = %d, %v, %v; want 50, true, nil", got, ok, err)
	}
}

func TestCheckedDeltaAndPercentRejectOverflow(t *testing.T) {
	c := Change{StartMinor: math.MinInt64, EndMinor: math.MaxInt64, Known: true}
	if _, err := c.DeltaMinorChecked(); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("DeltaMinorChecked error = %v; want overflow", err)
	}
	if _, ok, err := c.PercentChangeChecked(); !errors.Is(err, money.ErrOverflow) || ok {
		t.Fatalf("PercentChangeChecked = ok %v, err %v; want overflow and unavailable", ok, err)
	}
	baseline := Change{StartMinor: math.MinInt64, EndMinor: math.MinInt64 + 1, Known: true}
	if _, ok, err := baseline.PercentChangeChecked(); !errors.Is(err, money.ErrOverflow) || ok {
		t.Fatalf("MinInt64 baseline percent = ok %v, err %v; want overflow and unavailable", ok, err)
	}
}

func TestCheckedChangeDistinguishesZeroBaselineAndUnknown(t *testing.T) {
	zero := Change{StartMinor: 0, EndMinor: 10, Known: true}
	if got, ok, err := zero.PercentChangeChecked(); got != 0 || ok || err != nil {
		t.Fatalf("zero-baseline percent = %d, %v, %v; want 0, false, nil", got, ok, err)
	}
	if _, err := (Change{}).DeltaMinorChecked(); !errors.Is(err, ErrUnknownChange) {
		t.Fatalf("unknown delta error = %v; want ErrUnknownChange", err)
	}
}
