// SPDX-License-Identifier: MIT

package txnclassify

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// # Suspected transfers
//
// A row that is REALLY a move between your own accounts, but arrived from an
// import with no counterparty set, is indistinguishable from income or spending
// to every aggregation in the app: IsTransfer() is false, so the money counts.
// Money Flow shows a positive row filed under "Transfer" as income and a negative
// one as expenditure, and the totals are wrong by twice the amount moved.
//
// The fix is NOT to treat the category as the answer. A household may legitimately
// have a category called "Transfers" for something that really is spending, and
// hiding every row that wears the name would conceal real money — silently, which
// is worse than the leak. The structural relationship (TransferAccountID) remains
// the only thing that decides, exactly as it does today.
//
// What this file adds is a way to SAY SO. It reports the rows that look like
// transfers and are not linked, so they can be routed to review and shown as
// unresolved rather than quietly counted. Nothing here changes a total, and
// nothing here writes: a suspect is a question for a person, and the answer is
// Apply.

// SuspectReason is why a row looks like an unlinked transfer.
type SuspectReason string

const (
	// SuspectNone means the row does not look like a transfer.
	SuspectNone SuspectReason = ""
	// SuspectCategory means the row is filed under a category whose name is one
	// of the canonical transfer names.
	SuspectCategory SuspectReason = "category"
	// SuspectDescription means the row's description or payee reads like a
	// transfer ("Transfer to savings", "ONLINE XFER", "Payment to card").
	SuspectDescription SuspectReason = "description"
)

// transferCategoryNames are the category names that mean "this was a move, not a
// purchase" in every statement export seen so far. Matched on the NORMALIZED
// name (lowercased, collapsed whitespace) and only as the whole name, never as a
// substring: "Transfer" matches, "Transfer fees" does not — a fee genuinely is
// spending, and a household that pays one should see it.
var transferCategoryNames = map[string]bool{
	"transfer":            true,
	"transfers":           true,
	"transfer in":         true,
	"transfer out":        true,
	"internal transfer":   true,
	"account transfer":    true,
	"credit card payment": true,
	"card payment":        true,
	"loan payment":        true,
}

// transferPhrases are description fragments that name a move. Deliberately short
// and specific: each one has to be a phrase a person would only write about money
// changing hands between accounts, because a false positive here costs a user a
// review card about a row that was fine.
var transferPhrases = []string{
	"transfer to",
	"transfer from",
	"xfer to",
	"xfer from",
	"online transfer",
	"online xfer",
	"internal transfer",
	"payment to card",
	"card payment to",
	"autopay to",
}

// normalizeName lowercases and collapses whitespace so "  Transfer  In " and
// "transfer in" are the same name. Kept local rather than borrowed so this
// package stays dependency-free below domain.
func normalizeName(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// Suspected reports whether t looks like a transfer that was never linked, and
// why. categoryName is the display name of t's category ("" when it has none).
//
// It reports false for anything already settled:
//
//   - a row that IS structurally a transfer needs no suspicion, it is already out
//     of every total;
//   - a row already excluded from reports is not leaking into anything, so
//     flagging it would be noise about a number nobody is reading;
//   - a zero-amount row moves nothing either way.
//
// The category test runs before the description test because a category is a
// filing decision someone made, while a description is whatever the bank printed.
func Suspected(t domain.Transaction, categoryName string) (SuspectReason, bool) {
	if t.IsTransfer() || !t.CountsInReports() || t.Amount.Amount == 0 {
		return SuspectNone, false
	}
	if transferCategoryNames[normalizeName(categoryName)] {
		return SuspectCategory, true
	}
	text := normalizeName(t.Desc + " " + t.Payee)
	for _, p := range transferPhrases {
		if strings.Contains(text, p) {
			return SuspectDescription, true
		}
	}
	return SuspectNone, false
}

// Suspect is one unlinked row that looks like a transfer, paired with the reason.
type Suspect struct {
	Txn    domain.Transaction
	Reason SuspectReason
}

// Suspects returns every row in txns that looks like an unlinked transfer, in
// input order. categoryName resolves a category id to its display name; a nil
// func means no row can be suspected by category, which is what a caller without
// the category list should get rather than a panic.
func Suspects(txns []domain.Transaction, categoryName func(id string) string) []Suspect {
	out := make([]Suspect, 0)
	for _, t := range txns {
		name := ""
		if categoryName != nil {
			name = categoryName(t.CategoryID)
		}
		if reason, ok := Suspected(t, name); ok {
			out = append(out, Suspect{Txn: t, Reason: reason})
		}
	}
	return out
}

// LeakedTotals is what the suspects are worth to the figures they are currently
// landing in — the size of the error, in minor units of each row's own currency,
// so a caller can say "these rows are adding $2,300 to income that was never
// earned" instead of only "3 rows need review".
//
// Amounts are NOT FX-converted: this package has no rate table and inventing one
// would be worse than reporting per-row magnitudes the caller can convert.
type LeakedTotals struct {
	// IncomeMinor is the total of positive suspects — money counted as earned
	// that was only moved in.
	IncomeMinor int64
	// SpendingMinor is the total magnitude of negative suspects — money counted
	// as spent that was only moved out.
	SpendingMinor int64
	// Count is how many rows contribute.
	Count int
}

// Leaked sums what suspects are currently contributing to income and spending.
func Leaked(suspects []Suspect) LeakedTotals {
	var out LeakedTotals
	for _, s := range suspects {
		out.Count++
		if s.Txn.Amount.Amount > 0 {
			out.IncomeMinor += s.Txn.Amount.Amount
			continue
		}
		out.SpendingMinor += -s.Txn.Amount.Amount
	}
	return out
}
