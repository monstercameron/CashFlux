//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// profileConfigProps carries all state values and event handlers needed to render
// the profile-configuration card in the Allocate screen.
type profileConfigProps struct {
	ProfileValue    string
	ModeValue       string
	AmountStr       string
	ReserveStr      string
	MaxPerStr       string
	Base            string
	WReturns        string
	WStability      string
	WLiquidity      string
	WDebt           string
	WGoal           string
	ProfName        string
	ProfMsg         string
	WeightsOpen     bool
	TotalMinor      int64
	Remainder       int64
	SavedOpts       []ui.Node
	OnMode          any // event handler
	OnProfile       any
	OnAmount        any
	OnReserve       any
	OnMaxPer        any
	OnWReturns      any
	OnWStability    any
	OnWLiquidity    any
	OnWDebt         any
	OnWGoal         any
	OnProfName      any
	OnSaveProfile   any
	OnDeleteProfile any
	OnToggleWeights any
}

// ProfileConfig renders the allocation mode, profile selector, amount inputs,
// and (optionally) the weight-editor disclosure panel. It is a pure rendering
// function — all state is passed in as props, all hooks live in Allocate().
func ProfileConfig(p profileConfigProps) ui.Node {
	return Section(css.Class("card"),
		H2(css.Class("card-title"), uistate.T("allocate.profileTitle")),
		P(css.Class("muted"), uistate.T("allocate.profileDesc")),
		Form(css.Class("form-grid"),
			labeledField(uistate.T("allocate.modeLabel"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("allocate.modeLabel")), Attr("data-testid", "allocate-mode"), OnChange(p.OnMode),
					Option(Value("weighted"), SelectedIf(p.ModeValue == "weighted"), uistate.T("allocate.modeWeighted")),
					Option(Value("fill"), SelectedIf(p.ModeValue == "fill"), uistate.T("allocate.modeFillToTarget")),
				)),
			labeledField(uistate.T("allocate.profileLabel"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("allocate.profileLabel")), OnChange(p.OnProfile),
					Option(Value("balanced"), SelectedIf(p.ProfileValue == "balanced"), uistate.T("allocate.balanced")),
					Option(Value("returns"), SelectedIf(p.ProfileValue == "returns"), uistate.T("allocate.maxReturns")),
					Option(Value("safety"), SelectedIf(p.ProfileValue == "safety"), uistate.T("allocate.safety")),
					Option(Value("debt"), SelectedIf(p.ProfileValue == "debt"), uistate.T("allocate.debt")),
					Option(Value("goals"), SelectedIf(p.ProfileValue == "goals"), uistate.T("allocate.goals")),
					p.SavedOpts,
				)),
			labeledField("Amount to allocate",
				Input(css.Class("field"), Type("number"), Attr("aria-label", "Amount to allocate"), Placeholder(uistate.T("allocate.amountPlaceholder", p.Base)), Value(p.AmountStr), Step("0.01"), OnInput(p.OnAmount))),
			labeledField("Emergency buffer",
				Input(css.Class("field"), Type("number"), Attr("aria-label", "Emergency buffer"), Placeholder(uistate.T("allocate.reservePlaceholder", p.Base)), Value(p.ReserveStr), Step("0.01"), OnInput(p.OnReserve))),
			labeledField("Cap per destination",
				Input(css.Class("field"), Type("number"), Attr("aria-label", "Cap per destination"), Title(uistate.T("allocate.maxPerTitle")), Placeholder(uistate.T("allocate.maxPerPlaceholder", p.Base)), Value(p.MaxPerStr), Step("0.01"), OnInput(p.OnMaxPer))),
		),
		Button(css.Class("btn disclosure-toggle"), Type("button"),
			Attr("aria-expanded", ariaBool(p.WeightsOpen)), Attr("data-testid", "allocate-advanced-toggle"),
			OnClick(p.OnToggleWeights),
			IfElse(p.WeightsOpen, Text(uistate.T("allocate.advancedHide")), Text(uistate.T("allocate.advancedShow")))),
		If(p.WeightsOpen, WeightEditor(weightEditorProps{
			WReturns:        p.WReturns,
			WStability:      p.WStability,
			WLiquidity:      p.WLiquidity,
			WDebt:           p.WDebt,
			WGoal:           p.WGoal,
			ProfName:        p.ProfName,
			ProfileValue:    p.ProfileValue,
			OnWReturns:      p.OnWReturns,
			OnWStability:    p.OnWStability,
			OnWLiquidity:    p.OnWLiquidity,
			OnWDebt:         p.OnWDebt,
			OnWGoal:         p.OnWGoal,
			OnProfName:      p.OnProfName,
			OnSaveProfile:   p.OnSaveProfile,
			OnDeleteProfile: p.OnDeleteProfile,
		})),
		If(p.ProfMsg != "", P(css.Class("muted"), p.ProfMsg)),
		If(p.TotalMinor > 0 && p.Remainder > 0, P(css.Class("muted"), uistate.T("allocate.keptBack", fmtMoney(money.New(p.Remainder, p.Base))))),
	)
}
