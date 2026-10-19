// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetEditDraftResult carries the explicit results for useBudgetEditDraft.
type budgetEditDraftResult struct {
	ev                 entityVarState
	limitS             ui.State[string]
	periodS            ui.State[string]
	ownerS             ui.State[string]
	rolloverS          ui.State[bool]
	rolloverCapS       ui.State[string]
	methodologyS       ui.State[string]
	catsOpen           state.Atom[string]
	openCats           ui.Handler
	closeCats          ui.Handler
	advEditOpen        ui.State[bool]
	toggleAdvEdit      ui.Handler
	customEditVals     ui.State[map[string]string]
	topupAmt           ui.State[string]
	topupPermanentS    ui.State[bool]
	topupCoverOpenS    ui.State[bool]
	notesS             ui.State[string]
	onNotes            func(v string)
	coverAmtS          ui.State[string]
	coverSelS          ui.State[map[string]bool]
	coverWtS           ui.State[map[string]int]
	coverMaxS          ui.State[map[string]bool]
	amtFxS             ui.State[bool]
	amtFormulaS        ui.State[string]
	wtFxS              ui.State[bool]
	wtFormulaS         ui.State[string]
	recurringS         ui.State[bool]
	coverCustomVals    ui.State[map[string]string]
	errS               ui.State[string]
	targetKindS        ui.State[string]
	targetAmtS         ui.State[string]
	targetDateS        ui.State[string]
	linkedGoalS        ui.State[string]
	onTargetAmt        ui.Handler
	onTargetDate       ui.Handler
	onFillPick         func(major string)
	onLimit            ui.Handler
	onRollover         ui.Handler
	onTopupAmt         ui.Handler
	onToggleTopupCover ui.Handler
	onCoverAmt         ui.Handler
	fullCover          ui.Handler
	toggleAmtFx        ui.Handler
	onAmtFormula       ui.Handler
	toggleWtFx         ui.Handler
	onWtFormula        ui.Handler
	onToggleRecurring  ui.Handler
}

type budgetEditDraftProps struct {
	nameS           ui.State[string]
	b               domain.Budget
	limitMajor      string
	props           BudgetEditFormProps
	coverDefaultStr string
	found           bool
	dec             int
}

// useBudgetEditDraft registers every editor draft hook before mode selection.
func useBudgetEditDraft(p budgetEditDraftProps) budgetEditDraftResult {
	ev := useEntityVarField(budgetVarKind, p.nameS, p.b.VarName)
	limitS := ui.UseState(p.limitMajor)
	periodS := ui.UseState(string(p.b.Period))
	ownerS := ui.UseState(p.b.OwnerID)
	rolloverS := ui.UseState(p.b.Rollover)
	rolloverCapS := ui.UseState(strconv.Itoa(p.b.RolloverCapPeriods)) // BG5: rollover cap, "0" = no cap
	methodologyS := ui.UseState(p.b.Methodology)
	// C1 (one editor per concept): tracked categories are managed ONLY in the dedicated
	// searchable modal, rather than by embedding a second, slightly-different picker here.
	//
	// C521: it opens ON TOP of this form rather than replacing it. Closing this form
	// first destroyed the component, and with it every unsaved edit — so changing the
	// name, stepping into the tracked categories, and coming back lost the name. It
	// also left no way back: the return trip was main page → edit → categories →
	// main page, with the edit form skipped.
	//
	// Layering rather than lifting the draft into atoms is deliberate: the form holds
	// a dozen fields, and a component that never unmounts cannot lose them.
	catsOpen := uistate.UseBudgetCategoriesEdit()
	openCats := ui.UseEvent(Prevent(func() {
		catsOpen.Set(p.props.BudgetID)
	}))
	closeCats := ui.UseEvent(Prevent(func() { catsOpen.Set("") }))
	// C6: the engine-facing variable name is plumbing most edits never touch — it lives
	// behind this disclosure rather than between Name and Limit.
	advEditOpen := ui.UseState(false)
	toggleAdvEdit := ui.UseEvent(Prevent(func() { advEditOpen.Set(!advEditOpen.Get()) }))
	customEditVals := ui.UseState(customMapToStrings(p.b.Custom))
	topupAmt := ui.UseState("")
	// Top-up: whether the raise is permanent (changes the base Limit) or just this period
	// (a one-time boost via PeriodBoosts), and an optional "fund it from other budgets" panel.
	topupPermanentS := ui.UseState(false)
	topupCoverOpenS := ui.UseState(false)
	// Notes mode: the budget's free-text note.
	notesS := ui.UseState(p.b.Notes)
	onNotes := func(v string) { notesS.Set(v) }
	coverAmtS := ui.UseState(p.coverDefaultStr)
	coverSelS := ui.UseState(map[string]bool{})              // sourceID → checked
	coverWtS := ui.UseState(map[string]int{})                // sourceID → ratio weight (default 1)
	coverMaxS := ui.UseState(map[string]bool{})              // sourceID → "use all remaining"
	amtFxS := ui.UseState(recurringAmountFormula(p.b) != "") // amount is a formula, not a number
	amtFormulaS := ui.UseState(recurringAmountFormula(p.b))
	wtFxS := ui.UseState(recurringWeightFormula(p.b) != "") // weights come from a formula, not numbers
	wtFormulaS := ui.UseState(recurringWeightFormula(p.b))
	recurringS := ui.UseState(p.b.RecurringCover != nil)
	coverCustomVals := ui.UseState(customMapToStrings(recurringCoverCustom(p.b))) // "cover" custom fields
	errS := ui.UseState("")

	// BG1 funding-target draft state, seeded from the budget.
	targetAmtSeed := ""
	if p.found && p.b.TargetAmount.Amount > 0 {
		targetAmtSeed = money.FormatMinor(p.b.TargetAmount.Amount, p.dec)
	}
	targetDateSeed := ""
	if !p.b.TargetDate.IsZero() {
		targetDateSeed = dateutil.FormatDate(p.b.TargetDate)
	}
	targetKindS := ui.UseState(string(p.b.TargetKind))
	targetAmtS := ui.UseState(targetAmtSeed)
	targetDateS := ui.UseState(targetDateSeed)
	linkedGoalS := ui.UseState(p.b.LinkedGoalID)
	onTargetAmt := ui.UseEvent(func(v string) { targetAmtS.Set(v) })
	onTargetDate := ui.UseEvent(func(v string) { targetDateS.Set(v) })
	// BG4: a quick-fill chip seeds the amount field.
	onFillPick := func(major string) { limitS.Set(major) }

	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	onRollover := ui.UseEvent(func() { rolloverS.Set(!rolloverS.Get()) })
	onTopupAmt := ui.UseEvent(func(v string) { topupAmt.Set(v) })
	onToggleTopupCover := ui.UseEvent(func() { topupCoverOpenS.Set(!topupCoverOpenS.Get()) })
	onCoverAmt := ui.UseEvent(func(v string) { coverAmtS.Set(v) })
	fullCover := ui.UseEvent(Prevent(func() { coverAmtS.Set(p.coverDefaultStr) }))
	toggleAmtFx := ui.UseEvent(func() { amtFxS.Set(!amtFxS.Get()) })
	onAmtFormula := ui.UseEvent(func(v string) { amtFormulaS.Set(v) })
	toggleWtFx := ui.UseEvent(func() { wtFxS.Set(!wtFxS.Get()) })
	onWtFormula := ui.UseEvent(func(v string) { wtFormulaS.Set(v) })
	onToggleRecurring := ui.UseEvent(func() { recurringS.Set(!recurringS.Get()) })
	return budgetEditDraftResult{
		ev: ev, limitS: limitS, periodS: periodS, ownerS: ownerS, rolloverS: rolloverS, rolloverCapS: rolloverCapS,
		methodologyS: methodologyS, catsOpen: catsOpen, openCats: openCats, closeCats: closeCats, advEditOpen: advEditOpen,
		toggleAdvEdit: toggleAdvEdit, customEditVals: customEditVals, topupAmt: topupAmt, topupPermanentS: topupPermanentS,
		topupCoverOpenS: topupCoverOpenS, notesS: notesS, onNotes: onNotes, coverAmtS: coverAmtS, coverSelS: coverSelS, coverWtS: coverWtS,
		coverMaxS: coverMaxS, amtFxS: amtFxS, amtFormulaS: amtFormulaS, wtFxS: wtFxS, wtFormulaS: wtFormulaS, recurringS: recurringS,
		coverCustomVals: coverCustomVals, errS: errS, targetKindS: targetKindS, targetAmtS: targetAmtS, targetDateS: targetDateS,
		linkedGoalS: linkedGoalS, onTargetAmt: onTargetAmt, onTargetDate: onTargetDate, onFillPick: onFillPick, onLimit: onLimit,
		onRollover: onRollover, onTopupAmt: onTopupAmt, onToggleTopupCover: onToggleTopupCover, onCoverAmt: onCoverAmt,
		fullCover: fullCover, toggleAmtFx: toggleAmtFx, onAmtFormula: onAmtFormula, toggleWtFx: toggleWtFx, onWtFormula: onWtFormula,
		onToggleRecurring: onToggleRecurring,
	}
}

// budgetEditSourceCallbacks builds the edit source callbacks section.
func budgetEditSourceCallbacks(coverSelS ui.State[map[string]bool], coverWtS ui.State[map[string]int], coverMaxS ui.State[map[string]bool]) (func(sid string), func(sid string, val int), func(sid string)) {
	// Plain funcs (not hooks) passed down to each per-source row component, so the
	// checkbox/weight On* handlers live in the row, not in this loop.
	onToggleSrc := func(sid string) {
		sel := coverSelS.Get()
		ns := make(map[string]bool, len(sel)+1)
		for k, v := range sel {
			ns[k] = v
		}
		ns[sid] = !ns[sid]
		coverSelS.Set(ns)
		if ns[sid] { // default a newly-checked source to weight 1 (equal split)
			w := coverWtS.Get()
			if w[sid] == 0 {
				nw := make(map[string]int, len(w)+1)
				for k, v := range w {
					nw[k] = v
				}
				nw[sid] = 1
				coverWtS.Set(nw)
			}
		}
	}
	onWeightSrc := func(sid string, val int) {
		if val < 0 {
			val = 0
		}
		w := coverWtS.Get()
		nw := make(map[string]int, len(w)+1)
		for k, v := range w {
			nw[k] = v
		}
		nw[sid] = val
		coverWtS.Set(nw)
	}
	onToggleMax := func(sid string) {
		m := coverMaxS.Get()
		nm := make(map[string]bool, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[sid] = !nm[sid]
		coverMaxS.Set(nm)
	}

	return onToggleSrc, onWeightSrc, onToggleMax
}

// useBudgetEditCustomCallbacks registers cancel and supplies custom-field draft callbacks.
func useBudgetEditCustomCallbacks(done func(), customEditVals ui.State[map[string]string], coverCustomVals ui.State[map[string]string]) (ui.Handler, func(key string, value string), func(key string, value string)) {
	cancel := ui.UseEvent(Prevent(func() { done() }))
	onCustom := func(key, value string) {
		m := customEditVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, val := range m {
			nm[k] = val
		}
		nm[key] = value
		customEditVals.Set(nm)
	}
	onCoverCustom := func(key, value string) {
		m := coverCustomVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, val := range m {
			nm[k] = val
		}
		nm[key] = value
		coverCustomVals.Set(nm)
	}

	return cancel, onCustom, onCoverCustom
}
