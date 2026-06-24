// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSpendingByWeekday(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	// 2026-06-01 is a Monday; 06-05 is a Friday; 06-06 is a Saturday.
	txns := []domain.Transaction{
		expense("food", 100, dt(2026, time.June, 1)),                                            // Monday
		expense("food", 50, dt(2026, time.June, 8)),                                             // Monday
		expense("fun", 200, dt(2026, time.June, 5)),                                             // Friday
		expense("fun", 300, dt(2026, time.May, 29)),                                             // out of range — excluded
		{Amount: money.New(9999, "USD"), Date: dt(2026, time.June, 5)},                          // income Friday — excluded
		{Amount: money.New(-1000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 5)}, // transfer — excluded
	}
	totals, err := SpendingByWeekday(txns, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if totals[time.Monday] != 15000 {
		t.Errorf("Monday = %d, want 15000", totals[time.Monday])
	}
	if totals[time.Friday] != 20000 {
		t.Errorf("Friday = %d, want 20000", totals[time.Friday])
	}
	if totals[time.Wednesday] != 0 {
		t.Errorf("Wednesday = %d, want 0", totals[time.Wednesday])
	}
}

func TestPeakWeekday(t *testing.T) {
	var totals [7]int64
	if _, ok := PeakWeekday(totals); ok {
		t.Error("all-zero should report ok=false")
	}
	totals[time.Monday] = 15000
	totals[time.Friday] = 20000
	d, ok := PeakWeekday(totals)
	if !ok || d != time.Friday {
		t.Errorf("peak = %v ok=%v, want Friday true", d, ok)
	}
}

func TestPeakWeekdayTieEarliest(t *testing.T) {
	var totals [7]int64
	totals[time.Tuesday] = 5000
	totals[time.Thursday] = 5000
	d, ok := PeakWeekday(totals)
	if !ok || d != time.Tuesday {
		t.Errorf("tie peak = %v, want Tuesday (earliest)", d)
	}
}
