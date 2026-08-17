// SPDX-License-Identifier: MIT

// Package doublecount finds money that appears twice in the ledger (WF11).
//
// The ticket's stated purpose is "to prevent double-counting and make unusual
// activity legible", and the app already links refunds to purchases, groups
// split orders, and matches bills to recurrings. What nothing did was look for
// the same money counted twice.
//
// This package finds ONE shape, deliberately: a TRANSFER ENTERED AS TWO
// TRANSACTIONS. $400 leaves checking and arrives in savings, recorded as an
// expense and an income rather than as one transfer. Nothing left the household,
// but the ledger now shows $400 of spending and $400 of income, and every report
// built on either is wrong.
//
// # Why not repeated charges too
//
// Because internal/dedupe already owns that question, and its rule - same
// calendar date, signed amount, description - is described in the codebase as
// the app's CANONICAL duplicate definition, shared by the duplicate flags, the
// Duplicates page and the assistant's tool. A second, slightly different rule
// here would eventually disagree with it about the same two rows, and two
// screens contradicting each other about whether something is a duplicate is
// worse than either rule alone.
//
// A mirror pair is genuinely invisible to that rule: different accounts,
// opposite signs, and usually different descriptions.
//
// # Why it reports and never merges
//
// Two identical coffees on the same day are indistinguishable from one coffee
// entered twice — the ledger cannot tell, and neither can this package. Merging
// on a guess deletes a real transaction some of the time, and a tool that
// occasionally eats data is worse than one that occasionally asks a question.
// Everything here produces a QUESTION for a person, never an edit.
package doublecount

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// WindowDays is how far apart two sides may be and still be the same money.
//
// Three days. Accounts post on different schedules — a transfer can leave on
// Friday and land on Monday — and requiring the same date would miss the most
// common case. Beyond three days the coincidence rate climbs faster than the
// hit rate, and a finding that is usually wrong is one people stop reading.
const WindowDays = 3

// Kind is what sort of duplication was found.
type Kind string

const (
	// KindMirror is an expense and an income of the same amount in different
	// accounts — a transfer recorded as two separate transactions.
	KindMirror Kind = "mirror"
)

// Finding is one suspected double-count.
type Finding struct {
	Kind Kind
	// A and B are the two transactions, A always the earlier (ties by ID) so the
	// same pair is always reported the same way round.
	A, B domain.Transaction
	// AmountMinor is the magnitude counted twice.
	AmountMinor int64
	// DaysApart is how far apart they fell, so a reader can weigh the evidence
	// rather than take the finding on faith.
	DaysApart int
}

// Report is everything found.
type Report struct {
	Findings []Finding
	// MirrorMinor is the total counted twice.
	MirrorMinor int64
}

// Any reports whether anything was found.
func (r Report) Any() bool { return len(r.Findings) > 0 }

// Find scans a ledger for money counted twice.
func Find(txns []domain.Transaction) Report {
	var r Report
	usable := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if t.IsTransfer() || !t.CountsInReports() || t.Amount.Amount == 0 {
			// A transfer is already modelled correctly — it is the FIX, not the
			// problem — and an excluded row has been deliberately set aside.
			continue
		}
		usable = append(usable, t)
	}
	sort.SliceStable(usable, func(i, j int) bool {
		if !usable[i].Date.Equal(usable[j].Date) {
			return usable[i].Date.Before(usable[j].Date)
		}
		return usable[i].ID < usable[j].ID
	})

	paired := map[string]bool{}
	for i := range usable {
		a := usable[i]
		if paired[a.ID] {
			continue
		}
		for j := i + 1; j < len(usable); j++ {
			b := usable[j]
			if paired[b.ID] {
				continue
			}
			days := daysBetween(a.Date, b.Date)
			if days > WindowDays {
				// Sorted by date, so nothing later can be closer.
				break
			}
			k, ok := classify(a, b)
			if !ok {
				continue
			}
			amt := abs64(a.Amount.Amount)
			r.Findings = append(r.Findings, Finding{Kind: k, A: a, B: b, AmountMinor: amt, DaysApart: days})
			r.MirrorMinor += amt
			// Each transaction is claimed by at most one finding. Without this a
			// household with three identical weekly charges would produce three
			// pairings of the same money and read as a catastrophe.
			paired[a.ID], paired[b.ID] = true, true
			break
		}
	}
	return r
}

// classify decides whether two transactions are the same money.
func classify(a, b domain.Transaction) (Kind, bool) {
	if abs64(a.Amount.Amount) != abs64(b.Amount.Amount) {
		return "", false
	}
	if a.Amount.Currency != b.Amount.Currency {
		// Two amounts that happen to share a number in different currencies are
		// not the same money, and converting to compare would invent a match out
		// of an exchange rate.
		return "", false
	}
	if a.AccountID == b.AccountID {
		// Same account is dedupe's question, not this one.
		return "", false
	}
	if (a.Amount.Amount < 0) == (b.Amount.Amount < 0) {
		// Both out or both in is not money moving between them.
		return "", false
	}
	// Money left one account and arrived in another: a transfer nobody marked.
	return KindMirror, true
}

// daysBetween is the absolute whole-day gap.
func daysBetween(a, b time.Time) int {
	d := int(b.Sub(a).Hours() / 24)
	if d < 0 {
		return -d
	}
	return d
}

// abs64 is the magnitude of a signed minor-unit amount.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
