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
// same account, same calendar date, same signed amount and currency, and the
// same normalized description.
type Group struct {
	Date        string // the shared calendar date (YYYY-MM-DD)
	Description string // the shared (original-cased) description
	Amount      int64  // the shared signed amount, minor units
	Currency    string
	// AccountID is the account every member sits on. Part of the identity, not
	// decoration: the same subscription charged to two different cards on the
	// same day is two real payments, and merging them deletes one (C688).
	AccountID string
	IDs       []string // the duplicate transactions' ids (2 or more), sorted
}

// Key is the full duplicate-detection key for a transaction: its account, its
// calendar date, its signed amount + currency, and its case-insensitively-
// trimmed description. Two transactions sharing a Key are an accidental double
// entry; two sharing only a Signature may be two real payments on two accounts.
//
// It is the single source of truth for BOTH the after-the-fact grouping
// (FindDuplicates) and the importer's skip-if-already-present check, so the two
// can no longer disagree about the same rows (C688).
//
// It is not the whole key. Use Key for that — the account is part of a
// transaction's identity, and this function is exported only because callers
// that have already scoped themselves to one account (the importer's per-account
// seen-set) still want the cheaper half.
//
// Its doc used to claim it was "the single source of truth used both by
// FindDuplicates and by the CSV importer". That was false in the way that
// matters: the importer prefixed the account id and FindDuplicates did not, so
// the review screen grouped rows the importer would never have called duplicates
// (C688).
func Key(t domain.Transaction) string {
	return t.AccountID + "|" + Signature(t)
}

// Signature is the within-account half of Key.
func Signature(t domain.Transaction) string {
	day := t.Date.Format("2006-01-02")
	norm := strings.ToLower(strings.TrimSpace(t.Desc))
	return day + "|" + strconv.FormatInt(t.Amount.Amount, 10) + "|" + t.Amount.Currency + "|" + norm
}

// FindDuplicates groups transactions that share the same ACCOUNT, calendar date,
// signed amount + currency, and case-insensitively-trimmed description — the
// signature of an accidental double entry. Transfers are excluded (their paired
// legs are not duplicates). Only groups of two or more are returned, ordered by
// date then description for a stable display; the ids within each group are
// sorted.
//
// C688: the account used to be left out, while the importer's identical-looking
// check included it. A subscription billed to two cards on the same day — an
// OpenRouter or FAL charge split across a personal and a business card is exactly
// the shape — was therefore offered on the duplicates screen with a Merge button,
// and merging it deletes a real payment from the other account. Grouping is the
// input to a destructive action, so it has to be the stricter of the two rules,
// not the looser.
func FindDuplicates(txns []domain.Transaction) []Group {
	type bucket struct {
		date, desc, cur, acct string
		amount                int64
		ids                   []string
	}
	buckets := map[string]*bucket{}
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		day := t.Date.Format("2006-01-02")
		key := Key(t)
		b := buckets[key]
		if b == nil {
			b = &bucket{date: day, desc: strings.TrimSpace(t.Desc), cur: t.Amount.Currency,
				acct: t.AccountID, amount: t.Amount.Amount}
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
		out = append(out, Group{Date: b.date, Description: b.desc, Amount: b.amount, Currency: b.cur, AccountID: b.acct, IDs: ids})
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
//     from survivor — they are identical across a duplicate group by Key.
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
	// Attachments (receipts) union: keep the survivor's, then append any receipt
	// from an "other" not already present (dedupe by artifact id), so a merge never
	// drops a receipt that lived on the row being removed. (TXC-4.)
	attSeen := make(map[string]struct{}, len(survivor.Attachments))
	for _, a := range survivor.Attachments {
		attSeen[a.ArtifactID] = struct{}{}
	}

	// fill takes a value from the first "other" that has one, when the survivor's
	// field is empty — so merging never silently loses a category, payee, memo,
	// member, or a bill/subscription link that only the removed duplicate carried.
	fill := func(dst *string, pick func(domain.Transaction) string) {
		if *dst != "" {
			return
		}
		for _, o := range others {
			if v := pick(o); v != "" {
				*dst = v
				return
			}
		}
	}

	for _, other := range others {
		if other.Cleared {
			// The surviving row inherits the moment too, or a merge would erase how
			// long the charge took to clear (EC-4). MarkCleared keeps the earlier
			// stamp when one is already set.
			out = out.MarkCleared(true, other.ClearedAt)
		}
		for _, tag := range other.Tags {
			key := strings.ToLower(tag)
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				unified = append(unified, tag)
			}
		}
		for _, a := range other.Attachments {
			if _, ok := attSeen[a.ArtifactID]; !ok {
				attSeen[a.ArtifactID] = struct{}{}
				out.Attachments = append(out.Attachments, a)
			}
		}
	}
	if len(unified) > 0 {
		out.Tags = unified
	}

	// Fill empty identity/link fields from the removed duplicates (survivor wins
	// when it already has a value).
	fill(&out.CategoryID, func(t domain.Transaction) string { return t.CategoryID })
	fill(&out.Payee, func(t domain.Transaction) string { return t.Payee })
	fill(&out.Note, func(t domain.Transaction) string { return t.Note })
	fill(&out.MemberID, func(t domain.Transaction) string { return t.MemberID })
	fill(&out.BillAccountID, func(t domain.Transaction) string { return t.BillAccountID })
	fill(&out.SubscriptionName, func(t domain.Transaction) string { return t.SubscriptionName })
	fill(&out.SourceDocID, func(t domain.Transaction) string { return t.SourceDocID })

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
		seen[Key(t)] = true
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

// SameAccount reports whether every member of a merge sits on one account.
//
// Merging is destructive: the caller keeps the survivor and deletes the rest. The
// grouping rule (Key) already guarantees this, so a false answer means a caller
// assembled a group by some other route — and the failure it prevents is deleting
// a real payment from another account, which nothing downstream would notice
// because the survivor looks exactly like it (C688).
func SameAccount(survivor domain.Transaction, others []domain.Transaction) bool {
	for _, o := range others {
		if o.AccountID != survivor.AccountID {
			return false
		}
	}
	return true
}
