// SPDX-License-Identifier: MIT

package i18n

// fieldsSurfaceKeys holds the English strings for the redesigned /fields
// screen: the schema-registry ledger on the left and the labeled composer with
// its live "what this field will do" footprint on the right. Merged via init
// so this file does not touch en.go.
var fieldsSurfaceKeys = Catalog{
	"fld.registryKicker":    "Field registry",
	"fld.countNone":         "No fields defined yet",
	"fld.countOne":          "1 field defined",
	"fld.countMany":         "%d fields defined",
	"fld.registryLede":      "Fields you define here are added to that entity's add and edit forms. Number fields also become variables your formulas and dashboard widgets can read.",
	"fld.addAnother":        "Add another",
	"fld.defineFor":         "Define a field on %s",
	"fld.undefinedLabel":    "Nothing yet on",
	"fld.deleteWarn":        "Delete this field? Values saved on existing records go with it.",
	"fld.deleteFormulaWarn": "Formulas using %s will stop working.",
	"fld.deleteYes":         "Delete field",
	"fld.deleteNo":          "Keep it",
	"fld.livesOn":           "Lives on",
	"fld.typeLabel":         "Type",
	"fld.choicesLabel":      "Choices",
	"fld.requirementLabel":  "Requirement",
	"fld.compLede":          "Name it, pick a type, and it shows up everywhere that entity is added or edited.",
	"fld.footTitle":         "What this field will do",
	"fld.footForm":          "Shows up on every %s form.",
	"fld.footReports":       "Reports can group spending by it.",
	"fld.footFormula":       "Formulas can read it as",
	"fld.footFormulaHint":   "Give it a key and this number becomes a formula variable.",
	"fld.footRequired":      "It'll be required — the form won't save without it.",
	"fld.varTitle":          "Formula variable — use it in Formulas and widget bindings",
}

func init() {
	for k, v := range fieldsSurfaceKeys {
		english[k] = v
	}
}
