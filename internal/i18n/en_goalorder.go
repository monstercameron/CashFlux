// SPDX-License-Identifier: MIT

package i18n

// goalOrderKeys holds the English strings for the Goals "Needs a plan" lead
// section (2026-07-19 Watch-first ordering refinement): a compact section that
// floats the goals needing a decision — missed deadlines plus the Watch / At
// risk pace verdicts — above the healthy funds and goals, so the actionable
// items lead the first viewport. Its own map, self-registered via init, so the
// shared en.go is never touched by this concurrent lane.
var goalOrderKeys = Catalog{
	"goals.needsPlanSection": "Needs a plan",
	"goals.needsPlanHint":    "These are behind or past their target date — a missed deadline, or funding that can't keep pace. Re-date, set more aside, or archive what no longer matters.",
}

func init() {
	for k, v := range goalOrderKeys {
		english[k] = v
	}
}
