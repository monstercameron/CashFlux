// SPDX-License-Identifier: MIT

package uistate

import (
	"strings"
	"time"
)

// ─── C409: turning an alert into a resolution ────────────────────────────────
//
// Every notification already links somewhere, which answers "where do I go".
// The commoner need is "make this go away, correctly" — a bill alert wants
// marking paid, a stale-balance alert wants the balance confirmed. Sending the
// reader to a page to hunt for the row the alert was already about is a step the
// app can simply do.
//
// A feed item's ID is notify.DedupeKey(ruleID, occurrence) = "ruleID@occurrence",
// and each generator builds its occurrence key from the entity it is about. That
// makes the ID a durable reference to the thing that needs resolving — parsed
// here, in one place, rather than by each surface picking the string apart.
//
// This file carries no build constraints: it is pure string work over a stable
// format, and the format is exactly the kind of thing that should be covered by
// plain `go test`.

// ResolveKind names what an alert can be resolved by doing.
type ResolveKind string

const (
	// ResolveNone means the alert has no direct resolution — reading it, or
	// following its link, is all there is.
	ResolveNone ResolveKind = ""
	// ResolveBillPaid marks a specific bill occurrence paid.
	ResolveBillPaid ResolveKind = "bill-paid"
	// ResolveBalanceConfirmed confirms an account's balance as of today.
	ResolveBalanceConfirmed ResolveKind = "balance-confirmed"
)

// Resolution is what a notification can be resolved by, and against what.
type Resolution struct {
	Kind ResolveKind
	// EntityID is the account (balance) or bill (payment) the alert is about.
	EntityID string
	// Occurrence is the due date a bill resolution applies to; zero otherwise.
	// A bill alert is about ONE occurrence, and marking "the bill" paid without
	// saying which month is how a paid mark lands on the wrong one.
	Occurrence time.Time
}

// ResolutionFor reads a feed item's ID and reports what, if anything, it can be
// resolved by. An id it does not recognise resolves to nothing rather than to a
// guess: a wrong resolution writes to the wrong entity.
func ResolutionFor(id string) Resolution {
	rule, occurrence, ok := strings.Cut(id, "@")
	if !ok || occurrence == "" {
		return Resolution{}
	}
	switch rule {
	case "default-bill-due":
		// "<billID>@<YYYY-MM-DD>" — the bill id may itself contain a colon
		// (recurring:<id>), so split from the RIGHT.
		i := strings.LastIndex(occurrence, "@")
		if i <= 0 {
			return Resolution{}
		}
		due, err := time.Parse("2006-01-02", occurrence[i+1:])
		if err != nil {
			return Resolution{}
		}
		return Resolution{Kind: ResolveBillPaid, EntityID: occurrence[:i], Occurrence: due}
	case "default-stale":
		// "<accountID>@<weekKey>" — the account is everything before the last @.
		i := strings.LastIndex(occurrence, "@")
		if i <= 0 {
			return Resolution{}
		}
		return Resolution{Kind: ResolveBalanceConfirmed, EntityID: occurrence[:i]}
	}
	return Resolution{}
}

// Resolvable reports whether there is a direct action to offer.
func (r Resolution) Resolvable() bool { return r.Kind != ResolveNone && r.EntityID != "" }
