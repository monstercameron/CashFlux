// SPDX-License-Identifier: MIT

package i18n

// imaKeys supplies shared staged-import review copy for every source.
var imaKeys = Catalog{
	"ima.stalePreview":      "The import preview has changed. Review it again before importing.",
	"ima.duplicate":         "Already in your ledger",
	"ima.possibleDuplicate": "Possible duplicate. Review this separate occurrence.",
	"ima.invalidRow":        "This row needs correction before it can be imported.",
	"ima.unknownAccount":    "Choose an existing account for this row.",
	"ima.currencyMismatch":  "The currency must match the selected account.",
	"ima.savePending":       "Saving your imported rows. Keep this page open.",
	"ima.saveFailed":        "Your imported rows have not been saved to this device. Keep this page open and try saving again.",
}

func init() {
	for key, value := range imaKeys {
		english[key] = value
	}
}
