// SPDX-License-Identifier: MIT

package rules

import "strings"

// TxnView is the minimal transaction projection a Condition matches against — kept
// here so the rules engine stays pure (no domain dependency). Text is the payee +
// description; Amount is the absolute value in minor units.
type TxnView struct {
	Text      string
	AccountID string
	Amount    int64
}

// Condition is a richer rule match than a single description substring: all/any
// keyword sets, an account scope, and an inclusive amount range. Every field is
// optional — a zero-value Condition matches everything — so conditions compose by
// AND-ing only the ones that are set. Each set keyword is matched case-insensitively
// as a substring of Text (blank keywords are ignored).
type Condition struct {
	AllKeywords []string // every one must appear (AND)
	AnyKeywords []string // at least one must appear (OR)
	AccountID   string   // restrict to one account ("" = any)
	MinAmount   int64    // inclusive lower bound on the absolute amount (0 = no minimum)
	MaxAmount   int64    // inclusive upper bound (0 = no maximum)
}

// Matches reports whether the transaction satisfies every set part of the condition.
func (c Condition) Matches(t TxnView) bool {
	for _, kw := range c.AllKeywords {
		if strings.TrimSpace(kw) == "" {
			continue
		}
		if !matches(t.Text, kw) {
			return false
		}
	}
	if anyReal(c.AnyKeywords) {
		any := false
		for _, kw := range c.AnyKeywords {
			if strings.TrimSpace(kw) != "" && matches(t.Text, kw) {
				any = true
				break
			}
		}
		if !any {
			return false
		}
	}
	if c.AccountID != "" && t.AccountID != c.AccountID {
		return false
	}
	if c.MinAmount > 0 && t.Amount < c.MinAmount {
		return false
	}
	if c.MaxAmount > 0 && t.Amount > c.MaxAmount {
		return false
	}
	return true
}

// anyReal reports whether the list has at least one non-blank keyword.
func anyReal(kws []string) bool {
	for _, kw := range kws {
		if strings.TrimSpace(kw) != "" {
			return true
		}
	}
	return false
}
