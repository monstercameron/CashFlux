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

// Customize is the Customize super-screen. It composes two focused sub-components:
//
//   - FormulaCalculator — the expression editor, live result, saved formulas, and
//     available-variables reference (C61). Featured above the fold as the power-user tool.
//   - CustomFieldsManager — add/edit/delete custom-field definitions per entity type.
//     Placed below a section divider so the two-tool structure of the page is legible
//     rather than one flat 7-card stream (G15 §1).
func Customize() ui.Node {
	if appstate.Default == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	return Div(
		// Lead with the formula calculator (G15 §1): it's the featured power-user tool.
		FormulaCalculator(),
		// Custom fields are a separate, advanced tool — divider makes the two-tool
		// structure of the page legible instead of one flat 7-card stream (G15 §1).
		H3(css.Class("section-divider"), uistate.T("customize.customFieldsSection")),
		CustomFieldsManager(),
	)
}
