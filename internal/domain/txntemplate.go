// SPDX-License-Identifier: MIT

package domain

// TxnDirection records whether a quick-template posts money out (an expense) or
// in (income). Because there is no bank sync, users enter transactions by hand,
// and a template mirrors the quick-add form's Expense/Income toggle so a saved
// favourite fills the form in exactly the direction the user meant. The empty
// value is treated as an expense (the common case) by EffectiveDirection.
type TxnDirection string

const (
	// DirectionExpense is money leaving an account (the quick-add "Expense" kind,
	// which posts a negative Transaction.Amount).
	DirectionExpense TxnDirection = "expense"
	// DirectionIncome is money entering an account (the quick-add "Income" kind,
	// which posts a positive Transaction.Amount).
	DirectionIncome TxnDirection = "income"
)

// TxnTemplate is a saved, reusable "favourite" transaction: a named bundle of the
// fields a user would otherwise re-type every time (payee, category, account,
// amount, direction, optional note + tags). One click on a template pre-fills the
// quick-add form, which is high value in a local-first app with no bank sync where
// every transaction is entered manually.
//
// AmountMinor is the MAGNITUDE in integer minor units (e.g. cents) and is always
// non-negative — the sign is carried separately by Direction, matching how the
// quick-add form keeps a positive amount field plus an Expense/Income toggle. The
// whole value is persisted as one JSON blob in the KV store (no SQL schema), so
// new fields must stay additive and JSON-round-trip cleanly.
type TxnTemplate struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Payee       string       `json:"payee,omitempty"`
	CategoryID  string       `json:"categoryId,omitempty"`
	AccountID   string       `json:"accountId,omitempty"`
	AmountMinor int64        `json:"amountMinor"`
	Currency    string       `json:"currency,omitempty"`
	Direction   TxnDirection `json:"direction,omitempty"`
	// Note is copied into the new transaction's Desc when the template is applied
	// (the quick-add form requires a description). Kept as "note" here because from
	// the user's point of view it is the memo they attached to the favourite.
	Note string   `json:"note,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

// EffectiveDirection resolves the zero value to DirectionExpense so callers can
// switch on a concrete direction without special-casing legacy/blank templates.
func (t TxnTemplate) EffectiveDirection() TxnDirection {
	if t.Direction == DirectionIncome {
		return DirectionIncome
	}
	return DirectionExpense
}

// IsExpense reports whether applying this template posts money out.
func (t TxnTemplate) IsExpense() bool { return t.EffectiveDirection() == DirectionExpense }

// SignedMinor returns the amount as a signed minor-unit value: negative for an
// expense, positive for income. AmountMinor's own sign is ignored (it is treated
// as a magnitude), so a template is robust even if a caller stored a signed value.
func (t TxnTemplate) SignedMinor() int64 {
	a := t.AmountMinor
	if a < 0 {
		a = -a
	}
	if t.IsExpense() {
		return -a
	}
	return a
}
