// SPDX-License-Identifier: MIT

package i18n

// txnTemplateKeys holds the copy for transaction quick-templates ("favourites"):
// the picker strip in quick-add, the "save as template" action and its name
// prompt, the per-chip delete affordance, and the empty state shown before any
// template exists. Merged via init so this file does not touch en.go.
var txnTemplateKeys = Catalog{
	"txnTemplates.label":         "Templates",
	"txnTemplates.use":           "Use template",
	"txnTemplates.save":          "Save as template",
	"txnTemplates.namePrompt":    "Name this template",
	"txnTemplates.delete":        "Delete template",
	"txnTemplates.deleteConfirm": "Delete this template?",
	// Shown in the picker strip when there are no saved templates yet.
	"txnTemplates.empty": "No templates yet — fill in a transaction, then \"Save as template\" to reuse it in one click.",
	// Toasts after a successful save / delete.
	"txnTemplates.saved":   "Template saved.",
	"txnTemplates.deleted": "Template deleted.",
	// Shown if "Save as template" is tapped before the form has an amount.
	"txnTemplates.needAmount": "Add an amount before saving this as a template.",
}

func init() {
	for k, v := range txnTemplateKeys {
		english[k] = v
	}
}
