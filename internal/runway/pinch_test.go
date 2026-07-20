// SPDX-License-Identifier: MIT

package runway

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// mkRec builds a USD recurring via the package-shared rec helper (runway_test.go).
func mkRec(label string, minor int64, cadence domain.RecurringCadence, nextDue time.Time) domain.Recurring {
	return rec(label, minor, "USD", cadence, nextDue)
}

func TestTideline(t *testing.T) {
	base := currency.Rates{Base: "USD"}
	from := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		liquidStart    int64
		recs           []domain.Recurring
		wantWindow     int
		wantHasIncome  bool
		wantNextIncome int
		wantPinchNeg   bool
		checkPinch     func(t *testing.T, p Pinch)
	}{
		{
			name:        "income anchors window, positive pinch",
			liquidStart: 200000,
			recs: []domain.Recurring{
				mkRec("rent", -50000, domain.CadenceMonthly, from.AddDate(0, 0, 5)),
				mkRec("pay", 250000, domain.CadenceBiweekly, from.AddDate(0, 0, 14)),
			},
			wantWindow:     minPinchWindowDays,
			wantHasIncome:  true,
			wantNextIncome: 14,
			wantPinchNeg:   false,
			checkPinch: func(t *testing.T, p Pinch) {
				if p.AmountMinor != 150000 { // 200000 - 50000 rent on day 5
					t.Errorf("pinch amount = %d, want 150000", p.AmountMinor)
				}
				if p.Day != 5 {
					t.Errorf("pinch day = %d, want 5", p.Day)
				}
			},
		},
		{
			name:        "negative pinch before payday",
			liquidStart: 50000,
			recs: []domain.Recurring{
				mkRec("car", -80000, domain.CadenceMonthly, from.AddDate(0, 0, 5)),
				mkRec("pay", 200000, domain.CadenceBiweekly, from.AddDate(0, 0, 14)),
			},
			wantWindow:     minPinchWindowDays,
			wantHasIncome:  true,
			wantNextIncome: 14,
			wantPinchNeg:   true,
			checkPinch: func(t *testing.T, p Pinch) {
				if p.AmountMinor != -30000 {
					t.Errorf("pinch amount = %d, want -30000", p.AmountMinor)
				}
				if !p.Negative {
					t.Error("expected negative pinch")
				}
			},
		},
		{
			name:        "no income degrades to 30-day window",
			liquidStart: 100000,
			recs: []domain.Recurring{
				mkRec("gym", -5000, domain.CadenceMonthly, from.AddDate(0, 0, 10)),
			},
			wantWindow:     fallbackPinchWindowDays,
			wantHasIncome:  false,
			wantNextIncome: -1,
			wantPinchNeg:   false,
		},
		{
			name:        "imminent income floored to minimum window",
			liquidStart: 100000,
			recs: []domain.Recurring{
				mkRec("pay", 120000, domain.CadenceWeekly, from.AddDate(0, 0, 3)),
			},
			wantWindow:     minPinchWindowDays, // income day 3 floored to a full cycle
			wantHasIncome:  true,
			wantNextIncome: 3,
			wantPinchNeg:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := Tideline(tt.liquidStart, tt.recs, from, base)
			if err != nil {
				t.Fatalf("Tideline: %v", err)
			}
			if pc.WindowDays != tt.wantWindow {
				t.Errorf("window = %d, want %d", pc.WindowDays, tt.wantWindow)
			}
			if pc.HasIncome != tt.wantHasIncome {
				t.Errorf("hasIncome = %v, want %v", pc.HasIncome, tt.wantHasIncome)
			}
			if pc.NextIncomeDay != tt.wantNextIncome {
				t.Errorf("nextIncomeDay = %d, want %d", pc.NextIncomeDay, tt.wantNextIncome)
			}
			if pc.Pinch.Negative != tt.wantPinchNeg {
				t.Errorf("pinch negative = %v, want %v", pc.Pinch.Negative, tt.wantPinchNeg)
			}
			if len(pc.Cushion) != tt.wantWindow {
				t.Errorf("cushion curve has %d days, want %d", len(pc.Cushion), tt.wantWindow)
			}
			// Pinch date must equal from's day + pinch day offset.
			wantDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location()).AddDate(0, 0, pc.Pinch.Day)
			if !pc.Pinch.Date.Equal(wantDate) {
				t.Errorf("pinch date = %v, want %v", pc.Pinch.Date, wantDate)
			}
			if tt.checkPinch != nil {
				tt.checkPinch(t, pc.Pinch)
			}
		})
	}
}

// TestTidelineWindowHoldsAFullCycle locks the reason the floor is a month rather
// than a fortnight: the hero claims to show the household's rhythm, and a rhythm
// needs one complete turn of the dominant cadence to exist. Household
// commitments are overwhelmingly monthly, so whatever day of the month the page
// is opened on — and however soon the next paycheck lands — the window must
// still contain every monthly obligation at least once.
func TestTidelineWindowHoldsAFullCycle(t *testing.T) {
	base := currency.Rates{Base: "USD"}
	// A paycheck two days out would previously have produced a 14-day band, which
	// on a monthly household contains a paycheck and very little else.
	for day := 1; day <= 28; day++ {
		from := time.Date(2026, 7, day, 9, 0, 0, 0, time.UTC)
		recs := []domain.Recurring{
			mkRec("rent", -120000, domain.CadenceMonthly, nextDayOfMonth(from, 1)),
			mkRec("streaming", -1599, domain.CadenceMonthly, nextDayOfMonth(from, 12)),
			mkRec("pay", 250000, domain.CadenceBiweekly, from.AddDate(0, 0, 2)),
		}
		pc, err := Tideline(500000, recs, from, base)
		if err != nil {
			t.Fatalf("Tideline(from=%v): %v", from, err)
		}
		if pc.WindowDays < minPinchWindowDays {
			t.Fatalf("from %v: window = %d days, want at least a full cycle (%d)", from, pc.WindowDays, minPinchWindowDays)
		}
		// Every monthly commitment must fall inside the drawn band.
		events, err := Events(recs, from, pc.WindowDays, base)
		if err != nil {
			t.Fatalf("Events: %v", err)
		}
		seen := map[string]bool{}
		for _, e := range events {
			seen[e.Label] = true
		}
		for _, label := range []string{"rent", "streaming", "pay"} {
			if !seen[label] {
				t.Errorf("from %v: %q never appears in the %d-day band", from, label, pc.WindowDays)
			}
		}
	}
}

// nextDayOfMonth returns the next occurrence of day-of-month dom on or after
// from — the NextDue a monthly commitment actually carries in the store.
func nextDayOfMonth(from time.Time, dom int) time.Time {
	d := time.Date(from.Year(), from.Month(), dom, 0, 0, 0, 0, from.Location())
	if d.Before(time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())) {
		d = d.AddDate(0, 1, 0)
	}
	return d
}
