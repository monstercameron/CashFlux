package i18n

func init() {
	for key, value := range map[string]string{
		"tc3w4.saving":              "Saving...",
		"tc3w4.notSaved":            "Not saved",
		"tc3w4.changesNotSaved":     "These changes are not saved. Keep this page open and try again.",
		"tc3w4.transactionNotSaved": "This transaction is not saved. Keep this page open and try again.",
		"tc3w4.splitNotSaved":       "This split is not saved. Keep this window open and try again.",
		"tc3w4.categoryNotSaved":    "This category change is not saved. Keep this page open and try again.",
		"tc3w4.undoNotSaved":        "The undo is not saved. Keep this page open and try again.",
		"common.cancel":             "Cancel",
	} {
		english[key] = value
	}
}
