// SPDX-License-Identifier: MIT

// Package memberimpact computes what would change if a household member left:
// the accounts, fractional shares, budgets, goals, and tagged transactions that
// would need reassignment. It is the read-only preview over the same facts the
// reassign-on-delete machinery acts on — pure aggregation, no persistence.
package memberimpact

import "github.com/monstercameron/CashFlux/internal/domain"

// Breakdown lists everything attached to one member, by kind. Name slices keep
// the entities' display names in their incoming order.
type Breakdown struct {
	// AccountsOwned are accounts whose OwnerID is the member.
	AccountsOwned []string
	// AccountShares are accounts where the member holds a fractional
	// ownership share without being the sole owner.
	AccountShares []string
	// Budgets and Goals are entities whose OwnerID is the member.
	Budgets []string
	// Goals holds the goal names owned by the member.
	Goals []string
	// TxnCount is the number of transactions tagged with the member
	// (Transaction.MemberID), independent of account ownership.
	TxnCount int
}

// Total is the number of attachments that would need reassignment — the same
// figure the reassign-before-delete gate counts.
func (b Breakdown) Total() int {
	return len(b.AccountsOwned) + len(b.AccountShares) + len(b.Budgets) + len(b.Goals) + b.TxnCount
}

// Empty reports whether the member owns nothing — deletable with no reassignment.
func (b Breakdown) Empty() bool { return b.Total() == 0 }

// Compute assembles the departure breakdown for one member.
func Compute(memberID string, accounts []domain.Account, budgets []domain.Budget, goals []domain.Goal, txns []domain.Transaction) Breakdown {
	var b Breakdown
	for _, a := range accounts {
		switch {
		case a.OwnerID == memberID:
			b.AccountsOwned = append(b.AccountsOwned, a.Name)
		default:
			if _, ok := a.OwnershipShares[memberID]; ok {
				b.AccountShares = append(b.AccountShares, a.Name)
			}
		}
	}
	for _, bd := range budgets {
		if bd.OwnerID == memberID {
			b.Budgets = append(b.Budgets, bd.Name)
		}
	}
	for _, g := range goals {
		if g.OwnerID == memberID {
			b.Goals = append(b.Goals, g.Name)
		}
	}
	for _, t := range txns {
		if t.MemberID == memberID {
			b.TxnCount++
		}
	}
	return b
}
