package budgetview

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"testing"
	"time"
)

func TestMarkerTransitions(t *testing.T) {
	for bits := 0; bits < 8; bits++ {
		s := MarkerState{Open: bits&1 != 0, Pending: bits&2 != 0, Closing: bits&4 != 0}
		wantClass := ""
		if s.Pending {
			wantClass = " is-waiting"
		}
		if s.Open {
			wantClass += " is-open"
		}
		if s.Class() != wantClass {
			t.Fatal(s.Class())
		}
		for event := MarkerEnter; event <= MarkerDismiss+1; event++ {
			want := s
			switch event {
			case MarkerEnter:
				want.Closing = false
				if !want.Open {
					want.Pending = true
				}
			case MarkerLeave:
				want.Pending = false
				if want.Open {
					want.Closing = true
				}
			case MarkerIntent:
				want.Pending, want.Open = false, true
			case MarkerGrace:
				want.Closing, want.Open = false, false
			case MarkerFocus:
				if !want.Open {
					want.Pending = true
				}
			case MarkerBlur:
				want.Pending, want.Open = false, false
			case MarkerToggle:
				want.Pending, want.Closing, want.Open = false, false, !want.Open
			case MarkerAction:
				want = MarkerState{}
			case MarkerDismiss:
				want.Open = false
			}
			if got := s.Next(event); got != want {
				t.Fatal(s, event, got, want)
			}
		}
	}
	// A fast crossing cancels intent; the grace window keeps an open preview until
	// the external hover observation confirms both trigger and portal were left.
	s := MarkerState{}.Next(MarkerEnter).Next(MarkerLeave)
	if s.Pending || s.Open {
		t.Fatal(s)
	}
	s = s.Next(MarkerToggle).Next(MarkerLeave)
	if !s.Open || !s.Closing {
		t.Fatal(s)
	}
	if got := s.Next(MarkerEnter); !got.Open || got.Closing {
		t.Fatal(got)
	}
}

func TestPeriodKeyStart(t *testing.T) {
	west := time.FixedZone("west", -8*3600)
	for _, tc := range []struct {
		period           domain.Period
		ref, anchor, now time.Time
		week             time.Weekday
		want             time.Time
	}{
		{domain.PeriodMonthly, time.Date(2024, 2, 29, 23, 30, 0, 0, west), time.Time{}, time.Time{}, time.Monday, day(2024, 2, 1)},
		{domain.PeriodWeekly, day(2026, 10, 19), time.Time{}, time.Time{}, time.Sunday, day(2026, 10, 18)},
		{domain.PeriodWeekly, day(2026, 10, 19), time.Time{}, time.Time{}, time.Monday, day(2026, 10, 19)},
		{domain.PeriodMonthly, time.Time{}, time.Time{}, day(2026, 10, 19), time.Monday, day(2026, 10, 1)},
		{domain.PeriodBiweekly, day(2026, 10, 20), day(2026, 10, 5), time.Time{}, time.Monday, day(2026, 10, 19)},
		{domain.PeriodYearly, day(10000, 2, 1), time.Time{}, time.Time{}, time.Monday, day(10000, 1, 1)},
	} {
		if got := PeriodKeyStart(tc.period, tc.ref, tc.anchor, tc.now, tc.week); !got.Equal(tc.want) || got.Location() != time.UTC {
			t.Fatal(tc, got)
		}
	}
}
