// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// AllocProfileFormProps configures the strategy flip-modal body.
type AllocProfileFormProps struct {
	OnDone func() // called to close the modal (Done / Cancel)
}

// AllocProfileForm is the "Adjust strategy" flip-modal body: the split mode + ranking profile,
// the emergency buffer + per-destination cap, the five criterion weights, and save/delete a
// custom profile. It reads and writes the shared alloc:* atoms, so the ranked plan on the main
// surface re-ranks live as the user tunes — and closes via OnDone. Its own component so its many
// input hooks sit at stable positions.
func AllocProfileForm(props AllocProfileFormProps) ui.Node {
	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	_ = currency.Decimals(base)

	profile := uistate.UseAllocProfileSel()
	mode := uistate.UseAllocModeSel()
	reserveAtom := uistate.UseAllocReserveStr()
	maxPerAtom := uistate.UseAllocMaxPerStr()
	wReturns := uistate.UseAllocWReturns()
	wStability := uistate.UseAllocWStability()
	wLiquidity := uistate.UseAllocWLiquidity()
	wDebt := uistate.UseAllocWDebt()
	wGoal := uistate.UseAllocWGoal()
	profName := ui.UseState("")
	profMsg := ui.UseState("")

	setWeights := func(w allocate.Weights) {
		wReturns.Set(trimWeight(w.Returns))
		wStability.Set(trimWeight(w.Stability))
		wLiquidity.Set(trimWeight(w.Liquidity))
		wDebt.Set(trimWeight(w.DebtReduction))
		wGoal.Set(trimWeight(w.GoalProgress))
	}

	onMode := ui.UseEvent(func(e ui.Event) { mode.Set(e.GetValue()) })
	onProfile := ui.UseEvent(func(e ui.Event) {
		sel := e.GetValue()
		profile.Set(sel)
		if app != nil {
			setWeights(allocResolveWeights(app, sel))
		}
		profMsg.Set("")
	})
	onReserve := ui.UseEvent(func(v string) { reserveAtom.Set(v) })
	onMaxPer := ui.UseEvent(func(v string) { maxPerAtom.Set(v) })
	onWReturns := ui.UseEvent(func(v string) { wReturns.Set(v) })
	onWStability := ui.UseEvent(func(v string) { wStability.Set(v) })
	onWLiquidity := ui.UseEvent(func(v string) { wLiquidity.Set(v) })
	onWDebt := ui.UseEvent(func(v string) { wDebt.Set(v) })
	onWGoal := ui.UseEvent(func(v string) { wGoal.Set(v) })
	onProfName := ui.UseEvent(func(v string) { profName.Set(v) })

	saveProfile := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		name := strings.TrimSpace(profName.Get())
		if name == "" {
			profMsg.Set(uistate.T("allocate.profileNameRequired"))
			return
		}
		w := allocate.Weights{
			Returns: parseWeight(wReturns.Get()), Stability: parseWeight(wStability.Get()),
			Liquidity: parseWeight(wLiquidity.Get()), DebtReduction: parseWeight(wDebt.Get()),
			GoalProgress: parseWeight(wGoal.Get()),
		}
		p := domain.AllocationProfile{
			ID: id.New(), Name: name, Returns: w.Returns, Stability: w.Stability,
			Liquidity: w.Liquidity, DebtReduction: w.DebtReduction, GoalProgress: w.GoalProgress,
		}
		if err := app.PutAllocProfile(p); err != nil {
			profMsg.Set(err.Error())
			return
		}
		profName.Set("")
		profile.Set("saved:" + p.ID)
		profMsg.Set(uistate.T("allocate.profileSaved"))
	}))
	deleteProfile := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		sel := profile.Get()
		if !strings.HasPrefix(sel, "saved:") {
			return
		}
		_ = app.DeleteAllocProfile(strings.TrimPrefix(sel, "saved:"))
		profile.Set("balanced")
		setWeights(allocProfiles()["balanced"])
		profMsg.Set("")
	}))
	done := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	var saved []domain.AllocationProfile
	if app != nil {
		saved = app.AllocProfiles()
	}
	profOpts := []any{
		Option(Value("balanced"), SelectedIf(profile.Get() == "balanced"), uistate.T("allocate.balanced")),
		Option(Value("returns"), SelectedIf(profile.Get() == "returns"), uistate.T("allocate.maxReturns")),
		Option(Value("safety"), SelectedIf(profile.Get() == "safety"), uistate.T("allocate.safety")),
		Option(Value("debt"), SelectedIf(profile.Get() == "debt"), uistate.T("allocate.debt")),
		Option(Value("goals"), SelectedIf(profile.Get() == "goals"), uistate.T("allocate.goals")),
	}
	for _, p := range saved {
		key := "saved:" + p.ID
		profOpts = append(profOpts, Option(Value(key), SelectedIf(profile.Get() == key), p.Name))
	}

	return Div(css.Class("alloc-profile-modal"),
		Form(css.Class("alloc-profile-form"), OnSubmit(done),
			Div(css.Class("form-grid"),
				labeledField(uistate.T("allocate.modeLabel"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("allocate.modeLabel")), Attr("data-testid", "allocate-mode"), OnChange(onMode),
						Option(Value("weighted"), SelectedIf(mode.Get() == "weighted"), uistate.T("allocate.modeWeighted")),
						Option(Value("fill"), SelectedIf(mode.Get() == "fill"), uistate.T("allocate.modeFillToTarget")),
					)),
				labeledField(uistate.T("allocate.profileLabel"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("allocate.profileLabel")), OnChange(onProfile), profOpts)),
				labeledField(uistate.T("allocate.reserveFieldLabel"),
					Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("0.01"), Attr("aria-label", "Emergency buffer"),
						Placeholder(uistate.T("allocate.reservePlaceholder", base)), Value(reserveAtom.Get()), OnInput(onReserve))),
				labeledField(uistate.T("allocate.maxPerFieldLabel"),
					Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("0.01"), Attr("aria-label", "Cap per destination"),
						Title(uistate.T("allocate.maxPerTitle")), Placeholder(uistate.T("allocate.maxPerPlaceholder", base)),
						Value(maxPerAtom.Get()), OnInput(onMaxPer))),
			),
			Div(css.Class("alloc-weights"),
				Div(css.Class("alloc-weights-label", tw.TextDim), uistate.T("allocate.weightsLabel")),
				Div(css.Class("form-grid alloc-weights-grid"),
					allocWeightField(uistate.T("allocate.critReturns"), wReturns.Get(), onWReturns),
					allocWeightField(uistate.T("allocate.critStability"), wStability.Get(), onWStability),
					allocWeightField(uistate.T("allocate.critLiquidity"), wLiquidity.Get(), onWLiquidity),
					allocWeightField(uistate.T("allocate.critDebt"), wDebt.Get(), onWDebt),
					allocWeightField(uistate.T("allocate.critGoal"), wGoal.Get(), onWGoal),
				),
				Div(css.Class("alloc-save-profile"),
					Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("allocate.profileNameLabel")),
						Placeholder(uistate.T("allocate.profileNamePlaceholder")), Value(profName.Get()), OnInput(onProfName)),
					Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "allocate-save-profile"), OnClick(saveProfile), uistate.T("allocate.saveProfile")),
					If(len(saved) > 0, Button(css.Class("btn btn-sm"), Type("button"), OnClick(deleteProfile), uistate.T("allocate.deleteProfile"))),
				),
				If(profMsg.Get() != "", P(css.Class("muted"), profMsg.Get())),
			),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "allocate-strategy-done"), uistate.T("allocate.strategyDone")),
			),
		),
	)
}
