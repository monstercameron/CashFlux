// SPDX-License-Identifier: MIT

// Package receiptmatch finds the existing ledger transaction a scanned receipt
// most likely belongs to (#58), so extraction can offer "attach to this
// charge" instead of always creating a new transaction — the classic
// double-entry: the card import already recorded the purchase, then the
// receipt scan records it again.
//
// Pure and deterministic: exact-amount candidates within a date window are
// scored by date proximity and merchant-token overlap. The caller renders the
// score's parts (SameDay/DaysApart/MerchantHit) in plain English.
package receiptmatch

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Candidate is one existing transaction a receipt plausibly documents.
type Candidate struct {
	Txn domain.Transaction
	// Score orders candidates (higher = likelier). Amount equality is the
	// entry bar, not a score component.
	Score int
	// DaysApart is the absolute day distance between receipt and transaction.
	DaysApart int
	// MerchantHit reports a merchant-token overlap with the transaction's
	// payee/description.
	MerchantHit bool
}

// DefaultWindowDays is how far a receipt may sit from the charge it documents:
// pending card transactions commonly post several days late.
const DefaultWindowDays = 5

// maxCandidates caps the offer list — past three, the user is better served
// by "create new" than by scrolling lookalikes.
const maxCandidates = 3

// Match returns the best existing-transaction candidates for a receipt with
// the given positive total, merchant, and date. Only expense transactions with
// EXACTLY the receipt's amount qualify; transfers and already-split rows are
// skipped (a split row has already been documented). Results are best-first.
func Match(totalMinor int64, merchant string, when time.Time, txns []domain.Transaction, windowDays int) []Candidate {
	if totalMinor <= 0 {
		return nil
	}
	if windowDays <= 0 {
		windowDays = DefaultWindowDays
	}
	mTokens := tokens(merchant)
	var out []Candidate
	for _, t := range txns {
		if t.IsTransfer() || t.HasSplits() || t.Amount.Amount != -totalMinor {
			continue
		}
		days := int(when.Sub(t.Date).Hours() / 24)
		if days < 0 {
			days = -days
		}
		if days > windowDays {
			continue
		}
		c := Candidate{Txn: t, DaysApart: days}
		switch {
		case days == 0:
			c.Score += 25
		case days == 1:
			c.Score += 20
		case days <= 3:
			c.Score += 12
		default:
			c.Score += 5
		}
		if overlap(mTokens, tokens(t.Payee+" "+t.Desc)) {
			c.MerchantHit = true
			c.Score += 15
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].DaysApart < out[j].DaysApart
	})
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out
}

// tokens lowercases s and splits it into alphanumeric tokens of length ≥3 —
// short fragments ("st", "co") match everything and mean nothing.
func tokens(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() >= 3 {
			out = append(out, strings.ToLower(b.String()))
		}
		b.Reset()
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// overlap reports whether any token appears in both sets.
func overlap(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}
	for _, t := range b {
		if set[t] {
			return true
		}
	}
	return false
}
