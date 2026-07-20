// SPDX-License-Identifier: MIT

package i18n

// detail5Keys holds the English strings for the 2026-07-19 fine-detail polish
// lane on the Assistant / Insights surface: grouping a long run of same-kind
// findings into one collapsible row, a consistent "source" cue on deterministic
// insight rows, and a compact recent-conversations list in the empty chat body.
// Merged via init so this file does not touch en.go.
var detail5Keys = Catalog{
	// Grouped flagged-activity row: a run of same-kind findings folds into one
	// collapsible summary. The heading names the kind + the count so a wall of
	// "hasn't posted yet" reads as one line the user can expand on demand.
	"detail5.groupExpected":  "%d expected payments haven't posted yet",
	"detail5.groupDuplicate": "%d possible duplicate transactions",
	"detail5.groupSpike":     "%d spending spikes to review",
	"detail5.groupBalance":   "%d balance anomalies to review",
	"detail5.groupGeneric":   "%d related findings",

	// The group's expand/collapse toggle and its aria labels (the count rides the
	// name so assistive tech announces how many rows the control reveals).
	"detail5.groupExpand":      "Show all",
	"detail5.groupCollapse":    "Hide",
	"detail5.groupExpandAria":  "Show the %d findings in this group",
	"detail5.groupCollapseAria": "Hide the %d findings in this group",

	// The group's primary action for missed/expected payments: jump to the
	// recurring surface where bills and expected payments are managed.
	"detail5.reviewBills":     "Review bills",
	"detail5.reviewBillsAria": "Review your bills and expected payments",

	// A consistent "source" cue on the deterministic insight rows that drill to
	// the transactions behind them (category shifts, top merchants). It matches
	// the flagged-activity row's Source affordance so evidence is reachable the
	// same way from every deterministic finding.
	"detail5.sourceHint": "Open the transactions behind this",

	// Compact recent-conversations list in the empty chat body (returning users
	// see their last few chats without opening the side rail).
	"detail5.recentLabel": "Pick up a recent chat",
}

func init() {
	for k, v := range detail5Keys {
		english[k] = v
	}
}
