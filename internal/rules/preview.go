// SPDX-License-Identifier: MIT

package rules

import "github.com/monstercameron/CashFlux/internal/payeeclean"

// MatchCount returns how many of texts the rule would match — the "matches N
// existing transactions" preview a user wants before applying a rule to existing
// data (which is otherwise blind). Each text is typically a transaction's
// payee + " " + description, matched case-insensitively as a substring.
func (r Rule) MatchCount(texts []string) int {
	n := 0
	for _, t := range texts {
		if matches(t, r.Match) {
			n++
		}
	}
	return n
}

// Covered returns how many of texts are matched by at least one rule (first-match-
// wins), so the UI can show "N of M transactions auto-file by your rules" — a
// coverage signal that surfaces the gap of uncategorized-but-coverable entries.
func Covered(rs []Rule, texts []string) int {
	n := 0
	for _, t := range texts {
		if FirstMatch(rs, t) != nil {
			n++
		}
	}
	return n
}

// Uncovered returns how many of texts no rule matches — the rules a user might
// still want to add.
func Uncovered(rs []Rule, texts []string) int {
	return len(texts) - Covered(rs, texts)
}

// TxnCtx is a full transaction projection for conditions-aware previews: the
// same fields FirstMatchFull evaluates, so preview counts and live matching can
// never disagree.
type TxnCtx struct {
	Payee       string
	Desc        string
	AmountMinor int64
	AccountID   string
	Date        TxnDate
}

// matchesFull reports whether the rule matches one full transaction context —
// structured conditions when present (AND), else the legacy Match substring.
// This is the single-rule form of FirstMatchFull's per-rule check.
func (r Rule) matchesFull(c TxnCtx) bool {
	if len(r.Conditions) > 0 {
		return MatchConditions(r.Conditions, c.Payee, c.Desc, c.AmountMinor, c.AccountID, c.Date)
	}
	return matches(c.Payee+" "+c.Desc+" "+payeeclean.Suggest(c.Payee), r.Match)
}

// MatchesTxn reports whether the rule matches one full transaction context —
// the exported single-transaction form of MatchCountFull's check, used to build
// affected-transaction previews (the list behind the count).
func (r Rule) MatchesTxn(c TxnCtx) bool { return r.matchesFull(c) }

// MatchCountFull returns how many of the transaction contexts the rule would
// match, evaluating structured conditions when present. It supersedes
// MatchCount wherever full transaction context is available — a plain-text
// count reads "0 transactions caught" for a condition rule that actually
// catches hundreds.
func (r Rule) MatchCountFull(txns []TxnCtx) int {
	n := 0
	for _, c := range txns {
		if r.matchesFull(c) {
			n++
		}
	}
	return n
}

// CoveredFull returns how many transaction contexts are matched by at least
// one rule under first-match-wins, evaluating structured conditions — the
// conditions-aware form of Covered.
func CoveredFull(rs []Rule, txns []TxnCtx) int {
	n := 0
	for _, c := range txns {
		if FirstMatchFull(rs, c.Payee, c.Desc, c.AmountMinor, c.AccountID, c.Date) != nil {
			n++
		}
	}
	return n
}
