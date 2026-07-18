// SPDX-License-Identifier: MIT

package smart

import (
	"sort"
	"strings"
)

// Triage filtering + ordering for the hub's findings feed. A hundred-plus
// findings paginated ten at a time is workload, not insight — these pure
// helpers let the UI isolate what's urgent, high-dollar, or about one page
// (2026-07-18 assessment: "the count itself becomes workload").

// SortMode selects the findings-feed ordering.
type SortMode string

const (
	// SortBySeverity is the default: most urgent first (SortInsights order).
	SortBySeverity SortMode = "severity"
	// SortByAmount puts the largest headline amounts first; findings with no
	// amount trail in severity order.
	SortByAmount SortMode = "amount"
	// SortByPage groups findings by their page (display order), severity
	// within each page.
	SortByPage SortMode = "page"
)

// FilterInsights returns the insights matching every given criterion: a
// case-insensitive substring query over title/detail (empty matches all), a
// minimum severity (SeverityInfo matches all), and a page ("" matches all).
// The input is never mutated; the result preserves input order.
func FilterInsights(in []Insight, query string, minSev Severity, page Page) []Insight {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Insight, 0, len(in))
	for _, ins := range in {
		if ins.Severity < minSev {
			continue
		}
		if page != "" && ins.Page != page {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(ins.Title), q) &&
			!strings.Contains(strings.ToLower(ins.Detail), q) {
			continue
		}
		out = append(out, ins)
	}
	return out
}

// SortInsightsBy orders insights in place by the given mode; unknown modes
// fall back to severity order.
func SortInsightsBy(in []Insight, mode SortMode) {
	switch mode {
	case SortByAmount:
		SortInsights(in) // severity order as the tiebreak baseline
		sort.SliceStable(in, func(i, j int) bool {
			a, b := in[i], in[j]
			if a.HasAmount != b.HasAmount {
				return a.HasAmount
			}
			if a.HasAmount && b.HasAmount && a.Amount.Amount != b.Amount.Amount {
				return abs64(a.Amount.Amount) > abs64(b.Amount.Amount)
			}
			return false // keep severity order among equals
		})
	case SortByPage:
		SortInsights(in)
		order := make(map[Page]int, len(Pages())+1)
		for i, p := range Pages() {
			order[p] = i
		}
		order[PageHub] = len(Pages())
		sort.SliceStable(in, func(i, j int) bool {
			return order[in[i].Page] < order[in[j].Page]
		})
	default:
		SortInsights(in)
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
