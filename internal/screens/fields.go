// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Fields is the custom-field-definitions screen (/fields): add / edit / delete
// validated custom fields per entity type (Accounts, Transactions, Budgets,
// Goals, Members). Split out of the former Customize super-screen so each page
// owns a single theme — "your data shape" here, "build a metric" on /customize
// (FEATURE_MAP §5.3).
func Fields() ui.Node {
	if appstate.Default == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	return CustomFieldsManager()
}

// CustomFields is the registered view for the /fields route in screens.go.
// It delegates to Fields so the existing route registration continues to work
// without modifying screens.go (a shared file not owned by this commit).
func CustomFields() ui.Node { return Fields() }
