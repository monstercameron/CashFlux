// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetRowCardProps carries the explicit inputs for budgetRowCard.
type budgetRowCardProps struct {
	cardCls          string
	s                budgeting.Status
	title            string
	lastMonthTag     ui.Node
	width            int
	fillClass        string
	lastMonthMode    bool
	props            budgetRowProps
	limitEditing     ui.State[bool]
	saveLimitEdit    ui.Handler
	barSpent         string
	limit            money.Money
	limitKeyDown     ui.Handler
	onLimitDraft     ui.Handler
	limitErrID       string
	limitErrMsg      string
	limitDraft       ui.State[string]
	limitSaveTitle   string
	cancelLimitEdit  ui.Handler
	startLimitEdit   ui.Handler
	barPct           int
	paceLegend       ui.Node
	label            string
	effectiveCapLine ui.Node
	envLine          ui.Node
	envDebtLine      ui.Node
	fundedLine       ui.Node
	paceLine         ui.Node
	detailsOpen      ui.State[bool]
	toggleDetails    ui.Handler
	pieNode          ui.Node
	thisMonthRef     ui.Node
	coverageLine     ui.Node
	ownerLine        ui.Node
	methodLine       ui.Node
	customLine       ui.Node
	paceMarkLine     ui.Node
	proratedLine     ui.Node
	rolloverLine     ui.Node
	metricsStrip     ui.Node
	actionsRow       ui.Node
	hasSide          bool
	hasNotes         bool
	notesNode        ui.Node
	todosPanel       ui.Node
}

// budgetRowCard composes the full card without adding layout wrappers.
func budgetRowCard(p budgetRowCardProps) ui.Node {
	return Div(css.Class(p.cardCls), Attr("data-testid", "budget-card-"+p.s.Budget.ID),
		Div(css.Class("budget-head"),
			// The title gets the whole header line now (the spent/limit amount and the
			// percent moved INTO the bar below), so a long budget name has room to breathe.
			// C591: the card's title is a heading. It used to double as an
			// unlabelled drill-through — the same navigation the footer's labelled
			// "Transactions" button performs — so the page had two ways to do one
			// thing, one of them invisible to anyone not hovering it.
			Div(css.Class("budget-head-main"),
				Span(css.Class("row-desc"), p.title),
			),
		),
		// Overline tag naming the bar as last month's, when the overlay is on.
		p.lastMonthTag,
		// Keep the amount row in normal document flow. The bar itself is only five pixels
		// tall, so putting figures inside it makes them cover the pace legend below.
		ui.CreateElement(budgetRowFigures, p),
		p.paceLegend,
		// Everything below the bar sits in a flex row so a budget WITH a note can put the
		// note in a right-hand column (the lower-left content is short, leaving that space
		// empty). No note → the note column is omitted and the main content fills the width.
		Div(css.Class("budget-lower"),
			Div(css.Class("budget-lower-main"),
				// --- The primary summary: status, what is left, and anything that changes
				// what the headline figure MEANS. Always visible (C595). ---
				// One quiet metadata line beneath the bar. In last-month mode it reads the over/
				// under-budget gap + period; otherwise the health status · money left · period.
				IfElse(p.lastMonthMode,
					Span(css.Class("budget-sub"+lastMonthSubTone(p.props.LastMonthOver)), p.props.LastMonthDelta+" · "+periodLabel(p.s.Budget.Period)),
					Span(css.Class("budget-sub"), uistate.T("budgets.rowPrimary", p.label, budgetRowRemainPhrase(p.s))+" · "+periodLabel(p.s.Budget.Period))),
				// C344: while the period is too young to judge, last period's outcome is the
				// only reading here worth anything.
				If(p.props.PeriodEarly && p.props.PriorSpent != "",
					Span(css.Class("budget-sub"+lastMonthSubTone(p.props.PriorOver)),
						Attr("data-testid", "budget-prior-"+p.s.Budget.ID),
						uistate.T("budgets.priorPeriodOutcome", p.props.PriorSpent, p.props.PriorDelta))),
				// C395: the rollover policy badge — a glanceable chip, not a caption.
				If(p.s.Budget.Rollover, budgetRolloverBadgeFor(p.props)),
				// A cap that differs from the base limit, an envelope balance, an envelope in
				// debt, a projected overspend: each re-reads the big number above it, so
				// hiding them would hide the headline's meaning.
				p.effectiveCapLine,
				p.envLine,
				p.envDebtLine,
				p.fundedLine,
				p.paceLine,
				// --- Everything explanatory, behind one disclosure (C595). The card used to
				// stack all of it between the bar and the actions, so "how much is left?"
				// competed with a dozen explanations of how it was arrived at. ---
				ui.CreateElement(budgetRowExpandedDetails, p),
				p.actionsRow,
			),
			If(p.hasSide, Div(css.Class("budget-side-col"),
				If(p.hasNotes, p.notesNode),
				p.todosPanel,
			)),
		),
	)
}

// budgetRowFigures renders the bar, figures, and inline base-limit editor.
func budgetRowFigures(p budgetRowCardProps) ui.Node {
	return Fragment(
		Div(css.Class("budget-card-loader-figs"),
			// Spent carries foreground weight; the "/ limit" reads as muted context.
			// The limit figure is a direct affordance: click it to edit in place (G1).
			IfElse(p.limitEditing.Get(),
				Form(css.Class("budget-amount", "budget-limit-editform"), OnSubmit(p.saveLimitEdit),
					Span(css.Class("budget-spent"), p.barSpent), Span(" / "),
					Div(css.Class("budget-limit-edit-values"),
						Span(css.Class("budget-limit-editor-cap"), Strong(budgetEffectiveCapCopy(p.s.Budget.ID, p.limit)))),
					Input(append(append([]any{css.Class("field", "budget-limit-input"), Type("number"), OnKeyDown(p.limitKeyDown),
						Attr("data-testid", "budget-limit-input-"+p.s.Budget.ID), Attr("aria-label", uistate.T("budgets.limitLabel")), Step("0.01"), Attr("min", "0.01"), OnInput(p.onLimitDraft)},
						errAttrs(p.limitErrID, p.limitErrMsg)...), uiw.FieldValue(p.limitDraft.Get()))...),
					Button(append([]any{css.Class("btn btn-sm", "budget-limit-save"), Type("submit"), Attr("data-testid", "budget-limit-save-"+p.s.Budget.ID),
						Attr("aria-label", uistate.T("action.save")), Title(p.limitSaveTitle),
						uiw.Icon(icon.Check, css.Class(tw.W35, tw.H35))}, disabledAttr(p.limitErrMsg != "")...)...),
					Button(css.Class("btn btn-sm", "budget-limit-cancel"), Type("button"), Attr("data-testid", "budget-limit-cancel-"+p.s.Budget.ID),
						Attr("aria-label", uistate.T("action.cancel")), Title(uistate.T("action.cancel")), OnClick(p.cancelLimitEdit), uiw.Icon(icon.Close, css.Class(tw.W35, tw.H35))),
					errText(p.limitErrID, p.limitErrMsg),
				),
				Span(css.Class("budget-amount"),
					Span(css.Class("budget-spent"), p.barSpent), Span(" / "),
					Button(css.Class("budget-limit-btn"), Type("button"), Attr("data-testid", "budget-limit-btn-"+p.s.Budget.ID),
						Title(uistate.T("budgets.limitEditTitle")), Attr("aria-label", uistate.T("budgets.limitEditTitle")+" — "+p.title),
						OnClick(p.startLimitEdit), fmtMoney(p.limit)),
				)),
			// Percent-used, capped for display (e.g. "112%" when over).
			Span(css.Class("budget-pct"), strconv.Itoa(p.barPct)+"%"),
		),
		Div(css.Class("budget-card-loader"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(p.width)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("budgets.progressLabel")+" — "+p.title),
			Div(ClassStr(p.fillClass), Attr("style", fmt.Sprintf("width:%d%%", p.width))),
			// XC4: the committed band sits just past the spent fill (hidden in last-month
			// mode, where the bar shows a different period's spend).
			If(!p.lastMonthMode && p.props.HasCommitted && p.props.Committed.CommittedPct > 0,
				Div(css.Class("bar-committed"), Attr("data-testid", "budget-committed-seg-"+p.s.Budget.ID),
					Attr("style", fmt.Sprintf("left:%d%%;width:%d%%", p.props.Committed.SpentPct, p.props.Committed.CommittedPct)))),
			// BG3: the even-pace tick — a thin vertical marker at where discretionary
			// spending SHOULD be right now (committed money excluded). Hidden in
			// last-month mode (the bar then shows a different period).
			If(!p.lastMonthMode && p.props.PaceMarkerPct > 0,
				Div(css.Class("bar-pace-tick"), Attr("data-testid", "budget-pace-tick-"+p.s.Budget.ID),
					Attr("aria-hidden", "true"),
					Attr("style", fmt.Sprintf("position:absolute;top:0;bottom:0;left:%d%%;width:2px;background:var(--text);opacity:0.55;pointer-events:none", p.props.PaceMarkerPct)))),
		),
	)
}

// budgetRowExpandedDetails keeps explanatory content behind its existing disclosure.
func budgetRowExpandedDetails(p budgetRowCardProps) ui.Node {
	var body ui.Node = Fragment()
	if p.detailsOpen.Get() {
		var drivers ui.Node = Fragment()
		if !p.lastMonthMode && !p.s.Budget.IsSaving() && (p.s.State == budgeting.StateOver || p.s.State == budgeting.StateNear) {
			drivers = ui.CreateElement(budgetDriversPanel, budgetDriversPanelProps{Status: p.s, Budget: p.s.Budget, Title: p.title, Anchor: p.props.Anchor})
		}
		var suggestion ui.Node = Fragment()
		if !p.lastMonthMode {
			suggestion = ui.CreateElement(budgetSuggestHint, budgetSuggestProps{Budget: p.s.Budget})
		}
		body = Div(css.Class("budget-details"), Attr("data-testid", "budget-details-body-"+p.s.Budget.ID),
			// Spend-composition donut for a composite budget.
			p.pieNode,
			// Multi-category budgets: the categories the combined total is made of.
			If(p.props.TrackedCats != "", Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-tracked-cats-"+p.s.Budget.ID),
				uistate.T("budgets.catsTracking", p.props.TrackedCats))),
			// Cross-category tag tracking: the tags this budget also counts.
			If(len(p.s.Budget.TrackedTags) > 0, budgetTagLine(p.s.Budget.ID, p.s.Budget.TrackedTags)),
			// The full outstanding commitment and signed free amount are shown in the
			// always-visible pace line above; the clipped progress segment is only a bar.
			If(!p.lastMonthMode && p.props.HasCommitted && p.props.Committed.SetAsideNote != "",
				Span(css.Class("budget-sub", tw.TextFaint), Attr("data-testid", "budget-setaside-note-"+p.s.Budget.ID),
					p.props.Committed.SetAsideNote)),
			// BG1: funding-target summary ("Refill to $200 · $60 to go").
			If(budgetTargetLineVisible(p.s), budgetTargetLine(p.s)),
			p.thisMonthRef,
			p.coverageLine,
			p.ownerLine,
			p.methodLine,
			p.customLine,
			p.paceMarkLine,
			p.proratedLine,
			p.rolloverLine,
			p.metricsStrip,
			drivers,
			suggestion,
		)
	}
	return Fragment(
		Button(css.Class("btn cf-adv-toggle budget-details-toggle"), Type("button"),
			Attr("data-testid", "budget-details-"+p.s.Budget.ID),
			Attr("aria-expanded", ariaBool(p.detailsOpen.Get())),
			Attr("aria-label", uistate.T("budgets.detailsAria", p.title)),
			OnClick(p.toggleDetails), uistate.T(budgetDetailsKey(p.detailsOpen.Get()))),
		body,
	)
}
