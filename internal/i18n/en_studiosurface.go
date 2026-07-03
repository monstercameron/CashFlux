// SPDX-License-Identifier: MIT

package i18n

// studioSurfaceKeys holds the English strings for the redesigned Studio
// Formulas tab (the searchable variable palette and the compound-variable
// editor). Merged via init so this file does not touch en.go.
var studioSurfaceKeys = Catalog{
	// FormulaBuilder palette search.
	"customize.searchLabel":        "Search variables",
	"customize.searchPlaceholder":  "Search variables…",
	"customize.searchPlaceholderN": "Search %d variables…",
	"customize.searchResults":      "%d matches",
	"customize.searchEmpty":        "Nothing matches \"%s\" — try a shorter word, or browse the groups.",
	"customize.savedScopeHint":     "Saved formulas are your personal calculations — they don't change how the app computes anything.",

	// Molecule-editor guardrail.
	"studio.molLiveWarning": "⚠ Live definition — saving changes every page and widget that uses %s, immediately.",

	// Compound-variable (molecule) editor.
	"studio.molTitle":        "Compound variables",
	"studio.molHint":         "These figures are defined as formulas over the atoms — the app computes them from these exact definitions. Edit one to reshape it everywhere (your dashboard, /health, /credit, every widget).",
	"studio.molBuiltIn":      "built-in",
	"studio.molOverridden":   "edited",
	"studio.molCustom":       "yours",
	"studio.molEdit":         "Edit",
	"studio.molCancel":       "Cancel",
	"studio.molSave":         "Save definition",
	"studio.molReset":        "Reset to default",
	"studio.molDelete":       "Delete",
	"studio.molSaved":        "Saved — every surface now uses this definition.",
	"studio.molReverted":     "Restored the built-in definition.",
	"studio.molFormulaLabel": "Formula for %s",
	"studio.molPreview":      "= %s",
	"studio.molPreviewErr":   "This formula doesn't evaluate: %s",
	"studio.molNewTitle":     "New compound variable",
	"studio.molNewName":      "variable_name",
	"studio.molNewFormula":   "e.g. liquid_cash - bills_due",
	"studio.molCreate":       "Create variable",
	"studio.molNameHint":     "Lowercase letters and underscores — this becomes the name you reference in formulas and widgets.",
	"studio.molNameTaken":    "\"%s\" already exists — pick a different name.",
}

func init() {
	for k, v := range studioSurfaceKeys {
		english[k] = v
	}
}
