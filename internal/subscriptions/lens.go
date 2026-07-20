// SPDX-License-Identifier: MIT

package subscriptions

import "strings"

// subscriptionPhrases mark a commitment that reads as a SUBSCRIPTION: a service
// billed on a schedule that the household signed up for and could cancel.
//
// The list is deliberately POSITIVE. The tempting shortcut — "a subscription is
// anything that is not a loan payment and not a utility" — turns the lens into a
// junk drawer: HOA dues, property tax and home insurance are none of those
// things either, and none of them is a subscription. A lens that answers "what
// am I subscribed to?" with the property tax bill has lied about its own name,
// so membership has to be claimed, not inferred by elimination.
//
// Matched case-insensitively as substrings against the commitment's name and its
// category name.
var subscriptionPhrases = []string{
	"subscription",
	"subscribe",
	"membership",
	"member",
	"streaming",
	"stream",
	"software",
	"saas",
	"apps",
	"app store",
	"cloud",
	"hosting",
	"premium",
	"plus",
	"pro ",
	"music",
	"video",
	"tv",
	"news",
	"magazine",
	"gym",
	"fitness",
	"club",
	"box",
	"license",
	"seat",
	"storage",
	"backup",
	"vpn",
}

// IsSubscriptionLikeName reports whether a name reads as a subscription service.
// Exported for the same reason IsEssentialName is: callers hold different text
// (a commitment label, a resolved payee, a category name) and must all reach the
// same verdict from it.
func IsSubscriptionLikeName(s string) bool {
	lower := strings.ToLower(s)
	for _, phrase := range subscriptionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// IsSubscriptionCommitment is the whole judgment behind the Subscriptions lens:
// does this recurring commitment belong under "what am I subscribed to?".
//
// It answers yes only when the commitment is BOTH not something else — not a
// loan or card payment, not essential household spend (utilities, rent,
// insurance, tax), not a habitual merchant (the coffee run that looks like a
// subscription to every amount test) — AND positively subscription-shaped, by
// its own name, by the name of the category it is filed under, or because the
// detection engine already knows a subscription by that name.
//
// detected holds lower-cased names the subscription machinery has found or the
// user has confirmed; pass nil when that evidence is unavailable. categoryName
// may be empty.
func IsSubscriptionCommitment(name, categoryName string, detected map[string]bool) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	if IsLenderName(n) || IsEssentialName(n) || IsHabitualName(n) {
		return false
	}
	if categoryName != "" && (IsEssentialName(categoryName) || IsHabitualName(categoryName)) {
		return false
	}
	if detected != nil && detected[strings.ToLower(n)] {
		return true
	}
	return IsSubscriptionLikeName(n) || IsSubscriptionLikeName(categoryName)
}
