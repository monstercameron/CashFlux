// SPDX-License-Identifier: MIT

package budgeting

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/money"
)

// LimitProblem says why a typed budget limit cannot be used. It is deliberately a
// small enumeration rather than an error: every editor that takes a limit — the
// card's inline editor, the full edit form, the add form — has to make the SAME
// three decisions from it (is the commit button live, which field-level message
// shows, may the value be written), and each needs to name the problem in the
// user's language through the i18n catalog rather than surface a Go error string.
type LimitProblem string

// The limit problems. The empty value means the input is usable.
const (
	LimitOK          LimitProblem = ""             // parses to a positive amount
	LimitBlank       LimitProblem = "blank"        // nothing typed yet
	LimitMalformed   LimitProblem = "malformed"    // not a number ("abc", "1e5", "1.234", overflow)
	LimitNotPositive LimitProblem = "not-positive" // parses, but is zero or negative
)

// OK reports whether the parsed limit may be written.
func (p LimitProblem) OK() bool { return p == LimitOK }

// ParseLimitMajor parses a budget limit typed in major units (what the user sees
// in the field) into minor units, and reports why it is unusable when it is.
//
// A budget limit is the size of a plan, so only a strictly positive amount means
// anything: zero is a budget that can never be kept and a negative one inverts
// the meaning of every figure computed from it — remaining, percent-used, and
// over/under all read backwards. money.ParseMinor accepts a leading "-" because
// transaction amounts legitimately go both ways, which is exactly why the limit
// editors cannot use it unguarded.
func ParseLimitMajor(s string, decimals int) (int64, LimitProblem) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, LimitBlank
	}
	amt, err := money.ParseMinor(t, decimals)
	if err != nil {
		return 0, LimitMalformed
	}
	if amt <= 0 {
		return 0, LimitNotPositive
	}
	return amt, LimitOK
}
