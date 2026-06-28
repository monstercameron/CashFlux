// SPDX-License-Identifier: MIT

package rules

import (
	"strconv"
	"strings"
	"time"
)

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
		found := false
		for _, kw := range c.AnyKeywords {
			if strings.TrimSpace(kw) != "" && matches(t.Text, kw) {
				found = true
				break
			}
		}
		if !found {
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

// --- C105: structured field/op/value conditions ---

// TxnDate wraps a time.Time for the structured condition evaluator. The zero
// value (IsZero) means "no date" and date conditions will not match.
type TxnDate struct {
	t time.Time
}

// NewTxnDate wraps a time.Time for use in structured condition matching.
func NewTxnDate(t time.Time) TxnDate { return TxnDate{t: t} }

// MatchConditions evaluates a slice of RuleConditions against one transaction
// (provided via its individual fields) using AND semantics: all conditions must
// match for the function to return true. An empty slice returns true (matches all).
//
// Parameters:
//   - conditions: the set of structured conditions to evaluate (ANDed together)
//   - payee: the transaction payee string
//   - desc: the transaction description
//   - amountMinor: signed amount in minor currency units (e.g. cents)
//   - accountID: the account ID the transaction belongs to
//   - txnDate: the transaction date (zero value disables date conditions)
func MatchConditions(conditions []RuleCondition, payee, desc string, amountMinor int64, accountID string, txnDate TxnDate) bool {
	for _, c := range conditions {
		if !matchOneCondition(c, payee, desc, amountMinor, accountID, txnDate) {
			return false
		}
	}
	return true
}

// matchOneCondition evaluates a single RuleCondition. Returns false if the value
// cannot be parsed for numeric/date operators — an unparseable condition does not
// crash; it simply does not match.
func matchOneCondition(c RuleCondition, payee, desc string, amountMinor int64, accountID string, txnDate TxnDate) bool {
	switch c.Field {
	case ConditionFieldPayee:
		return matchText(payee, c.Op, c.Value)
	case ConditionFieldDescription:
		return matchText(desc, c.Op, c.Value)
	case ConditionFieldAmount:
		return matchAmount(amountMinor, c.Op, c.Value)
	case ConditionFieldAccount:
		return matchAccount(accountID, c.Op, c.Value)
	case ConditionFieldDate:
		return matchDate(txnDate.t, c.Op, c.Value)
	default:
		// Unknown field: treat as not-matching so users notice typos.
		return false
	}
}

// matchText evaluates a text condition (payee or description field).
func matchText(fieldVal string, op ConditionOp, value string) bool {
	switch op {
	case ConditionOpContains:
		return matches(fieldVal, value)
	case ConditionOpEquals:
		return strings.EqualFold(strings.TrimSpace(fieldVal), strings.TrimSpace(value))
	default:
		return false
	}
}

// matchAmount evaluates a numeric condition against the signed amount in minor
// units. The condition value must be an integer string in the same denomination
// as amountMinor (e.g. cents). An unparseable value returns false.
func matchAmount(amountMinor int64, op ConditionOp, value string) bool {
	v, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return false
	}
	switch op {
	case ConditionOpEq:
		return amountMinor == v
	case ConditionOpNeq:
		return amountMinor != v
	case ConditionOpLt:
		return amountMinor < v
	case ConditionOpGt:
		return amountMinor > v
	case ConditionOpLte:
		return amountMinor <= v
	case ConditionOpGte:
		return amountMinor >= v
	default:
		return false
	}
}

// matchAccount evaluates an account identity condition.
func matchAccount(accountID string, op ConditionOp, value string) bool {
	switch op {
	case ConditionOpIs:
		return accountID == strings.TrimSpace(value)
	case ConditionOpIsNot:
		return accountID != strings.TrimSpace(value)
	default:
		return false
	}
}

// matchDate evaluates a date condition. value formats:
//   - "YYYY-MM-DD" for on / before / after
//   - "YYYY-MM"    for in-month
//
// A zero txnDate causes all date conditions to return false.
func matchDate(txnDate time.Time, op ConditionOp, value string) bool {
	if txnDate.IsZero() {
		return false
	}
	value = strings.TrimSpace(value)
	switch op {
	case ConditionOpInMonth:
		// Expect "YYYY-MM"
		parts := strings.SplitN(value, "-", 2)
		if len(parts) != 2 {
			return false
		}
		year, err1 := strconv.Atoi(parts[0])
		month, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return txnDate.Year() == year && int(txnDate.Month()) == month
	case ConditionOpOn:
		target, err := time.Parse("2006-01-02", value)
		if err != nil {
			return false
		}
		return sameDay(txnDate, target)
	case ConditionOpBefore:
		target, err := time.Parse("2006-01-02", value)
		if err != nil {
			return false
		}
		// Before the start of the target day.
		return txnDate.UTC().Before(time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC))
	case ConditionOpAfter:
		target, err := time.Parse("2006-01-02", value)
		if err != nil {
			return false
		}
		// After the end of the target day.
		dayEnd := time.Date(target.Year(), target.Month(), target.Day(), 23, 59, 59, 999999999, time.UTC)
		return txnDate.UTC().After(dayEnd)
	default:
		return false
	}
}

// sameDay reports whether a and b fall on the same calendar day in UTC.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}
