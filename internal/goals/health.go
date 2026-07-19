// SPDX-License-Identifier: MIT

package goals

// Health is the unified pace verdict for a goal — the single answer the Goals card
// badge and the Smart assistant both render, so the two can never contradict ("On
// track" on the card while "your deadline is tight" in Smart). It compares the
// contribution a goal needs each month to hit its deadline against the free monthly
// cash realistically available to fund it.
//
// The verdict is deliberately NOT derived from calendar runway alone (the old card
// badge said "on track" for any dated goal with a far deadline, regardless of whether
// it was actually being funded fast enough). It is the same required-vs-available
// comparison the assistant uses, so the two surfaces agree by construction.
type Health string

const (
	// HealthNone means there is nothing to judge: the goal is covered, paused,
	// undated, or there is no free monthly cash to fund goals from this period. No
	// pace claim is made — the card shows no on-track badge rather than a false one.
	HealthNone Health = ""
	// HealthOnTrack means the required monthly contribution fits within the goal's
	// fair share of the free cash — comfortably fundable at the current pace.
	HealthOnTrack Health = "ontrack"
	// HealthWatch means the goal is fundable, but only by taking MORE than a fair
	// share of the free cash (starving other goals) — the deadline is a stretch.
	HealthWatch Health = "watch"
	// HealthAtRisk means the deadline is unreachable even if ALL free cash went to
	// this one goal — the required pace exceeds the entire monthly surplus.
	HealthAtRisk Health = "atrisk"
)

// AssessHealth returns the shared health verdict from three base-currency figures:
// requiredMonthly (this goal's MonthlyNeeded to hit its deadline), monthlySurplus
// (the household's free cash per month), and activeDeadlinedGoals (how many active,
// incomplete, dated goals share that surplus — for the fair-share split).
//
// The thresholds mirror exactly what the assistant already flags, so a goal it calls
// "tight" reads as Watch/At risk on its card, and only a goal within its fair share
// reads On track:
//
//	required ≤ fairShare              → On track
//	fairShare < required ≤ surplus    → Watch   (needs more than its share)
//	required > surplus                → At risk (unaffordable even with all slack)
//
// A non-positive requiredMonthly (nothing needed) or non-positive surplus (no free
// cash to judge against) returns HealthNone — no assurance is invented.
func AssessHealth(requiredMonthly, monthlySurplus int64, activeDeadlinedGoals int) Health {
	if requiredMonthly <= 0 || monthlySurplus <= 0 {
		return HealthNone
	}
	if requiredMonthly > monthlySurplus {
		return HealthAtRisk
	}
	n := activeDeadlinedGoals
	if n < 1 {
		n = 1
	}
	if requiredMonthly <= monthlySurplus/int64(n) {
		return HealthOnTrack
	}
	return HealthWatch
}
