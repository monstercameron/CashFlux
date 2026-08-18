// SPDX-License-Identifier: MIT

package reconcile

import "github.com/monstercameron/CashFlux/internal/domain"

// This file owns the one question reconciliation cannot avoid: which sign is a
// debt (C683).
//
// # The two conventions
//
// The app STATES money in one convention everywhere a person reads it: positive
// is money you have or are owed, negative is money you owe. A card carrying a
// $500 balance reads "-$500.00", because that is what it does to net worth.
//
// It STORES a liability in either of two, and always has. The "amount you owe"
// add form writes a debt POSITIVE; the sample data and several importers write it
// NEGATIVE. Both are load-bearing and neither can be migrated away without
// touching every household's data, so the app tolerates both and normalizes at
// the edges (ledger.NetWorth takes magnitudes; the account card takes -Abs).
//
// # Why that broke reconciliation
//
// The reconcile dialog compared the RAW STORED cleared balance against the figure
// the user typed off their statement, and said nothing about which sign it wanted.
// So the identical action gave opposite answers depending on an invisible
// per-account storage choice: a $500 debt stored positive reconciled against a
// typed "500", and the same debt stored negative reconciled against "-500". The
// account card, meanwhile, showed "-$500.00" for both — so the dialog and the card
// disagreed about the same account, which is the reported symptom.
//
// Everything here converts between the two so the dialog can work, and speak,
// entirely in the stated convention.

// Convention is how one account stores a liability balance at rest.
type Convention int

const (
	// PositiveOwed stores a debt as a positive number — what the "amount you owe"
	// add form writes. It is the default for an account with nothing to infer
	// from, because that form is how most liabilities are created.
	PositiveOwed Convention = iota
	// NegativeOwed stores a debt as a negative number — the sample-data
	// convention, and what several importers produce.
	NegativeOwed
	// AsStated is an asset: stored and stated are the same number.
	AsStated
)

// ConventionOf infers how an account holds its balance, and says whether the
// answer was READ or GUESSED.
//
// An asset is AsStated and needs no inference, so known is always true for one.
//
// For a liability the opening balance decides it: that is what the account was
// CREATED with, so it records the choice its writer made, and unlike the booked
// balance it does not move. known is true whenever it is non-zero.
//
// With no opening balance there is nothing solid left. The booked balance is the
// only remaining signal and it is a poor one — it crosses zero the moment a card
// goes into credit, and then reports the opposite convention for the same
// account. It is still the best guess available (a card sitting at a positive
// balance is far more often positive-owed debt than negative-owed credit), so it
// is used, but known is false and anything making a CLAIM on the strength of it
// should decline instead.
//
// That distinction is the whole reason for the second return value. Displaying a
// balance has to pick something and a good guess beats nothing; warning a person
// that their data is wrong does not, because a warning built on a coin flip fires
// on correct data, and a check that cries wolf is one people switch off.
func ConventionOf(acc domain.Account, bookedMinor int64) (c Convention, known bool) {
	if acc.Class != domain.ClassLiability {
		return AsStated, true
	}
	if o := acc.OpeningBalance.Amount; o != 0 {
		if o > 0 {
			return PositiveOwed, true
		}
		return NegativeOwed, true
	}
	if bookedMinor > 0 {
		return PositiveOwed, false
	}
	if bookedMinor < 0 {
		return NegativeOwed, false
	}
	// Nothing at all to read. The two conventions agree about zero, so the choice
	// only matters for a figure typed against it later; PositiveOwed is what the
	// "amount you owe" add form writes, which makes it the likelier of the two.
	return PositiveOwed, false
}

// Stated converts a stored minor-unit balance into the convention the app shows
// and the user types: positive is money you have, negative is money you owe.
//
// It is lossless and its own inverse (see Stored), so an overpaid card — a
// liability carrying a credit balance — states as a POSITIVE figure rather than
// collapsing into "even more debt", which is what taking -Abs does.
func Stated(storedMinor int64, c Convention) int64 {
	if c == PositiveOwed {
		return -storedMinor
	}
	return storedMinor
}

// Stored converts a figure in the stated convention back into what the account
// holds at rest — the inverse of Stated, and the conversion any transaction
// written from a reconciliation must go through before it is posted.
func Stored(statedMinor int64, c Convention) int64 {
	if c == PositiveOwed {
		return -statedMinor
	}
	return statedMinor
}
