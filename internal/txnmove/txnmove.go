// SPDX-License-Identifier: MIT

// Package txnmove reposts a transaction to a different account — the correction
// for a row that arrived on the wrong one.
//
// # Why this exists
//
// An import can put a row on the wrong account: two statements loaded in the
// wrong order, a credit union that exports its sub-accounts into one file, a
// manual entry made before the right account existed. Every other fact about such
// a row is correctable from the editor — payee, amount, date, category, whether
// it is a transfer — and the account it is posted to was not. The only way out
// was to delete the row and retype it, which throws away its links, its splits,
// its history and its id.
//
// # What moving actually touches
//
// Only AccountID. The amount is what the money did and does not change with the
// account it is filed under; the date, category and payee describe the purchase,
// not the account.
//
// Two things DO have to follow the move, and they are the reason this is a
// package rather than a one-line setter:
//
//   - A transfer leg's partner names this row's account as ITS counterparty. Move
//     one leg and the other still points at the account the money is no longer on,
//     so the pair stops describing a real movement. The partner comes along.
//   - Currency is not cosmetic. Balances fold every row into the account's own
//     currency, and a mismatch does not just skip the row — ledger.bulkBalances
//     drops the whole ACCOUNT from the result. A move that mixed currencies would
//     silently blank a balance, so it is refused outright rather than converted:
//     converting would rewrite what the bank actually recorded.
package txnmove

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

// Plan is what moving a transaction would do, computed before anything is
// written so the consequence can be shown and confirmed.
type Plan struct {
	// Err is why the move would be refused, if it would be.
	Err error
	// NoOp reports that the row is already on the requested account. Not an error:
	// re-choosing the current account is a person confirming, not a mistake.
	NoOp bool
	// FromName and ToName are the two accounts, for a sentence about the move.
	FromName, ToName string
	// Partner is the transfer counterpart that must follow this move, and
	// HasPartner whether there is one. The partner is returned so a preview can
	// say the pair moves together rather than let a user discover it after.
	Partner    domain.Transaction
	HasPartner bool
	// BreaksReconciliation reports that the row is cleared and dated at or before
	// the SOURCE account's reconciled-through date — so moving it changes a
	// balance somebody has already checked against a statement. Not a refusal: it
	// is exactly the case where a mis-import most needs correcting, and the person
	// doing it is entitled to know what it disturbs.
	BreaksReconciliation bool
}

// PlanMove computes the move of t onto toAccountID without writing anything.
func PlanMove(t domain.Transaction, toAccountID string, accounts []domain.Account,
	txns []domain.Transaction) Plan {
	var p Plan
	from, hasFrom := domain.AccountByID(accounts, t.AccountID)
	if hasFrom {
		p.FromName = from.Name
	}
	to, ok := domain.AccountByID(accounts, toAccountID)
	if !ok {
		p.Err = fmt.Errorf("txnmove: account %q not found", toAccountID)
		return p
	}
	p.ToName = to.Name
	if toAccountID == t.AccountID {
		p.NoOp = true
		return p
	}
	// A transfer cannot have itself on both sides. Moving a leg onto its own
	// counterparty would make it exactly that, so it is refused with the reason
	// rather than silently clearing the transfer.
	if t.IsTransfer() && t.TransferAccountID == toAccountID {
		p.Err = fmt.Errorf("txnmove: %s is already the other side of this transfer, so the row cannot move onto it", to.Name)
		return p
	}
	// Currency: refused, not converted. See the package doc — a mismatch drops the
	// whole account out of the balance map, and converting would rewrite what the
	// bank recorded.
	if t.Amount.Currency != "" && to.Currency != "" && t.Amount.Currency != to.Currency {
		p.Err = fmt.Errorf("txnmove: this row is in %s and %s is in %s — moving it would leave the account's balance uncomputable",
			t.Amount.Currency, to.Name, to.Currency)
		return p
	}

	if t.IsTransfer() {
		if partner, found := findPartner(t, txns); found {
			p.Partner, p.HasPartner = partner, true
		}
	}
	if t.Cleared && hasFrom {
		if through, has := reconcile.Through(from.Reconciliations); has && !t.Date.After(through) {
			p.BreaksReconciliation = true
		}
	}
	return p
}

// Apply moves t onto toAccountID and returns the moved row, plus the transfer
// partner updated to name the new account when there is one.
//
// The partner is returned rather than written: this package is pure, and the
// caller writes both rows in one store mutation so a half-moved pair cannot
// survive a failure between them.
func Apply(t domain.Transaction, toAccountID string, accounts []domain.Account,
	txns []domain.Transaction) (moved domain.Transaction, partner domain.Transaction, hasPartner bool, err error) {
	p := PlanMove(t, toAccountID, accounts, txns)
	if p.Err != nil {
		return t, domain.Transaction{}, false, p.Err
	}
	if p.NoOp {
		return t, domain.Transaction{}, false, nil
	}
	moved = t
	moved.AccountID = toAccountID
	if p.HasPartner {
		partner = p.Partner
		// The partner's counterparty was this row's OLD account. It has to name the
		// new one, or the pair describes a movement between accounts that no longer
		// has a row on one end.
		partner.TransferAccountID = toAccountID
		hasPartner = true
	}
	return moved, partner, hasPartner, nil
}

// findPartner locates the far side of a transfer leg. It prefers the durable
// group id (C680) and falls back to the resemblance match for legs written
// before that existed — the same order appstate uses, for the same reason: an
// edited or FX pair never satisfies the resemblance test.
func findPartner(t domain.Transaction, txns []domain.Transaction) (domain.Transaction, bool) {
	if t.TransferGroupID != "" {
		for _, c := range txns {
			if c.ID != t.ID && c.TransferGroupID == t.TransferGroupID {
				return c, true
			}
		}
		// Grouped but alone: do NOT fall through and adopt a look-alike. A row that
		// names a relationship whose other end is missing is a question for a
		// person, not an invitation to guess.
		return domain.Transaction{}, false
	}
	for _, c := range txns {
		if c.ID == t.ID || !c.IsTransfer() {
			continue
		}
		if c.AccountID == t.TransferAccountID && c.TransferAccountID == t.AccountID &&
			c.Amount.Amount == -t.Amount.Amount && sameDay(c.Date, t.Date) {
			return c, true
		}
	}
	return domain.Transaction{}, false
}

func sameDay(a, b time.Time) bool { return a.Equal(b) }
