// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Customize is the formula-calculator screen — the "build a metric" tool: an
// expression editor, live result, saved formulas, and the available-variables
// reference (C61). It owns a single theme; custom-field definitions live on their
// own /fields screen (CustomFields) so the formula tool and the schema tool no
// longer share one super-screen (themed-remap §5.2/§5.3, item 7).
func Customize() ui.Node {
	if appstate.Default == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	return FormulaCalculator()
}

// CustomFields is the custom-field-definitions screen (/fields): add / edit /
// delete validated custom fields per entity type (Accounts, Transactions,
// Budgets, Goals, Members). Split out of the former Customize super-screen so each
// page owns a single theme — "your data shape" here, "build a metric" on /customize.
func CustomFields() ui.Node {
	if appstate.Default == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	return CustomFieldsManager()
}
