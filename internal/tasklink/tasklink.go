// SPDX-License-Identifier: MIT

// Package tasklink provides pure helpers for resolving a domain.Task's
// RelatedType + RelatedID pair into a display name and navigation route.
// It has no syscall/js dependency and is fully unit-testable on native Go.
package tasklink

import (
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Route returns the screen path for a given RelatedType, e.g. "account" → "/accounts".
// An empty string is returned for RelatedNone or unrecognised types.
func Route(rt domain.RelatedType) string {
	switch rt {
	case domain.RelatedAccount:
		return "/accounts"
	case domain.RelatedBudget:
		return "/budgets"
	case domain.RelatedGoal:
		return "/goals"
	case domain.RelatedTransaction:
		return "/transactions"
	case domain.RelatedReviewQueue:
		// The aggregate review task lives on the transactions page; the UI layer
		// additionally opens the guided Review inbox on arrival (UX-10).
		return "/transactions"
	default:
		return ""
	}
}

// TypeLabel returns a short human-readable noun for the RelatedType, suitable
// for use as an <option> label (e.g. "Account", "Budget").
func TypeLabel(rt domain.RelatedType) string {
	switch rt {
	case domain.RelatedAccount:
		return "Account"
	case domain.RelatedBudget:
		return "Budget"
	case domain.RelatedGoal:
		return "Goal"
	case domain.RelatedTransaction:
		return "Transaction"
	case domain.RelatedReviewQueue:
		return "Review inbox"
	default:
		return "None"
	}
}

// EntityName searches the provided slices for the entity whose ID matches id
// and returns its display name. The return value is ("", false) when no match
// is found (entity was deleted or id is empty).
func EntityName(
	rt domain.RelatedType,
	id string,
	accounts []domain.Account,
	budgets []domain.Budget,
	goals []domain.Goal,
	transactions []domain.Transaction,
) (name string, ok bool) {
	if id == "" || rt == domain.RelatedNone || rt == "" {
		return "", false
	}
	switch rt {
	case domain.RelatedAccount:
		for _, a := range accounts {
			if a.ID == id {
				return a.Name, true
			}
		}
	case domain.RelatedBudget:
		for _, b := range budgets {
			if b.ID == id {
				return b.Name, true
			}
		}
	case domain.RelatedGoal:
		for _, g := range goals {
			if g.ID == id {
				return g.Name, true
			}
		}
	case domain.RelatedTransaction:
		for _, tx := range transactions {
			if tx.ID == id {
				n := tx.Payee
				if n == "" {
					n = tx.Desc
				}
				return n, true
			}
		}
	case domain.RelatedReviewQueue:
		// The link target is the inbox itself, not a record — always resolvable.
		return "Review inbox", true
	}
	return "", false
}
