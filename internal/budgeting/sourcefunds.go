// SPDX-License-Identifier: MIT

package budgeting

import "github.com/monstercameron/CashFlux/internal/money"

// SourceFunds answers "how much of this budget can I move somewhere else?" for
// the cover and top-up flows (C606).
//
// The funding lists used to show a source's whole LIMIT under the word
// "available" — Groceries offered $354.85 while its own card said $76.56 left.
// The limit is not available; it is the size of the plan, most of which has
// already been spent. Worse, the figure was shown next to controls that DO cap
// correctly, so the number on screen and the number the app would move were two
// different things, and only the smaller one was true.
//
// There are three honest answers and they are all worth showing:
//
//   - Movable is what can actually be transferred: what is left after spending,
//     kept a cent below the limit so the source keeps a positive one (a budget
//     with a non-positive limit fails validation).
//   - Committed is the part of that which recurring charges this period have
//     already claimed. Moving it is allowed — it is the user's money and their
//     decision — but doing it unknowingly is how a bill stops fitting.
//   - Free is what is left over once those commitments are honoured: the amount
//     that can be moved with no consequence at all.
type SourceFunds struct {
	Limit     money.Money
	Remaining money.Money
	Committed money.Money
	Movable   money.Money
	Free      money.Money
}

// HasCommitment reports whether any of the movable money is already spoken for,
// so the UI shows the caveat only when there is one.
func (f SourceFunds) HasCommitment() bool { return f.Committed.Amount > 0 }

// CanGive reports whether this source has anything to offer at all.
func (f SourceFunds) CanGive() bool { return f.Movable.Amount > 0 }

// AfterGiving returns what would remain movable in this source after moving amt
// out of it. It floors at zero rather than reporting a negative, because the
// caller caps the move at Movable — a negative here would mean the cap failed,
// and a preview is not the place to display that.
func (f SourceFunds) AfterGiving(amtMinor int64) money.Money {
	left := f.Movable.Amount - amtMinor
	if left < 0 {
		left = 0
	}
	return money.New(left, f.Movable.Currency)
}

// SourceFundsOf resolves the three figures from a budget's limit, its remaining
// money for the period, and the part of that remaining which is committed.
//
// limit is the EFFECTIVE cap the card shows (rollover carry and one-time boosts
// already folded in), so the source list and the card agree by construction —
// which is the whole point of the ticket.
func SourceFundsOf(limit, remaining, committed money.Money) SourceFunds {
	cur := limit.Currency
	if cur == "" {
		cur = remaining.Currency
	}
	movable := remaining.Amount
	if capMax := limit.Amount - 1; movable > capMax {
		movable = capMax
	}
	if movable < 0 {
		movable = 0
	}
	com := committed.Amount
	if com < 0 {
		com = 0
	}
	if com > movable {
		com = movable
	}
	return SourceFunds{
		Limit:     money.New(limit.Amount, cur),
		Remaining: money.New(remaining.Amount, cur),
		Committed: money.New(com, cur),
		Movable:   money.New(movable, cur),
		Free:      money.New(movable-com, cur),
	}
}
