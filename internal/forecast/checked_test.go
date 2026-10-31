// SPDX-License-Identifier: MIT
package forecast

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/money"
	"math"
	"reflect"
	"testing"
)

// frozenProject is the pre-checked algorithm, with unchanged addition order.
func frozenProject(start int64, recurring []Recurring, oneTimes []OneTime, months int) []int64 {
	if months <= 0 {
		return nil
	}
	var net int64
	for _, r := range recurring {
		net += r.Monthly
	}
	curve := make([]int64, months)
	balance := start
	for i := range months {
		balance += net
		for _, o := range oneTimes {
			if o.Month == i+1 {
				balance += o.Amount
			}
		}
		curve[i] = balance
	}
	return curve
}
func TestProjectCheckedMatchesPriorOrderingAndWindows(t *testing.T) {
	for _, months := range []int{-3, 0, 1, 12, 1200} {
		recurring := []Recurring{{Monthly: 5000}, {Monthly: -2000}, {Monthly: -499}}
		one := []OneTime{{Month: 0, Amount: 99}, {Month: -1, Amount: 99}, {Month: 1, Amount: -300}, {Month: 1, Amount: 150}, {Month: 13, Amount: 1000}, {Month: 1300, Amount: 100}}
		got, err := ProjectChecked(-10000, recurring, one, months)
		if err != nil || !reflect.DeepEqual(got, frozenProject(-10000, recurring, one, months)) || !reflect.DeepEqual(got, Project(-10000, recurring, one, months)) {
			t.Fatalf("months%d: %v / %v", months, got, err)
		}
	}
	for _, start := range []int64{math.MinInt64, math.MaxInt64, 0} {
		curve, err := ProjectChecked(start, nil, nil, 2)
		if err != nil || !reflect.DeepEqual(curve, []int64{start, start}) {
			t.Fatalf("boundary identity: %v %v", curve, err)
		}
	}
}
func TestProjectCheckedRejectsIntermediateOverflowWithoutPartialCurve(t *testing.T) {
	for _, tc := range []struct {
		name   string
		start  int64
		rec    []Recurring
		one    []OneTime
		months int
	}{
		{"recurring sum", 0, []Recurring{{Monthly: math.MaxInt64}, {Monthly: 1}, {Monthly: -1}}, nil, 1},
		{"recurring underflow", 0, []Recurring{{Monthly: math.MinInt64}, {Monthly: -1}, {Monthly: 1}}, nil, 1},
		{"balance", math.MaxInt64, []Recurring{{Monthly: 1}}, nil, 1},
		{"late balance", math.MaxInt64 - 1, []Recurring{{Monthly: 1}}, nil, 2},
		{"one time order", math.MaxInt64, nil, []OneTime{{Month: 1, Amount: 1}, {Month: 1, Amount: -1}}, 1},
		{"recurring before event", math.MinInt64, []Recurring{{Monthly: -1}}, []OneTime{{Month: 1, Amount: 1}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			curve, err := ProjectChecked(tc.start, tc.rec, tc.one, tc.months)
			if !errors.Is(err, money.ErrOverflow) || curve != nil {
				t.Fatalf("overflow exposed curve: %v %v", curve, err)
			}
		})
	}
	if _, err := MonthlyNetChecked([]Recurring{{Monthly: math.MaxInt64}, {Monthly: 1}, {Monthly: -1}}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("monthly overflow: %v", err)
	}
	net, err := MonthlyNetChecked([]Recurring{{Monthly: math.MaxInt64}, {Monthly: -math.MaxInt64}, {Monthly: 1}})
	if err != nil || net != 1 {
		t.Fatalf("valid cancellation: %d %v", net, err)
	}
}
func TestCheckedProjectionBoundsAreExplicit(t *testing.T) {
	for _, tc := range []struct {
		months, items int
		fail          bool
	}{{1200, 0, false}, {1201, 0, true}, {1000, 999, false}, {1000, 1000, true}, {0, MaxProjectionWork, true}, {1, -1, true}, {math.MaxInt, 0, true}, {0, 0, false}, {-1, 1, false}} {
		err := CheckProjectionBounds(tc.months, tc.items)
		if errors.Is(err, ErrProjectionBounds) != tc.fail {
			t.Fatalf("bounds %+v: %v", tc, err)
		}
	}
	if curve, err := ProjectChecked(1, nil, nil, MaxProjectionMonths+1); curve != nil || !errors.Is(err, ErrProjectionBounds) {
		t.Fatalf("unbounded horizon: %v %v", curve, err)
	}
}
