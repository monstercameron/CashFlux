// SPDX-License-Identifier: MIT

package reports

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRunwayWideRemainderAndMonthlyAverageOverflow(t *testing.T) {
	got := EstimateRunway(math.MaxInt64-1, math.MaxInt64/2+1)
	if got.Months != 1 || got.Days != 29 {
		t.Fatalf("wide runway = %+v", got)
	}
	if income, expense, active, err := MonthlyAveragesChecked([]PeriodFlow{{Income: math.MaxInt64}, {Income: 1}}); !errors.Is(err, money.ErrOverflow) || income != 0 || expense != 0 || active != 0 {
		t.Fatalf("overflowing averages = %d / %d / %d / %v", income, expense, active, err)
	}
}

func TestRunwayRetainsWideMonthCountOnEveryArchitecture(t *testing.T) {
	got := EstimateRunway(math.MaxInt64, 1)
	// The assignment also makes narrowing Months to int a compile-time failure
	// on native 64-bit hosts, which could otherwise conceal the wasm32 defect.
	var months int64 = got.Months
	if months != math.MaxInt64 || got.Days != 0 || got.Sustainable {
		t.Fatalf("month quotient was narrowed: %+v", got)
	}
}
