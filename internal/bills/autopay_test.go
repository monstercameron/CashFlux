// SPDX-License-Identifier: MIT

package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestUpcomingAllPropagatesAutopay guards C154/C157: a recurring bill flagged
// Autopay must carry that flag onto its derived Bill (so the Bills screen can show
// an "Autopay" badge), and a non-autopay recurring must not.
func TestUpcomingAllPropagatesAutopay(t *testing.T) {
	now := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	recs := []domain.Recurring{
		{ID: "rent", Label: "Rent", Amount: money.New(-120000, "USD"), Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 0, 5), Autopay: true},
		{ID: "gym", Label: "Gym", Amount: money.New(-5000, "USD"), Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 0, 6), Autopay: false},
	}
	out := UpcomingAll(nil, recs, now)
	got := map[string]bool{}
	for _, b := range out {
		got[b.Name] = b.Autopay
	}
	if !got["Rent"] {
		t.Errorf("Rent bill Autopay = false, want true")
	}
	if got["Gym"] {
		t.Errorf("Gym bill Autopay = true, want false")
	}
}
