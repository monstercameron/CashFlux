// SPDX-License-Identifier: MIT

// Package rules is the auto-categorization engine: user-defined rules that match
// a transaction's payee/description and assign a category (and tags). Matching is
// pure and deterministic — first matching rule wins — and unit-tested on native
// Go. The UI manages the rules and applies them on entry/import.
package rules

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/payeeclean"
)

// ConditionField names the transaction field a structured condition inspects.
type ConditionField string

const (
	ConditionFieldPayee              ConditionField = "payee"
	ConditionFieldDescription        ConditionField = "description"
	ConditionFieldAmount             ConditionField = "amount"
	ConditionFieldFlow               ConditionField = "flow"
	ConditionFieldLegacySignedAmount ConditionField = "legacySignedAmount"
	ConditionFieldAccount            ConditionField = "account"
	ConditionFieldDate               ConditionField = "date"
)

// ConditionOp is the comparison operator for a structured condition.
type ConditionOp string

const (
	// Text operators; equals also compares flow direction.
	ConditionOpContains ConditionOp = "contains"
	ConditionOpEquals   ConditionOp = "equals"
	// Numeric operators (amount — matched against magnitude in minor units).
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
// Value is stored as a string; the matching logic parses it as minor units or a
// date when the field/operator requires it. Amount comparisons use magnitude;
// flow conditions use "inflow" or "outflow". An invalid parse causes the
// condition to be treated as not-matching.
type RuleCondition struct {
	Field ConditionField `json:"field"`
	Op    ConditionOp    `json:"op"`
	Value string         `json:"value"`
	// Currency scopes amount conditions to this currency. Value remains integer
	// minor units; an empty currency preserves transaction-local matching.
	Currency string `json:"currency,omitempty"`
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
	ID    string
	Match string // case-insensitive substring matched against payee + description
	// LegacySubstring preserves the exact matching behavior of saved rules.
	LegacySubstring bool `json:",omitempty"`
	SetCategoryID   string
	SetTags         []string
	RenameDesc      string `json:",omitempty"` // when set, overwrites the transaction description on match
	// SetBillAccountID, when set, links a matching transaction as a BILL PAYMENT toward
	// that account (Transaction.BillAccountID) — so future/imported payments to a merchant
	// auto-tie to the account the user first linked one to. Applied only when the
	// transaction has no bill link yet (a manual link is never overwritten).
	SetBillAccountID string `json:",omitempty"`
	// SetMemberID, SetReviewed and SetExcludeFromReports are the three actions the
	// commercial-benchmark audit found missing (C373).
	//
	// All three are APPLY-ONCE, never-undo, matching the bill-link precedent
	// above and for the same reason: a rule is a standing instruction, and a
	// standing instruction that can silently reverse a person's explicit choice
	// is a different and much worse thing. So a member is assigned only to a
	// transaction that has none — a rule must never quietly reassign whose
	// spending something was — and Reviewed/ExcludeFromReports only ever go from
	// false to true.
	//
	// Two benchmark actions are deliberately absent rather than stubbed. SPLIT is
	// a feature, not a gap: it needs percentages or amounts per line and a
	// remainder policy, none of which the rule form has a vocabulary for.
	// GOAL-LINK has no target: a transaction carries no goal reference at all —
	// goals attach to accounts and categories — so it is a data-model question,
	// not a missing action.
	SetMemberID           string          `json:",omitempty"`
	SetReviewed           bool            `json:",omitempty"`
	SetExcludeFromReports bool            `json:",omitempty"`
	Order                 int             `json:",omitempty"` // precedence: lower runs first (first match wins)
	Conditions            []RuleCondition `json:",omitempty"` // C105: structured conditions (ANDed); overrides Match when non-empty
	// HitCount and LastRunAt are the rule's DURABLE record of what it has
	// actually done: how many transactions it has filed since it was created, and
	// when it last filed one (C372).
	//
	// The workbench already shows a live match count, recomputed every render —
	// but that answers "what would this rule catch in today's ledger", which is a
	// different question from "has this rule ever done anything". A rule whose
	// merchant stopped appearing shows a live count of zero and looks broken;
	// with a hit count it reads as a rule that worked forty times and has gone
	// quiet, which is what the user needs to decide whether to keep it.
	//
	// Incremented by every path that files a transaction through a rule:
	// auto-apply on add/import, the full backfill, and a single-rule backfill.
	HitCount  int       `json:",omitempty"`
	LastRunAt time.Time `json:",omitempty"`
}

// RecordHits returns a copy of the rule credited with n more matches at the
// given time. A non-positive count is a no-op, so a caller can hand it a tally
// without checking first.
func (r Rule) RecordHits(n int, at time.Time) Rule {
	if n <= 0 {
		return r
	}
	r.HitCount += n
	if at.After(r.LastRunAt) {
		r.LastRunAt = at
	}
	return r
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

// matchesRule reports whether pattern occurs in text beginning at a word
// boundary. The pattern may continue into the rest of a word. Unicode letters
// and digits form words; punctuation and whitespace separate them.
func matchesRule(text, pattern string) bool {
	textRunes, patternRunes := []rune(strings.ToLower(text)), []rune(strings.ToLower(strings.TrimSpace(pattern)))
	if len(patternRunes) == 0 || len(patternRunes) > len(textRunes) {
		return false
	}
	wordStart := unicode.IsLetter(patternRunes[0]) && !unicode.In(patternRunes[0], unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai)
	wordEnd := isRuleWordRune(patternRunes[len(patternRunes)-1])
	for start := 0; start+len(patternRunes) <= len(textRunes); start++ {
		if wordStart && start > 0 && isRuleWordRune(textRunes[start-1]) {
			continue
		}
		end := start + len(patternRunes)
		if wordEnd && end < len(textRunes) && isRuleWordRune(textRunes[end]) {
			continue
		}
		match := true
		for i, r := range patternRunes {
			if textRunes[start+i] != r {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func isRuleWordRune(r rune) bool {
	if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai) {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
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
		matched := matchesRule(text, rs[i].Match)
		if rs[i].LegacySubstring {
			matched = matches(text, rs[i].Match)
		}
		if matched {
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
// The legacy haystack includes the deterministically CLEANED payee alongside
// the raw one, so a rule written against the tidy merchant name ("Blue Bottle
// Coffee") matches the processor descriptor ("SQ *BLUE BOTTLE #47") too —
// commercial parity: rules target either the original or the cleaned value.
func ruleMatchesFull(r Rule, payee, desc string, amountMinor int64, accountID string, txnDate TxnDate) bool {
	return ruleMatchesFullCurrency(r, payee, desc, amountMinor, "", accountID, txnDate, payeeclean.Suggest(payee))
}

func ruleMatchesFullCurrency(r Rule, payee, desc string, amountMinor int64, currency, accountID string, txnDate TxnDate, cleanPayee string) bool {
	if len(r.Conditions) > 0 {
		return MatchConditionsWithCurrency(r.Conditions, payee, desc, amountMinor, currency, accountID, txnDate)
	}
	haystack := payee + " " + desc + " " + cleanPayee
	if r.LegacySubstring {
		return matches(haystack, r.Match)
	}
	return matchesRule(haystack, r.Match)
}

// MigrateLegacyRuleSemantics upgrades pre-magnitude amount conditions and
// preserves the substring matcher used by previously saved phrase rules.
func MigrateLegacyRuleSemantics(rs []Rule) ([]Rule, bool) {
	out := append([]Rule(nil), rs...)
	changed := false
	for i := range out {
		r := &out[i]
		if !r.LegacySubstring && len(r.Conditions) == 0 && strings.TrimSpace(r.Match) != "" {
			r.LegacySubstring = true
			changed = true
		}
		conds := append([]RuleCondition(nil), r.Conditions...)
		conditionChanged := false
		for j := range conds {
			c := &conds[j]
			if c.Field != ConditionFieldAmount {
				continue
			}
			n, err := strconv.ParseInt(strings.TrimSpace(c.Value), 10, 64)
			if err != nil {
				continue
			}
			hasFlow := false
			for _, existing := range r.Conditions {
				if existing.Field == ConditionFieldFlow {
					hasFlow = true
					break
				}
			}
			if hasFlow {
				continue
			}
			flow := ""
			switch {
			case n < 0 && c.Op == ConditionOpLt:
				flow, c.Op, c.Value = "outflow", ConditionOpGt, strconv.FormatInt(amountMagnitude(n), 10)
			case n < 0 && c.Op == ConditionOpLte:
				flow, c.Op, c.Value = "outflow", ConditionOpGte, strconv.FormatInt(amountMagnitude(n), 10)
			case n < 0 && c.Op == ConditionOpEq:
				flow, c.Value = "outflow", strconv.FormatInt(amountMagnitude(n), 10)
			case n > 0 && c.Op == ConditionOpEq:
				flow = "inflow"
			case n == 0 && (c.Op == ConditionOpEq || c.Op == ConditionOpNeq):
				// Magnitude zero has the same equality result for either sign.
			case n == 0 && c.Op == ConditionOpLt:
				flow = "outflow"
			case n == 0 && c.Op == ConditionOpGt:
				flow = "inflow"
			case n > 0 && c.Op == ConditionOpGt, n > 0 && c.Op == ConditionOpGte:
				flow = "inflow"
			}
			if flow != "" {
				conds = append(conds, RuleCondition{Field: ConditionFieldFlow, Op: ConditionOpEquals, Value: flow})
				changed = true
				conditionChanged = true
			} else if c.Field == ConditionFieldAmount && (n != 0 || (c.Op != ConditionOpEq && c.Op != ConditionOpNeq)) {
				// Signed ranges that cross zero (and signed !=) cannot be
				// represented as an AND of magnitude and flow predicates.
				c.Field = ConditionFieldLegacySignedAmount
				changed = true
				conditionChanged = true
			}
		}
		if conditionChanged {
			r.Conditions = conds
		}
	}
	return out, changed
}

// FirstMatchFullWhere returns the first rule that both matches the transaction and
// satisfies pred (nil pred = any match). It lets a caller pick, independently, the
// first rule carrying a PARTICULAR action — e.g. the first matching rule that sets a
// bill account — so that action isn't shadowed by an earlier rule whose only job is a
// different action (category/tags/rename). Preserves first-match-wins ordering.
func FirstMatchFullWhere(rs []Rule, payee, desc string, amountMinor int64, accountID string, txnDate TxnDate, pred func(Rule) bool) *Rule {
	return FirstMatchFullWhereCurrency(rs, payee, desc, amountMinor, "", accountID, txnDate, pred)
}

// FirstMatchFullWhereCurrency returns the first matching rule with the
// transaction amount currency available to structured amount conditions.
func FirstMatchFullWhereCurrency(rs []Rule, payee, desc string, amountMinor int64, currency, accountID string, txnDate TxnDate, pred func(Rule) bool) *Rule {
	cleanPayee := payeeclean.Suggest(payee)
	for i := range rs {
		if pred != nil && !pred(rs[i]) {
			continue
		}
		if ruleMatchesFullCurrency(rs[i], payee, desc, amountMinor, currency, accountID, txnDate, cleanPayee) {
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
			if earlier != "" && matchesRule(later, earlier) {
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
