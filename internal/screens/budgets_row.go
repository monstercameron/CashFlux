// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// BudgetRow renders one budget's spend vs limit with a progress bar. Clicking
// Edit swaps in an inline form for the name, limit, and period. It owns all its
// hooks (declared unconditionally) so the edit toggle never disturbs hook order.
func BudgetRow(props budgetRowProps) ui.Node {
	s := props.Status
	limitMajor := money.FormatMinor(s.Budget.Limit.Amount, currency.Decimals(s.Budget.Limit.Currency))

	del := ui.UseEvent(Prevent(func() { props.OnDelete(s.Budget.ID) }))
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(s.Budget.CategoryID)
		}
	}))
	editing := ui.UseState(false)
	nameS := ui.UseState(s.Budget.Name)
	limitS := ui.UseState(limitMajor)
	periodS := ui.UseState(string(s.Budget.Period))
	ownerS := ui.UseState(s.Budget.OwnerID)
	rolloverS := ui.UseState(s.Budget.Rollover)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	// onPeriod/onOwner hooks kept for stable hook ordering; SelectInput owns the
	// change event internally so these handlers are no longer wired to DOM.
	ui.UseEvent(func(e ui.Event) { periodS.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	onRollover := ui.UseEvent(func() { rolloverS.Set(!rolloverS.Get()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(s.Budget.Name)
		limitS.Set(limitMajor)
		periodS.Set(string(s.Budget.Period))
		ownerS.Set(s.Budget.OwnerID)
		rolloverS.Set(s.Budget.Rollover)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(s.Budget.ID, nameS.Get(), limitS.Get(), periodS.Get(), ownerS.Get(), rolloverS.Get())
		editing.Set(false)
	}))

	// "Cover…" inline form (L1): move money from another budget to clear an overspend.
	covering := ui.UseState(false)
	coverFrom := ui.UseState("")
	coverAmt := ui.UseState("")
	coverErr := ui.UseState("")
	// onCoverFrom hook kept for stable hook ordering; SelectInput owns the change event.
	ui.UseEvent(func(e ui.Event) { coverFrom.Set(e.GetValue()) })
	onCoverAmt := ui.UseEvent(func(v string) { coverAmt.Set(v) })
	firstSource := func() string {
		for _, src := range props.CoverSources {
			if src.ID != s.Budget.ID {
				return src.ID
			}
		}
		return ""
	}
	startCover := ui.UseEvent(Prevent(func() {
		coverFrom.Set(firstSource())
		coverAmt.Set(props.CoverDefault)
		coverErr.Set("")
		covering.Set(true)
	}))
	cancelCover := ui.UseEvent(Prevent(func() { covering.Set(false) }))
	fullCover := ui.UseEvent(Prevent(func() { coverAmt.Set(props.CoverDefault) }))
	submitCover := ui.UseEvent(Prevent(func() {
		from := coverFrom.Get()
		if from == "" {
			from = firstSource()
		}
		if err := props.OnCover(s.Budget.ID, from, coverAmt.Get()); err != nil {
			coverErr.Set(err.Error())
			return
		}
		coverErr.Set("")
		covering.Set(false)
	}))

	// "Top up…" inline form (L43): increase this budget's limit by a chosen amount,
	// available on budgets that are not already over (proactive capacity add).
	toppingUp := ui.UseState(false)
	topupAmt := ui.UseState("")
	topupErr := ui.UseState("")
	onTopupAmt := ui.UseEvent(func(v string) { topupAmt.Set(v) })
	startTopup := ui.UseEvent(Prevent(func() {
		topupAmt.Set("")
		topupErr.Set("")
		toppingUp.Set(true)
	}))
	cancelTopup := ui.UseEvent(Prevent(func() { toppingUp.Set(false) }))
	submitTopup := ui.UseEvent(Prevent(func() {
		if err := props.OnTopUp(s.Budget.ID, topupAmt.Get()); err != nil {
			topupErr.Set(err.Error())
			return
		}
		topupErr.Set("")
		toppingUp.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("budget-edit-" + s.Budget.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		return Div(css.Class("budget"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("common.name"),
					Input(css.Class("field"), Attr("id", "budget-edit-"+s.Budget.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
				labeledField(uistate.T("budgets.limitLabel"),
					Input(css.Class("field"), Type("number"), Placeholder(uistate.T("budgets.limitLabel")), Value(limitS.Get()), Step("0.01"), OnInput(onLimit))),
				labeledField(uistate.T("budgets.period"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   periodOptions(periodS.Get()),
						Selected:  periodS.Get(),
						OnChange:  func(v string) { periodS.Set(v) },
						AriaLabel: uistate.T("budgets.period"),
					})),
				labeledField(uistate.T("common.owner"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   ownerSelectOptions(props.Members, ownerS.Get()),
						Selected:  ownerS.Get(),
						OnChange:  func(v string) { ownerS.Set(v) },
						AriaLabel: uistate.T("common.owner"),
					})),
				Label(css.Class("field", tw.Flex, tw.ItemsCenter, tw.Gap2),
					Input(append([]any{Type("checkbox"), OnChange(onRollover)}, checkedAttr(rolloverS.Get())...)...),
					Span(uistate.T("budgets.rollover")),
				),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	limit, _ := s.Spent.Add(s.Remaining) // limit in base currency

	width := s.Percent
	if width > 100 {
		width = 100
	}
	fillClass := "bar-fill"
	label := uistate.T("budgets.onTrack")
	switch s.State {
	case budgeting.StateNear:
		fillClass = "bar-fill near"
		label = uistate.T("budgets.nearLimit")
	case budgeting.StateOver:
		fillClass = "bar-fill over"
		label = uistate.T("budgets.overBudget")
	default:
		// Not over/near yet, but the pace projection says this budget is trending
		// to overspend — don't claim "On track" while also warning of an overspend
		// (the L35 contradiction). Call it "At risk" instead.
		if props.PaceOver != "" {
			fillClass = "bar-fill near"
			label = uistate.T("budgets.atRisk")
		}
	}

	// Show "name · category" only when they add information (see budgetTitle).
	title := budgetTitle(s.Budget.Name, props.Category)

	// Owner tag (L106 learning): an INDIVIDUAL budget only counts its owner's spending, so a household
	// can't otherwise tell why a shared expense didn't move it. Flag whose it is — but only for
	// individual budgets (OwnerID matches a real member); shared/household budgets (the common default,
	// OwnerID = group) stay unlabeled to keep rows clean.
	var ownerLine ui.Node = Fragment()
	for _, m := range props.Members {
		if m.ID == s.Budget.OwnerID {
			ownerLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.individualOwner", m.Name))
			break
		}
	}

	// Envelope methodology: show the carried-forward balance under the period row.
	var envLine ui.Node = Fragment()
	if props.Envelope != "" {
		cls := "budget-sub " + tw.Fold(tw.FontDisplay)
		if props.EnvelopeNeg {
			cls += " " + tw.Fold(tw.TextDown)
		}
		envLine = Span(ClassStr(cls), uistate.T("budgets.envelopeRow", props.Envelope))
	}

	// Pace projection (D2): a gentle heads-up when current spending would blow the
	// budget by period end, shown only while the period is still in progress.
	var paceLine ui.Node = Fragment()
	if props.PaceOver != "" {
		paceLine = Span(css.Class("budget-sub", tw.TextDown), uistate.T("budgets.paceOver", props.PaceOver))
	}

	var rolloverLine ui.Node = Fragment()
	if props.RolloverCarry != "" {
		cls := "budget-sub " + tw.Fold(tw.FontDisplay)
		if props.RolloverNeg {
			cls += " " + tw.Fold(tw.TextDown)
		}
		rolloverLine = Span(ClassStr(cls), uistate.T("budgets.rolloverCarry", props.RolloverCarry))
	}

	// "Cover…" is offered only on an over-budget row that has another budget to
	// pull from. The inline form picks a source and an amount (prefilled to the
	// exact overspend), so Maya can clear the overspend without leaving the screen.
	isOver := s.State == budgeting.StateOver
	hasSource := firstSource() != ""
	var coverBtn ui.Node = Fragment()
	if isOver && hasSource && !covering.Get() {
		coverBtn = Button(css.Class("btn"), Type("button"), Title(uistate.T("budgets.coverTitle")), OnClick(startCover), "Cover…")
	}

	// "Top up…" is offered on budgets that are not over, letting the user raise
	// the limit proactively before they hit the ceiling (L43).
	var topupBtn ui.Node = Fragment()
	if !isOver && props.OnTopUp != nil && !toppingUp.Get() && !covering.Get() {
		topupBtn = Button(css.Class("btn"), Type("button"), Title(uistate.T("budgets.topupTitle")), OnClick(startTopup), "Top up…")
	}
	var topupForm ui.Node = Fragment()
	if toppingUp.Get() {
		var topupErrLine ui.Node = Fragment()
		if topupErr.Get() != "" {
			topupErrLine = P(css.Class("budget-sub", tw.TextDown), topupErr.Get())
		}
		topupForm = Div(css.Class("cover-form"),
			Span(css.Class("budget-sub"), "Increase this budget's limit by:"),
			Form(css.Class("form-grid"), OnSubmit(submitTopup),
				Input(css.Class("field"), Type("number"), Attr("aria-label", uistate.T("budgets.amountToAdd")), Placeholder("Amount"), Value(topupAmt.Get()), Step("0.01"), OnInput(onTopupAmt)),
				Button(css.Class("btn btn-primary"), Type("submit"), "Add funds"),
				Button(css.Class("btn"), Type("button"), OnClick(cancelTopup), uistate.T("action.cancel")),
			),
			topupErrLine,
		)
	}
	var coverForm ui.Node = Fragment()
	if covering.Get() {
		srcOpts := make([]uiw.SelectOption, 0, len(props.CoverSources))
		for _, src := range props.CoverSources {
			if src.ID == s.Budget.ID {
				continue
			}
			srcOpts = append(srcOpts, uiw.SelectOption{Value: src.ID, Label: src.Label})
		}
		var coverErrLine ui.Node = Fragment()
		if coverErr.Get() != "" {
			coverErrLine = P(css.Class("budget-sub", tw.TextDown), coverErr.Get())
		}
		coverForm = Div(css.Class("cover-form"),
			Span(css.Class("budget-sub"), "Cover the "+props.CoverShortfall+" over by moving money from another budget:"),
			Form(css.Class("form-grid"), OnSubmit(submitCover),
				uiw.SelectInput(uiw.SelectInputProps{
					Options:   srcOpts,
					Selected:  coverFrom.Get(),
					OnChange:  func(v string) { coverFrom.Set(v) },
					AriaLabel: "Cover from budget",
				}),
				Input(css.Class("field"), Type("number"), Attr("aria-label", uistate.T("budgets.amountToMove")), Placeholder("Amount"), Value(coverAmt.Get()), Step("0.01"), OnInput(onCoverAmt)),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("budgets.fullOverspendTitle")), OnClick(fullCover), "Full "+props.CoverShortfall),
				Button(css.Class("btn btn-primary"), Type("submit"), "Cover"),
				Button(css.Class("btn"), Type("button"), OnClick(cancelCover), uistate.T("action.cancel")),
			),
			coverErrLine,
		)
	}

	return Div(css.Class("budget"),
		Div(css.Class("budget-head"),
			IfElse(s.Budget.CategoryID != "",
				Button(css.Class("row-desc budget-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drill),
					Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "text-align": "left", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
					title),
				Span(css.Class("row-desc"), title)),
			Span(css.Class("budget-amount"), fmtMoney(s.Spent)+" / "+fmtMoney(limit)),
			coverBtn,
			topupBtn,
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("budgets.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
			Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("budgets.deleteTitle")), Title(uistate.T("budgets.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		Div(css.Class("bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")), Div(ClassStr(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
		// Sub-line split (G4 §8): the primary line carries the at-a-glance signal —
		// health status + money left; the period and percent drop to a dimmer
		// secondary line so they read as low-signal context, not equal weight.
		Span(css.Class("budget-sub"), uistate.T("budgets.rowPrimary", label, fmtMoney(s.Remaining))),
		Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.rowSecondary", s.Budget.Period.Label(), width)),
		ownerLine,
		paceLine,
		rolloverLine,
		envLine,
		coverForm,
		topupForm,
	)
}
