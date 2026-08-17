// SPDX-License-Identifier: MIT

// Package txnclassify decides what KIND of movement a transaction is — ordinary
// spending, a neutral move between your own accounts, or a payment toward a debt
// — and applies that decision to a transaction without touching its money.
//
// # Why this exists
//
// A statement import files every row as plain income or expense, because that is
// all a bank line says. Money moved from checking to savings arrives as an
// expense; the savings side arrives as income; a credit-card payment arrives as
// spending. None of that is true — nothing was earned and nothing was consumed —
// and every figure built on income or expense inherits the error.
//
// The app already knows how to model this correctly:
// domain.Transaction.TransferAccountID makes IsTransfer() true, which drops the
// row from income, expense, budget spend, category spend and reports in every
// aggregation that asks. What was missing was a way to SET it on a row that
// already exists. Transfers could only be born as transfers (CreateTransferPair,
// the assistant); an imported row had no path to become one.
//
// # What it deliberately does not do
//
// It never touches Amount, Date or CategoryID.
//
//   - The amount and its sign are already correct: an imported row records what
//     the account actually did, and a leg's sign is which side of the move it is,
//     not a classification. Re-signing it here would corrupt correct data.
//   - The category is inert on a transfer (every aggregation short-circuits on
//     IsTransfer before it reads a category), so clearing it would destroy the
//     import's evidence to no effect, and un-classifying would not bring it back.
//
// It also never creates the counterpart leg. Classifying an imported row means
// its far side is usually ALSO an imported row waiting to be classified; writing
// a third transaction would invent money that never moved.
package txnclassify

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Kind is how a transaction reads to the reporting layer.
type Kind string

const (
	// KindPlain is ordinary income or spending — the money entered or left the
	// household. Every imported row starts here.
	KindPlain Kind = "plain"
	// KindNeutral is a move between two accounts you own, neither of which is a
	// debt: savings sweeps, moving cash between checking accounts. Nothing was
	// earned or spent, and the household is exactly as rich as before.
	KindNeutral Kind = "neutral"
	// KindDebt is a move whose counterpart is a liability — a card or loan
	// payment. Also not spending, but distinct from KindNeutral because it is
	// the act that reduces what you owe, and the debt surfaces report on it.
	KindDebt Kind = "debt"
)

// KindOf reports how t currently reads, resolving the counterparty against the
// live account list. A transfer pointing at an account that no longer exists
// reads as KindNeutral: it is still not income or spending, which is the part
// that matters to every total, and guessing KindDebt from a missing account
// would be inventing a fact.
func KindOf(t domain.Transaction, accounts []domain.Account) Kind {
	if !t.IsTransfer() {
		return KindPlain
	}
	if acc, ok := domain.AccountByID(accounts, t.TransferAccountID); ok && acc.Class == domain.ClassLiability {
		return KindDebt
	}
	return KindNeutral
}

// Choice is one account a transaction could be classified against.
type Choice struct {
	// AccountID is the counterparty account's id.
	AccountID string
	// Name is its display name.
	Name string
	// IsDebt reports whether it is a liability, so the caller can offer the
	// debt-payment option only where it means something.
	IsDebt bool
}

// Counterparties lists the accounts t could name as the far side of a move: every
// account except the one it is already posted to. Order follows the input, so a
// caller that sorted its accounts keeps that order.
func Counterparties(t domain.Transaction, accounts []domain.Account) []Choice {
	out := make([]Choice, 0, len(accounts))
	for _, a := range accounts {
		if a.ID == t.AccountID {
			continue // a transaction cannot be a transfer to its own account
		}
		out = append(out, Choice{AccountID: a.ID, Name: a.Name, IsDebt: a.Class == domain.ClassLiability})
	}
	return out
}

// Apply returns t reclassified against counterpartyID.
//
// An empty counterpartyID clears the classification: the row becomes plain income
// or spending again, and any debt-payment link is dropped with it (the link is
// only meaningful while the row names a debt).
//
// countTowardDebt additionally tags the row as a payment toward the counterparty
// liability (domain.Transaction.BillAccountID), which is what the debt surfaces
// read for "the payment actually made this month". It is rejected for a
// counterparty that is not a liability rather than ignored, because silently
// dropping half of what the caller asked for is how two surfaces end up
// disagreeing about the same row.
//
// Reviewed is set when the classification actually changes, on the same reasoning
// CreateTransferPair records its own legs as reviewed: choosing what a row IS is
// an explicit decision by a person, and a row that goes on being flagged
// "needs review" after someone reviewed it contradicts them.
func Apply(t domain.Transaction, counterpartyID string, countTowardDebt bool, accounts []domain.Account) (domain.Transaction, error) {
	before := t.TransferAccountID
	beforeBill := t.BillAccountID

	if counterpartyID == "" {
		if countTowardDebt {
			return t, fmt.Errorf("txnclassify: a debt payment needs an account to pay toward")
		}
		t.TransferAccountID = ""
		t.BillAccountID = ""
		if before != "" || beforeBill != "" {
			t.Reviewed = true
		}
		return t, nil
	}

	if counterpartyID == t.AccountID {
		return t, fmt.Errorf("txnclassify: a transaction cannot be a transfer to its own account")
	}
	acc, ok := domain.AccountByID(accounts, counterpartyID)
	if !ok {
		return t, fmt.Errorf("txnclassify: account %q not found", counterpartyID)
	}
	if countTowardDebt && acc.Class != domain.ClassLiability {
		return t, fmt.Errorf("txnclassify: %s is not a debt, so nothing can be paid toward it", acc.Name)
	}

	t.TransferAccountID = counterpartyID
	t.BillAccountID = ""
	if countTowardDebt {
		t.BillAccountID = counterpartyID
	}
	if before != t.TransferAccountID || beforeBill != t.BillAccountID {
		t.Reviewed = true
	}
	return t, nil
}
