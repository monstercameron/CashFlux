// SPDX-License-Identifier: MIT

// Package validate checks CashFlux domain entities for structural correctness:
// required fields, valid enum values, positive amounts, consistent currencies,
// and sane references. It returns all problems at once as Issues so forms can
// surface them together.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package validate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Issue is a single validation problem tied to a field.
type Issue struct {
	Field   string
	Message string
}

// Issues is a collection of validation problems. The empty slice means valid.
type Issues []Issue

// OK reports whether there are no issues.
func (is Issues) OK() bool { return len(is) == 0 }

// Error implements error, joining all issues.
func (is Issues) Error() string {
	if len(is) == 0 {
		return ""
	}
	parts := make([]string, len(is))
	for i, issue := range is {
		parts[i] = fmt.Sprintf("%s %s", issue.Field, issue.Message)
	}
	return strings.Join(parts, "; ")
}

func (is *Issues) add(field, message string) { *is = append(*is, Issue{field, message}) }

func (is *Issues) require(field, value string) {
	if strings.TrimSpace(value) == "" {
		is.add(field, "is required")
	}
}

func validCode(c string) bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func (is *Issues) requireCurrency(field, code string) {
	if !validCode(code) {
		is.add(field, "must be a 3-letter currency code")
	}
}

// ValidateMember checks a member.
func ValidateMember(m domain.Member) Issues {
	var is Issues
	is.require("name", m.Name)
	return is
}

// ValidateAccount checks an account, including class/type consistency, currency,
// opening-balance currency match, and score ranges.
func ValidateAccount(a domain.Account) Issues {
	var is Issues
	is.require("name", a.Name)
	is.require("ownerId", a.OwnerID)
	if !a.Scope.Valid() {
		is.add("scope", "is invalid")
	}
	if !a.Type.Valid() {
		is.add("type", "is invalid")
	}
	if !a.Class.Valid() {
		is.add("class", "is invalid")
	} else if a.Type.Valid() && a.Type != domain.TypeOther && a.Class != a.Type.Class() {
		// "Other" is the catch-all type with no natural class, so it may be either an
		// asset or an explicit liability (e.g. an HOA obligation). Every other type has a
		// fixed class the stored class must match.
		is.add("class", "does not match the account type")
	}
	is.requireCurrency("currency", a.Currency)
	if a.OpeningBalance.Currency != "" && a.OpeningBalance.Currency != a.Currency {
		is.add("openingBalance", "currency must match the account currency")
	}
	if a.LiquidityScore < 0 || a.LiquidityScore > 100 {
		is.add("liquidityScore", "must be between 0 and 100")
	}
	if a.StabilityScore < 0 || a.StabilityScore > 100 {
		is.add("stabilityScore", "must be between 0 and 100")
	}
	if a.DueDayOfMonth < 0 || a.DueDayOfMonth > 28 {
		is.add("dueDayOfMonth", "must be between 1 and 28")
	}
	if a.InterestRateAPR < 0 {
		is.add("interestRateApr", "cannot be negative")
	}
	return is
}

// ValidateCategory checks a category.
func ValidateCategory(c domain.Category) Issues {
	var is Issues
	is.require("name", c.Name)
	if !c.Kind.Valid() {
		is.add("kind", "is invalid")
	}
	return is
}

// ValidateTransaction checks a transaction.
func ValidateTransaction(t domain.Transaction) Issues {
	var is Issues
	is.require("accountId", t.AccountID)
	is.require("desc", t.Desc)
	is.requireCurrency("amount", t.Amount.Currency)
	if t.Date.IsZero() {
		is.add("date", "is required")
	}
	if t.IsTransfer() && t.TransferAccountID == t.AccountID {
		is.add("transferAccountId", "must differ from the source account")
	}
	return is
}

// ValidateBudget checks a budget.
func ValidateBudget(b domain.Budget) Issues {
	var is Issues
	is.require("name", b.Name)
	is.require("ownerId", b.OwnerID)
	is.require("categoryId", b.CategoryID)
	if !b.Scope.Valid() {
		is.add("scope", "is invalid")
	}
	if !b.Period.Valid() {
		is.add("period", "is invalid")
	}
	if b.Limit.Amount <= 0 {
		is.add("limit", "must be greater than zero")
	}
	return is
}

// ValidateGoal checks a goal. Validation is kind-aware: a financial goal needs a
// positive money target, a habit needs a positive check-in target, and checklist
// / milestone goals (whose progress comes from linked to-dos or a manual done
// flag) require neither. The empty kind is treated as financial.
func ValidateGoal(g domain.Goal) Issues {
	var is Issues
	is.require("name", g.Name)
	is.require("ownerId", g.OwnerID)
	if !g.Scope.Valid() {
		is.add("scope", "is invalid")
	}
	if g.Kind != "" && !g.Kind.Valid() {
		is.add("kind", "is invalid")
	}
	switch g.EffectiveKind() {
	case domain.GoalKindFinancial:
		if g.TargetAmount.Amount <= 0 {
			is.add("targetAmount", "must be greater than zero")
		}
		if g.CurrentAmount.Currency != "" && g.TargetAmount.Currency != "" &&
			g.CurrentAmount.Currency != g.TargetAmount.Currency {
			is.add("currentAmount", "currency must match the target amount")
		}
	case domain.GoalKindHabit:
		if g.HabitTarget <= 0 {
			is.add("habitTarget", "must be greater than zero")
		}
	}
	return is
}

// ValidateTask checks a task.
func ValidateTask(t domain.Task) Issues {
	var is Issues
	is.require("title", t.Title)
	if !t.Status.Valid() {
		is.add("status", "is invalid")
	}
	if !t.Priority.Valid() {
		is.add("priority", "is invalid")
	}
	if t.RelatedType != "" && !t.RelatedType.Valid() {
		is.add("relatedType", "is invalid")
	}
	if t.RelatedType.Valid() && t.RelatedType != domain.RelatedNone && strings.TrimSpace(t.RelatedID) == "" {
		is.add("relatedId", "is required when a related type is set")
	}
	return is
}
