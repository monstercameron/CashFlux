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

// NeedsAttention reports whether an insight belongs in the "Needs you" bucket:
// Warn or Alert severity — a decision or an action, not a calm observation.
// Nudge and Info findings are "Watching" material. The hub defaults to showing
// only Needs-you findings so a hundred-strong catalog doesn't greet the user as
// a wall of homework.
func NeedsAttention(i Insight) bool { return i.Severity >= SeverityWarn }

// DedupeInsights removes findings that repeat the same conclusion — an identical
// Title and Detail — keeping the FIRST occurrence and preserving order. Multiple
// engines can independently reach the same read (e.g. two rules both flagging a
// low balance before payday); surfacing it twice is noise. Callers that sort by
// severity first keep the strongest-toned copy. The input is not mutated; the
// result is freshly allocated.
func DedupeInsights(in []Insight) []Insight {
	seen := make(map[string]struct{}, len(in))
	out := make([]Insight, 0, len(in))
	for _, ins := range in {
		sig := ins.Title + "\x00" + ins.Detail
		if _, dup := seen[sig]; dup {
			continue
		}
		seen[sig] = struct{}{}
		out = append(out, ins)
	}
	return out
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
