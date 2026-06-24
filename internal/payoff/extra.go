// SPDX-License-Identifier: MIT

package payoff

// SuggestedExtra proposes a starting monthly extra to throw at the debts so the
// snowball vs avalanche comparison is actually meaningful — at $0 extra the two
// strategies are identical. It is a quarter of the total minimum payments, or 1%
// of the total balance when minimums are unknown, with a one-minor-unit floor so a
// real debt always suggests something actionable. No debts suggests zero. Amounts
// are integer minor units.
func SuggestedExtra(debts []Debt) int64 {
	var sumMin, sumBal int64
	for _, d := range debts {
		if d.Balance <= 0 {
			continue
		}
		sumBal += d.Balance
		if d.MinPayment > 0 {
			sumMin += d.MinPayment
		}
	}
	if sumBal == 0 {
		return 0
	}
	extra := sumMin / 4
	if extra <= 0 {
		extra = sumBal / 100
	}
	if extra < 1 {
		extra = 1
	}
	return extra
}
