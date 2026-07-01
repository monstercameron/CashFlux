// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/icon"
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

	// Secondary actions (Top up, Delete) live in a "⋯" overflow menu so the row stays
	// uncluttered — matching the /accounts row. Selecting one closes the menu. Escape +
	// outside-pointerdown dismiss it; AnchorPopover flips it left/up near the edge.
	menuOpen := ui.UseState(false)
	menuID := ui.UseId()
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))
	uiw.DismissPopover(menuOpen.Get(), menuID, func() { menuOpen.Set(false) })
	uiw.AnchorPopover(menuOpen.Get(), menuID)

	del := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnDelete(s.Budget.ID) }))
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(s.Budget.CategoryID)
		}
	}))
	// Edit and Top up open the shell-root flip modal (BudgetEditHost) rather than an
	// inline row form: a row sits under transformed bento/tile ancestors, which threw an
	// in-row modal off-centre. SetBudgetEdit updates the atom the host captured. Edit is
	// the lower-frequency action, so it lives in the ⋯ menu (and closes it); Top up is a
	// visible card button.
	openEdit := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeEdit})
	}))
	openTopup := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeTopup})
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

	// C118: show a small method badge when this budget has its own method override,
	// so the user can see at a glance which budget uses a different approach from
	// the household default. Hidden when the budget inherits the global method.
	var methodLine ui.Node = Fragment()
	if s.Budget.Methodology != "" {
		methodLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.methodOverrideRow", budgetMethodLabel(budgeting.ParseMethodology(s.Budget.Methodology))))
	}

	// Custom-field summary (e.g. "Priority: High · Review: Q3") — shown when the budget
	// has any user-defined field values, so custom data stays visible on the row.
	var customLine ui.Node = Fragment()
	if cs := customSummary(props.BudgetDefs, s.Budget.Custom); cs != "" {
		customLine = Span(css.Class("budget-sub", tw.TextFaint), cs)
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
			// C134: a carried-in deficit is a heads-up about where the period STARTED,
			// not a "you've overspent now" alert — render it in the caution amber
			// (TextWarn) so it reads distinctly from the danger-red overspend badge,
			// instead of conflating the two as the same alarming red.
			cls += " " + tw.Fold(tw.TextWarn)
		}
		rolloverLine = Span(ClassStr(cls), uistate.T("budgets.rolloverCarry", props.RolloverCarry))
	}

	// C136: show the effective cap (carry-in limit) on rollover budgets so the user
	// can see at a glance the maximum they can spend this period, not just their
	// base limit. Hidden when the carry is zero (cap == base limit, no note needed).
	var effectiveCapLine ui.Node = Fragment()
	if props.EffectiveCap != "" {
		effectiveCapLine = Span(css.Class("budget-sub", tw.TextFaint), uistate.T("budgets.effectiveCap", props.EffectiveCap))
	}

	// C143: even-pace guidance — how much of what's left can be spent over the days
	// still in the period, so the user knows the sustainable daily-ish pace instead
	// of seeing only a lump remaining. Quiet faint line; only set while in-progress.
	var proratedLine ui.Node = Fragment()
	if props.ProratedRest != "" {
		proratedLine = Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-prorated"),
			uistate.T("budgets.proratedRest", props.ProratedRest))
	}

	// "Cover…" is offered only on an over-budget row that has another budget to
	// pull from. The inline form picks a source and an amount (prefilled to the
	// exact overspend), so Maya can clear the overspend without leaving the screen.
	isOver := s.State == budgeting.StateOver
	hasSource := firstSource() != ""
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
	}
	var coverBtn ui.Node = Fragment()
	if isOver && hasSource && !covering.Get() {
		coverBtn = Button(css.Class("btn"), Type("button"), Title(uistate.T("budgets.coverTitle")), OnClick(startCover), "Cover…")
	}
	// Top up is a visible card button (the frequent proactive action) on budgets that
	// aren't over; Edit lives in the ⋯ menu as the lower-frequency action.
	var topupBtn ui.Node = Fragment()
	if !isOver && !covering.Get() {
		topupBtn = Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-topup-btn-"+s.Budget.ID), Title(uistate.T("budgets.topupTitle")), OnClick(openTopup), "Top up…")
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

	return Div(css.Class("budget "+budgetRowStateClass(s, props.PaceOver)),
		Div(css.Class("budget-head"),
			// Left group: the title (truncates), the spent/limit amount, and the percent
			// chip. Kept in its own flex box so the action buttons on the right form a
			// stable, column-aligned cluster instead of being pushed around by a long title.
			Div(css.Class("budget-head-main"),
				IfElse(s.Budget.CategoryID != "",
					Button(css.Class("row-desc budget-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drill),
						Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "text-align": "left", "cursor": "pointer"}),
						title),
					Span(css.Class("row-desc"), title)),
				// Spent carries foreground weight; the "/ limit" reads as muted context
				// (the name is the card's title, the amount is secondary — see styles).
				Span(css.Class("budget-amount"), Span(css.Class("budget-spent"), fmtMoney(s.Spent)), " / "+fmtMoney(limit)),
				// A compact percent-used chip, tinted by health state — an at-a-glance focal
				// figure that complements the bar (capped display, e.g. "112%" when over).
				Span(css.Class("budget-pct"), strconv.Itoa(s.Percent)+"%"),
			),
			// Right group: the row actions, as a fixed, no-wrap cluster so they line up in
			// a column across rows and never wrap onto a second line. Top up (the frequent
			// action) is a visible card button; Edit + the destructive Delete live in the ⋯
			// menu (like /accounts), so the row stays uncluttered and a misclick can't
			// delete a budget.
			Div(css.Class("budget-actions"),
				coverBtn,
				topupBtn,
				Div(css.Class("add-wrap"), Attr("id", menuID),
					Button(css.Class("btn"), Type("button"), Attr("title", uistate.T("budgets.moreActions")), Attr("aria-label", uistate.T("budgets.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
					Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
					Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
						Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "edit-budget-btn-"+s.Budget.ID), Title(uistate.T("budgets.editTitle")), OnClick(openEdit), uistate.T("budgets.editAction")),
						Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "delete-budget-btn-"+s.Budget.ID), Attr("aria-label", uistate.T("budgets.deleteTitle")), Title(uistate.T("budgets.deleteTitle")), OnClick(del), uistate.T("budgets.deleteAction")),
					),
				),
			),
		),
		Div(css.Class("bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")), Div(ClassStr(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
		// One quiet metadata line beneath the bar: health status · money left · period.
		// The old separate "Period · X% used" line is dropped — the bar and the percent
		// chip already carry the "% used" signal, so it was redundant clutter (design
		// critique). C124: budgetRemainPhrase yields "$50.00 left"/"$50.00 over" (no
		// accounting parens).
		Span(css.Class("budget-sub"), uistate.T("budgets.rowPrimary", label, budgetRemainPhrase(s.Remaining))+" · "+periodLabel(s.Budget.Period)),
		ownerLine,
		methodLine,
		customLine,
		paceLine,
		proratedLine,
		rolloverLine,
		effectiveCapLine,
		envLine,
		coverForm,
	)
}

// budgetRowStateClass maps a budget's health to the row's visual-state modifier class
// (the /budgets styles use it for the left accent stripe, the over-tint, and the
// percent-chip color): over → is-over, near-limit → is-near, pace-trending-over →
// is-risk, otherwise is-ontrack.
func budgetRowStateClass(s budgeting.Status, paceOver string) string {
	switch s.State {
	case budgeting.StateOver:
		return "is-over"
	case budgeting.StateNear:
		return "is-near"
	}
	if paceOver != "" {
		return "is-risk"
	}
	return "is-ontrack"
}

// budgetMethodLabel returns a short, localized label for a methodology value —
// reused by the per-budget method badge and the method select options.
func budgetMethodLabel(m budgeting.Methodology) string {
	switch m {
	case budgeting.MethodZeroBased:
		return uistate.T("settings.budgetMethodZero")
	case budgeting.MethodEnvelope:
		return uistate.T("settings.budgetMethodEnvelope")
	default:
		return uistate.T("settings.budgetMethodSimple")
	}
}

// budgetMethodOptions builds the SelectOptions for the per-budget method
// override picker. The first option ("Use global default") stores an empty
// value so that saving it clears the override, restoring global-method
// inheritance. The remaining options mirror the global method picker labels.
func budgetMethodOptions(selected string) []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "", Label: uistate.T("budgets.methodDefault")},
		{Value: string(budgeting.MethodSimple), Label: uistate.T("settings.budgetMethodSimple")},
		{Value: string(budgeting.MethodZeroBased), Label: uistate.T("settings.budgetMethodZero")},
		{Value: string(budgeting.MethodEnvelope), Label: uistate.T("settings.budgetMethodEnvelope")},
	}
}
