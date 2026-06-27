// SPDX-License-Identifier: MIT

package i18n

// learnTallyKeys holds the English strings for the self-learning categorization
// features (C33/C34/C35). Defined in their own file and merged via init so this
// does not touch the user-WIP en.go; mirrors the en_quickaddfix.go pattern.
//
//   - C34: "Use '<category>'" chip label shown in Quick-Add when a learned
//     suggestion crosses the threshold.
//   - C35: settings label + hint for the user-tunable suggestion threshold.
var learnTallyKeys = Catalog{
	// C34: chip prefix shown before the category name in Quick-Add.
	// Full rendering: "✦ Use 'Groceries'" (via SmartFieldAssist which appends the name).
	"quickAdd.learnedCatSuggest": "Learned: use",

	// C35: label and hint for the auto-category threshold setting in the
	// notifications / auto-categorize section of Settings.
	"settings.learnThresholdLabel": "Auto-category suggestion threshold",
	"settings.learnThresholdHint":  "Number of times you must assign the same category to a payee before a suggestion appears in Quick-Add.",
}

func init() {
	for k, v := range learnTallyKeys {
		english[k] = v
	}
}
