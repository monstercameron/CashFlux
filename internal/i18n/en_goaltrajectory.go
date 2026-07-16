// SPDX-License-Identifier: MIT

package i18n

// goalTrajectoryKeys holds the copy for a goal card's savings-trajectory chart:
// the section heading, the on-track readout, the "beyond the horizon" readout for
// a very slow pace, and the empty state shown when no contribution is set yet.
// Merged via init so this file does not touch en.go.
var goalTrajectoryKeys = Catalog{
	// Section heading above the projected-balance chart.
	"goaltrajectory.heading": "Savings trajectory",
	// On-track readout when NO target date is set — the projection supplies the date.
	// %s = target amount, %s = landing month ("March 2027"), %d = whole months from now.
	"goaltrajectory.onTrack": "On track to reach %s by %s — %d months away.",
	// One-month edge case reads naturally. %s = target amount, %s = landing month.
	"goaltrajectory.onTrackOneMonth": "On track to reach %s by %s — about a month away.",
	// When a target DATE is set, foreground the delta vs. that date instead of
	// restating it. %s = the target month ("Dec 2026"); %d = whole months of slack.
	"goaltrajectory.onPace":    "On pace for your %s target.",
	"goaltrajectory.ahead":     "About %d months ahead of your %s target.",
	"goaltrajectory.aheadOne":  "About a month ahead of your %s target.",
	"goaltrajectory.behind":    "About %d months past your %s target — consider a larger monthly amount.",
	"goaltrajectory.behindOne": "About a month past your %s target — consider a larger monthly amount.",
	// Reachable this month (already at target after this period). %s = target amount.
	"goaltrajectory.reachedNow": "You've reached %s — goal complete.",
	// Pace too slow to land within the projection horizon. %s = target amount.
	"goaltrajectory.beyond": "At this pace, %s is more than ten years away — consider a larger monthly amount.",
	// Empty state: no monthly contribution to project from yet.
	"goaltrajectory.empty": "Add a monthly contribution to see a savings projection.",
	// Accessible label for the chart itself. %s = goal name.
	"goaltrajectory.chartLabel": "Projected balance over time for %s",
}

func init() {
	for k, v := range goalTrajectoryKeys {
		english[k] = v
	}
}
