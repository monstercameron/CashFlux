// SPDX-License-Identifier: MIT

package reconcile

import (
	"math"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Display is how an account's booked and cleared balances read on screen.
type Display struct {
	// Balance is the booked balance in the account's stated convention.
	Balance money.Money
	// Cleared is the cleared-only balance in the same convention.
	Cleared money.Money
	// ShowCleared is set when the cleared figure helps: the account type is
	// reconciled against statements and the cleared figure differs from the booked
	// one. Beside a real balance a cleared figure reads as a second claim about
	// what is owed, so it is hidden for accounts that are not reconciled.
	ShowCleared bool
}

// DisplayBalance converts an account's stored balances for display. A liability
// goes through the account's own storage convention rather than -Abs, because -Abs
// turns an overpaid card (money the lender owes you) into more debt. Editing works
// on the stored value; only the displayed figures are converted.
func DisplayBalance(a domain.Account, balance, cleared money.Money) Display {
	d := Display{
		Balance:     balance,
		Cleared:     cleared,
		ShowCleared: a.Type.IsReconcilable() && cleared.Amount != balance.Amount,
	}
	if a.Class == domain.ClassLiability {
		conv, _ := ConventionOf(a, balance.Amount)
		d.Balance = money.New(Stated(balance.Amount, conv), balance.Currency)
		d.Cleared = money.New(Stated(cleared.Amount, conv), cleared.Currency)
	}
	return d
}

// OwedMinor is the amount a stated balance owes, as a non-negative figure: a negative stated
// balance (money you owe) owes its magnitude, and a zero or positive one owes nothing. The result
// saturates at math.MaxInt64 instead of overflowing on math.MinInt64.
func OwedMinor(stated int64) int64 {
	switch {
	case stated >= 0:
		return 0
	case stated == math.MinInt64:
		return math.MaxInt64
	default:
		return -stated
	}
}

// StatedOwed is the stated balance for an amount owed: the magnitude of owed as a negative figure.
// It is the inverse of OwedMinor, and math.MinInt64 is returned as it is because its magnitude does
// not fit.
func StatedOwed(owed int64) int64 {
	if owed == math.MinInt64 {
		return math.MinInt64
	}
	if owed < 0 {
		owed = -owed
	}
	return -owed
}
