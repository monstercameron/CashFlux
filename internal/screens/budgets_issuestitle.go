// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// budgetIssuesTitle names what the plan-level rail is holding, instead of
// counting it (C588).
//
// The rail said "2 items to review from this period", which sounds like a
// transaction or budget review inbox. It is neither: the three things it can
// carry live on three different pages — an over-assignment goes to Allocate,
// follow-ups to the To-do list, a sinking-fund shortfall to Goals — and none of
// that was predictable before clicking. Naming the contents makes the
// destination guessable, and each row's own button names the page it opens.
//
// The parts are joined rather than templated per combination: seven sentences
// for three independent flags would be seven strings to keep in agreement, and
// the joined form reads the same either way ("an over-assignment and 2
// follow-ups from this period").
func budgetIssuesTitle(overAssigned, fundShort, followUps, hist bool) string {
	var parts []string
	if overAssigned {
		parts = append(parts, uistate.T("budgets.issuePartOverAssigned"))
	}
	if fundShort {
		parts = append(parts, uistate.T("budgets.issuePartFundShort"))
	}
	if followUps {
		parts = append(parts, uistate.T("budgets.issuePartFollowUps"))
	}
	if len(parts) == 0 {
		return uistate.T("budgets.issuesRailOne")
	}
	joined := joinWithAnd(parts)
	if hist {
		return uistate.T("budgets.issuesRailHist", joined)
	}
	return uistate.T("budgets.issuesRailNamed", joined)
}

// joinWithAnd renders a list as English: "a", "a and b", "a, b and c".
func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " " + uistate.T("common.and") + " " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " " + uistate.T("common.and") + " " + parts[len(parts)-1]
}
