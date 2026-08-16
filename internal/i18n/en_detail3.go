// SPDX-License-Identifier: MIT

package i18n

// detail3Keys holds copy for the 2026-07-19 fine-detail polish (detail-lane 3) on
// /budgets: the density (compact-list) toggle surfaced directly on the page toolbar,
// so it no longer hides inside the Budget-settings popover. Merged via init so this
// file does not touch en.go.
// C596 retired this file's only key: the density toggle no longer overrides its
// accessible name with an aria-label. Its visible text IS its name now, so a
// screen-reader user and a sighted user refer to the same control, and the state
// lives in aria-pressed rather than in a swapped label. See en_budgetdensity.go.
var detail3Keys = Catalog{}

func init() {
	for k, v := range detail3Keys {
		english[k] = v
	}
}
