// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// accountIssueMessage is the translated message for one problem the account builder reports. The
// builder names the field and the reason; the view turns them into the words a person reads.
func accountIssueMessage(is accountedit.Issue) string {
	if is.Field == accountedit.FieldOwnerShares && is.Reason == accountedit.Total {
		return uistate.T("account.shareSumError", is.Sum)
	}
	if is.Field == accountedit.FieldName && is.Reason == accountedit.Required {
		return uistate.T("accounts.nameRequired")
	}
	if is.Field == accountedit.FieldOpening && is.Reason == accountedit.Invalid {
		return uistate.T("accounts.invalidOpening")
	}
	label := uistate.T(accountFieldLabelKey(is.Field))
	switch is.Reason {
	case accountedit.Required:
		return uistate.T("accounts.fieldRequired", label)
	case accountedit.Range:
		return uistate.T("accounts.fieldRange", label)
	default:
		return uistate.T("accounts.fieldInvalid", label)
	}
}

// accountFieldLabelKey is the i18n key for the label of an account form field.
func accountFieldLabelKey(f accountedit.Field) string {
	switch f {
	case accountedit.FieldName:
		return "accounts.field.name"
	case accountedit.FieldOpening:
		return "accounts.field.opening"
	case accountedit.FieldCreditLimit:
		return "accounts.field.creditLimit"
	case accountedit.FieldAPR:
		return "accounts.field.apr"
	case accountedit.FieldAPY:
		return "accounts.field.apy"
	case accountedit.FieldMinPayment:
		return "accounts.field.minPayment"
	case accountedit.FieldDueDay:
		return "accounts.field.dueDay"
	case accountedit.FieldStatementDay:
		return "accounts.field.statementDay"
	case accountedit.FieldExpectedRet:
		return "accounts.field.expectedReturn"
	case accountedit.FieldLiquidity:
		return "accounts.field.liquidity"
	case accountedit.FieldStability:
		return "accounts.field.stability"
	case accountedit.FieldLockUntil:
		return "accounts.field.lockUntil"
	case accountedit.FieldSnoozeUntil:
		return "accounts.field.snoozeUntil"
	case accountedit.FieldRevalueDays:
		return "accounts.field.revalueDays"
	case accountedit.FieldOwnerShares:
		return "accounts.field.ownerShares"
	}
	return "accounts.field.name"
}
