// SPDX-License-Identifier: MIT

package acctproject

import (
	"testing"
	"time"
)

func d(day int) time.Time { return time.Date(2026, 3, day, 0, 0, 0, 0, time.UTC) }

func TestProjectLowPoint(t *testing.T) {
	// Start $2,340; rent −$1,400 on the 1st, paycheck +$1,200 on the 15th.
	drivers := []Driver{
		{Label: "Paycheck", Date: d(15), Amount: 120000},
		{Label: "Rent", Date: d(1), Amount: -140000},
	}
	p := Project(234000, d(1), drivers, 30)
	if p.Low != 94000 {
		t.Errorf("Low = %d, want 94000", p.Low)
	}
	if !p.LowDate.Equal(d(1)) {
		t.Errorf("LowDate = %v, want the 1st", p.LowDate)
	}
	if p.End != 214000 {
		t.Errorf("End = %d, want 214000", p.End)
	}
	if !p.HasLowDip() {
		t.Errorf("HasLowDip should be true (94000 < 234000)")
	}
	// Drivers sorted by date: rent first, then paycheck.
	if len(p.Drivers) != 2 || p.Drivers[0].Label != "Rent" {
		t.Errorf("drivers not date-sorted: %+v", p.Drivers)
	}
}

func TestProjectHorizonExcludesLateDrivers(t *testing.T) {
	drivers := []Driver{{Label: "Late", Date: d(40), Amount: -100000}}
	p := Project(100000, d(1), drivers, 30)
	if len(p.Drivers) != 0 || p.End != 100000 {
		t.Errorf("driver past horizon should be dropped: %+v", p)
	}
}

func TestProjectPastDriverLandsToday(t *testing.T) {
	drivers := []Driver{{Label: "Overdue bill", Date: d(-5 + 1), Amount: -5000}}
	// asOf the 10th; a driver dated the 4th (before asOf) lands on the 10th.
	p := Project(20000, d(10), []Driver{{Label: "Overdue", Date: d(4), Amount: -5000}}, 30)
	if len(p.Drivers) != 1 || !p.Drivers[0].Date.Equal(d(10)) {
		t.Errorf("past driver should land today: %+v", p.Drivers)
	}
	_ = drivers
}

func TestProjectFlatNoDip(t *testing.T) {
	p := Project(50000, d(1), nil, 30)
	if p.HasLowDip() {
		t.Errorf("flat account should not report a dip")
	}
	if p.Low != 50000 || p.End != 50000 {
		t.Errorf("flat projection wrong: %+v", p)
	}
}

func TestProjectNonPositiveHorizon(t *testing.T) {
	p := Project(50000, d(1), []Driver{{Label: "x", Date: d(2), Amount: -1000}}, 0)
	if p.End != 50000 || len(p.Drivers) != 0 {
		t.Errorf("zero horizon should be flat: %+v", p)
	}
}

// ─── C381: the curve ─────────────────────────────────────────────────────────

func c381Day(d int) time.Time { return time.Date(2026, time.August, d, 0, 0, 0, 0, time.UTC) }

// The shape has to be daily. An account that ends the window healthy and dips
// below zero mid-way is invisible at month granularity, and the dip is the whole
// reason to draw the line.
func TestSeriesIsDailyAndStartsWhereTheAccountIs(t *testing.T) {
	p := Project(50000, c381Day(16), []Driver{
		{Label: "Rent", Date: c381Day(20), Amount: -120000},
		{Label: "Pay", Date: c381Day(25), Amount: 200000},
	}, 20)
	pts := p.Series(c381Day(16), 20)

	if len(pts) != 21 {
		t.Fatalf("got %d points, want 21 (20 days plus today)", len(pts))
	}
	// Today is untouched, so the drawn line cannot contradict the balance printed
	// beside it.
	if pts[0].Balance != 50000 || pts[0].In != 0 || pts[0].Out != 0 {
		t.Errorf("day zero = %+v, want the untouched starting balance", pts[0])
	}
	if pts[4].Balance != -70000 {
		t.Errorf("after rent on the 20th = %d, want -70000", pts[4].Balance)
	}
	if pts[4].Out != 120000 {
		t.Errorf("the 20th reported out %d", pts[4].Out)
	}
	if got := pts[len(pts)-1].Balance; got != 130000 {
		t.Errorf("end = %d, want 130000", got)
	}
	// The endpoint is healthy while the curve went deeply negative — the exact
	// case a coarser projection hides.
	if p.End <= 0 {
		t.Fatalf("End = %d, expected the healthy endpoint this case is about", p.End)
	}
}

func TestSeriesAggregatesSameDayDrivers(t *testing.T) {
	p := Project(0, c381Day(16), []Driver{
		{Label: "In", Date: c381Day(18), Amount: 30000},
		{Label: "Out", Date: c381Day(18), Amount: -10000},
	}, 10)
	pt := p.Series(c381Day(16), 10)[2]
	if pt.In != 30000 || pt.Out != 10000 || pt.Balance != 20000 {
		t.Errorf("the 18th = %+v, want in 30000 out 10000 balance 20000", pt)
	}
}

func TestSeriesHorizonGuard(t *testing.T) {
	p := Project(100, c381Day(16), nil, 10)
	if got := p.Series(c381Day(16), 0); got != nil {
		t.Errorf("a zero horizon produced %d points", len(got))
	}
	if got := p.Series(c381Day(16), -3); got != nil {
		t.Errorf("a negative horizon produced %d points", len(got))
	}
}

// FIRST crossing, not the deepest: knowing when trouble starts is what leaves
// time to act.
func TestFirstNegativeIsTheFirstCrossing(t *testing.T) {
	p := Project(10000, c381Day(16), []Driver{
		{Label: "a", Date: c381Day(18), Amount: -15000}, // crosses here
		{Label: "b", Date: c381Day(19), Amount: 2000},
		{Label: "c", Date: c381Day(22), Amount: -30000}, // deepest here
	}, 20)

	at, ok := p.FirstNegative(c381Day(16))
	if !ok {
		t.Fatal("FirstNegative said no for a curve that went to -430")
	}
	if !at.Equal(c381Day(18)) {
		t.Errorf("FirstNegative = %s, want the 18th", at.Format("2006-01-02"))
	}
	if !p.LowDate.Equal(c381Day(22)) {
		t.Errorf("LowDate = %s, want the 22nd — the two must not be conflated",
			p.LowDate.Format("2006-01-02"))
	}
}

// An already-overdrawn account must not report "never goes negative", which is
// the most misleading answer available.
func TestFirstNegativeCountsAnOverdrawnStartAsToday(t *testing.T) {
	p := Project(-2500, c381Day(16), nil, 10)
	at, ok := p.FirstNegative(c381Day(16))
	if !ok || !at.Equal(c381Day(16)) {
		t.Errorf("FirstNegative = %s,%v want today,true", at.Format("2006-01-02"), ok)
	}
}

func TestFirstNegativeOnAHealthyAccount(t *testing.T) {
	p := Project(10000, c381Day(16), []Driver{{Label: "Pay", Date: c381Day(20), Amount: 5000}}, 10)
	if _, ok := p.FirstNegative(c381Day(16)); ok {
		t.Error("a rising positive account reported going negative")
	}
}

// No drivers is the ABSENCE of information, not a forecast of a flat balance.
func TestKnownDistinguishesNoDataFromAFlatLine(t *testing.T) {
	if Project(10000, c381Day(16), nil, 30).Known() {
		t.Error("a projection with no drivers claimed to know something")
	}
	if !Project(10000, c381Day(16), []Driver{{Date: c381Day(20), Amount: 1}}, 30).Known() {
		t.Error("a projection with a driver reported unknown")
	}
}
