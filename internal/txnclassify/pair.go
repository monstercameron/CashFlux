// SPDX-License-Identifier: MIT

package txnclassify

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// # Finding the other leg
//
// Classifying an imported row does not create its counterpart — see the package
// doc for why inventing a third transaction would be inventing money. But the
// far side usually DOES exist, as another imported row waiting for the same
// treatment, and whether it was found changes what the user should be told:
//
//	found     — "the matching row in Savings will be linked too"
//	not found — "only this side was imported; the other account will not balance"
//
// Saying nothing is what leaves someone to discover a one-sided transfer later,
// from a balance that disagrees with their bank.
//
// This is a READ. It reports what exists; it never writes either leg.

// PairWindow is how far apart two legs of the same move may be dated and still
// be recognised as a pair. Banks post the two sides on different days —
// same-day is common, next-day is normal, and a weekend can push it further —
// so the window is deliberately wider than "same date" and still narrow enough
// that two unrelated round-number moves in the same month do not collide.
const PairWindow = 4 * 24 * time.Hour

// Match is a candidate far side of a move.
type Match struct {
	// Txn is the counterpart row.
	Txn domain.Transaction
	// Exact reports whether the amounts are exactly opposite. A near match
	// (same account pair and window, different magnitude) is still worth showing
	// — a wire fee often makes the two sides differ by a few units — but the
	// caller must be able to say which it found.
	Exact bool
}

// FindReciprocal looks for the far side of t in txns, assuming t is (or is about
// to be) a transfer to counterpartyID.
//
// A candidate has to be a DIFFERENT row, posted to the counterparty account,
// dated within PairWindow, and moving money the other way. The best exact match
// wins; failing that the closest-dated opposite-direction row is returned as a
// near match. Rows already linked to some other transaction are skipped, because
// a leg that is spoken for is not available to be this row's partner.
//
// The second return reports whether anything was found at all, so a caller can
// distinguish "no partner exists" from "a poor partner exists".
func FindReciprocal(t domain.Transaction, counterpartyID string, txns []domain.Transaction) (Match, bool) {
	if counterpartyID == "" || t.Amount.Amount == 0 {
		return Match{}, false
	}
	var best Match
	var found bool
	var bestGap time.Duration
	for _, c := range txns {
		if c.ID == t.ID || c.AccountID != counterpartyID {
			continue
		}
		// Opposite direction. Two rows moving the same way are not two sides of
		// one move, however close their dates.
		if (c.Amount.Amount > 0) == (t.Amount.Amount > 0) {
			continue
		}
		// Already paired with something else.
		if c.IsTransfer() && c.TransferAccountID != t.AccountID {
			continue
		}
		gap := c.Date.Sub(t.Date)
		if gap < 0 {
			gap = -gap
		}
		if gap > PairWindow {
			continue
		}
		exact := c.Amount.Amount == -t.Amount.Amount && c.Amount.Currency == t.Amount.Currency
		// An exact match always beats a near one; between two of the same kind,
		// the closer date wins.
		if !found || (exact && !best.Exact) || (exact == best.Exact && gap < bestGap) {
			best, bestGap, found = Match{Txn: c, Exact: exact}, gap, true
		}
	}
	return best, found
}

// Preview is what classifying a row would do, computed before anything is
// written so the consequence can be shown rather than discovered.
type Preview struct {
	// Kind is how the row would read afterwards.
	Kind Kind
	// CounterpartyName is the account it would point at ("" when clearing).
	CounterpartyName string
	// Partner is the far side, when one was found.
	Partner Match
	// HasPartner reports whether Partner is set.
	HasPartner bool
	// LeavesTotalsMinor is the amount that would stop counting as income or
	// spending — the size of the correction, in the row's own currency.
	LeavesTotalsMinor int64
	// Err is why the classification would be refused, if it would be. A preview
	// that cannot be applied has to say so before the button is pressed.
	Err error
}

// PreviewApply reports what Apply would do to t, without doing it. It runs the
// same validation Apply does, so a preview never promises a save that will fail.
func PreviewApply(t domain.Transaction, counterpartyID string, countTowardDebt bool,
	accounts []domain.Account, txns []domain.Transaction) Preview {
	var p Preview
	next, err := Apply(t, counterpartyID, countTowardDebt, accounts)
	if err != nil {
		p.Err = err
		return p
	}
	p.Kind = KindOf(next, accounts)
	if acc, ok := domain.AccountByID(accounts, counterpartyID); ok {
		p.CounterpartyName = acc.Name
	}
	// Only a row that currently counts is leaving anything; re-pointing a row
	// that was already a transfer changes which account it names, not the totals.
	if !t.IsTransfer() && next.IsTransfer() && t.CountsInReports() {
		amt := t.Amount.Amount
		if amt < 0 {
			amt = -amt
		}
		p.LeavesTotalsMinor = amt
	}
	if counterpartyID != "" {
		p.Partner, p.HasPartner = FindReciprocal(t, counterpartyID, txns)
	}
	return p
}
