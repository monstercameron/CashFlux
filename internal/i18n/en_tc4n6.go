// SPDX-License-Identifier: MIT

package i18n

var tc4n6Keys = Catalog{
	"tc4n6.count":             "%d %s",
	"tc4n6.amountRequired":    "amount is missing",
	"tc4n6.amountPrecision":   "amount has too many decimal places for its currency",
	"tc4n6.amountInvalid":     "amount could not be read",
	"tc4n6.dateInvalid":       "date could not be read",
	"tc4n6.currencyInvalid":   "currency is missing or invalid",
	"tc4n6.repeatedID":        "transaction ID is repeated in this file",
	"tc4n6.possibleDuplicate": "possible duplicate; check the existing transaction",
	"tc4n6.clearedInvalid":    "cleared status could not be read",
	"tc4n6.memberIDsInvalid":  "transaction owners could not be read",
	"tc4n6.ambiguousDates":    "%d dates could use either month/day or day/month",
	"tc4n6.ambiguousAmounts":  "%d amounts could use either decimal or thousands commas",
	"tc4n6.rowFieldFailure":   "row %d has an unreadable %s",
	"tc4n6.fieldDate":         "date",
	"tc4n6.fieldAmount":       "amount",
	"tc4n6.fieldDebit":        "debit",
	"tc4n6.fieldCredit":       "credit",
	"tc4n6.fieldBalance":      "balance",
	"tc4n6.profileNoRows":     "No rows matched this profile. Check the column choices.",
	"tc4n6.receiptImported":   "Imported the receipt as one transaction split across %s.",
	"tc4n6.profilePasteFirst": "Paste a statement above, then choose Parse before applying a profile.",
	"tc4n6.profileSaveFailed": "Couldn't save this profile: %s",
	"tc4n6.reminderFailed":    "Couldn't create reminder: %s",
}

func init() {
	for key, value := range tc4n6Keys {
		english[key] = value
	}
}
