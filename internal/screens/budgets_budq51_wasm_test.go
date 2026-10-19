// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBUDQ51SundayWeeklyRaiseLimitUsesPageReadKey(t *testing.T) {
	weekStart := time.Sunday
	anchor := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "weekly", Period: domain.PeriodWeekly, Limit: money.New(10_000, "USD")}

	// This is the one-period write used by the raise-limit path.
	start := budgetPeriodKeyStartWithWeekStart(b.Period, anchor, time.Time{}, weekStart)
	written := b.WithPeriodBoost(start, 2_500)

	// The Budgets page resolves boosts from its configured week-start key.
	readKey := budgeting.PeriodKeyWithWeekStart(b.Period, anchor, time.Time{}, weekStart)
	boost := written.PeriodBoosts[readKey]
	effectiveCap := written.Limit.Amount + boost
	if start.Format("2006-01-02") != "2026-10-18" {
		t.Fatalf("Sunday weekly write start = %s, want 2026-10-18", start.Format("2006-01-02"))
	}
	if boost != 2_500 || effectiveCap != 12_500 {
		t.Fatalf("page effective cap = %d (boost %d), want 12500 (boost 2500)", effectiveCap, boost)
	}
}

func TestBUDQ51RolloverTooltipKeepsSixPeriodsAndCountsEarlier(t *testing.T) {
	chain := make([]budgeting.RolloverPeriodState, 8)
	for i := range chain {
		start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		amount := money.New(int64(100+i), "USD")
		chain[i] = budgeting.RolloverPeriodState{
			Start: start, Base: amount, CarryIn: money.Zero("USD"), EffectiveCap: amount,
			Spend: money.Zero("USD"), CarryOut: amount,
		}
	}

	got := budgetRolloverChainLines(chain)
	if !strings.Contains(got, "and 2 earlier") {
		t.Fatalf("tooltip does not report the two hidden periods: %q", got)
	}
	if strings.Contains(got, "2026-01-01") || strings.Contains(got, "2026-01-02") {
		t.Fatalf("tooltip still includes periods older than the visible six: %q", got)
	}
	if !strings.Contains(got, "2026-01-03") || !strings.Contains(got, "2026-01-08") {
		t.Fatalf("tooltip does not show the last six periods: %q", got)
	}
	if count := strings.Count(got, "base"); count != 6 {
		t.Fatalf("tooltip has %d period entries, want 6: %s", count, got)
	}
}
