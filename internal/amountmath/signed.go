// SPDX-License-Identifier: MIT

package amountmath

import (
	"errors"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/money"
)

// Direction is which way money moves for an amount the user typed. It is the
// value behind every "Money in / Money out" control in the app.
type Direction int

const (
	// Out is spending: the amount is stored NEGATIVE.
	Out Direction = iota
	// In is income: the amount is stored POSITIVE.
	In
)

// String renders the direction as the stable token the forms persist ("out" /
// "in"), so a control's value and this type never drift apart.
func (d Direction) String() string {
	if d == In {
		return "in"
	}
	return "out"
}

// DirectionFrom maps a control's stored value back to a Direction. Anything
// that is not exactly "in" reads as Out, which matches the forms' historical
// default (a new transaction is a spend until said otherwise).
func DirectionFrom(s string) Direction {
	if strings.TrimSpace(strings.ToLower(s)) == "in" {
		return In
	}
	return Out
}

// ErrZeroAmount reports an amount that parsed cleanly but is zero. It is
// separate from an unparseable amount because the two deserve different
// messages: "that isn't a number" versus "a transaction of nothing".
var ErrZeroAmount = errors.New("amountmath: amount is zero")

// ParseSigned resolves a typed amount field and a direction control into the
// signed minor-unit value to store, plus the direction that actually applied.
//
// The amount field holds a MAGNITUDE and the direction control holds the sign —
// but users type a minus sign anyway, because that is how money is written
// everywhere else. Rather than reject it (which reads as "the app won't let me
// fix my mistake") or ignore it (which silently stores the opposite of what was
// typed), an explicit leading minus is read as INTENT and wins:
//
//	ParseSigned("50",  2, Out) → -5000, Out  // magnitude + control
//	ParseSigned("50",  2, In)  → +5000, In
//	ParseSigned("-50", 2, Out) → -5000, Out  // typed sign agrees with the control
//	ParseSigned("-50", 2, In)  → -5000, Out  // typed sign OVERRIDES the control
//	ParseSigned("+50", 2, Out) → -5000, Out  // an explicit plus is not a direction
//
// The returned Direction is what the caller must write back into its control,
// so the form never displays "Money in" over a stored expense. An explicit
// PLUS is deliberately not symmetrical: "+50" is how people write a plain
// positive number, not an assertion that this is income, so it leaves the
// control alone.
//
// A zero amount returns ErrZeroAmount; anything unparseable returns a wrapped
// money.ErrInvalidAmount. Both leave the caller free to keep its own copy for
// the field's error message.
func ParseSigned(input string, decimals int, want Direction) (minor int64, resolved Direction, err error) {
	trimmed := strings.TrimSpace(input)
	parsed, err := money.ParseMinor(trimmed, decimals)
	if err != nil {
		return 0, want, fmt.Errorf("amountmath: %q is not an amount: %w", trimmed, err)
	}
	if parsed == 0 {
		return 0, want, ErrZeroAmount
	}

	// A typed minus is an instruction, not noise: honour it and report the
	// direction back so the control can follow. Note this reads the SIGN of the
	// parsed value rather than the leading byte, so "-0.00" (which parses to 0)
	// is already handled above as a zero rather than as a direction change.
	resolved = want
	if parsed < 0 {
		resolved = Out
		parsed = -parsed
	}

	if resolved == Out {
		return -parsed, resolved, nil
	}
	return parsed, resolved, nil
}
