// SPDX-License-Identifier: MIT

// Package dedupe finds likely-duplicate transactions — the same charge imported
// or entered twice, a common mess after a CSV import. It is a pure read over the
// transactions (no store, no syscall/js) and is unit-tested on native Go; the UI
// surfaces the groups it returns.
package dedupe

import (
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Group is a set of transaction IDs that look like duplicates of one another —
// same calendar date, same signed amount and currency, and the same normalized
// description.
type Group struct {
	Date        string // the shared calendar date (YYYY-MM-DD)
	Description string // the shared (original-cased) description
	Amount      int64  // the shared signed amount, minor units
	Currency    string
	IDs         []string // the duplicate transactions' ids (2 or more), sorted
}

// Signature is the duplicate-detection key for a transaction: its calendar date,
// signed amount + currency, and case-insensitively-trimmed description. Two
// transactions with the same signature are accidental double entries. It is the
// single source of truth used both by FindDuplicates (to group after the fact)
// and by the CSV importer (to skip rows already present before writing).
func Signature(t domain.Transaction) string {
	day := t.Date.Format("2006-01-02")
	norm := strings.ToLower(strings.TrimSpace(t.Desc))
	return day + "|" + strconv.FormatInt(t.Amount.Amount, 10) + "|" + t.Amount.Currency + "|" + norm
}

// FindDuplicates groups transactions that share the same calendar date, signed
// amount + currency, and case-insensitively-trimmed description — the signature
// of an accidental double entry. Transfers are excluded (their paired legs are
// not duplicates). Only groups of two or more are returned, ordered by date then
// description for a stable display; the ids within each group are sorted.
func FindDuplicates(txns []domain.Transaction) []Group {
	type bucket struct {
		date, desc, cur string
		amount          int64
		ids             []string
	}
	buckets := map[string]*bucket{}
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		day := t.Date.Format("2006-01-02")
		key := Signature(t)
		b := buckets[key]
		if b == nil {
			b = &bucket{date: day, desc: strings.TrimSpace(t.Desc), cur: t.Amount.Currency, amount: t.Amount.Amount}
			buckets[key] = b
		}
		b.ids = append(b.ids, t.ID)
	}

	var out []Group
	for _, b := range buckets {
		if len(b.ids) < 2 {
			continue
		}
		ids := append([]string(nil), b.ids...)
		sort.Strings(ids)
		out = append(out, Group{Date: b.date, Description: b.desc, Amount: b.amount, Currency: b.cur, IDs: ids})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date // newest first
		}
		return out[i].Description < out[j].Description
	})
	return out
}

// Merge returns a copy of survivor with its metadata unioned from all transactions
// in others. Specifically:
//   - Tags are combined case-insensitively (survivor order preserved, duplicates dropped,
//     tags from others appended in the order they first appear).
//   - Cleared is set to true if any transaction in the group (survivor or others) is cleared.
//   - Amount, Date, AccountID, ID, and all other identity fields are taken unchanged
//     from survivor — they are identical across a duplicate group by signature.
//
// Merge is pure (no store access, no syscall/js) so it is unit-testable on native Go.
func Merge(survivor domain.Transaction, others []domain.Transaction) domain.Transaction {
	out := survivor

	// Union tags case-insensitively; preserve the survivor's original order first,
	// then append any new tags seen in others.
	seen := make(map[string]struct{}, len(survivor.Tags))
	unified := make([]string, 0, len(survivor.Tags))
	for _, tag := range survivor.Tags {
		key := strings.ToLower(tag)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			unified = append(unified, tag)
		}
	}
	for _, other := range others {
		if other.Cleared {
			out.Cleared = true
		}
		for _, tag := range other.Tags {
			key := strings.ToLower(tag)
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				unified = append(unified, tag)
			}
		}
	}
	if len(unified) > 0 {
		out.Tags = unified
	}

	return out
}

// Count returns how many transactions across all groups are duplicates beyond the
// first in each group — i.e. how many entries you could remove. It's the headline
// "N possible duplicates" figure.
func Count(groups []Group) int {
	n := 0
	for _, g := range groups {
		n += len(g.IDs) - 1
	}
	return n
}

// CountIncomingDuplicates returns how many rows in incoming would be skipped as
// duplicates if imported into an account that already contains existing. A row is
// a duplicate when its per-account signature (AccountID + "|" + Signature) matches
// an existing transaction OR an earlier row in the same incoming batch — mirroring
// exactly the logic in appstate.ImportTransactionsCSV so the pre-import count is
// always consistent with the post-import "skipped" tally.
//
// accountID is the fallback account used for incoming rows whose AccountID is
// blank; it should match the fallbackAccountID passed to ImportTransactionsCSV.
func CountIncomingDuplicates(incoming []domain.Transaction, existing []domain.Transaction, accountID string) int {
	seen := make(map[string]bool, len(existing))
	for _, t := range existing {
		seen[t.AccountID+"|"+Signature(t)] = true
	}
	dupes := 0
	for _, t := range incoming {
		acct := t.AccountID
		if acct == "" {
			acct = accountID
		}
		sig := acct + "|" + Signature(t)
		if seen[sig] {
			dupes++
		} else {
			seen[sig] = true
		}
	}
	return dupes
}
