// SPDX-License-Identifier: MIT

package coverformula

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBCS2CoverFormulaUsesHistoryAndEffectiveCap(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "b", CategoryID: "parent", Period: domain.PeriodMonthly, Limit: money.New(60000, "USD"), LimitHistory: map[string]int64{"2026-01-01": 30000, "2026-11-01": 60000}, PeriodBoosts: map[string]int64{"2026-10-01": 5000}}
	c := Context{Now: now, Rates: currency.Rates{Base: "USD"}, Categories: []domain.Category{{ID: "parent"}, {ID: "child", ParentID: "parent"}}, Txns: []domain.Transaction{{ID: "t", CategoryID: "child", Date: now, Amount: money.New(-40000, "USD")}}}
	for _, tc := range []struct {
		expr string
		want int64
	}{{"limit", 35000}, {"spent", 40000}, {"overspend", 5000}} {
		got, err := c.AmountMinor(tc.expr, b)
		if err != nil || got != tc.want {
			t.Fatalf("%s=%d err=%v want=%d", tc.expr, got, err, tc.want)
		}
	}
	for _, expr := range []string{"1e100", "-1e100"} {
		if _, err := c.AmountMinor(expr, b); err == nil {
			t.Fatalf("out-of-range amount accepted: %s", expr)
		}
	}
	if _, err := c.Weight("1e100", b); err == nil {
		t.Fatal("out-of-range weight accepted")
	}
}
