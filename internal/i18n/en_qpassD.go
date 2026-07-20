// SPDX-License-Identifier: MIT

package i18n

// qpassDKeys holds English copy for the 2026-07-19 v1.2.7 review lane-D fix:
// making the assistant's token/cost labels interpretable. The per-message note
// used to call the WHOLE request "This reply" and could print a false "$0.00";
// these keys split reply(output) from context(input), route sub-cent spend
// through the honest cost formatter, and label the composer's pre-send estimate
// for what it actually measures (context + the prompt to come).
//
// Kept in its own file and merged via init so the shared en.go is never touched
// under concurrent work.
var qpassDKeys = Catalog{
	// Per-message usage, full split available (live turns + conversations saved
	// after the split was recorded): reply/output tokens lead, context/input is
	// secondary, cost comes last. Args: reply-out tokens, context-in tokens, cost.
	"insights.replyUsageSplit": "Reply: %s tokens out · %s in (context) · %s",
	// Per-message usage, legacy turn whose input/output split wasn't saved: only
	// the total survived, so an accurate cost can't be recomputed. Arg: total.
	"insights.replyUsageTotalNA": "Reply used %s tokens · cost unavailable",
	// Shown in the cost slot when the model has no known pricing.
	"insights.costUnavailable": "cost unavailable",
	// Composer pre-send estimate: names what the number measures rather than an
	// unqualified "~1,164 tokens". Args: privacy scope, grouped token estimate.
	"insights.nextScopeContext": "Next message sends %s from this device · ~%s tokens of context + your prompt",
}

func init() {
	for k, v := range qpassDKeys {
		english[k] = v
	}
}
