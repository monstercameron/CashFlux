package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func midnight(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestNoSpendDaysCurrentPeriod(t *testing.T) {
	start, end := midnight(2026, time.June, 1), midnight(2026, time.July, 1)
	now := dt(2026, time.June, 10) // 10 days elapsed (June 1..10)
	txns := []domain.Transaction{
		expense("food", 50, dt(2026, time.June, 3)),
		expense("food", 20, dt(2026, time.June, 5)),
		expense("food", 99, dt(2026, time.June, 5)),                                                             // same day as above → still one spend day
		expense("food", 99, dt(2026, time.June, 20)),                                                            // future day (after now) → not yet elapsed
		{CategoryID: "x", Amount: money.New(500, "USD"), Date: dt(2026, time.June, 7)},                          // income — not spending
		{CategoryID: "y", Amount: money.New(-700, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 8)}, // transfer — not spending
	}
	// 10 elapsed days, spend on June 3 and June 5 → 8 no-spend days.
	if got := NoSpendDays(txns, start, end, now); got != 8 {
		t.Errorf("NoSpendDays = %d, want 8", got)
	}
}

func TestNoSpendDaysPastFullMonth(t *testing.T) {
	start, end := midnight(2026, time.June, 1), midnight(2026, time.July, 1)
	now := midnight(2026, time.August, 1) // month fully elapsed
	txns := []domain.Transaction{
		expense("food", 50, dt(2026, time.June, 3)),
		expense("food", 20, dt(2026, time.June, 15)),
	}
	// June has 30 days, spend on 2 → 28 no-spend.
	if got := NoSpendDays(txns, start, end, now); got != 28 {
		t.Errorf("NoSpendDays = %d, want 28", got)
	}
}

func TestNoSpendDaysFuturePeriod(t *testing.T) {
	start, end := midnight(2026, time.August, 1), midnight(2026, time.September, 1)
	now := midnight(2026, time.June, 15) // before the window
	if got := NoSpendDays(nil, start, end, now); got != 0 {
		t.Errorf("future period = %d, want 0", got)
	}
}
