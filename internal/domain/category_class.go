// SPDX-License-Identifier: MIT

package domain

// CategoryClass groups an expense category by how flex budgeting (BG2) treats it.
// It is orthogonal to CategoryKind (income/expense): only expense categories are
// meaningfully classified, but the field is stored on any category.
//
//   - ClassFixed: an expected, unmanaged commitment (rent, insurance). Rendered as
//     an expected-vs-actual checkoff, composed with recurring occurrences.
//   - ClassNonMonthly: an irregular cost smoothed into a monthly set-aside (XC3),
//     shown as its accrued sinking-fund balance rather than a hard limit.
//   - ClassFlex: day-to-day discretionary spending pooled into ONE flex number the
//     user manages, instead of a per-category limit. This is the default.
type CategoryClass string

const (
	// ClassFlex is the default: discretionary spending managed as one pooled number.
	ClassFlex CategoryClass = "flex"
	// ClassFixed is an expected, unmanaged commitment rendered as a checkoff.
	ClassFixed CategoryClass = "fixed"
	// ClassNonMonthly is an irregular cost shown as a smoothed monthly accrual.
	ClassNonMonthly CategoryClass = "non-monthly"
)

// AllCategoryClasses lists every valid category class, in assignment-sheet order.
var AllCategoryClasses = []CategoryClass{ClassFlex, ClassFixed, ClassNonMonthly}

// String returns the class as a plain string.
func (c CategoryClass) String() string { return string(c) }

// Valid reports whether c is a known category class.
func (c CategoryClass) Valid() bool {
	switch c {
	case ClassFlex, ClassFixed, ClassNonMonthly:
		return true
	default:
		return false
	}
}

// ClassOf returns the category's effective class, defaulting an empty or unknown
// stored value to ClassFlex — so older datasets without the field, and any future
// value this build doesn't know, behave as the safe discretionary default.
func (c Category) ClassOf() CategoryClass {
	if c.CategoryClass.Valid() {
		return c.CategoryClass
	}
	return ClassFlex
}
