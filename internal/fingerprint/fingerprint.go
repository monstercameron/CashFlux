// SPDX-License-Identifier: MIT

// Package fingerprint provides stable, account-scoped duplicate keys for
// CashFlux transactions. It is a pure package (no syscall/js, no store
// dependency) and is unit-tested on native Go.
//
// Relationship to internal/dedupe: the existing dedupe package finds
// duplicates by matching on (date, amount, description) within a single
// call-site's slice. fingerprint adds two things dedupe lacks:
//  1. Account-scoping — two identical charges on different accounts are NOT
//     duplicates (e.g., a shared subscription billed to two cards).
//  2. POS noise normalisation — leading "#"/"*" and surrounding punctuation
//     are stripped before matching, so "# STARBUCKS 0042" and
//     "STARBUCKS 0042" produce the same fingerprint.
//
// The fingerprint is intended for import-time de-dup and the /duplicates merge
// screen (tickets C86–C89 / IMPL R8-engine).
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// NormalizePayee returns a cleaned payee string suitable for fingerprinting:
//   - lowercased
//   - internal whitespace collapsed to a single space
//   - leading and trailing whitespace trimmed
//   - leading POS noise stripped: sequences of '#', '*', '/', '\', and
//     surrounding punctuation/whitespace that card networks prepend to merchant
//     names (e.g. "# MERCHANT", "*MERCHANT", "//MERCHANT").
//
// The returned string may be empty if the input was all noise.
func NormalizePayee(s string) string {
	// Collapse and trim whitespace first so the noise-prefix logic sees a clean
	// leading character.
	s = collapseWhitespace(s)

	// Strip the POS-noise prefix: one or more leading characters that are
	// '#', '*', '/', '\', or other punctuation that is not letter/digit,
	// followed by optional whitespace.
	s = stripPOSPrefix(s)

	// A second collapse pass in case stripping left leading/trailing spaces.
	s = collapseWhitespace(s)
	return s
}

// collapseWhitespace lowercases s, trims leading/trailing whitespace, and
// collapses every internal run of whitespace to a single ASCII space.
func collapseWhitespace(s string) string {
	s = strings.ToLower(s)
	fields := strings.Fields(s) // splits on any whitespace, drops empties
	return strings.Join(fields, " ")
}

// stripPOSPrefix removes a leading sequence of non-alphanumeric "noise"
// characters (commonly '#', '*', '/', '\\') followed by optional whitespace.
// It keeps going while the first rune is punctuation/symbol and not a letter
// or digit, capped at a reasonable lookahead so it cannot consume meaningful
// content.
func stripPOSPrefix(s string) string {
	for len(s) > 0 {
		r := []rune(s)[0]
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			break
		}
		// Strip this leading noise rune plus any following whitespace.
		s = strings.TrimLeftFunc(s[len(string(r)):], unicode.IsSpace)
	}
	return s
}

// Fingerprint returns a stable 16-hex-character string that identifies a
// transaction by its economic identity: the calendar date, signed amount in
// minor units, normalized payee, and the account it belongs to.
//
// The fingerprint is the first 16 hex characters (8 bytes) of the SHA-256
// of the pipe-delimited join:
//
//	date.Format("2006-01-02") + "|" + strconv.FormatInt(amountMinor, 10)
//	  + "|" + strings.ToUpper(NormalizePayee(payee)) + "|" + accountID
//
// Signed amount: a charge of -1000 and a credit of +1000 on the same date
// have different fingerprints, as intended. Account-scoped: the same charge
// on two different accounts produces different fingerprints.
func Fingerprint(date time.Time, amountMinor int64, payee, accountID string) string {
	day := date.Format("2006-01-02")
	amt := strconv.FormatInt(amountMinor, 10)
	normPayee := strings.ToUpper(NormalizePayee(payee))
	raw := day + "|" + amt + "|" + normPayee + "|" + accountID
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:16]
}

// TxFingerprint is a convenience wrapper that derives the Fingerprint
// arguments from a domain.Transaction.
//
// Payee resolution: if Transaction.Payee is non-empty it is used; otherwise
// Transaction.Desc is the fallback. This mirrors how the ledger displays the
// merchant name: Payee is the normalised counterparty name, Desc is the raw
// bank narrative — either can uniquely identify a merchant depending on the
// import source.
//
// Amount: Transaction.Amount.Amount (int64 minor units, signed) is used
// directly. The currency is NOT included in the fingerprint because the
// account already fixes the currency; including it would add redundancy and
// break fingerprints if the currency field is corrected after import.
//
// Account: Transaction.AccountID scopes the fingerprint so the same charge
// on two separate accounts (e.g. a shared subscription billed to two cards)
// is not treated as a duplicate.
func TxFingerprint(t domain.Transaction) string {
	payee := t.Payee
	if payee == "" {
		payee = t.Desc
	}
	return Fingerprint(t.Date, t.Amount.Amount, payee, t.AccountID)
}

// GroupDuplicates groups the supplied transactions by their TxFingerprint and
// returns only groups with two or more members. Each group preserves the
// original input order of its members. The group order is deterministic:
// groups are returned in ascending order of the first member's position in the
// input slice, so the caller sees duplicates in the same sequence as the input.
//
// Transfer transactions are included: an imported transfer leg can be a
// duplicate of a manually entered one, so the caller decides whether to filter
// transfers before or after calling this function.
func GroupDuplicates(txns []domain.Transaction) [][]domain.Transaction {
	// Map fingerprint → positions of matching transactions in txns.
	type entry struct {
		firstIdx int
		members  []domain.Transaction
	}
	seen := map[string]*entry{}
	order := []string{} // insertion order of fingerprints

	for i, t := range txns {
		fp := TxFingerprint(t)
		if e, ok := seen[fp]; ok {
			e.members = append(e.members, t)
		} else {
			seen[fp] = &entry{firstIdx: i, members: []domain.Transaction{t}}
			order = append(order, fp)
		}
	}

	// Collect groups of ≥2 members in first-member-index order.
	type indexedGroup struct {
		firstIdx int
		group    []domain.Transaction
	}
	var groups []indexedGroup
	for _, fp := range order {
		e := seen[fp]
		if len(e.members) >= 2 {
			groups = append(groups, indexedGroup{firstIdx: e.firstIdx, group: e.members})
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].firstIdx < groups[j].firstIdx
	})

	out := make([][]domain.Transaction, len(groups))
	for i, g := range groups {
		out[i] = g.group
	}
	return out
}

// MergeResolve produces a single merged transaction from two duplicates, a and b.
// The caller is responsible for deleting b from the store after replacing a
// with the merged result; a's ID is kept so existing references (attachments,
// audit logs) remain valid.
//
// Merge rules (applied field by field):
//   - ID: a's ID is kept (b is the record to delete).
//   - AccountID, Date, Amount, MemberID, SourceDocID: a's values are kept;
//     these identify the canonical leg and must not change under the caller.
//   - Date: the earlier of a.Date and b.Date is kept (import dates can drift by
//     one day due to time-zone rounding; earlier is more conservative).
//   - Payee: prefer non-empty; if both non-empty, prefer a's.
//   - Desc: prefer the longer string (raw bank narrative carries more detail).
//   - CategoryID: prefer a set (non-empty) value; if both set, prefer a's.
//   - Cleared: true wins (a transaction that has cleared is authoritative).
//   - Tags: union of both slices, deduplicated, sorted for stability.
//   - Reviewed: true wins (an explicitly reviewed entry is authoritative).
//   - TransferAccountID: prefer non-empty; if both non-empty, prefer a's.
//   - Splits: prefer non-nil/non-empty; if both non-empty, prefer a's.
//   - Attachments: union of both slices (deduplicated by ArtifactID).
//   - Custom: union of both maps; a's values win on key conflicts.
func MergeResolve(a, b domain.Transaction) domain.Transaction {
	merged := a // start from a so all fields default to a's values

	// Date: keep the earlier.
	if b.Date.Before(a.Date) {
		merged.Date = b.Date
	}

	// Payee: prefer non-empty.
	if merged.Payee == "" {
		merged.Payee = b.Payee
	}

	// Desc: prefer the longer raw narrative.
	if len(b.Desc) > len(a.Desc) {
		merged.Desc = b.Desc
	}

	// CategoryID: prefer set over empty.
	if merged.CategoryID == "" {
		merged.CategoryID = b.CategoryID
	}

	// Cleared: true wins.
	if b.Cleared {
		merged.Cleared = true
	}

	// Reviewed: true wins.
	if b.Reviewed {
		merged.Reviewed = true
	}

	// TransferAccountID: prefer non-empty.
	if merged.TransferAccountID == "" {
		merged.TransferAccountID = b.TransferAccountID
	}

	// Splits: prefer non-nil/non-empty.
	if len(merged.Splits) == 0 && len(b.Splits) > 0 {
		merged.Splits = b.Splits
	}

	// Tags: union, deduplicated, sorted.
	merged.Tags = unionTags(a.Tags, b.Tags)

	// Attachments: union by ArtifactID.
	merged.Attachments = unionAttachments(a.Attachments, b.Attachments)

	// Custom: union; a's values win on conflict.
	merged.Custom = unionCustom(a.Custom, b.Custom)

	return merged
}

// unionTags returns the deduplicated, sorted union of two tag slices.
func unionTags(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	for _, t := range a {
		seen[t] = struct{}{}
	}
	for _, t := range b {
		seen[t] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// unionAttachments returns the union of two AttachmentRef slices, deduplicated
// by ArtifactID. a's entries appear first, then any from b not already present.
func unionAttachments(a, b []domain.AttachmentRef) []domain.AttachmentRef {
	seen := make(map[string]struct{}, len(a))
	out := make([]domain.AttachmentRef, 0, len(a)+len(b))
	for _, ref := range a {
		if _, ok := seen[ref.ArtifactID]; !ok {
			seen[ref.ArtifactID] = struct{}{}
			out = append(out, ref)
		}
	}
	for _, ref := range b {
		if _, ok := seen[ref.ArtifactID]; !ok {
			seen[ref.ArtifactID] = struct{}{}
			out = append(out, ref)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// unionCustom returns the union of two custom-field maps with a's values
// winning on key conflicts. Returns nil if both inputs are nil.
func unionCustom(a, b map[string]any) map[string]any {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := make(map[string]any, len(a)+len(b))
	maps.Copy(out, b)
	maps.Copy(out, a) // a wins on conflict
	return out
}
