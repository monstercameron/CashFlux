// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRolloverChainCarriesRemainingFunds(t *testing.T) {
	tests := []struct {
		name          string
		prevRemaining int64
		limit         int64
		want          int64
	}{
		{"unspent rolls forward", 3000, 10000, 13000},
		{"overspend carries as debt", -2000, 10000, 8000},
		{"exactly spent", 0, 10000, 10000},
		{"deep overdraw goes negative", -15000, 10000, -5000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := money.New(tc.limit, "USD")
			spent := money.New(tc.limit-tc.prevRemaining, "USD")
			states, err := BuildRolloverChain([]RolloverPeriod{
				{Start: mustDate("2026-08-01"), Base: base, Spend: spent},
				{Start: mustDate("2026-09-01"), Base: base, Spend: money.Zero("USD")},
			}, 0)
			if err != nil {
				t.Fatalf("BuildRolloverChain: %v", err)
			}
			if got := states[1].EffectiveCap.Amount; got != tc.want {
				t.Errorf("next effective cap = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRolloverChainRejectsCurrencyMismatch(t *testing.T) {
	_, err := BuildRolloverChain([]RolloverPeriod{{
		Start: mustDate("2026-08-01"), Base: money.New(100, "USD"), Spend: money.New(100, "EUR"),
	}}, 0)
	if err == nil {
		t.Fatal("rollover chain accepted mismatched currencies")
	}
}

func TestPreviousPeriodRange(t *testing.T) {
	ref := mustDate("2026-06-15")

	monthlyStart, monthlyEnd := PreviousPeriodRange(domain.PeriodMonthly, ref, time.Sunday)
	if monthlyStart != mustDate("2026-05-01") || monthlyEnd != mustDate("2026-06-01") {
		t.Fatalf("monthly previous = %s..%s", monthlyStart.Format("2006-01-02"), monthlyEnd.Format("2006-01-02"))
	}

	weeklyStart, weeklyEnd := PreviousPeriodRange(domain.PeriodWeekly, ref, time.Monday)
	if weeklyStart != mustDate("2026-06-08") || weeklyEnd != mustDate("2026-06-15") {
		t.Fatalf("weekly previous = %s..%s", weeklyStart.Format("2006-01-02"), weeklyEnd.Format("2006-01-02"))
	}

	quarterStart, quarterEnd := PreviousPeriodRange(domain.PeriodQuarterly, ref, time.Sunday)
	if quarterStart != mustDate("2026-01-01") || quarterEnd != mustDate("2026-04-01") {
		t.Fatalf("quarter previous = %s..%s", quarterStart.Format("2006-01-02"), quarterEnd.Format("2006-01-02"))
	}
}

func TestSinkingFundContribution(t *testing.T) {
	tests := []struct {
		name    string
		target  int64
		periods int
		want    int64
	}{
		{"even split", 120000, 12, 10000},
		{"ceils the remainder", 100000, 12, 8334},
		{"single period", 5000, 1, 5000},
		{"zero periods funds all now", 5000, 0, 5000},
		{"negative periods funds all now", 5000, -3, 5000},
		{"zero target", 0, 12, 0},
		{"negative target", -100, 12, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SinkingFundContribution(money.New(tc.target, "USD"), tc.periods)
			if got.Amount != tc.want {
				t.Errorf("SinkingFundContribution(%d, %d) = %d, want %d", tc.target, tc.periods, got.Amount, tc.want)
			}
		})
	}
}

func TestSinkingFundContributionReachesTarget(t *testing.T) {
	cases := []struct {
		target  int64
		periods int
	}{{100000, 12}, {99999, 7}, {1, 3}, {77777, 13}}
	for _, c := range cases {
		per := SinkingFundContribution(money.New(c.target, "USD"), c.periods)
		if per.Amount*int64(c.periods) < c.target {
			t.Errorf("target %d over %d periods: %d×%d = %d < target", c.target, c.periods, per.Amount, c.periods, per.Amount*int64(c.periods))
		}
	}
}
