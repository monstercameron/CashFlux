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
// reference (C61). Custom-field definitions live on their own /fields screen
// (Fields/CustomFields in fields.go) so the formula tool and the schema tool no
// longer share one super-screen (FEATURE_MAP §5.3).
func Customize() ui.Node {
	if appstate.Default == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	return FormulaCalculator()
}
