// SPDX-License-Identifier: MIT

// Package billmatch links an expected recurring occurrence to the actual
// transaction that paid it (TX9). It is the pure logic behind the durable
// bill-match link (domain.TxnLinkBillMatch): given the recurring rules, their
// upcoming occurrences, and the ledger, it decides which transaction settles
// which occurrence, derives paid/unpaid state, and reports the variance between
// what was expected and what actually posted ("ran $2 over").
//
// It is deterministic and has no syscall/js dependency (unit-tested on native
// Go). The caller resolves noisy processor payees to clean names before calling
// (payeealias/TX1) and passes the resolved name on each transaction and
// occurrence, so matching keys on the merchant identity rather than raw strings.
//
// Matching rules (see Candidates / AutoMatches):
//   - amount within AmountTolerance (~5%) of the occurrence's expected amount;
//   - transaction date within DateWindowDays (±4) of the occurrence due date;
//   - the resolved payee matches (case-insensitively, substring either way) OR
//     the category matches when both carry one;
//   - a transaction settles at most one occurrence, and an occurrence is settled
//     by at most one transaction (1:1).
//
// AutoMatches returns only the UNAMBIGUOUS 1:1 pairs (exactly one candidate txn
// for an occurrence and that txn's only candidate occurrence is this one), so the
// appstate auto-match hook never guesses; ambiguous occurrences are left for a
// manual match.
package billmatch

import (
	"sort"
	"strings"
	"time"
)

// DateWindowDays is how far (in days) a transaction may sit from an occurrence's
// due date and still match it.
const DateWindowDays = 4

// AmountTolerancePermille is the amount tolerance as parts-per-thousand of the
// expected amount (50 = 5%). Variable bills wobble; a fixed floor keeps small
// bills matchable.
const AmountTolerancePermille = 50

// minToleranceMinor is the smallest absolute amount tolerance, so a tiny bill
// (where 5% rounds to near zero) still absorbs rounding.
const minToleranceMinor = 100 // $1.00 in cents

// Occurrence is one expected instance of a recurring rule the matcher reasons
// about: which rule, when it is due, and the expected magnitude/identity used to
// find its payment. AmountMinor is a positive magnitude (the expected outflow).
type Occurrence struct {
	RecurringID string
	DueDate     time.Time
	Payee       string // resolved display name (payeealias) or recurring label
	CategoryID  string
	AmountMinor int64
	Currency    string
}

// Txn is one candidate transaction from the ledger. AmountMinor is signed as it
// reads in the ledger (an expense is negative); the matcher compares magnitudes.
// Payee is the resolved display name.
type Txn struct {
	ID          string
	Date        time.Time
	Payee       string
	CategoryID  string
	AmountMinor int64
	Currency    string
}

// Match is a decided pairing of one occurrence and the transaction that settled
// it, with the variance (actual magnitude − expected magnitude): positive means
// the payment ran OVER the expected amount, negative means it came in under.
type Match struct {
	RecurringID   string
	DueDate       time.Time
	TxnID         string
	ExpectedMinor int64
	ActualMinor   int64 // magnitude of the matched transaction
	VarianceMinor int64 // ActualMinor − ExpectedMinor (signed)
}

// magnitude returns the absolute value of a signed minor amount.
func magnitude(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// tolerance returns the allowed amount deviation for an expected magnitude.
func tolerance(expectedMinor int64) int64 {
	tol := magnitude(expectedMinor) * AmountTolerancePermille / 1000
	if tol < minToleranceMinor {
		tol = minToleranceMinor
	}
	return tol
}

// day canonicalizes a time to its UTC calendar day so mixed-zone due dates and
// transaction dates compare as "same day".
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// payeeMatch reports whether two resolved payee names refer to the same merchant
// (case-insensitive substring either direction). Empty on either side is not a
// match — identity must be positively established by payee or category.
func payeeMatch(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

// isCandidate reports whether txn could settle occ: same currency, amount within
// tolerance, date within the window, and identity established by payee OR
// category.
func isCandidate(occ Occurrence, t Txn) bool {
	if occ.Currency != "" && t.Currency != "" && occ.Currency != t.Currency {
		return false
	}
	if diff := magnitude(t.AmountMinor) - magnitude(occ.AmountMinor); diff > tolerance(occ.AmountMinor) || diff < -tolerance(occ.AmountMinor) {
		return false
	}
	d := int(day(t.Date).Sub(day(occ.DueDate)).Hours() / 24)
	if d < 0 {
		d = -d
	}
	if d > DateWindowDays {
		return false
	}
	identity := payeeMatch(occ.Payee, t.Payee) ||
		(occ.CategoryID != "" && occ.CategoryID == t.CategoryID)
	return identity
}

// Candidates returns, for the given occurrence, every transaction that could
// settle it, best match first (closest amount, then closest date). Transactions
// whose ids are in the excluded set (already matched to another occurrence) are
// skipped, so a caller can enforce the 1:1 rule across occurrences.
func Candidates(occ Occurrence, txns []Txn, excluded map[string]bool) []Txn {
	var out []Txn
	for _, t := range txns {
		if excluded[t.ID] {
			continue
		}
		if isCandidate(occ, t) {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di := magnitude(magnitude(out[i].AmountMinor) - magnitude(occ.AmountMinor))
		dj := magnitude(magnitude(out[j].AmountMinor) - magnitude(occ.AmountMinor))
		if di != dj {
			return di < dj
		}
		gi := magnitude(int64(day(out[i].Date).Sub(day(occ.DueDate)).Hours() / 24))
		gj := magnitude(int64(day(out[j].Date).Sub(day(occ.DueDate)).Hours() / 24))
		if gi != gj {
			return gi < gj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// AutoMatches returns the unambiguous 1:1 pairings among the occurrences and
// transactions: an occurrence is auto-matched only when it has exactly one
// candidate transaction AND that transaction is the sole candidate of exactly
// one occurrence. Occurrences already settled (their key present in the
// alreadyMatched set) and their transactions are excluded up front, so a re-run
// after some links exist never double-matches. Results are ordered by due date
// then recurring id for determinism.
//
// alreadyMatched maps an occurrence key (RecurringID|YYYY-MM-DD) to the txn id
// that already settles it; those txn ids are treated as consumed.
func AutoMatches(occs []Occurrence, txns []Txn, alreadyMatched map[string]string) []Match {
	consumed := map[string]bool{}
	for _, txnID := range alreadyMatched {
		consumed[txnID] = true
	}

	// Build the candidate lists for the still-open occurrences.
	type occCands struct {
		occ   Occurrence
		cands []Txn
	}
	var open []occCands
	for _, occ := range occs {
		if _, done := alreadyMatched[Key(occ.RecurringID, occ.DueDate)]; done {
			continue
		}
		cands := Candidates(occ, txns, consumed)
		if len(cands) > 0 {
			open = append(open, occCands{occ: occ, cands: cands})
		}
	}

	// Count, per transaction, how many open occurrences claim it as their sole
	// candidate. A 1:1 auto-match requires an occurrence with exactly one
	// candidate whose txn is not contended by any other occurrence.
	claims := map[string]int{}
	for _, oc := range open {
		if len(oc.cands) == 1 {
			claims[oc.cands[0].ID]++
		}
	}

	var out []Match
	used := map[string]bool{}
	for _, oc := range open {
		if len(oc.cands) != 1 {
			continue // ambiguous occurrence — leave for manual match
		}
		t := oc.cands[0]
		if claims[t.ID] != 1 || used[t.ID] {
			continue // txn contended by multiple occurrences — ambiguous
		}
		used[t.ID] = true
		exp := magnitude(oc.occ.AmountMinor)
		act := magnitude(t.AmountMinor)
		out = append(out, Match{
			RecurringID:   oc.occ.RecurringID,
			DueDate:       day(oc.occ.DueDate),
			TxnID:         t.ID,
			ExpectedMinor: exp,
			ActualMinor:   act,
			VarianceMinor: act - exp,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].DueDate.Equal(out[j].DueDate) {
			return out[i].DueDate.Before(out[j].DueDate)
		}
		return out[i].RecurringID < out[j].RecurringID
	})
	return out
}

// Key returns the stable occurrence key (RecurringID|YYYY-MM-DD), matching
// domain.OccurrenceKey so the two layers agree on identity.
func Key(recurringID string, due time.Time) string {
	return recurringID + "|" + day(due).Format("2006-01-02")
}
