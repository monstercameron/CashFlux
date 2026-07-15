// SPDX-License-Identifier: MIT

package domain

import "time"

// Event is a first-class spending event — a trip, a wedding, a home project —
// that transactions are MAPPED to (TX10). It is a real entity, stored and
// exported like any other, NOT a saved filter: the mapping lives in the TxnLink
// table (kind TxnLinkEventTxn) so the transaction core schema stays untouched
// per the strong-schema rule, and a transaction can belong to an event AND carry
// other relations at once.
//
// The date range is inclusive of Start and exclusive of End ([Start, End)), the
// half-open convention the auto-association logic uses. A zero End means the
// event is open-ended (only Start bounds it).
type Event struct {
	// ID is the stable identifier for the event.
	ID string `json:"id"`
	// Name is the user-facing label ("Portugal trip", "Kitchen remodel").
	Name string `json:"name"`
	// Start is the first day the event covers (inclusive).
	Start time.Time `json:"start"`
	// End is the day the event's range ends (exclusive). Zero = open-ended.
	End time.Time `json:"end,omitempty"`
	// Note is an optional free-text description.
	Note string `json:"note,omitempty"`
	// Icon is an optional icon name (from the shared icon set) for the event chip.
	Icon string `json:"icon,omitempty"`
	// CreatedAt is when the event was created.
	CreatedAt time.Time `json:"createdAt"`
}

// Covers reports whether the given date falls inside the event's half-open
// range [Start, End). An open-ended event (zero End) covers every date on or
// after Start. Comparison is date-only against the calendar day of when.
func (e Event) Covers(when time.Time) bool {
	if when.Before(e.Start) {
		return false
	}
	if e.End.IsZero() {
		return true
	}
	return when.Before(e.End)
}
