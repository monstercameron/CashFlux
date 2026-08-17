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

	"github.com/monstercameron/CashFlux/internal/catname"
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
	if a.StatementDay < 0 || a.StatementDay > 31 {
		is.add("statementDay", "must be between 1 and 31")
	}
	// A rate that was never recorded is not a negative rate (WF4-b): there is
	// nothing to reject when nobody has filled one in.
	if r, ok := a.RateAPR(); ok && r < 0 {
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
	// C495: a category cannot be its own parent. categorytree.Build only defends
	// at RENDER time (it re-roots the offender), so without this the bad row
	// persists happily and merely looks odd.
	if c.ParentID != "" && c.ParentID == c.ID {
		is.add("parentId", "cannot be the category itself")
	}
	return is
}

// ValidateCategoryInTree checks c against the categories that already exist:
// the parent must be real, must share c's kind, and must not create a cycle.
// ValidateCategory covers the context-free rules; this one needs the whole set,
// so the write seam calls both.
func ValidateCategoryInTree(c domain.Category, existing []domain.Category) Issues {
	var is Issues
	// C537: a name must be unique among its SIBLINGS. Two "Gas" categories under
	// different parents are legitimate — that is how the tree reads — but two
	// under the same parent are indistinguishable in every list the app draws.
	//
	// Only an edit that actually SETS or CHANGES the name is held to the rule
	// (catname.NameChanged). A household that already has a duplicate pair must
	// still be able to edit one's color or class; blocking an unrelated field
	// because of pre-existing data would be hostile, and the remedy for the pair
	// is a merge, not a jammed form.
	if catname.NameChanged(existing, c) {
		if other, clash := catname.Collision(existing, c); clash {
			where := "at the top level"
			if c.ParentID != "" {
				where = "under the same parent"
			}
			is.add("name", "is already used by "+other.Name+" "+where)
		}
	}
	if c.ParentID == "" {
		return is
	}
	byID := make(map[string]domain.Category, len(existing))
	for _, e := range existing {
		byID[e.ID] = e
	}
	parent, ok := byID[c.ParentID]
	if !ok {
		is.add("parentId", "names a category that does not exist")
		return is
	}
	// An expense nested under income (or the reverse) would roll up into a total
	// that means nothing.
	if parent.Kind != c.Kind {
		is.add("parentId", "must be the same kind as the category")
	}
	// Walk up from the proposed parent: reaching c means the edge closes a loop.
	// Bounded by the number of categories, so a pre-existing cycle in the stored
	// data cannot hang the walk.
	seen := map[string]bool{c.ID: true}
	for id, steps := c.ParentID, 0; id != "" && steps <= len(existing); steps++ {
		if seen[id] {
			is.add("parentId", "would create a loop in the category tree")
			return is
		}
		seen[id] = true
		next, ok := byID[id]
		if !ok {
			break
		}
		id = next.ParentID
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
	// A budget must track something — a single category, several categories, or (cross-
	// category) tags. Historically this was CategoryID-only; the check now also accepts a
	// multi-category or tag-tracking budget.
	if b.CategoryID == "" && len(b.CategoryIDs) == 0 && len(b.TrackedTags) == 0 {
		is.add("categoryId", "is required (or track categories or tags)")
	}
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
