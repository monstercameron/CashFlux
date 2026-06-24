// SPDX-License-Identifier: MIT

package forecast

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func eq(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMonthlyNet(t *testing.T) {
	got := MonthlyNet([]Recurring{{Monthly: 5000}, {Monthly: -2000}, {Monthly: -500}})
	if got != 2500 {
		t.Errorf("MonthlyNet = %d, want 2500", got)
	}
}

func TestProjectRecurring(t *testing.T) {
	rec := []Recurring{{Label: "salary", Monthly: 5000}, {Label: "rent", Monthly: -2000}}
	got := Project(100000, rec, nil, 3)
	want := []int64{103000, 106000, 109000}
	if !eq(got, want) {
		t.Errorf("Project = %v, want %v", got, want)
	}
}

func TestProjectOneTime(t *testing.T) {
	rec := []Recurring{{Monthly: 3000}}
	one := []OneTime{{Label: "bonus", Month: 2, Amount: 10000}}
	got := Project(100000, rec, one, 3)
	want := []int64{103000, 116000, 119000}
	if !eq(got, want) {
		t.Errorf("Project with one-time = %v, want %v", got, want)
	}
}

func TestProjectFlatAndEmpty(t *testing.T) {
	if got := Project(500, nil, nil, 2); !eq(got, []int64{500, 500}) {
		t.Errorf("flat projection = %v, want [500 500]", got)
	}
	if got := Project(500, nil, nil, 0); got != nil {
		t.Errorf("zero horizon = %v, want nil", got)
	}
	if got := Project(500, nil, nil, -3); got != nil {
		t.Errorf("negative horizon = %v, want nil", got)
	}
}

func TestProjectOneTimesOutsideHorizonIgnored(t *testing.T) {
	one := []OneTime{
		{Label: "month-0", Month: 0, Amount: 9999},   // 1-based: month 0 never occurs
		{Label: "beyond", Month: 5, Amount: 9999},    // past the 3-month horizon
		{Label: "negative", Month: -1, Amount: 9999}, // nonsensical month
	}
	if got := Project(1000, nil, one, 3); !eq(got, []int64{1000, 1000, 1000}) {
		t.Errorf("out-of-horizon one-times should be ignored, got %v", got)
	}
}

func TestProjectMultipleOneTimesSameMonth(t *testing.T) {
	one := []OneTime{{Month: 2, Amount: 500}, {Month: 2, Amount: 300}}
	if got := Project(0, nil, one, 2); !eq(got, []int64{0, 800}) {
		t.Errorf("same-month one-times should sum, got %v", got)
	}
}

func TestProjectCanGoNegative(t *testing.T) {
	// A draining balance is reported truthfully — no flooring at zero.
	if got := Project(1000, []Recurring{{Monthly: -600}}, nil, 3); !eq(got, []int64{400, -200, -800}) {
		t.Errorf("projection should allow negative, got %v", got)
	}
}

// TestProjectSpendingDeltaShiftsEndBalance backs the Planning "trim spending"
// what-if (D10): trimming spending by `delta`/month pulls the projected curve
// ahead by delta each month, so by the horizon end it's delta*months higher.
func TestProjectSpendingDeltaShiftsEndBalance(t *testing.T) {
	const start, baseMonthly, delta, months = 100000, 50000, 20000, 12
	base := Project(start, []Recurring{{Monthly: baseMonthly}}, nil, months)
	trimmed := Project(start, []Recurring{{Monthly: baseMonthly + delta}}, nil, months)
	for i := range base {
		if got, want := trimmed[i]-base[i], int64(delta*(i+1)); got != want {
			t.Errorf("month %d delta = %d, want %d", i, got, want)
		}
	}
	if got, want := trimmed[months-1]-base[months-1], int64(delta*months); got != want {
		t.Errorf("end-of-horizon delta = %d, want %d", got, want)
	}
}

func TestProjectFromLedgerNetWorthFeed(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{
		{ID: "checking", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(100000, "USD")},
		{ID: "card", Class: domain.ClassLiability, Currency: "USD", OpeningBalance: money.New(-20000, "USD")},
	}
	txns := []domain.Transaction{
		{AccountID: "checking", Date: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(10000, "USD")},
	}
	series, err := ledger.NetWorthSeries(accounts, txns, []time.Time{time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)}, rates)
	if err != nil {
		t.Fatalf("NetWorthSeries error: %v", err)
	}
	got := Project(series[0].Amount, []Recurring{{Monthly: 5000}}, []OneTime{{Month: 2, Amount: -12000}}, 3)
	want := []int64{95000, 88000, 93000}
	if !eq(got, want) {
		t.Errorf("forecast from net-worth feed = %v, want %v", got, want)
	}
}
