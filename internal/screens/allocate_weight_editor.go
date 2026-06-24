// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// weightEditorProps carries the weight values and callbacks for the advanced
// criterion-weight editor inside the Allocate profile card.
type weightEditorProps struct {
	WReturns        string
	WStability      string
	WLiquidity      string
	WDebt           string
	WGoal           string
	ProfName        string
	ProfileValue    string // used to show the delete-profile button
	OnWReturns      any
	OnWStability    any
	OnWLiquidity    any
	OnWDebt         any
	OnWGoal         any
	OnProfName      any
	OnSaveProfile   any // form OnSubmit handler
	OnDeleteProfile any
}

// WeightEditor renders the five criterion weight inputs and the save/delete
// profile form. It is a pure rendering function — all hooks live in Allocate().
func WeightEditor(p weightEditorProps) ui.Node {
	return Div(
		P(css.Class("set-label"), uistate.T("allocate.weightsTitle")),
		Form(css.Class("form-grid"),
			Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wReturns")),
				Input(css.Class("field"), Type("number"), Value(p.WReturns), Step("0.5"), OnInput(p.OnWReturns))),
			Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wStability")),
				Input(css.Class("field"), Type("number"), Value(p.WStability), Step("0.5"), OnInput(p.OnWStability))),
			Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wLiquidity")),
				Input(css.Class("field"), Type("number"), Value(p.WLiquidity), Step("0.5"), OnInput(p.OnWLiquidity))),
			Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wDebt")),
				Input(css.Class("field"), Type("number"), Value(p.WDebt), Step("0.5"), OnInput(p.OnWDebt))),
			Label(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Span(css.Class("muted", tw.Text11), uistate.T("allocate.wGoal")),
				Input(css.Class("field"), Type("number"), Value(p.WGoal), Step("0.5"), OnInput(p.OnWGoal))),
		),
		Form(css.Class("form-grid"), OnSubmit(p.OnSaveProfile),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("allocate.profileNamePlaceholder")), Value(p.ProfName), OnInput(p.OnProfName)),
			Button(css.Class("btn btn-primary fit"), Type("submit"), uistate.T("allocate.saveProfile")),
			If(strings.HasPrefix(p.ProfileValue, "saved:"), Button(css.Class("btn"), Type("button"), OnClick(p.OnDeleteProfile), uistate.T("allocate.deleteProfile"))),
		),
	)
}
