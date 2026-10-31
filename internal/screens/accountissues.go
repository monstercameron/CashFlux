// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"slices"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// rateStep is the step of a percentage-rate box (APR, APY, expected return). It is fine enough for
// a rate such as 6.125%.
const rateStep = "0.001"

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
	if is.Field == accountedit.FieldVarName && is.Reason == accountedit.Invalid {
		return uistate.T("accounts.varNameNoLetters")
	}
	key, known := accountFieldLabelKey(is.Field)
	if !known {
		return uistate.T("accounts.fieldInvalidGeneric")
	}
	label := uistate.T(key)
	switch is.Reason {
	case accountedit.Required:
		return uistate.T("accounts.fieldRequired", label)
	case accountedit.Range:
		if lo, hi, ok := accountedit.RangeOf(is.Field); ok {
			return uistate.T("accounts.fieldRangeBetween", label, lo, hi)
		}
		return uistate.T("accounts.fieldRange", label)
	default:
		return uistate.T("accounts.fieldInvalid", label)
	}
}

// accountFieldLabelKey is the i18n key for the label of an account form field. known is false for a
// field the form has no label for, so the caller shows the generic message instead of naming the
// wrong field.
func accountFieldLabelKey(f accountedit.Field) (key string, known bool) {
	switch f {
	case accountedit.FieldName:
		return "accounts.field.name", true
	case accountedit.FieldType:
		return "accounts.field.type", true
	case accountedit.FieldOwner:
		return "accounts.field.owner", true
	case accountedit.FieldCurrency:
		return "accounts.field.currency", true
	case accountedit.FieldOpening:
		return "accounts.field.opening", true
	case accountedit.FieldCreditLimit:
		return "accounts.field.creditLimit", true
	case accountedit.FieldAPR:
		return "accounts.field.apr", true
	case accountedit.FieldAPY:
		return "accounts.field.apy", true
	case accountedit.FieldMinPayment:
		return "accounts.field.minPayment", true
	case accountedit.FieldDueDay:
		return "accounts.field.dueDay", true
	case accountedit.FieldStatementDay:
		return "accounts.field.statementDay", true
	case accountedit.FieldExpectedRet:
		return "accounts.field.expectedReturn", true
	case accountedit.FieldLiquidity:
		return "accounts.field.liquidity", true
	case accountedit.FieldStability:
		return "accounts.field.stability", true
	case accountedit.FieldLockUntil:
		return "accounts.field.lockUntil", true
	case accountedit.FieldSnoozeUntil:
		return "accounts.field.snoozeUntil", true
	case accountedit.FieldRevalueDays:
		return "accounts.field.revalueDays", true
	case accountedit.FieldOwnerShares:
		return "accounts.field.ownerShares", true
	case accountedit.FieldVarName:
		return "accounts.varNameLabel", true
	}
	return "", false
}

// accountAdvancedField reports whether the field sits in the forms' Advanced section, which is
// closed until the person opens it.
func accountAdvancedField(f accountedit.Field) bool {
	switch f {
	case accountedit.FieldAPY, accountedit.FieldExpectedRet, accountedit.FieldLiquidity, accountedit.FieldStability,
		accountedit.FieldLockUntil, accountedit.FieldSnoozeUntil, accountedit.FieldRevalueDays, accountedit.FieldVarName:
		return true
	}
	return false
}

// accountFieldInputID is the id of the input for a field that has one (every field of the builder
// except the name, the type, the owner, the currency and the share split, which the forms name
// themselves or build without an id), and "" for the rest.
func accountFieldInputID(prefix string, f accountedit.Field) string {
	switch f {
	case accountedit.FieldOpening, accountedit.FieldCreditLimit, accountedit.FieldAPR, accountedit.FieldAPY,
		accountedit.FieldMinPayment, accountedit.FieldDueDay, accountedit.FieldStatementDay,
		accountedit.FieldExpectedRet, accountedit.FieldLiquidity, accountedit.FieldStability,
		accountedit.FieldLockUntil, accountedit.FieldSnoozeUntil, accountedit.FieldRevalueDays:
		return prefix + "-" + string(f)
	}
	return ""
}

// accountInputIDs maps a field to the id of its input in a form whose inputs share the prefix and
// whose name input has the id nameID.
func accountInputIDs(prefix, nameID string) func(accountedit.Field) string {
	return func(f accountedit.Field) string {
		if f == accountedit.FieldName {
			return nameID
		}
		return accountFieldInputID(prefix, f)
	}
}

// formProblem is what is wrong with a form after a refused save: the sentence to show, the inputs
// that need fixing (the first is where focus goes), and whether any of them is in the closed
// Advanced section. The zero value means nothing is wrong.
type formProblem struct {
	Message       string
	Inputs        []string
	NeedsAdvanced bool
	// Unplaced is set when part of the message belongs to no single input (a share total, a custom
	// value).
	Unplaced bool
}

// problemOfIssues builds the problem for the issues the account builder reported. inputID maps a
// field to its input's id, or "" when the field has no input to move to.
func problemOfIssues(issues []accountedit.Issue, inputID func(accountedit.Field) string) formProblem {
	if len(issues) == 0 {
		return formProblem{}
	}
	messages := make([]string, 0, len(issues))
	var p formProblem
	for _, is := range issues {
		messages = append(messages, accountIssueMessage(is))
		if id := inputID(is.Field); id != "" {
			p.Inputs = append(p.Inputs, id)
		} else {
			p.Unplaced = true
		}
		if accountAdvancedField(is.Field) {
			p.NeedsAdvanced = true
		}
	}
	p.Message = strings.Join(messages, " ")
	return p
}

// And returns the problem with one more thing wrong: its message follows the existing ones, and the
// input, when there is one, needs fixing too. advanced says that input is in the Advanced section.
func (p formProblem) And(message, inputID string, advanced bool) formProblem {
	if message == "" {
		return p
	}
	if p.Message != "" {
		p.Message += " "
	}
	p.Message += message
	if inputID != "" {
		p.Inputs = append(slices.Clone(p.Inputs), inputID)
	} else {
		p.Unplaced = true
	}
	p.NeedsAdvanced = p.NeedsAdvanced || advanced
	return p
}

// Any reports whether something is wrong.
func (p formProblem) Any() bool { return p.Message != "" }

// Has reports whether the input needs fixing.
func (p formProblem) Has(inputID string) bool {
	for _, id := range p.Inputs {
		if id == inputID {
			return true
		}
	}
	return false
}

// First is the input to move to, or "" when the problem belongs to no single input.
func (p formProblem) First() string {
	if len(p.Inputs) == 0 {
		return ""
	}
	return p.Inputs[0]
}

// memberIDsOf lists the ids of the members, never nil, so the account builder checks owners and
// shares against exactly the members that exist.
func memberIDsOf(members []domain.Member) []string {
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	return ids
}

// accountCheck is what a save check needs besides the fields: the existing accounts (for the
// variable-name clash), the account being edited (SelfID, blank on add), the custom field
// definitions and typed values, and where to move to for each problem.
type accountCheck struct {
	SelfID     string
	Accounts   []domain.Account
	Defs       []customfields.Def
	Custom     map[string]string
	IDs        func(accountedit.Field) string
	VarInputID string
}

// checkAccountForm checks the fields the way a save does, for the add and the edit form alike. It
// returns the account the fields build, with its custom values, and everything that is wrong: what
// the account builder refuses, a new variable name another account already uses, and a custom value
// that does not validate. The account is only fit to save when the problem is empty.
func checkAccountForm(base domain.Account, f accountedit.Fields, in accountCheck) (domain.Account, formProblem) {
	acc, issues := accountedit.Apply(base, f)
	problem := problemOfIssues(issues, func(field accountedit.Field) string {
		if field == accountedit.FieldVarName {
			return in.VarInputID
		}
		return in.IDs(field)
	})
	// A handle the account already had is not checked again, so two accounts saved before the rule
	// existed with the same handle can still each have their balance updated.
	handleChanged := strings.TrimSpace(f.VarName) != strings.TrimSpace(base.VarName) || strings.TrimSpace(f.Name) != strings.TrimSpace(base.Name)
	if handleChanged {
		if warn := entityVarCollision(accountVarKind, accountVarEntities(in.Accounts), in.SelfID, f.VarName, f.Name); warn != "" {
			problem = problem.And(warn, in.VarInputID, true)
		}
	}
	if len(in.Defs) > 0 {
		custom, customIssues := customValuesToMap(in.Defs, in.Custom)
		if len(customIssues) > 0 {
			problem = problem.And(strings.Join(customIssues, " "), "", false)
		} else {
			acc.Custom = custom
		}
	}
	return acc, problem
}

// fieldInvalidName is the "field-invalid" danger-outline class when the problem names the input and ""
// otherwise. The caller passes it as an argument of the input's single css.Class call, because an
// element takes one class option and a second one would replace the first.
func fieldInvalidName(p formProblem, inputID string) string {
	return condName(inputID != "" && p.Has(inputID), "field-invalid")
}

// fieldInvalidAttrs marks an input the problem names with aria-invalid and a pointer to the error
// line; the danger outline comes from fieldInvalidName. It is nil for any other input. It registers
// no hooks, so it is safe to spread into an input that is built conditionally.
func fieldInvalidAttrs(p formProblem, inputID, errID string) []any {
	if inputID == "" || !p.Has(inputID) {
		return nil
	}
	return []any{Attr("aria-invalid", "true"), Attr("aria-describedby", errID)}
}

// focusRequest asks the form to move the person to what needs fixing. Seq changes with every
// request, so asking twice for the same input still moves focus the second time.
type focusRequest struct {
	InputID string
	Seq     int
}

// useProblemFocus returns the function a refused save calls to move the person to what needs
// fixing: the input when the problem has one, otherwise the error line, scrolled into view. The move
// happens after the form has rendered, so an input in a section that the save has just opened is
// there to receive it. Call it once, unconditionally, with the id of the form's error line.
func useProblemFocus(errID string) func(inputID string) {
	focus := ui.UseFocusManager()
	req := ui.UseState(focusRequest{})
	current := req.Get()
	ui.UseEffect(func() func() {
		if current.Seq > 0 {
			showFormProblem(focus, current.InputID, errID)
		}
		return nil
	}, current)
	return func(inputID string) {
		req.Set(focusRequest{InputID: inputID, Seq: req.Get().Seq + 1})
	}
}

// showFormProblem focuses the input, or scrolls the error line into view when there is no input.
func showFormProblem(focus ui.FocusManager, inputID, errID string) {
	if inputID != "" && focus.FocusByID(inputID) {
		return
	}
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	el := doc.Call("getElementById", errID)
	if el.Truthy() && el.Get("scrollIntoView").Type() == js.TypeFunction {
		el.Call("scrollIntoView", map[string]any{"block": "nearest"})
	}
}
