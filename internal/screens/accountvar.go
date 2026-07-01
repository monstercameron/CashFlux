// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// accountVarPlaceholder is the auto-derived variable slug for a name, shown as the
// account var-name field's placeholder so the user sees what they'd get by leaving it
// blank.
func accountVarPlaceholder(name string) string {
	if s := engineenv.AccountVarSlug(name); s != "" {
		return s
	}
	return uistate.T("accounts.varNamePlaceholder")
}

// accountVarBase is the base handle an account exposes ("account_<slug>"), from the
// explicit var name when set else the display name.
func accountVarBase(varName, name string) string {
	src := varName
	if src == "" {
		src = name
	}
	slug := engineenv.AccountVarSlug(src)
	if slug == "" {
		slug = "…"
	}
	return "account_" + slug
}

// accountVarCollision returns a warning when the resolved variable slug for (varName
// else name) clashes with another account's variable — so two accounts can't silently
// produce the same handle. Empty when there's no clash.
func accountVarCollision(accounts []domain.Account, selfID, varName, name string) string {
	src := varName
	if src == "" {
		src = name
	}
	slug := engineenv.AccountVarSlug(src)
	if slug == "" {
		return ""
	}
	for _, aa := range accounts {
		if aa.ID == selfID {
			continue
		}
		other := aa.VarName
		if other == "" {
			other = aa.Name
		}
		if engineenv.AccountVarSlug(other) == slug {
			return uistate.T("accounts.varNameTaken", aa.Name)
		}
	}
	return ""
}

// accountVarField renders the shared "Variable name" editor for accounts: the input, a
// live chip showing the exact variable generated (account_<slug>), the fields it
// exposes, and a collision warning.
func accountVarField(accounts []domain.Account, selfID, inputID, warnTestID, varName, name string, onInput ui.Handler) ui.Node {
	base := accountVarBase(varName, name)
	warn := accountVarCollision(accounts, selfID, varName, name)
	return Div(css.Class("budget-var-block"),
		Input(css.Class("field"), Attr("id", inputID), Type("text"),
			Placeholder(accountVarPlaceholder(name)), Value(varName), OnInput(onInput)),
		Div(css.Class("budget-var-preview"),
			Span(css.Class("budget-var-preview-lead"), uistate.T("budgets.varNameGenerates")),
			Span(ClassStr("budget-var-chip"), base+"_balance"),
			Span(css.Class("budget-var-preview-fields"), uistate.T("accounts.varNameFields")),
		),
		If(warn != "", Span(css.Class("cover-fx-err"), Attr("data-testid", warnTestID), warn)),
	)
}
