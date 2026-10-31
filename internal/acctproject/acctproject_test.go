// SPDX-License-Identifier: MIT

package acctproject

import (
	"math"
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
	p := mustProject(234000, d(1), drivers, 30)
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
	p := mustProject(100000, d(1), drivers, 30)
	if len(p.Drivers) != 0 || p.End != 100000 {
		t.Errorf("driver past horizon should be dropped: %+v", p)
	}
}

func TestProjectPastDriverLandsToday(t *testing.T) {
	drivers := []Driver{{Label: "Overdue bill", Date: d(-5 + 1), Amount: -5000}}
	// asOf the 10th; a driver dated the 4th (before asOf) lands on the 10th.
	p := mustProject(20000, d(10), []Driver{{Label: "Overdue", Date: d(4), Amount: -5000}}, 30)
	if len(p.Drivers) != 1 || !p.Drivers[0].Date.Equal(d(10)) {
		t.Errorf("past driver should land today: %+v", p.Drivers)
	}
	_ = drivers
}

func TestProjectFlatNoDip(t *testing.T) {
	p := mustProject(50000, d(1), nil, 30)
	if p.HasLowDip() {
		t.Errorf("flat account should not report a dip")
	}
	if p.Low != 50000 || p.End != 50000 {
		t.Errorf("flat projection wrong: %+v", p)
	}
}

func TestProjectNonPositiveHorizon(t *testing.T) {
	p := mustProject(50000, d(1), []Driver{{Label: "x", Date: d(2), Amount: -1000}}, 0)
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
	p := mustProject(50000, c381Day(16), []Driver{
		{Label: "Rent", Date: c381Day(20), Amount: -120000},
		{Label: "Pay", Date: c381Day(25), Amount: 200000},
	}, 20)
	pts := p.mustSeries(c381Day(16), 20)

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
	p := mustProject(0, c381Day(16), []Driver{
		{Label: "In", Date: c381Day(18), Amount: 30000},
		{Label: "Out", Date: c381Day(18), Amount: -10000},
	}, 10)
	pt := p.mustSeries(c381Day(16), 10)[2]
	if pt.In != 30000 || pt.Out != 10000 || pt.Balance != 20000 {
		t.Errorf("the 18th = %+v, want in 30000 out 10000 balance 20000", pt)
	}
}

func TestSeriesHorizonGuard(t *testing.T) {
	p := mustProject(100, c381Day(16), nil, 10)
	if got := p.mustSeries(c381Day(16), 0); got != nil {
		t.Errorf("a zero horizon produced %d points", len(got))
	}
	if got := p.mustSeries(c381Day(16), -3); got != nil {
		t.Errorf("a negative horizon produced %d points", len(got))
	}
}

// FIRST crossing, not the deepest: knowing when trouble starts is what leaves
// time to act.
func TestFirstNegativeIsTheFirstCrossing(t *testing.T) {
	p := mustProject(10000, c381Day(16), []Driver{
		{Label: "a", Date: c381Day(18), Amount: -15000}, // crosses here
		{Label: "b", Date: c381Day(19), Amount: 2000},
		{Label: "c", Date: c381Day(22), Amount: -30000}, // deepest here
	}, 20)

	at, ok := p.mustFirstNegative(c381Day(16))
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
	p := mustProject(-2500, c381Day(16), nil, 10)
	at, ok := p.mustFirstNegative(c381Day(16))
	if !ok || !at.Equal(c381Day(16)) {
		t.Errorf("FirstNegative = %s,%v want today,true", at.Format("2006-01-02"), ok)
	}
}

func TestFirstNegativeOnAHealthyAccount(t *testing.T) {
	p := mustProject(10000, c381Day(16), []Driver{{Label: "Pay", Date: c381Day(20), Amount: 5000}}, 10)
	if _, ok := p.mustFirstNegative(c381Day(16)); ok {
		t.Error("a rising positive account reported going negative")
	}
}

// No drivers is the ABSENCE of information, not a forecast of a flat balance.
func TestKnownDistinguishesNoDataFromAFlatLine(t *testing.T) {
	if mustProject(10000, c381Day(16), nil, 30).Known() {
		t.Error("a projection with no drivers claimed to know something")
	}
	if !mustProject(10000, c381Day(16), []Driver{{Date: c381Day(20), Amount: 1}}, 30).Known() {
		t.Error("a projection with a driver reported unknown")
	}
}

func TestProjectRejectsPrefixOverflowEvenWhenFinalWouldFit(t *testing.T) {
	_, err := Project(math.MaxInt64, c381Day(16), []Driver{
		{Label: "overflow first", Date: c381Day(17), Amount: 1},
		{Label: "offset later", Date: c381Day(18), Amount: -1},
	}, 10)
	if err == nil {
		t.Fatal("accepted an overflowing intermediate balance because a later driver would offset it")
	}
}

func TestSeriesRejectsDailyFlowAggregateOverflow(t *testing.T) {
	t.Run("inflows", func(t *testing.T) {
		p, err := Project(math.MinInt64, c381Day(16), []Driver{
			{Label: "a", Date: c381Day(17), Amount: math.MaxInt64},
			{Label: "b", Date: c381Day(17), Amount: math.MaxInt64},
		}, 3)
		if err != nil {
			t.Fatalf("valid running balances rejected: %v", err)
		}
		if _, err := p.Series(c381Day(16), 3); err == nil {
			t.Fatal("daily inflow overflow was accepted")
		}
	})
	t.Run("outflows", func(t *testing.T) {
		p, err := Project(math.MaxInt64, c381Day(16), []Driver{
			{Label: "a out", Date: c381Day(17), Amount: -math.MaxInt64},
			{Label: "b in", Date: c381Day(17), Amount: math.MaxInt64},
			{Label: "c out", Date: c381Day(17), Amount: -1},
		}, 3)
		if err != nil {
			t.Fatalf("valid running balances rejected: %v", err)
		}
		if _, err := p.Series(c381Day(16), 3); err == nil {
			t.Fatal("daily outflow magnitude overflow was accepted")
		}
	})
}

func TestSeriesRejectsMinIntDebitMagnitude(t *testing.T) {
	p, err := Project(0, c381Day(16), []Driver{{Label: "minimum", Date: c381Day(17), Amount: math.MinInt64}}, 3)
	if err != nil {
		t.Fatalf("representable balance should project: %v", err)
	}
	if _, err := p.Series(c381Day(16), 3); err == nil {
		t.Fatal("MinInt64 outflow magnitude was accepted")
	}
}

func TestSeriesEmitsOnePointPerDayWithClosingDayZero(t *testing.T) {
	p, err := Project(1000, c381Day(16), []Driver{
		{Label: "pay", Date: c381Day(16), Amount: 300},
		{Label: "bill", Date: c381Day(16), Amount: -100},
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	points, err := p.Series(c381Day(16), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 {
		t.Fatalf("got %d points, want horizon+1 = 3", len(points))
	}
	for i, pt := range points {
		if want := c381Day(16 + i); !pt.Date.Equal(want) {
			t.Fatalf("point %d dated %s, want %s", i, pt.Date.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}
	if points[0].Balance != 1200 || points[0].In != 300 || points[0].Out != 100 {
		t.Fatalf("day zero is not the closing balance: %+v", points[0])
	}
	if points[2].Balance != 1200 {
		t.Fatalf("end = %d, want 1200", points[2].Balance)
	}
}

func TestSeriesIgnoresDriversBeforeItsOwnAsOf(t *testing.T) {
	p, err := Project(1000, c381Day(16), []Driver{{Label: "early", Date: c381Day(17), Amount: -400}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	points, err := p.Series(c381Day(20), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 4 || points[3].Balance != 1000 {
		t.Fatalf("driver before the series start leaked into the line: %+v", points)
	}
}

// Same-day drivers are one closing balance: neither the label order nor an
// intra-day dip that recovers by close may move Low, LowDate or FirstNegative.
func TestSameDayDriversUseTheClosingBalance(t *testing.T) {
	for _, labels := range [][2]string{{"A pay", "B rent"}, {"B pay", "A rent"}} {
		p := mustProject(1000, c381Day(16), []Driver{
			{Label: labels[0], Date: c381Day(18), Amount: 5000},
			{Label: labels[1], Date: c381Day(18), Amount: -5500},
		}, 10)
		if p.Low != 500 || !p.LowDate.Equal(c381Day(18)) {
			t.Errorf("labels %v: Low = %d on %s, want 500 on the 18th", labels, p.Low, p.LowDate.Format("2006-01-02"))
		}
		if _, ok := p.mustFirstNegative(c381Day(16)); ok {
			t.Errorf("labels %v: a day that closes at 500 was reported negative", labels)
		}
	}

	p := mustProject(1000, c381Day(16), []Driver{
		{Label: "A rent", Date: c381Day(18), Amount: -1500},
		{Label: "B pay", Date: c381Day(18), Amount: 200},
	}, 10)
	at, ok := p.mustFirstNegative(c381Day(16))
	if !ok || !at.Equal(c381Day(18)) || p.Low != -300 {
		t.Errorf("FirstNegative = %s,%v Low = %d, want the 18th, true, -300", at.Format("2006-01-02"), ok, p.Low)
	}
}

func mustProject(start int64, asOf time.Time, drivers []Driver, horizon int) Projection {
	p, err := Project(start, asOf, drivers, horizon)
	if err != nil {
		panic(err)
	}
	return p
}
func (p Projection) mustSeries(asOf time.Time, horizon int) []Point {
	pts, err := p.Series(asOf, horizon)
	if err != nil {
		panic(err)
	}
	return pts
}
func (p Projection) mustFirstNegative(asOf time.Time) (time.Time, bool) {
	at, ok, err := p.FirstNegative(asOf)
	if err != nil {
		panic(err)
	}
	return at, ok
}

func TestProjectUsesTheWallClockCalendarDay(t *testing.T) {
	west := time.FixedZone("UTC-8", -8*60*60)
	east := time.FixedZone("UTC+9", 9*60*60)
	cases := []struct {
		name string
		asOf time.Time
		due  time.Time
		want time.Time
	}{
		{"evening in the west is still that calendar day", time.Date(2026, 10, 20, 22, 0, 0, 0, west), time.Date(2026, 10, 20, 23, 0, 0, 0, west), time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)},
		{"morning in the east is that calendar day", time.Date(2026, 10, 20, 1, 0, 0, 0, east), time.Date(2026, 10, 20, 0, 30, 0, 0, east), time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Project(1000, tc.asOf, []Driver{{Label: "bill", Date: tc.due, Amount: -100}}, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Drivers) != 1 || !got.Drivers[0].Date.Equal(tc.want) || !got.LowDate.Equal(tc.want) {
				t.Fatalf("drivers = %+v, low date %v, want day %v", got.Drivers, got.LowDate, tc.want)
			}
		})
	}
}
