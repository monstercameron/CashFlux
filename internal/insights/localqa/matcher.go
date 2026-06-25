// Package localqa provides a deterministic keyword/regex intent matcher for the
// no-API-key AI fallback path. It has no dependencies outside the Go standard
// library and is safe to test natively (no syscall/js, no internal imports).
//
// # Intent precedence
//
// When a phrase could conceivably match more than one Intent the most-specific
// pattern wins. The evaluation order (highest→lowest specificity) is:
//
//  1. SpendingByCategory – phrases like "how much did i spend on X" are more
//     specific than a bare balance enquiry.
//  2. SafeToSpend       – "can i spend" / "safe to spend" is narrower than
//     a generic balance question.
//  3. NetWorth          – "total assets" / "net worth" / "overall financial position".
//  4. UpcomingBills     – due / bills related.
//  5. GoalProgress      – goal / savings goal queries.
//  6. HealthScore       – financial health / health score.
//  7. Balance           – broad "balance" / "how much do i have" catch-all.
//
// IntentNone is returned when none of the above match.
package localqa

import (
	"strings"
)

// Intent identifies the user's financial query intent.
type Intent int

const (
	// IntentNone is returned when no pattern matches the input.
	IntentNone Intent = iota

	// IntentBalance covers questions about current account balance.
	IntentBalance

	// IntentSpendingByCategory covers questions about spending in a specific category.
	IntentSpendingByCategory

	// IntentSafeToSpend covers questions about discretionary spending capacity.
	IntentSafeToSpend

	// IntentNetWorth covers questions about overall asset / net worth position.
	IntentNetWorth

	// IntentUpcomingBills covers questions about bills that are due soon.
	IntentUpcomingBills

	// IntentGoalProgress covers questions about savings goal progress.
	IntentGoalProgress

	// IntentHealthScore covers questions about overall financial health.
	IntentHealthScore
)

// String returns a human-readable label for the Intent.
func (i Intent) String() string {
	switch i {
	case IntentBalance:
		return "Balance"
	case IntentSpendingByCategory:
		return "SpendingByCategory"
	case IntentSafeToSpend:
		return "SafeToSpend"
	case IntentNetWorth:
		return "NetWorth"
	case IntentUpcomingBills:
		return "UpcomingBills"
	case IntentGoalProgress:
		return "GoalProgress"
	case IntentHealthScore:
		return "HealthScore"
	default:
		return "None"
	}
}

// intentRule pairs a set of trigger phrases with an Intent. The phrases are
// matched against the lowercased input using simple substring containment, so
// no regex overhead is needed. Order within the rules slice encodes precedence
// (first match wins — see package-level precedence documentation).
type intentRule struct {
	intent  Intent
	phrases []string
}

// rules are evaluated in order; the first rule whose any phrase is found in
// the lowercased input wins.
var rules = []intentRule{
	{
		IntentSpendingByCategory,
		[]string{
			"spent on",
			"spending on",
			"how much did i spend on",
			"how much have i spent on",
		},
	},
	{
		IntentSafeToSpend,
		[]string{
			"safe to spend",
			"can i spend",
			"how much can i spend",
			"free to spend",
		},
	},
	{
		IntentNetWorth,
		[]string{
			"net worth",
			"total assets",
			"overall financial position",
		},
	},
	{
		IntentUpcomingBills,
		[]string{
			"upcoming bills",
			"bills this month",
			"what's due",
			"whats due",
			"due this month",
			"bills due",
		},
	},
	{
		IntentGoalProgress,
		[]string{
			"goal progress",
			"savings goal",
			"how close am i to",
			"goal status",
		},
	},
	{
		IntentHealthScore,
		[]string{
			"financial health",
			"health score",
			"how am i doing financially",
		},
	},
	{
		IntentBalance,
		[]string{
			"balance",
			"how much do i have",
			"what's in my account",
			"whats in my account",
			"checking balance",
		},
	},
}

// Match classifies text into one of the seven supported Intents.
// It lowercases the input, then evaluates each rule in precedence order
// (see package documentation). Returns (IntentNone, false) when no rule
// matches.
func Match(text string) (Intent, bool) {
	lower := strings.ToLower(text)
	for _, rule := range rules {
		for _, phrase := range rule.phrases {
			if strings.Contains(lower, phrase) {
				return rule.intent, true
			}
		}
	}
	return IntentNone, false
}

// ExtractCategory attempts to pull the category phrase from a
// SpendingByCategory query. It looks for the last occurrence of " on " in the
// lowercased text and returns everything that follows, trimmed of whitespace.
// Returns "" when " on " is not present or the text after it is empty.
//
// Example:
//
//	ExtractCategory("How much did I spend on groceries") → "groceries"
//	ExtractCategory("What is my balance")               → ""
func ExtractCategory(text string) string {
	lower := strings.ToLower(text)
	idx := strings.LastIndex(lower, " on ")
	if idx < 0 {
		return ""
	}
	after := strings.TrimSpace(text[idx+4:]) // preserve original casing
	return after
}
