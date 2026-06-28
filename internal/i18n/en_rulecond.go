// SPDX-License-Identifier: MIT

package i18n

// ruleCondKeys holds the English strings for the C105 structured rule conditions
// feature. Defined in their own file and merged via init so this does not touch
// the user-WIP en.go; mirrors the en_learntally.go pattern.
var ruleCondKeys = Catalog{
	// Condition field labels (used in the rule form dropdowns).
	"rulecond.field.payee":       "Payee",
	"rulecond.field.description": "Description",
	"rulecond.field.amount":      "Amount",
	"rulecond.field.account":     "Account",
	"rulecond.field.date":        "Date",

	// Condition operator labels (text fields).
	"rulecond.op.contains": "contains",
	"rulecond.op.equals":   "equals",

	// Condition operator labels (numeric / amount fields).
	"rulecond.op.eq":  "equals (=)",
	"rulecond.op.neq": "not equal (≠)",
	"rulecond.op.lt":  "less than (<)",
	"rulecond.op.gt":  "greater than (>)",
	"rulecond.op.lte": "at most (≤)",
	"rulecond.op.gte": "at least (≥)",

	// Condition operator labels (account field).
	"rulecond.op.is":    "is",
	"rulecond.op.is-not": "is not",

	// Condition operator labels (date field).
	"rulecond.op.on":       "on",
	"rulecond.op.before":   "before",
	"rulecond.op.after":    "after",
	"rulecond.op.in-month": "in month",

	// UI chrome: condition slot labels and helpers.
	"rulecond.slot1":      "Condition 1",
	"rulecond.slot2":      "Condition 2",
	"rulecond.slot3":      "Condition 3",
	"rulecond.sectionLabel": "Additional conditions (optional, all must match)",
	"rulecond.valueLabel":  "Value",
	"rulecond.fieldLabel":  "Field",
	"rulecond.opLabel":     "Operator",
	"rulecond.enableLabel": "Use this condition",
	"rulecond.amountHint":  "Enter amount in cents (e.g. 500 = $5.00)",
	"rulecond.dateHint":    "Enter date as YYYY-MM-DD (or YYYY-MM for in-month)",
}

func init() {
	for k, v := range ruleCondKeys {
		english[k] = v
	}
}
