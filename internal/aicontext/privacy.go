// SPDX-License-Identifier: MIT

package aicontext

import "strings"

// ConversationTier is the per-conversation privacy choice for what the assistant
// is allowed to see (AG17). It is deliberately coarser and more legible than the
// graded Tier above: the user picks between sharing everything or sharing only
// aggregates, and a visible chip states which is active. Enforcement lives here,
// in one pure choke point, rather than in prompt wording the model might ignore.
type ConversationTier string

const (
	// TierFull shares the complete bounded context: aggregates, KPIs, breakdowns,
	// and recent transactions — the default, richest experience.
	TierFull ConversationTier = "full"
	// TierAggregatesOnly shares engine variables (KPIs) and category totals but
	// ZERO transaction- or payee-level detail — right-sized for questions like
	// "am I saving enough?" that never need a specific merchant.
	TierAggregatesOnly ConversationTier = "aggregates-only"
)

// ParseConversationTier maps a stored/UI string to a ConversationTier, defaulting
// to TierFull for anything unrecognized so a bad value fails open to the normal
// experience rather than silently blanking the context.
func ParseConversationTier(s string) ConversationTier {
	if strings.TrimSpace(strings.ToLower(s)) == string(TierAggregatesOnly) {
		return TierAggregatesOnly
	}
	return TierFull
}

// Label is a short, human-readable name for the tier, for the chip that states the
// active privacy level in the chat.
func (t ConversationTier) Label() string {
	if t == TierAggregatesOnly {
		return "Aggregates only"
	}
	return "Full detail"
}

// Redact returns a copy of in with any transaction- or payee-level detail removed
// when the tier is aggregates-only: RecentTxns and TopPayees are dropped, so no
// merchant name, transaction description, or per-transaction amount can reach the
// model. Engine variables (Formulas) and category totals (TopCategories) — the
// aggregate signal — are preserved. For TierFull the inputs pass through unchanged.
//
// This is the single enforcement point AG17 relies on: filtering the DATA, not
// hoping the prompt holds.
func Redact(in Inputs, tier ConversationTier) Inputs {
	if tier != TierAggregatesOnly {
		return in
	}
	out := in
	out.RecentTxns = nil
	out.TopPayees = nil
	return out
}

// DetailToolNames are the read tools that expose individual transaction- or
// payee-level rows to the model. Under TierAggregatesOnly the caller withholds
// these so the aggregates-only promise holds for tool results too, not just the
// injected context block.
var DetailToolNames = map[string]bool{
	"list_transactions":               true,
	"list_uncategorized_transactions": true,
	"find_duplicate_transactions":     true,
}

// ToolAllowed reports whether a tool may be offered to the model at the given
// tier. Every tool is allowed at TierFull; under TierAggregatesOnly the
// detail-exposing read tools are withheld while aggregate and action tools remain.
func ToolAllowed(name string, tier ConversationTier) bool {
	if tier != TierAggregatesOnly {
		return true
	}
	return !DetailToolNames[name]
}
