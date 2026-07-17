// SPDX-License-Identifier: MIT

package notify

import "strings"

// TargetKind classifies what specific entity a notification is about, so a click
// can land on that exact item (a filtered ledger, a flashed account/budget row)
// instead of the whole page.
type TargetKind string

const (
	TargetNone    TargetKind = ""
	TargetTxn     TargetKind = "txn"     // a specific transaction (unusual/large charge, paycheck)
	TargetAccount TargetKind = "account" // a specific account (stale/low balance, bill-due source)
	TargetBudget  TargetKind = "budget"  // a specific budget (near/over threshold)
)

// Target is the specific entity a notification points at.
type Target struct {
	Kind TargetKind
	ID   string // the entity id (transaction / account / budget)
}

// ParseTarget extracts the specific entity a notification id points at, from the
// DedupeKey format "ruleID@occurrence" the feed uses. It is the inverse of the
// *Candidates occurrence keys and is pure/deterministic, so a click handler can
// resolve exactly which transaction/account/budget to focus. Unknown shapes return
// TargetNone (the caller falls back to a plain page navigation).
func ParseTarget(id string) Target {
	rule, occ, ok := strings.Cut(id, "@")
	if !ok {
		return Target{}
	}
	switch {
	case strings.HasPrefix(occ, "unusual:"):
		return Target{TargetTxn, strings.TrimPrefix(occ, "unusual:")}
	case strings.HasPrefix(occ, "txn:"):
		return Target{TargetTxn, strings.TrimPrefix(occ, "txn:")}
	case strings.HasPrefix(occ, "paycheck:"):
		return Target{TargetTxn, strings.TrimPrefix(occ, "paycheck:")}
	case strings.HasPrefix(occ, "lowbal:"):
		return Target{TargetAccount, beforeAt(strings.TrimPrefix(occ, "lowbal:"))}
	}
	switch rule {
	case "default-stale", "default-bill-due":
		// occ = "<acctID>@<week|date>"
		return Target{TargetAccount, beforeAt(occ)}
	case "default-budget":
		// occ = "<budgetID>:<state>@<month>"
		return Target{TargetBudget, beforeColon(beforeAt(occ))}
	}
	return Target{}
}

// beforeAt returns everything up to the first "@" (or the whole string when absent).
func beforeAt(s string) string {
	if i := strings.IndexByte(s, '@'); i >= 0 {
		return s[:i]
	}
	return s
}

// beforeColon returns everything up to the first ":" (or the whole string when absent).
func beforeColon(s string) string {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[:i]
	}
	return s
}
