// SPDX-License-Identifier: MIT

package txnclassify

import (
	"github.com/monstercameron/CashFlux/internal/domain"
)

// # Classifying many rows at once
//
// A statement import does not produce one mis-filed transfer, it produces a
// column of them: every savings sweep for a year, every card payment, each one
// needing the same answer. Asking for that answer once per row is not a workflow,
// it is a punishment for importing.
//
// The hard part is not the loop. It is that a bulk action has to be honest about
// the rows it CANNOT do. A selection made by eye will contain a row already
// posted to the chosen account, or a liability claim that does not apply to it,
// and an operation that silently skips those has told the user it did something
// it did not. So every row gets a verdict, and the ones that fail say why, by id.
//
// This file computes; it does not write. The caller applies the resulting slice
// in one store mutation so the surface re-renders once and a partial failure
// cannot leave half a decision behind.

// BulkFailure is one row the classification could not be applied to, and why.
type BulkFailure struct {
	// TxnID is the row that could not be classified.
	TxnID string
	// Err is the refusal, already phrased for a person by Apply.
	Err error
}

// BulkPlan is what a bulk classification would do, computed before anything is
// written so the consequence can be shown and confirmed rather than discovered.
type BulkPlan struct {
	// Applied holds the rows as they WOULD be saved, in input order. A caller that
	// proceeds writes exactly these.
	Applied []domain.Transaction
	// Failures holds the rows that cannot take this classification, with reasons.
	// A bulk action that silently drops them would be claiming work it did not do.
	Failures []BulkFailure
	// LeavesTotalsMinor is what stops counting as income or spending across the
	// whole plan, in minor units. Not FX-converted — this package has no rate
	// table, and mixing currencies into one figure would be worse than leaving the
	// caller to convert per row.
	LeavesTotalsMinor int64
	// DebtRows is how many of the applied rows would be claimed as debt payments,
	// so a confirmation can state that separately from the transfer itself (C677).
	DebtRows int
}

// PlanBulk computes the classification of every row in txns against one
// counterparty, without writing.
//
// countTowardDebt is a request, not an instruction, and it is applied ONLY where
// it means something: rows whose counterparty is a liability. A selection that
// mixes a card payment with a savings sweep is the normal case, and refusing the
// whole batch because half of it cannot be a debt payment would make the option
// unusable exactly where it is most useful. Rows that cannot take the claim are
// still classified as transfers — they simply do not carry the payment intent.
//
// Nothing here creates a counterpart row, and nothing touches Amount, Date or
// CategoryID: a bulk action is the widest blast radius in the app, and it gets
// the narrowest possible mandate.
func PlanBulk(txns []domain.Transaction, counterpartyID string, countTowardDebt bool,
	accounts []domain.Account) BulkPlan {
	var plan BulkPlan
	// Whether the claim is even offerable is a property of the ACCOUNT, so it is
	// resolved once rather than per row.
	claimApplies := false
	if acc, ok := domain.AccountByID(accounts, counterpartyID); ok {
		claimApplies = countTowardDebt && acc.Class == domain.ClassLiability
	}
	for _, t := range txns {
		next, err := Apply(t, counterpartyID, claimApplies, accounts)
		if err != nil {
			plan.Failures = append(plan.Failures, BulkFailure{TxnID: t.ID, Err: err})
			continue
		}
		if !t.IsTransfer() && next.IsTransfer() && t.CountsInReports() {
			amt := t.Amount.Amount
			if amt < 0 {
				amt = -amt
			}
			plan.LeavesTotalsMinor += amt
		}
		if next.BillAccountID != "" {
			plan.DebtRows++
		}
		plan.Applied = append(plan.Applied, next)
	}
	return plan
}

// Eligible splits txns into the rows a bulk classification against counterpartyID
// could touch and the rows it could not, so a selection UI can disable or explain
// the incompatible ones before the user commits rather than after.
//
// The common incompatibility is a row already posted TO the chosen account: a
// transaction cannot be a transfer to its own account, and a selection made by
// eye across a mixed ledger will contain some.
func Eligible(txns []domain.Transaction, counterpartyID string) (ok, blocked []domain.Transaction) {
	for _, t := range txns {
		if counterpartyID != "" && t.AccountID == counterpartyID {
			blocked = append(blocked, t)
			continue
		}
		ok = append(ok, t)
	}
	return ok, blocked
}
