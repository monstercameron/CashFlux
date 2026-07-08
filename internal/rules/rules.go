// SPDX-License-Identifier: MIT

// Package rules is the auto-categorization engine: user-defined rules that match
// a transaction's payee/description and assign a category (and tags). Matching is
// pure and deterministic — first matching rule wins — and unit-tested on native
// Go. The UI manages the rules and applies them on entry/import.
package rules

import "strings"

// ConditionField names the transaction field a structured condition inspects.
type ConditionField string

const (
	ConditionFieldPayee       ConditionField = "payee"
	ConditionFieldDescription ConditionField = "description"
	ConditionFieldAmount      ConditionField = "amount"
	ConditionFieldAccount     ConditionField = "account"
	ConditionFieldDate        ConditionField = "date"
)

// ConditionOp is the comparison operator for a structured condition.
type ConditionOp string

const (
	// Text operators (payee, description).
	ConditionOpContains ConditionOp = "contains"
	ConditionOpEquals   ConditionOp = "equals"
	// Numeric operators (amount — matched against the signed amount in minor units).
	ConditionOpEq  ConditionOp = "=="
	ConditionOpNeq ConditionOp = "!="
	ConditionOpLt  ConditionOp = "<"
	ConditionOpGt  ConditionOp = ">"
	ConditionOpLte ConditionOp = "<="
	ConditionOpGte ConditionOp = ">="
	// Account identity operators.
	ConditionOpIs    ConditionOp = "is"
	ConditionOpIsNot ConditionOp = "is-not"
	// Date operators (value format: "YYYY-MM-DD" or "YYYY-MM" for in-month).
	ConditionOpOn      ConditionOp = "on"
	ConditionOpBefore  ConditionOp = "before"
	ConditionOpAfter   ConditionOp = "after"
	ConditionOpInMonth ConditionOp = "in-month" // value = "YYYY-MM"
)

// RuleCondition is a single structured condition on one transaction field.
// Value is always stored as a string; the matching logic parses it to float64
// or a date when the field/op requires it. An invalid parse causes the
// condition to be treated as not-matching.
type RuleCondition struct {
	Field ConditionField `json:"field"`
	Op    ConditionOp    `json:"op"`
	Value string         `json:"value"`
}

// Rule matches transaction text and assigns a category and/or tags. When
// RenameDesc is non-empty the matching transaction's description is replaced
// with that value, enabling a "clean up payee/description" action (C102).
//
// Structured conditions (C105): if Conditions is non-empty, all conditions are
// evaluated with AND and the legacy Match field is ignored. If Conditions is
// empty (nil or zero-length), matching falls back to the legacy substring match
// against Match. This design is fully backward-compatible: existing rules with
// no Conditions continue to work exactly as before.
type Rule struct {
	ID            string
	Match         string // case-insensitive substring matched against payee + description
	SetCategoryID string
	SetTags       []string
	RenameDesc    string `json:",omitempty"` // when set, overwrites the transaction description on match
	// SetBillAccountID, when set, links a matching transaction as a BILL PAYMENT toward
	// that account (Transaction.BillAccountID) — so future/imported payments to a merchant
	// auto-tie to the account the user first linked one to. Applied only when the
	// transaction has no bill link yet (a manual link is never overwritten).
	SetBillAccountID string          `json:",omitempty"`
	Order            int             `json:",omitempty"` // precedence: lower runs first (first match wins)
	Conditions       []RuleCondition `json:",omitempty"` // C105: structured conditions (ANDed); overrides Match when non-empty
}

// matches reports whether pattern (trimmed, case-insensitive) is a substring of
// text. An empty pattern never matches.
func matches(text, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}

// FirstMatch returns the first rule whose Match is found in text, or nil.
// Rules with structured Conditions are skipped by this function — use
// FirstMatchFull when you have all transaction fields available.
func FirstMatch(rs []Rule, text string) *Rule {
	for i := range rs {
		// Skip condition-bearing rules; they require full transaction context.
		if len(rs[i].Conditions) > 0 {
			continue
		}
		if matches(text, rs[i].Match) {
			return &rs[i]
		}
	}
	return nil
}

// FirstMatchFull returns the first rule that matches the full transaction
// context: structured conditions when present, otherwise the legacy Match
// substring. It supersedes FirstMatch at all call-sites that have a full
// transaction view.
func FirstMatchFull(rs []Rule, payee, desc string, amountMinor int64, accountID string, txnDate TxnDate) *Rule {
	return FirstMatchFullWhere(rs, payee, desc, amountMinor, accountID, txnDate, nil)
}

// ruleMatchesFull reports whether one rule matches the full transaction context
// (structured conditions when present, otherwise the legacy substring Match).
func ruleMatchesFull(r Rule, payee, desc string, amountMinor int64, accountID string, txnDate TxnDate) bool {
	if len(r.Conditions) > 0 {
		return MatchConditions(r.Conditions, payee, desc, amountMinor, accountID, txnDate)
	}
	return matches(payee+" "+desc, r.Match)
}

// FirstMatchFullWhere returns the first rule that both matches the transaction and
// satisfies pred (nil pred = any match). It lets a caller pick, independently, the
// first rule carrying a PARTICULAR action — e.g. the first matching rule that sets a
// bill account — so that action isn't shadowed by an earlier rule whose only job is a
// different action (category/tags/rename). Preserves first-match-wins ordering.
func FirstMatchFullWhere(rs []Rule, payee, desc string, amountMinor int64, accountID string, txnDate TxnDate, pred func(Rule) bool) *Rule {
	for i := range rs {
		if pred != nil && !pred(rs[i]) {
			continue
		}
		if ruleMatchesFull(rs[i], payee, desc, amountMinor, accountID, txnDate) {
			return &rs[i]
		}
	}
	return nil
}

// Conflict reports a rule that can never fire under first-match-wins.
type Conflict struct {
	Index      int // the shadowed rule's index
	ShadowedBy int // the earlier rule that shadows it, or -1 if the rule has no match phrase
}

// Conflicts returns rules that never run. A rule is shadowed when an earlier
// rule's match phrase is a substring of its own (case-insensitive): any text that
// matches the later rule already matched the earlier one, which wins. A rule with
// an empty match phrase matches nothing and is reported with ShadowedBy -1. Only
// the first shadower found is reported per rule. Condition-bearing rules are
// excluded from shadow analysis (their conditions may be disjoint).
func Conflicts(rs []Rule) []Conflict {
	var out []Conflict
	for j := range rs {
		// Condition-bearing rules skip shadow analysis.
		if len(rs[j].Conditions) > 0 {
			continue
		}
		later := strings.ToLower(strings.TrimSpace(rs[j].Match))
		if later == "" {
			out = append(out, Conflict{Index: j, ShadowedBy: -1})
			continue
		}
		for i := 0; i < j; i++ {
			if len(rs[i].Conditions) > 0 {
				continue
			}
			earlier := strings.ToLower(strings.TrimSpace(rs[i].Match))
			if earlier != "" && strings.Contains(later, earlier) {
				out = append(out, Conflict{Index: j, ShadowedBy: i})
				break
			}
		}
	}
	return out
}

// Category returns the category id assigned by the first rule matching the
// payee/description (legacy Match path only), or "" if none match.
func Category(rs []Rule, payee, desc string) string {
	if r := FirstMatch(rs, payee+" "+desc); r != nil {
		return r.SetCategoryID
	}
	return ""
}

// Tags returns the tags assigned by the first rule matching the payee/
// description, or nil if none match.
func Tags(rs []Rule, payee, desc string) []string {
	if r := FirstMatch(rs, payee+" "+desc); r != nil {
		return r.SetTags
	}
	return nil
}

// RenamedDesc returns the replacement description set by the first matching
// rule, or "" if no rule matches or the matching rule has no RenameDesc.
func RenamedDesc(rs []Rule, payee, desc string) string {
	if r := FirstMatch(rs, payee+" "+desc); r != nil {
		return r.RenameDesc
	}
	return ""
}
