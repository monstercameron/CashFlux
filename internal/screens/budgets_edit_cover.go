// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetEditCoverProps carries the explicit inputs for budgetEditCover.
type budgetEditCoverProps struct {
	coverSrcs         []budgetCoverSource
	coverIncomplete   bool
	app               *appstate.App
	pr                prefs.Prefs
	coverPayAnchor    time.Time
	amtFxS            ui.State[bool]
	amtFormulaS       ui.State[string]
	b                 domain.Budget
	cur               string
	coverAmtS         ui.State[string]
	dec               int
	coverWtS          ui.State[map[string]int]
	wtFxS             ui.State[bool]
	coverSelS         ui.State[map[string]bool]
	wtFormulaS        ui.State[string]
	coverMaxS         ui.State[map[string]bool]
	onToggleSrc       func(sid string)
	onWeightSrc       func(sid string, val int)
	onToggleMax       func(sid string)
	submitCover       ui.Handler
	coverShortfallStr string
	onAmtFormula      ui.Handler
	onCoverAmt        ui.Handler
	coverDefaultStr   string
	fullCover         ui.Handler
	toggleAmtFx       ui.Handler
	toggleWtFx        ui.Handler
	onWtFormula       ui.Handler
	onToggleRecurring ui.Handler
	recurringS        ui.State[bool]
	coverCustomVals   ui.State[map[string]string]
	onCoverCustom     func(key string, value string)
	errLine           ui.Node
	cancel            ui.Handler
}

// budgetEditCover builds the edit cover section.
func budgetEditCover(p budgetEditCoverProps) ui.Node {
	if len(p.coverSrcs) == 0 {
		return Div(css.Class("acct-edit-form"),
			If(p.coverIncomplete, P(css.Class("err"), Attr("role", "status"), Attr("data-testid", "budget-cover-incomplete"), uistate.T("budgets.committedIncomplete"))),
			P(css.Class("empty"), uistate.T("budgets.coverNoSources")))
	}
	preview := budgetEditCoverPreview(p)
	return Form(css.Class("acct-edit-form", "cover-form"), OnSubmit(p.submitCover),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("budgets.coverHint", p.coverShortfallStr)),
			// C597: covering moves LIMIT between budgets, permanently — a very
			// different act from a one-month top-up, and the form never said so.
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cover-impact"),
				Style(map[string]string{"margin": "0"}), fundsImpactLine(budgeting.ImpactCover)),
			// Amount: a number or a formula (ƒx toggle). A formula is evaluated live in
			// this budget's context and re-evaluated each period when recurring.
			labeledField(uistate.T("budgets.amountToMove"),
				Div(css.Class("cover-amount-block"),
					Div(css.Class("cover-amount-row"),
						IfElse(p.amtFxS.Get(),
							Input(css.Class("field"), Attr("id", "budget-cover-formula"), Type("text"),
								Placeholder("overspend"), OnInput(p.onAmtFormula), uiw.FieldValue(p.amtFormulaS.Get())),
							Input(css.Class("field"), Attr("id", "budget-cover-amt"), Attr("autofocus", ""), Type("number"),
								Attr("aria-label", uistate.T("budgets.amountToMove")), Placeholder(uistate.T("budgets.amountToMove")), Step("0.01"), Attr("min", "0.01"), OnInput(p.onCoverAmt), uiw.FieldValue(p.coverAmtS.Get()))),
						If(!p.amtFxS.Get() && p.coverDefaultStr != "", Button(css.Class("btn"), Type("button"), Title(uistate.T("budgets.fullOverspendTitle")),
							OnClick(p.fullCover), uistate.T("budgets.coverFull", p.coverShortfallStr))),
						Button(css.Class("btn", "cover-fx-toggle"), Type("button"), Attr("aria-pressed", ariaBool(p.amtFxS.Get())),
							Attr("data-testid", "cover-fx-toggle"), Title(uistate.T("budgets.coverFormulaTitle")), OnClick(p.toggleAmtFx), "ƒx"),
					),
					If(p.amtFxS.Get() && preview.amtFxErr == "", Span(css.Class("cover-fx-preview"), Attr("data-testid", "cover-fx-preview"), uistate.T("budgets.coverFormulaPreview", preview.amtPreview))),
					If(preview.amtFxErr != "", Span(css.Class("cover-fx-err"), uistate.T("budgets.coverFormulaErr", preview.amtFxErr))),
					If(p.amtFxS.Get(), Span(css.Class("cover-fx-hint"), uistate.T("budgets.coverFormulaHint"))),
				)),
			// Multi-source: check the budgets to pull from (split equally by default);
			// the ratio input per source lets the split be unequal — or drive every
			// source's ratio from a single formula (ƒx) evaluated in each source's context.
			Div(css.Class("cover-spread-head"),
				Div(css.Class("cover-spread-titles"),
					Span(css.Class("cover-spread-label"), uistate.T("budgets.coverSpreadLabel")),
					Span(css.Class("cover-spread-sub"), preview.spreadSub),
				),
				// The "ƒx ratios" toggle only appears once a source is picked (it weights the
				// selected sources), so the default view has just one ƒx — the amount one —
				// instead of two identical buttons. Labelled to read as the ratio control.
				If(preview.selCount > 0, Button(css.Class("btn", "cover-fx-toggle", "cover-fx-ratio"), Type("button"), Attr("aria-pressed", ariaBool(p.wtFxS.Get())),
					Attr("data-testid", "cover-wt-fx-toggle"), Title(uistate.T("budgets.coverWeightFxTitle")), OnClick(p.toggleWtFx),
					Span(css.Class("cover-fx-ratio-label"), uistate.T("budgets.coverWeightFxBtn")), "ƒx")),
			),
			If(p.wtFxS.Get() && preview.selCount > 0, Div(css.Class("cover-weight-fx"),
				Input(css.Class("field"), Attr("id", "budget-cover-wt-formula"), Type("text"),
					Placeholder("cf_budget_priority"), OnInput(p.onWtFormula), uiw.FieldValue(p.wtFormulaS.Get())),
				If(preview.wtFxErr != "", Span(css.Class("cover-fx-err"), uistate.T("budgets.coverFormulaErr", preview.wtFxErr))),
				Span(css.Class("cover-fx-hint"), uistate.T("budgets.coverWeightFxHint")),
			)),
			// A running "Selected N · splitting $X" caption pinned above the list, so the
			// split total reads at a glance even as rows scroll. Kept always-present (hidden
			// when empty) so toggling it never restructures the siblings and forces the keyed
			// list below to remount — which would detach a row mid-click.
			Span(ClassStr(coverCapClass(preview.selCount)), coverCapText(preview.selCount, preview.selTotal, p.cur)),
			// One keyed list, sorted so the picked budgets cluster at the top; the CSS tints
			// the checked group and rules a divider before the remaining sources.
			Div(css.Class("cover-sources"),
				MapKeyed(preview.sortedCoverSrcs, preview.srcKey, preview.renderSrc),
			),
			// Repeat this cover automatically at the start of each new period.
			Label(css.Class("cover-recurring-toggle"),
				Input(append([]any{Type("checkbox"), Attr("data-testid", "cover-recurring"), OnChange(p.onToggleRecurring)}, checkedAttr(p.recurringS.Get())...)...),
				Span(uistate.T("budgets.coverRecurring")),
			),
			// Custom fields on the standing rule (metadata like a reason / review-by), shown
			// only when recurring is on and the household has defined "cover" fields.
			If(p.recurringS.Get() && len(p.app.CustomFieldDefsFor("cover")) > 0,
				Div(css.Class("cover-custom-fields"),
					P(css.Class("cover-fx-hint"), uistate.T("budgets.coverCustomHint")),
					MapKeyed(p.app.CustomFieldDefsFor("cover"), func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
						return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: p.coverCustomVals.Get()[d.Key], OnChange: p.onCoverCustom}))
					}),
				)),
			p.errLine,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(p.cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("budgets.coverAction")),
		),
	)
}

type budgetEditCoverPreviewResult struct {
	amtPreview      string
	amtFxErr        string
	wtFxErr         string
	spreadSub       string
	selCount        int
	selTotal        int64
	sortedCoverSrcs []budgetCoverSource
	renderSrc       func(budgetCoverSource) ui.Node
	srcKey          func(budgetCoverSource) any
}

// budgetEditCoverPreview resolves the current amount, weights, and keyed source rows.
func budgetEditCoverPreview(p budgetEditCoverProps) budgetEditCoverPreviewResult {
	// Effective amount + live preview: in formula mode evaluate the amount formula in
	// this budget's context; otherwise parse the number. The shares below use it.
	coverCtx := buildCoverContext(p.app, p.pr.WeekStartWeekday(), p.coverPayAnchor)
	var amtMinor int64
	amtPreview, amtFxErr := "—", ""
	if p.amtFxS.Get() {
		if strings.TrimSpace(p.amtFormulaS.Get()) != "" {
			if m, ferr := coverCtx.AmountMinor(p.amtFormulaS.Get(), p.b); ferr != nil {
				amtFxErr = ferr.Error()
			} else {
				amtMinor, amtPreview = m, fmtMoney(money.New(m, p.cur))
			}
		}
	} else {
		amtMinor, _ = money.ParseMinor(strings.TrimSpace(p.coverAmtS.Get()), p.dec)
	}
	// Effective ratio weights: a per-source formula (evaluated in each source's own
	// context) or the fixed ratio inputs. The shares + the displayed ratios use these.
	effWeights := p.coverWtS.Get()
	wtFxErr := ""
	if p.wtFxS.Get() {
		effWeights, wtFxErr = resolveCoverWeights(p.app, coverCtx, p.coverSrcs, p.coverSelS.Get(), p.coverWtS.Get(), p.wtFormulaS.Get())
	}
	shares := splitCoverAmount(amtMinor, p.coverSrcs, p.coverSelS.Get(), effWeights, p.coverMaxS.Get())
	spreadSub := uistate.T("budgets.coverSpreadSub")
	if p.wtFxS.Get() {
		spreadSub = uistate.T("budgets.coverWeightFxSub")
	}
	// Keyed single list sorted checked-first, so the picked budgets cluster at the top
	// (the CSS tints them into one visible group and rules a divider before the rest).
	// A single keyed list — rather than two — keeps each row's DOM node across a toggle,
	// so a click never detaches the element being interacted with.
	selNow := p.coverSelS.Get()
	var selCount int
	var selTotal int64
	for _, sc := range p.coverSrcs {
		if selNow[sc.ID] {
			selCount++
			selTotal += shares[sc.ID]
		}
	}
	sortedCoverSrcs := make([]budgetCoverSource, len(p.coverSrcs))
	copy(sortedCoverSrcs, p.coverSrcs)
	sort.SliceStable(sortedCoverSrcs, func(i, j int) bool {
		return selNow[sortedCoverSrcs[i].ID] && !selNow[sortedCoverSrcs[j].ID]
	})
	renderSrc := func(sc budgetCoverSource) ui.Node {
		shareStr, over := "", false
		if sh, ok := shares[sc.ID]; ok && sh > 0 {
			shareStr = fmtMoney(money.New(sh, p.cur))
			// C606: "over" means the share exceeds what this source can
			// actually give — not what its LIMIT is. Comparing against the limit
			// meant a source could be asked for everything it had left and be
			// reported as fine.
			over = sh > coverMaxGive(sc)
		}
		return ui.CreateElement(coverSourceRow, coverSourceRowProps{
			ID: sc.ID, Label: sc.Label, RemainStr: fmtMoney(money.New(sc.MovableMinor, p.cur)),
			Selected: selNow[sc.ID], Weight: effWeights[sc.ID], ShareStr: shareStr,
			Over: over, AvailStr: fmtMoney(money.New(sc.MovableMinor, p.cur)), Max: p.coverMaxS.Get()[sc.ID],
			CommittedStr: committedCaveat(sc, p.cur), AfterStr: sourceAfterGiving(sc, shares[sc.ID], p.cur),
			WeightLocked: p.wtFxS.Get(), OnToggle: p.onToggleSrc, OnWeight: p.onWeightSrc, OnToggleMax: p.onToggleMax,
		})
	}
	srcKey := func(sc budgetCoverSource) any { return sc.ID }

	return budgetEditCoverPreviewResult{
		amtPreview: amtPreview, amtFxErr: amtFxErr, wtFxErr: wtFxErr, spreadSub: spreadSub, selCount: selCount, selTotal: selTotal,
		sortedCoverSrcs: sortedCoverSrcs, renderSrc: renderSrc, srcKey: srcKey,
	}
}
