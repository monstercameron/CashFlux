// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"log/slog"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// acctWriteErrorText turns an error from an account write into a sentence worth putting on screen.
// A validate.Issues names the field and what it needs, and the errors appstate exports for this
// page each have their own wording. Anything else came from a layer that describes machines, so the
// person gets a plain sentence and the detail stays in the log.
func acctWriteErrorText(err error) string {
	var issues validate.Issues
	switch {
	case errors.As(err, &issues) && !issues.OK():
		return acctIssueText(issues)
	case errors.Is(err, appstate.ErrReadOnly):
		return uistate.T("accounts.writeReadOnly")
	case errors.Is(err, appstate.ErrAccountHasTransactions):
		return uistate.T("accounts.deleteHasTxns")
	case errors.Is(err, appstate.ErrAccountNotFound):
		return uistate.T("accounts.writeNotFound")
	case errors.Is(err, appstate.ErrAccountCurrencyLocked):
		return uistate.T("accounts.currencyLocked")
	case errors.Is(err, appstate.ErrTransferRateUnavailable):
		return uistate.T("accounts.writeRateUnavailable")
	case errors.Is(err, appstate.ErrStaleDataset):
		return uistate.T("accounts.writeStale")
	case errors.Is(err, appstate.ErrDurabilityUnavailable):
		return uistate.T("accounts.writeNotReady")
	case errors.Is(err, appstate.ErrSweepProposalStale):
		return uistate.T("acctSweep.stale")
	case errors.Is(err, appstate.ErrSweepPostedUnmarked):
		return uistate.T("acctSweep.postedUnmarked")
	default:
		return uistate.T("accounts.writeFailed")
	}
}

// acctIssueKeys maps the field a validate issue names to the sentence that tells the person what
// to fix. A field with no entry gets the generic account-details sentence, so a field code is never
// shown.
var acctIssueKeys = map[string]string{
	"name":            "accounts.issue.name",
	"ownerId":         "accounts.issue.owner",
	"openingBalance":  "accounts.issue.openingCurrency",
	"liquidityScore":  "accounts.issue.score",
	"stabilityScore":  "accounts.issue.score",
	"dueDayOfMonth":   "accounts.issue.dueDay",
	"statementDay":    "accounts.issue.statementDay",
	"interestRateApr": "accounts.issue.apr",
}

// acctIssueText words the first validation issue as a plain sentence.
func acctIssueText(issues validate.Issues) string {
	if key, ok := acctIssueKeys[issues[0].Field]; ok {
		return uistate.T(key)
	}
	return uistate.T("accounts.issue.generic")
}

// postAcctWriteError logs a rejected account write under op and tells the person why in a notice.
func postAcctWriteError(op string, err error) {
	slog.Error("accounts: write rejected", "op", op, "err", err)
	uistate.PostNotice(acctWriteErrorText(err), true)
}
