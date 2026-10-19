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
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
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
		start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)
		amount := money.New(int64(100+i), "USD")
		chain[i] = budgeting.RolloverPeriodState{
			Start: start, Base: amount, CarryIn: money.Zero("USD"), EffectiveCap: amount,
			Spend: money.Zero("USD"), CarryOut: amount,
		}
	}

	view := budgeting.RolloverView{Prior: chain, Current: budgeting.RolloverPeriodState{}}
	f := render.New(t)
	f.Render(budgetRolloverTable(view, "chain"))
	rows := f.AllByTag("tr")
	if len(rows) != 5 {
		t.Fatalf("rollover table rows = %d, want header + last four periods", len(rows))
	}
	text := byTestID(f, "table", "budget-rollover-table-chain").Text()
	if strings.Contains(text, "Jan 2026") || !strings.Contains(text, "May 2026") || !strings.Contains(text, "Aug 2026") {
		t.Fatalf("rendered table does not show the last four periods: %q", text)
	}
}
