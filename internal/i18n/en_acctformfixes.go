// SPDX-License-Identifier: MIT

package i18n

func init() {
	// %d = how many transactions could not be marked cleared.
	english["accounts.reconMarkAllPartial"] = "%d transactions could not be marked cleared. The rest were."
	english["accounts.varNameNoLetters"] = "The variable name needs at least one letter or digit from A to Z. Add one or clear the field to use the account name."
	// %s = the account name.
	english["accounts.editSavedBalanceNot"] = "Your other changes to %s were saved, but the new balance was not recorded. Enter the balance again to record it."
	english["accounts.fieldInvalidGeneric"] = "Check the account details. One of them is not a value the account can use."
	english["acctSweepCfg.unknownAccount"] = "Unknown account"
	english["services.addFailed"] = "Could not add the service. Try again."
	english["services.removeFailed"] = "Could not remove the service. Try again."
	// %s = the service being removed; the accessible name of its Remove button.
	english["services.removeNamed"] = "Remove %s"
	// %s = the reason, such as "no exchange rate for a service's currency".
	english["services.unavailableReason"] = "Unavailable (%s)"
	english["services.reasonNoRate"] = "no exchange rate for a service's currency"
	english["services.reasonOther"] = "the amount could not be calculated"
	english["accounts.field.type"] = "Account type"
	english["accounts.field.owner"] = "Owner"
	english["accounts.field.currency"] = "Currency"
	english["accounts.setBalanceAsOfInvalid"] = "Enter the date the balance was true as a real date."
	english["accounts.documentTypeUnsupported"] = "That kind of file cannot be attached. Use a PDF or an image."
	// Shown in place of an institution whose name is blank.
	english["accounts.institutionUntitled"] = "Untitled institution"
	english["accounts.currencyLocked"] = "This account already has transactions, holdings, recurring bills or goal links, so its currency can no longer be changed."
	english["acctSweep.stale"] = "This suggestion is out of date because your balances or rule changed. The list has been refreshed."
	english["accounts.documentEmpty"] = "That file is empty, so it cannot be attached."
	// %d = the largest file size in megabytes.
	english["accounts.documentTooLarge"] = "That file is too large to attach. The limit is %d MB."
	english["accounts.transferUnknownAccount"] = "One of the accounts in this transfer no longer exists. Reload the page and try again."
	english["accounts.transferNotAllowed"] = "That account can't be used in a transfer. Pick another account."
	english["accounts.documentFailed"] = "The document could not be saved. Try again."
	english["accounts.documentFileMissing"] = "This document's file is missing, so it cannot be opened."
	// %s = the field's name, %d = the lowest allowed figure, %d = the highest.
	english["accounts.fieldRangeBetween"] = "Enter %s from %d to %d."
	english["accounts.documentPopupBlocked"] = "Your browser blocked the document from opening. Allow pop-ups for this site and try again."
}
