// SPDX-License-Identifier: MIT

// Package budgetplan holds the pure forward-planning calculations behind the
// annual budget grid's future months:
//
//   - Project (C394) expands recurring bill schedules and goal contribution
//     plans into the months of a year that have not happened yet, so a future
//     cell can pre-fill with what it is already committed to.
//   - Evaluate (C393) runs the scenario "what goes underfunded if income
//     changes by X" funding waterfall for a single month.
//
// Money is integer minor units end to end, and every result exposes its
// component breakdown (recurring vs goal; plan vs funded) so a grid cell can
// explain its own number rather than presenting a black box. No syscall/js —
// unit-tested on native Go.
package budgetplan

// MonthAmounts is one calendar year of minor-unit amounts, index 0 = January …
// 11 = December.
type MonthAmounts [12]int64

// addInto accumulates src into dst month by month (helper for folding).
func addInto(dst *MonthAmounts, src MonthAmounts) {
	for i := 0; i < 12; i++ {
		dst[i] += src[i]
	}
}
