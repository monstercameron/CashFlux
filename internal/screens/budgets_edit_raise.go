// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetEditRaiseLimitProps carries the explicit inputs for budgetEditRaiseLimit.
type budgetEditRaiseLimitProps struct {
	topupPermanentS    ui.State[bool]
	b                  domain.Budget
	selfEffCap         string
	selfCapMath        string
	coverSelS          ui.State[map[string]bool]
	coverSrcs          []budgetCoverSource
	topupCoverOpenS    ui.State[bool]
	topupAmt           ui.State[string]
	dec                int
	vw                 period.Window
	submitTopup        ui.Handler
	onTopupAmt         ui.Handler
	onToggleTopupCover ui.Handler
	cur                string
	onToggleSrc        func(sid string)
	coverIncomplete    bool
	errLine            ui.Node
	cancel             ui.Handler
}

// budgetEditRaiseLimit builds the edit raise limit section.
func budgetEditRaiseLimit(p budgetEditRaiseLimitProps) ui.Node {
	durVal := "month"
	if p.topupPermanentS.Get() {
		durVal = "perm"
	}
	// C669: the dialog used to open with "Increase Groceries (currently $500.00)"
	// beside a card reading an effective cap of $721.71, and never said which of
	// the two the typed amount would land on. Both figures are named now, with
	// the arithmetic between them (the same string the card's cap line carries),
	// and the duration hint states which one the top-up changes and what happens
	// to it next period.
	capLine, _ := budgetTopupCapCopy(fmtMoney(p.b.Limit), p.selfEffCap, p.selfCapMath, p.topupPermanentS.Get())
	selNow := p.coverSelS.Get()
	coverToggleLabel := uistate.T("budgets.topupCoverShow")
	if p.topupCoverOpenS.Get() {
		coverToggleLabel = uistate.T("budgets.topupCoverHide")
	}
	funded := false
	for id, selected := range selNow {
		if selected && id != "" {
			funded = true
			break
		}
	}
	impact := budgeting.TopUpImpact(p.topupPermanentS.Get(), funded)
	reach := fundsReachThisPeriod
	if p.topupPermanentS.Get() {
		reach = fundsReachPermanent
	}
	// C606: the split the top-up WOULD take from each source, so every row can
	// show what it would be left with before anything is written. The same
	// splitCoverAmount the submit uses, so the preview cannot promise a
	// different number than the write performs.
	var topupShares map[string]int64
	if amt, perr := money.ParseMinor(strings.TrimSpace(p.topupAmt.Get()), p.dec); perr == nil && amt > 0 {
		topupShares = splitCoverAmount(amt, p.coverSrcs, selNow, nil, nil)
	}
	return Form(css.Class("acct-edit-form"), Attr("data-testid", "budget-raise-form"), OnSubmit(p.submitTopup),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "topup-cap-math"),
				Style(map[string]string{"margin": "0"}), capLine),
			uiw.Field(uiw.FieldProps{ID: "budget-topup-amt", Label: uistate.T("budgets.raiseAmount"), Required: true,
				TestID: "budget-raise-amount-field", Control: func(a uiw.FieldA11y) ui.Node {
					return Input(css.Class("field"), Attr("id", a.ID), Attr("data-testid", "budget-topup-amt"), Attr("autofocus", ""), Attr("aria-required", "true"), Type("number"),
						Placeholder(uistate.T("budgets.raiseAmount")), Step("0.01"), Attr("min", "0.01"), OnInput(p.onTopupAmt), uiw.FieldValue(p.topupAmt.Get()))
				}}),
			// Duration: this period only (a one-time boost) vs a permanent cap change.
			labeledField(uistate.T("budgets.topupDuration"),
				uiw.Segmented(uiw.SegmentedProps{
					Label:    uistate.T("budgets.topupDuration"),
					Selected: durVal,
					Options: []uiw.SegOption{
						{Value: "month", Label: uistate.T("budgets.topupThisMonth"), TestID: "topup-dur-month"},
						{Value: "perm", Label: uistate.T("budgets.topupPermanent"), TestID: "topup-dur-perm"},
					},
					OnSelect: func(v string) { p.topupPermanentS.Set(v == "perm") },
				})),
			// C597: the duration hint plus the shared impact vocabulary â€” reach,
			// whether other budgets are touched, that no account balance moves,
			// and that it can be undone. Every funds-moving action on this page
			// answers the same four questions in the same words.
			// C669: which of the two figures above the amount lands on, and what
			// happens to it at the period boundary.
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "topup-changes"),
				Style(map[string]string{"margin": "0"}), fundsImpactLine(impact, fundsImpactOption{
					reach: reach, movesFromSelectedBudgets: funded,
				})),
			// Optional: fund the top-up by pulling from budgets that have room to give.
			If(len(p.coverSrcs) > 0, Fragment(
				Button(css.Class("btn cf-adv-toggle"), Type("button"), Attr("aria-expanded", ariaBool(p.topupCoverOpenS.Get())),
					Attr("data-testid", "topup-cover-toggle"), OnClick(p.onToggleTopupCover), Text(coverToggleLabel)),
				If(p.topupCoverOpenS.Get(), Div(css.Class("budget-topup-cover"),
					P(css.Class("t-caption", tw.TextDim), uistate.T("budgets.topupCoverHint")),
					Div(css.Class("cover-sources"),
						MapKeyed(p.coverSrcs, func(sc budgetCoverSource) any { return sc.ID }, func(sc budgetCoverSource) ui.Node {
							return ui.CreateElement(budgetTopupSourceRow, budgetTopupSourceRowProps{
								// C606: the movable amount, not the limit â€” the figure
								// this list showed was the size of the source's plan.
								ID: sc.ID, Label: sc.Label, AvailStr: fmtMoney(money.New(sc.MovableMinor, p.cur)),
								CommittedStr: committedCaveat(sc, p.cur),
								AfterStr:     sourceAfterGiving(sc, topupShares[sc.ID], p.cur),
								Selected:     selNow[sc.ID], OnToggle: p.onToggleSrc,
							})
						})),
				)),
			)),
			If(p.coverIncomplete, P(css.Class("err"), Attr("role", "status"), Attr("data-testid", "budget-cover-incomplete"), uistate.T("budgets.committedIncomplete"))),
			p.errLine,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(p.cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "budget-raise-submit"), uistate.T("budgets.raiseLimit")),
		),
	)
}
