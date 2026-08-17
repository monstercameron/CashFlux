// SPDX-License-Identifier: MIT

package integrity

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
)

// ─── LF-8: data hygiene, as distinct from data integrity ─────────────────────
//
// The checks in integrity.go find things that are WRONG — a transfer with one
// leg, a split that does not sum, a liability with a positive balance. Every one
// of them means a figure somewhere is incorrect.
//
// Hygiene is a different question: how much of the data is simply not finished.
// An uncategorized transaction is not an error; it is work. A stale account
// balance is not a contradiction; it is a number nobody has confirmed lately.
// Conflating the two would either bury real errors under a pile of chores, or
// dress ordinary housekeeping up as corruption — and a panel that cries
// corruption over twelve uncategorized coffees is one people stop reading.
//
// So these are counts with somewhere to go, not findings with a severity.

// HygieneCount is one "N things need attention" figure and where to fix it.
type HygieneCount struct {
	// Kind names the count, for the caller's copy lookup.
	Kind string
	// N is how many. Zero means the caller should not render it at all — "0
	// uncategorized" is noise, and a panel listing everything you have already
	// done is a panel about itself.
	N int
	// Route is where to go to work through them.
	Route string
}

// Hygiene names the three counts, so callers switch on constants rather than
// strings.
const (
	HygieneUncategorized = "uncategorized"
	HygieneStaleAccounts = "stale-accounts"
	HygieneUnreconciled  = "unreconciled"
)

// HygieneInput is what the counts read. Windows and Now come from the caller so
// the staleness rule matches the one the accounts page already applies —
// computing it twice with two different windows is how two screens come to
// disagree about the same account.
type HygieneInput struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Windows      freshness.Windows
	Now          time.Time
}

// Hygiene returns the unfinished-work counts, always in the same order, and
// always all three — including the zeroes, so a caller can decide whether to
// render a "nothing to do" state rather than guessing from a short slice.
func Hygiene(in HygieneInput) []HygieneCount {
	return []HygieneCount{
		{Kind: HygieneUncategorized, N: countUncategorized(in.Transactions), Route: "/transactions"},
		{Kind: HygieneStaleAccounts, N: len(freshness.StaleAccounts(in.Accounts, in.Windows, in.Now)), Route: "/accounts"},
		{Kind: HygieneUnreconciled, N: countUnreconciled(in.Accounts), Route: "/accounts"},
	}
}

// HygieneTotal is how many items need attention across all counts — the number
// for a single headline.
func HygieneTotal(counts []HygieneCount) int {
	var n int
	for _, c := range counts {
		n += c.N
	}
	return n
}

// countUncategorized counts transactions with no category.
//
// Transfers are excluded: moving money between your own accounts has no category
// to assign, so counting them would present permanent, unfixable work — the
// worst possible thing to put in a list of things to do.
func countUncategorized(txns []domain.Transaction) int {
	var n int
	for _, t := range txns {
		if t.IsTransfer() || t.CategoryID != "" {
			continue
		}
		n++
	}
	return n
}

// countUnreconciled counts accounts that have never been reconciled against a
// statement.
//
// Only accounts that CAN be reconciled: a property valuation or a manually-
// tracked asset has no statement to match, so listing it as unreconciled would
// be asking for work that does not exist. Archived accounts are skipped for the
// same reason.
func countUnreconciled(accounts []domain.Account) int {
	var n int
	for _, a := range accounts {
		if a.Archived || !reconcilable(a.Type) {
			continue
		}
		if len(a.Reconciliations) == 0 {
			n++
		}
	}
	return n
}

// reconcilable reports whether an account type has statements to reconcile
// against — the cash-like and card-like ones a bank or issuer sends.
func reconcilable(t domain.AccountType) bool {
	switch t {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings,
		domain.TypeCreditCard, domain.TypeLineOfCredit:
		return true
	}
	return false
}
