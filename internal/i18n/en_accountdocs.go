// SPDX-License-Identifier: MIT

package i18n

// accountDocsKeys holds the English strings for the account-documents drawer that
// sit beside the keys in accountsFilingKeys: the joined date line and the attach errors.
var accountDocsKeys = Catalog{
	// %s (1) = the filed date line, %s (2) = the renewal date line.
	"accounts.documentMeta": "%s · %s",
	// Shown when the renewal date typed on the attach form is not a calendar date.
	"accounts.documentExpiryInvalid": "That renewal date isn't a valid date. Pick one from the calendar, or leave it blank.",
	// Shown when the account was removed before the document could be filed.
	"accounts.documentAccountMissing": "This account no longer exists, so the document was not attached.",
	// Shown when the document is no longer filed on the account.
	"accounts.documentMissing": "This document is no longer filed on the account.",
}

func init() {
	for k, v := range accountDocsKeys {
		english[k] = v
	}
}
