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
