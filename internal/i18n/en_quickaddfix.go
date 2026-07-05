// SPDX-License-Identifier: MIT

package i18n

// quickAddFixKeys holds the English strings added for the F5 quick-add fixes.
// Defined in their own file and merged via init so this does not touch the
// user-WIP en.go; mirrors the en_setup.go pattern.
//
// C47: clarified "reviewed" checkbox label and helper caption so the field is
// self-explanatory without needing to read documentation.
var quickAddFixKeys = Catalog{
	// C47: primary label for the "mark as reviewed" checkbox. Replaces the
	// terse "quickAdd.reviewed" key in the rendered label so the user
	// immediately understands what checking it does.
	"quickAdd.reviewedClear": "Mark as reviewed",

	// C47: short muted caption shown beneath the checkbox explaining what
	// leaving it unchecked means, so the user never has to guess at the
	// consequences of either choice.
	"quickAdd.reviewedHelp": "Leave unchecked to tag it #needs-review for follow-up.",
}

func init() {
	for k, v := range quickAddFixKeys {
		english[k] = v
	}
}
