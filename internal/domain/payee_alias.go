// SPDX-License-Identifier: MIT

package domain

import "time"

// PayeeAlias is a learned view-layer mapping from a raw payee string to a clean
// display name (TX1). It is a single persisted row: the raw payee exactly as it
// arrives on the transaction (processor noise and all) paired with the name the
// user wants to see everywhere it renders.
//
// Single-source rule: an alias NEVER mutates the transaction. The raw payee stays
// on the txn (exports keep it); resolution happens at the view layer and anywhere
// payees are matched (rules, search, recurring), so one clean name unifies
// filtering, reports, and the recurring detector.
//
// Matching is exact on the raw payee, case-insensitive (see payeealias.Resolver).
// A learned alias always wins over the built-in normalizer rule pack.
type PayeeAlias struct {
	// ID is the stable identifier for the alias row.
	ID string `json:"id"`
	// RawPayee is the exact payee string as it arrives on transactions, matched
	// case-insensitively. It is the key of the mapping.
	RawPayee string `json:"rawPayee"`
	// Display is the clean name shown in place of RawPayee.
	Display string `json:"display"`
	// CreatedAt records when the alias was learned.
	CreatedAt time.Time `json:"createdAt"`
}
