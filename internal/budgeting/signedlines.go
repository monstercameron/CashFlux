// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// signedLineTotal sums split-line amounts without changing their signs.
// Keeping discounts and refunds signed until after netting prevents positive
// receipt remainder lines from increasing expense.
func signedLineTotal(lines []money.Money) (money.Money, error) {
	total, err := money.Sum(lines...)
	if err != nil {
		return money.Money{}, fmt.Errorf("sum signed split lines: %w", err)
	}
	return total, nil
}

// expenseContribution flips the expense sign once, after any split lines have
// been netted. A positive discount line therefore subtracts from the owner's
// contribution instead of becoming additional spend.
func expenseContribution(signed money.Money) (money.Money, error) {
	contribution, err := signed.NegChecked()
	if err != nil {
		return money.Money{}, fmt.Errorf("flip signed expense contribution: %w", err)
	}
	return contribution, nil
}
