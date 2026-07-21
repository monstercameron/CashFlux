// SPDX-License-Identifier: MIT

// Package moneyleaks surfaces recurring money drains the rest of the app tracks
// but doesn't add up in one place: how much of your income the subscriptions
// quietly eat, and which ones are the biggest. Spending "creep" (a category
// running above its own norm) is already computed by reports.TrimTargets; this
// package owns only the subscription-load read, so the health page can show both
// without re-deriving either.
//
// Pure Go; amounts are integer minor units of one base currency (the caller
// FX-normalizes first).
package moneyleaks

import "sort"

// Sub is one recurring charge, normalized to its per-month cost.
type Sub struct {
	Label        string
	MonthlyMinor int64
	Autopay      bool
}

// SubReport summarizes the subscription load.
type SubReport struct {
	TotalMonthly int64   // Σ monthly-equivalent cost of every subscription
	TotalAnnual  int64   // ... × 12, the figure that actually lands
	Count        int     //
	SharePct     float64 // subscriptions as a percent of monthly income (0 when income ≤ 0)
	Heavy        bool    // true when the share crosses HeavySharePct — worth a prune
	Top          []Sub   // the largest subscriptions, biggest first, up to the requested N
}

// HeavySharePct: subscriptions above this share of income are flagged as heavy —
// the point where "a few small subscriptions" has quietly become real money.
const HeavySharePct = 10.0

// Subscriptions rolls a set of recurring charges into a single load figure: the
// total monthly and annual cost, the count, the share of income, and the largest
// topN, biggest first. Zero- and negative-cost entries are ignored.
func Subscriptions(subs []Sub, monthlyIncome int64, topN int) SubReport {
	var rep SubReport
	kept := make([]Sub, 0, len(subs))
	for _, s := range subs {
		if s.MonthlyMinor <= 0 {
			continue
		}
		kept = append(kept, s)
		rep.TotalMonthly += s.MonthlyMinor
		rep.Count++
	}
	rep.TotalAnnual = rep.TotalMonthly * 12
	if monthlyIncome > 0 {
		rep.SharePct = float64(rep.TotalMonthly) / float64(monthlyIncome) * 100
		rep.Heavy = rep.SharePct >= HeavySharePct
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].MonthlyMinor > kept[j].MonthlyMinor })
	if topN > 0 && len(kept) > topN {
		kept = kept[:topN]
	}
	rep.Top = kept
	return rep
}
