package afford

import "testing"

func TestCanAfford(t *testing.T) {
	tests := []struct {
		name                            string
		amount, start, monthlyNet       int64
		months                          int
		reserved                        int64
		affordable                      bool
		projected, available, shortfall int64
		monthsNeeded                    int
	}{
		{
			name:   "affordable right now from the balance",
			amount: 100000, start: 500000, monthlyNet: 0, months: 0, reserved: 0,
			affordable: true, projected: 500000, available: 500000, shortfall: 0, monthsNeeded: 0,
		},
		{
			name:   "affordable by the target date via savings",
			amount: 200000, start: 50000, monthlyNet: 50000, months: 4, reserved: 0,
			affordable: true, projected: 250000, available: 250000, shortfall: 0, monthsNeeded: 3,
		},
		{
			name:   "not affordable by the date but reaches it later",
			amount: 200000, start: 50000, monthlyNet: 50000, months: 2, reserved: 0,
			affordable: false, projected: 150000, available: 150000, shortfall: 50000, monthsNeeded: 3,
		},
		{
			name:   "never at a negative cash flow",
			amount: 100000, start: 50000, monthlyNet: -10000, months: 6, reserved: 0,
			affordable: false, projected: -10000, available: -10000, shortfall: 110000, monthsNeeded: -1,
		},
		{
			name:   "reserved (commitments/goals) reduces what's available",
			amount: 100000, start: 200000, monthlyNet: 0, months: 0, reserved: 150000,
			affordable: false, projected: 200000, available: 50000, shortfall: 50000, monthsNeeded: -1,
		},
		{
			name:   "exactly at the boundary is affordable",
			amount: 100000, start: 0, monthlyNet: 25000, months: 4, reserved: 0,
			affordable: true, projected: 100000, available: 100000, shortfall: 0, monthsNeeded: 4,
		},
		{
			name:   "negative months is treated as now",
			amount: 100000, start: 120000, monthlyNet: 50000, months: -3, reserved: 0,
			affordable: true, projected: 120000, available: 120000, shortfall: 0, monthsNeeded: 0,
		},
		{
			name:   "rounds months up when savings don't divide evenly",
			amount: 100000, start: 0, monthlyNet: 30000, months: 3, reserved: 0,
			affordable: false, projected: 90000, available: 90000, shortfall: 10000, monthsNeeded: 4, // ceil(100000/30000)
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CanAfford(tc.amount, tc.start, tc.monthlyNet, tc.months, tc.reserved)
			if got.Affordable != tc.affordable || got.ProjectedBalance != tc.projected ||
				got.Available != tc.available || got.Shortfall != tc.shortfall || got.MonthsNeeded != tc.monthsNeeded {
				t.Errorf("CanAfford = %+v, want affordable=%v projected=%d available=%d shortfall=%d monthsNeeded=%d",
					got, tc.affordable, tc.projected, tc.available, tc.shortfall, tc.monthsNeeded)
			}
		})
	}
}
