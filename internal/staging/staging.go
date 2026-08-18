// SPDX-License-Identifier: MIT

// Package staging is the step between reading a statement and writing it:
// stage → match → approve → import (C689).
//
// # Why a staging model rather than more preview code
//
// The app already previewed imports in two places, each with its own idea of what
// a duplicate is, what a row normalizes to, and what "ready" means. That is how
// the review screen and the importer came to disagree about the same rows (C688),
// and how an import could report counts nobody could reconcile against the ledger
// (C687). A second preview is not the fix; one model that both the preview and
// the write path read from is.
//
// So a staged batch is computed ONCE. Every row is normalized, hashed, and given
// a verdict, and the importer's job afterwards is to write the rows that were
// approved — not to re-decide anything.
//
// # The hash
//
// Each row carries a content hash of its normalized (account, date, amount,
// merchant). It is what makes a staged batch checkable: the same file staged
// twice produces the same hashes, two files overlapping produce colliding hashes
// against each other, and a row already in the ledger produces a hash that
// matches the ledger's. All three questions are answered by the same value, so
// they cannot drift apart the way three hand-rolled comparisons did.
//
// The duplicate rule itself is dedupe.Key — the app's one definition — rather
// than a fourth opinion invented here.
package staging

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeealias"
)

// Verdict is what staging concluded about one row, and what will happen to it.
type Verdict int

const (
	// New is a row the ledger does not have. It imports.
	New Verdict = iota
	// Duplicate is a row the ledger already holds. It is skipped — not a
	// failure, and re-importing an overlapping statement is a normal thing to do.
	Duplicate
	// RepeatedInFile is a row that appears twice in THIS file. The first copy
	// imports and the rest are held, because a statement listing the same charge
	// twice is far more often a parsing artefact than two identical charges.
	RepeatedInFile
	// Unusable is a row that could not be read: no date, no amount, or an amount
	// that is not a number. Its money is not going into the ledger and nothing
	// else will say so, which is why it is counted apart from a duplicate.
	Unusable
)

// String renders the verdict for logs and test failures.
func (v Verdict) String() string {
	switch v {
	case Duplicate:
		return "duplicate"
	case RepeatedInFile:
		return "repeated-in-file"
	case Unusable:
		return "unusable"
	default:
		return "new"
	}
}

// Row is one staged line: what it was, what it normalized to, and what will
// happen to it.
type Row struct {
	// Index is the row's position in the file, so a review table can point at the
	// line a person is looking at rather than at a row number of its own.
	Index int
	// RawDescription is what the file said, kept because the cleaned merchant is
	// a guess and categorization stays debuggable only while the original is
	// visible.
	RawDescription string

	// AccountID, Date, AmountMinor and Merchant are the normalized row.
	AccountID   string
	Date        time.Time
	AmountMinor int64
	Merchant    string

	// Hash is the content hash of the normalized row.
	Hash string
	// Verdict is what will happen to it.
	Verdict Verdict
	// Reason explains an Unusable row in the file's own terms.
	Reason string
	// MatchID is the existing transaction this row duplicates, when it does.
	MatchID string
}

// Approved reports whether this row will be written.
func (r Row) Approved() bool { return r.Verdict == New }

// Batch is a whole staged file.
type Batch struct {
	// Rows are every line, in file order, including the ones that will not import
	// — a review table that hides skipped rows cannot be checked against the file.
	Rows []Row
	// Counts summarizes the verdicts.
	Counts Counts
}

// Counts is the summary a person reads before approving.
type Counts struct {
	Total, New, Duplicate, RepeatedInFile, Unusable int
}

// Input is one line as read from a statement, before staging.
type Input struct {
	// Date is the row's date. Zero means the file did not give a usable one.
	Date time.Time
	// Description is the raw descriptor.
	Description string
	// AmountMinor is the signed amount. HasAmount distinguishes "zero" from
	// "absent", which a bare int64 cannot.
	AmountMinor int64
	HasAmount   bool
	// AccountID is the account the row belongs to, when the file said. Empty
	// falls back to the caller's chosen account.
	AccountID string
}

// Stage classifies a whole file against the existing ledger.
//
// fallbackAccountID is the account the user picked for rows the file does not
// assign. A row with neither is Unusable rather than guessed at: writing a
// transaction to the wrong account is not recoverable by looking at it later.
func Stage(inputs []Input, existing []domain.Transaction, fallbackAccountID, currency string) Batch {
	seen := make(map[string]string, len(existing)) // dedupe key → existing txn id
	for _, t := range existing {
		seen[dedupe.Key(t)] = t.ID
	}
	inFile := map[string]bool{}

	b := Batch{Rows: make([]Row, 0, len(inputs))}
	for i, in := range inputs {
		r := Row{Index: i, RawDescription: in.Description}

		r.AccountID = in.AccountID
		if r.AccountID == "" {
			r.AccountID = fallbackAccountID
		}
		r.Date = in.Date
		r.AmountMinor = in.AmountMinor
		r.Merchant = Normalize(in.Description)

		switch {
		case r.AccountID == "":
			r.Verdict, r.Reason = Unusable, "no account"
		case in.Date.IsZero():
			r.Verdict, r.Reason = Unusable, "no date"
		case !in.HasAmount:
			r.Verdict, r.Reason = Unusable, "no amount"
		}
		if r.Verdict == Unusable {
			b.Rows = append(b.Rows, r)
			continue
		}

		r.Hash = Hash(r.AccountID, r.Date, r.AmountMinor, r.Merchant)

		key := dedupe.Key(domain.Transaction{
			AccountID: r.AccountID, Date: r.Date, Desc: in.Description,
			Amount: money.New(r.AmountMinor, currency),
		})
		switch {
		case seen[key] != "":
			r.Verdict, r.MatchID = Duplicate, seen[key]
		case inFile[key]:
			r.Verdict = RepeatedInFile
		default:
			r.Verdict = New
			inFile[key] = true
		}
		b.Rows = append(b.Rows, r)
	}

	b.Counts = count(b.Rows)
	return b
}

func count(rows []Row) Counts {
	var c Counts
	for _, r := range rows {
		c.Total++
		switch r.Verdict {
		case Duplicate:
			c.Duplicate++
		case RepeatedInFile:
			c.RepeatedInFile++
		case Unusable:
			c.Unusable++
		default:
			c.New++
		}
	}
	return c
}

// Approved returns the rows that will be written, in file order.
func (b Batch) Approved() []Row {
	var out []Row
	for _, r := range b.Rows {
		if r.Approved() {
			out = append(out, r)
		}
	}
	return out
}

// Clean reports whether the batch needs no explanation before importing: nothing
// unreadable. Duplicates are not a problem — they are the safeguard working.
func (b Batch) Clean() bool { return b.Counts.Unusable == 0 }

// Hash is the content hash of a normalized row.
//
// The parts are joined with a separator that cannot occur in any of them, so
// "AB" + "C" and "A" + "BC" cannot collide — the classic way a concatenated hash
// key quietly merges two different rows.
func Hash(accountID string, date time.Time, amountMinor int64, merchant string) string {
	var b strings.Builder
	b.WriteString(accountID)
	b.WriteByte(0)
	b.WriteString(date.UTC().Format("2006-01-02"))
	b.WriteByte(0)
	b.WriteString(money.FormatMinor(amountMinor, 0))
	b.WriteByte(0)
	b.WriteString(merchant)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:12])
}

// Normalize reduces a raw descriptor to the merchant it is about, reusing the
// app's existing cleaner so a staged row and an imported one agree about what a
// merchant is called. Lower-cased and whitespace-collapsed, because the hash must
// not treat "AMZN  Mktp" and "AMZN Mktp" as different rows.
func Normalize(desc string) string {
	return strings.Join(strings.Fields(strings.ToLower(payeealias.Normalize(desc))), " ")
}

// Collisions groups the batch's rows by hash, returning only the hashes with more
// than one row — the file disagreeing with itself, listed so a person can look
// rather than being told a count.
func Collisions(b Batch) map[string][]Row {
	byHash := map[string][]Row{}
	for _, r := range b.Rows {
		if r.Hash == "" {
			continue
		}
		byHash[r.Hash] = append(byHash[r.Hash], r)
	}
	for h, rows := range byHash {
		if len(rows) < 2 {
			delete(byHash, h)
		}
	}
	for _, rows := range byHash {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Index < rows[j].Index })
	}
	return byHash
}
