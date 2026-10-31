// SPDX-License-Identifier: MIT

package reconcile

// Shape names the kind of balance update a person is making, with both figures in
// the stated convention (positive is money you have, negative is money you owe).
// The Update balance dialog uses it to explain the change in plain words.
type Shape int

const (
	// ShapeNone means the typed figure already matches the current balance.
	ShapeNone Shape = iota
	// ShapeDebtFalls means the account is still owed money, and owes less afterwards.
	ShapeDebtFalls
	// ShapeDebtRises means the account is still owed money, and owes more afterwards.
	ShapeDebtRises
	// ShapeOther covers every other change: money held before or after, or a balance
	// that crosses zero between debt and credit.
	ShapeOther
)

// ClassifyChange reports the Shape of moving a balance from currentStated to
// targetStated. Both arguments are minor units in the stated convention.
func ClassifyChange(currentStated, targetStated int64) Shape {
	switch {
	case currentStated == targetStated:
		return ShapeNone
	case currentStated < 0 && targetStated < 0 && targetStated > currentStated:
		return ShapeDebtFalls
	case currentStated < 0 && targetStated < 0:
		return ShapeDebtRises
	default:
		return ShapeOther
	}
}

// LooksLikeUnsignedDebt reports whether a figure typed as positive would be read as
// money held on an account that is currently owed money. It flags the likely slip of
// leaving off the minus sign. It does not block the entry, because a debt can
// legitimately be paid off into credit.
func LooksLikeUnsignedDebt(currentStated, typedStated int64) bool {
	return currentStated < 0 && typedStated > 0
}
