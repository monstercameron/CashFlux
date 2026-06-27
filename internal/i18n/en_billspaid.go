// SPDX-License-Identifier: MIT

package i18n

// billsPaidKeys holds the English strings for the per-bill paid-status UI
// (C154). Defined in their own file and merged via init so this does not touch
// the shared en.go; mirrors the en_setup.go pattern.
var billsPaidKeys = Catalog{
	// Paid chip shown in the bill row when the occurrence has been marked paid.
	"bills.paidBadge": "Paid",
	// Tooltip on the paid chip.
	"bills.paidBadgeTitle": "You've marked this bill as paid for this due date.",
	// Button label / tooltip to undo a paid mark.
	"bills.unmarkPaid":      "Unmark paid",
	"bills.unmarkPaidTitle": "Remove the paid mark for this due date",
	// Notice posted after toggling paid off.
	"bills.unpaidLogged": "Removed paid mark for %s.",
}

func init() {
	for k, v := range billsPaidKeys {
		english[k] = v
	}
}
