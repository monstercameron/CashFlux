// SPDX-License-Identifier: MIT

package moneyleaks

import "testing"

func TestSubscriptions(t *testing.T) {
	subs := []Sub{
		{Label: "Streaming A", MonthlyMinor: 1599},
		{Label: "Gym", MonthlyMinor: 4500},
		{Label: "Cloud", MonthlyMinor: 999},
		{Label: "Free trial", MonthlyMinor: 0}, // ignored
	}
	// Income $500/mo (50000 cents). Total = 1599+4500+999 = 7098 → ~14.2% share (heavy).
	rep := Subscriptions(subs, 50000, 2)
	if rep.TotalMonthly != 7098 {
		t.Errorf("total = %d, want 7098", rep.TotalMonthly)
	}
	if rep.TotalAnnual != 7098*12 {
		t.Errorf("annual = %d, want %d", rep.TotalAnnual, 7098*12)
	}
	if rep.Count != 3 {
		t.Errorf("count = %d, want 3 (zero-cost ignored)", rep.Count)
	}
	if rep.SharePct < 14.1 || rep.SharePct > 14.3 {
		t.Errorf("share = %.2f, want ~14.2", rep.SharePct)
	}
	if !rep.Heavy {
		t.Error("14.2%% share should be flagged heavy")
	}
	// Top 2, biggest first: Gym (4500) then Streaming A (1599).
	if len(rep.Top) != 2 || rep.Top[0].Label != "Gym" || rep.Top[1].Label != "Streaming A" {
		t.Errorf("top = %+v, want [Gym, Streaming A]", rep.Top)
	}
}

func TestSubscriptionsNoIncomeNotHeavy(t *testing.T) {
	rep := Subscriptions([]Sub{{Label: "X", MonthlyMinor: 9999}}, 0, 5)
	if rep.SharePct != 0 || rep.Heavy {
		t.Errorf("no income → share 0, not heavy; got %+v", rep)
	}
	if rep.TotalMonthly != 9999 {
		t.Errorf("total still summed regardless of income; got %d", rep.TotalMonthly)
	}
}

func TestSubscriptionsEmpty(t *testing.T) {
	rep := Subscriptions(nil, 100000, 3)
	if rep.Count != 0 || rep.TotalMonthly != 0 || rep.Heavy || len(rep.Top) != 0 {
		t.Errorf("empty → zero report; got %+v", rep)
	}
}
