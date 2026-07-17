// SPDX-License-Identifier: MIT

// Package importsafe holds the pure pre-import dependability checks (#57):
// the balance-impact preview an import shows before it commits, the
// implausible-jump warning, the "why matched" explanation for duplicate rows,
// and detection of incoming rows that look like the second leg of a transfer
// already in the ledger. All functions are deterministic and side-effect free
// so the previews can never disagree with what the import then does.
package importsafe

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Impact sums the incoming rows destined for accountID (rows with a blank
// AccountID fall back to it, mirroring the importer) and returns the net
// signed change plus the resulting balance.
func Impact(currentMinor int64, incoming []domain.Transaction, accountID string) (netMinor, afterMinor int64) {
	for _, t := range incoming {
		acct := t.AccountID
		if acct == "" {
			acct = accountID
		}
		if acct != accountID {
			continue
		}
		netMinor += t.Amount.Amount
	}
	return netMinor, currentMinor + netMinor
}

// jumpFloorMinor is the absolute net change below which an import is never
// flagged: $10,000 in minor units. Small imports are routine whatever their
// relative size (a first import into an empty account must not warn).
const jumpFloorMinor = 1_000_000

// JumpWarning reports whether an import's net change looks implausible for
// the account: at least the absolute floor AND large relative to the current
// balance (more than 3× its magnitude). Both conditions keep the warning
// rare and explainable — a $12k statement into a $40k account is normal; a
// $12k swing on a $900 account deserves a second look before committing.
func JumpWarning(currentMinor, netMinor int64) bool {
	abs := netMinor
	if abs < 0 {
		abs = -abs
	}
	if abs < jumpFloorMinor {
		return false
	}
	cur := currentMinor
	if cur < 0 {
		cur = -cur
	}
	return abs > 3*cur
}

// WhyDup explains one incoming row that would be skipped as a duplicate: the
// fields it collides on (the dedupe signature — same calendar day, same signed
// amount and currency, same normalized description, same account).
type WhyDup struct {
	Date        string // YYYY-MM-DD
	Desc        string
	AmountMinor int64
	Currency    string
	// InBatch is true when the collision is with an EARLIER ROW OF THE SAME
	// IMPORT (the file itself repeats the row) rather than the existing ledger.
	InBatch bool
}

// Duplicates returns the why-matched detail for every incoming row that the
// importer would skip, in input order. It mirrors dedupe.CountIncomingDuplicates
// exactly (per-account signature vs the ledger, then vs earlier batch rows), so
// len(Duplicates(...)) always equals the preview count.
func Duplicates(incoming, existing []domain.Transaction, accountID string) []WhyDup {
	seen := make(map[string]bool, len(existing))
	for _, t := range existing {
		seen[t.AccountID+"|"+dedupe.Signature(t)] = true
	}
	var out []WhyDup
	batch := make(map[string]bool, len(incoming))
	for _, t := range incoming {
		acct := t.AccountID
		if acct == "" {
			acct = accountID
		}
		sig := acct + "|" + dedupe.Signature(t)
		switch {
		case seen[sig]:
			out = append(out, WhyDup{Date: t.Date.Format("2006-01-02"), Desc: strings.TrimSpace(t.Desc),
				AmountMinor: t.Amount.Amount, Currency: t.Amount.Currency, InBatch: batch[sig]})
			batch[sig] = true
		default:
			seen[sig] = true
			batch[sig] = true
		}
	}
	return out
}

// Pair is one incoming row that mirrors an existing transaction in a DIFFERENT
// account — the classic missed transfer: the card statement's "PAYMENT THANK
// YOU" arriving after the checking export already recorded the outflow.
type Pair struct {
	IncomingDesc string
	IncomingDate string // YYYY-MM-DD
	AmountMinor  int64
	OtherAccount string // the existing transaction's account ID
	OtherDesc    string
}

// TransferPairs finds incoming rows whose signed amount is exactly opposite an
// existing non-transfer transaction in another account within windowDays. Each
// existing transaction is claimed at most once; rows and matches already
// marked as transfers are skipped. Results are in incoming order.
func TransferPairs(incoming, existing []domain.Transaction, accountID string, windowDays int) []Pair {
	claimed := make(map[string]bool)
	var out []Pair
	for _, in := range incoming {
		if in.IsTransfer() || in.Amount.Amount == 0 {
			continue
		}
		acct := in.AccountID
		if acct == "" {
			acct = accountID
		}
		for _, ex := range existing {
			if claimed[ex.ID] || ex.IsTransfer() || ex.AccountID == acct {
				continue
			}
			if ex.Amount.Amount != -in.Amount.Amount || ex.Amount.Currency != in.Amount.Currency {
				continue
			}
			days := in.Date.Sub(ex.Date).Hours() / 24
			if days < 0 {
				days = -days
			}
			if days > float64(windowDays) {
				continue
			}
			claimed[ex.ID] = true
			out = append(out, Pair{
				IncomingDesc: strings.TrimSpace(in.Desc), IncomingDate: in.Date.Format("2006-01-02"),
				AmountMinor: in.Amount.Amount, OtherAccount: ex.AccountID, OtherDesc: strings.TrimSpace(ex.Desc),
			})
			break
		}
	}
	return out
}
