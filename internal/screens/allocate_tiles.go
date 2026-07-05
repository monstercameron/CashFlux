// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// allocTile wraps a tile body in the shared Widget chrome + the full-width bento column.
func allocTile(id string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: id, Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// allocSection wraps a tile body with a serif section title + optional action, reusing the
// debt-section chrome so /allocate matches /debt and /investments.
func allocSection(id, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if id != "" {
		args = append(args, Attr("id", id))
	}
	if title != "" {
		args = append(args, Div(css.Class("debt-section-head"),
			H2(css.Class("debt-section-title"), title),
			If(action != nil, action),
		))
	}
	args = append(args, body)
	return Div(args...)
}

// allocStatChip renders one headline figure (reuses the debt-stat chrome).
func allocStatChip(label, value, valueCls string) ui.Node {
	return Div(css.Class("debt-stat"),
		Div(css.Class("debt-stat-label", tw.TextDim), label),
		Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+valueCls), value),
	)
}

// --- alloc-hero ------------------------------------------------------------------

type allocHeroProps struct {
	View            allocView
	AmountStr       string
	OnAmount        any
	ShowIncomeNudge bool
	MonthIncome     money.Money
	OnPrefillIncome any
	OnDismissIncome any
}

// allocHeroTile is the command center: the amount to put to work (a prominent input) with an
// income pre-fill nudge, and the derived split figures (allocatable / reserve / destinations).
func allocHeroTile(props allocHeroProps) ui.Node {
	v := props.View
	base := v.Base

	var nudge ui.Node = Fragment()
	if props.ShowIncomeNudge {
		nudge = Div(css.Class("alloc-income-nudge"), Attr("data-testid", "income-nudge"),
			Attr("aria-label", uistate.T("allocate.incomeNudgeLabel")),
			P(css.Class("muted"), uistate.T("allocate.incomeNudgeDesc", fmtMoney(props.MonthIncome))),
			Div(css.Class(tw.Flex, tw.Gap2, tw.ItemsCenter),
				Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "income-nudge-apply"),
					OnClick(props.OnPrefillIncome), uistate.T("allocate.incomeNudgeApply", fmtMoney(props.MonthIncome))),
				Button(css.Class("btn btn-sm"), Type("button"), OnClick(props.OnDismissIncome), uistate.T("allocate.incomeNudgeDismiss")),
			),
		)
	}

	var kept ui.Node = Fragment()
	if v.TotalMinor > 0 && v.Remainder > 0 {
		kept = P(css.Class("muted alloc-kept"), uistate.T("allocate.keptBack", fmtMoney(money.New(v.Remainder, base))))
	}

	figs := Div(css.Class("debt-chips"),
		allocStatChip(uistate.T("allocate.figAllocatable"), fmtMoney(money.New(v.Allocatable(), base)), " "+tw.ColorClass("text-up")),
		allocStatChip(uistate.T("allocate.figReserve"), fmtMoney(money.New(v.ReserveMinor, base)), ""),
		allocStatChip(uistate.T("allocate.figDestinations"), fmt.Sprintf("%d", len(v.Ranked)), ""),
	)

	body := Div(css.Class("alloc-hero"), Attr("id", "sec-plan"),
		Div(css.Class("alloc-hero-main"),
			Div(css.Class("alloc-hero-label", tw.TextDim), uistate.T("allocate.heroLabel")),
			Div(css.Class("alloc-amount-field"),
				Span(css.Class("alloc-amount-affix", tw.FontDisplay), Attr("aria-hidden", "true"), currency.Symbol(base)),
				Input(css.Class("alloc-amount-input", tw.FontDisplay), Type("number"), Attr("min", "0"), Step("0.01"),
					Attr("data-testid", "allocate-amount"), Attr("aria-label", uistate.T("allocate.heroLabel")),
					Placeholder(uistate.T("allocate.amountFieldPlaceholder")), Value(props.AmountStr), OnInput(props.OnAmount)),
			),
			nudge,
			kept,
		),
		figs,
	)
	return allocTile("alloc-hero", body)
}

// --- alloc-strategy (compact summary; edited in the flip modal) ------------------

type allocStrategyProps struct {
	View             allocView
	Profile          string
	Mode             string
	ReserveMinor     int64
	MaxPerMinor      int64
	ShowFormulas     bool
	OnEditStrategy   any
	OnToggleFormulas any
}

// allocStrategyTile is the compact strategy summary on the main surface: the active profile +
// split mode (and any buffer/cap) as read-only chips, an "Adjust strategy" button that opens the
// flip modal where they're edited, a plan-metrics toggle, and a Manage-accounts link.
func allocStrategyTile(props allocStrategyProps) ui.Node {
	base := props.View.Base

	metricsCls := "strip-toggle"
	if props.ShowFormulas {
		metricsCls += " is-on"
	}

	chips := []any{css.Class("alloc-strategy-chips")}
	chips = append(chips,
		allocStrategyChip(uistate.T("allocate.profileLabel"), allocProfileLabel(props.Profile)),
		allocStrategyChip(uistate.T("allocate.modeLabel"), allocModeLabel(props.Mode)),
	)
	if props.ReserveMinor > 0 {
		chips = append(chips, allocStrategyChip(uistate.T("allocate.reserveFieldLabel"), fmtMoney(money.New(props.ReserveMinor, base))))
	}
	if props.MaxPerMinor > 0 {
		chips = append(chips, allocStrategyChip(uistate.T("allocate.maxPerFieldLabel"), fmtMoney(money.New(props.MaxPerMinor, base))))
	}

	editBtn := Button(css.Class("btn btn-sm", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "allocate-edit-strategy"), OnClick(props.OnEditStrategy),
		uiw.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("allocate.adjustStrategy")))

	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(css.Class(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(props.ShowFormulas)),
				Attr("data-testid", "allocate-toggle-formulas"), Title(uistate.T("allocate.metricsTitle")),
				OnClick(props.OnToggleFormulas), Text(allocMetricsLabel(props.ShowFormulas))),
			A(css.Class("btn btn-ghost"), Href(uistate.RoutePath("/accounts")), uistate.T("debt.linkAccounts")),
		),
	)

	body := allocSection("sec-strategy", uistate.T("allocate.profileTitle"), editBtn,
		Fragment(
			P(css.Class("muted"), uistate.T("allocate.profileDesc")),
			Div(chips...),
			toolbar,
		))
	return allocTile("alloc-controls", body)
}

// allocStrategyChip is one read-only "label: value" summary chip.
func allocStrategyChip(label, value string) ui.Node {
	return Span(css.Class("alloc-strategy-chip"),
		Span(css.Class("alloc-strategy-chip-label", tw.TextDim), label),
		Span(css.Class("alloc-strategy-chip-val"), value),
	)
}

// allocModeLabel is the display label for a split mode.
func allocModeLabel(mode string) string {
	if mode == "fill" {
		return uistate.T("allocate.modeFillToTarget")
	}
	return uistate.T("allocate.modeWeighted")
}

// allocWeightField is one criterion weight input (a small number field with a caption).
func allocWeightField(label, value string, onInput any) ui.Node {
	return labeledField(label,
		Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("0.5"), Attr("aria-label", label), Value(value), OnInput(onInput)))
}

func allocMetricsLabel(on bool) string {
	if on {
		return uistate.T("allocate.metricsHide")
	}
	return uistate.T("allocate.metricsShow")
}

// --- alloc-plan ------------------------------------------------------------------

type allocPlanProps struct {
	View         allocView
	AmountFor    func(string) string
	OnExclude    func(string)
	OnViewSource func(route string)
	ExcludedRows []ui.Node
}

// allocPlanTile is the ranked, explainable list of destinations — the core output.
func allocPlanTile(props allocPlanProps) ui.Node {
	v := props.View

	var listBody ui.Node
	if len(v.Ranked) == 0 {
		listBody = P(css.Class("empty"), Attr("data-testid", "alloc-empty"), uistate.T("allocate.emptyRanked"))
	} else {
		cards := make([]any, 0, len(v.Ranked)+1)
		cards = append(cards, css.Class("alloc-plan-list"))
		for i, r := range v.Ranked {
			cards = append(cards, ui.CreateElement(allocDestRow, allocDestRowProps{
				R: r, Rank: i + 1, Amount: props.AmountFor(r.Candidate.ID),
				OnExclude: props.OnExclude, OnViewSource: props.OnViewSource,
			}))
		}
		listBody = Div(cards...)
	}

	extras := Fragment(
		If(v.HiddenZero, P(css.Class("muted alloc-hidden-note"), uistate.T("allocate.hiddenZero"))),
		If(len(props.ExcludedRows) > 0, Div(css.Class("alloc-excluded"),
			Div(css.Class("alloc-excluded-label", tw.TextDim), uistate.T("allocate.excludedLabel")),
			Div(css.Class("alloc-excluded-list"), props.ExcludedRows),
		)),
		If(v.TotalMinor == 0, P(css.Class("muted alloc-apply-hint"), uistate.T("allocate.applyHint"))),
	)

	body := allocSection("sec-ranked", uistate.T("allocate.rankedTitle"),
		investOwnerLink("/accounts", uistate.T("debt.linkAccounts")),
		Fragment(listBody, extras))
	return allocTile("alloc-plan", body)
}

// --- alloc-explain ---------------------------------------------------------------

type allocExplainProps struct {
	HasRanked      bool
	AiResult       string
	AiLoading      bool
	AiErr          string
	AlgoSummary    string
	OnExplain      any
	OnGoToSettings any
}

// allocExplainTile explains the ranking: a no-key algorithmic summary plus an opt-in AI narrative.
func allocExplainTile(props allocExplainProps) ui.Node {
	if !props.HasRanked {
		return Fragment()
	}
	var aiBody ui.Node = Fragment()
	switch {
	case props.AiLoading:
		aiBody = P(css.Class("muted"), uistate.T("allocate.aiLoading"))
	case props.AiErr != "":
		// The whole notice carries role="alert" so the message AND its "Open settings"
		// action are announced/scoped together (the no-key error links to Settings).
		aiBody = Div(css.Class("err alloc-ai-err"), Attr("role", "alert"),
			P(css.Class("muted"), props.AiErr),
			If(props.AiErr == uistate.T("allocate.needKey"),
				Button(css.Class("btn btn-sm"), Type("button"), OnClick(props.OnGoToSettings), uistate.T("allocate.openSettings"))),
		)
	case props.AiResult != "":
		aiBody = P(css.Class("alloc-ai-result"), props.AiResult)
	}

	body := allocSection("sec-why", uistate.T("allocate.whyTitle"), Fragment(),
		Fragment(
			If(props.AlgoSummary != "", P(css.Class("alloc-algo"), props.AlgoSummary)),
			Div(css.Class("alloc-ai"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "allocate-explain"),
					OnClick(props.OnExplain), uistate.T("allocate.aiExplain")),
				aiBody,
			),
		))
	return allocTile("alloc-explain", body)
}

// --- alloc-apply -----------------------------------------------------------------

type allocApplyProps struct {
	HasAmount     bool
	Confirming    bool
	Msg           string
	Err           string
	DidApply      bool
	ConfirmLabels []string
	OnOpenConfirm any
	OnConfirm     any
	OnCancel      any
	OnUndo        any
}

// allocApplyTile is the commit step: it earmarks / funds the plan (with a confirm step) and can
// undo the last application. Hidden until an amount is entered.
func allocApplyTile(props allocApplyProps) ui.Node {
	if !props.HasAmount {
		return Fragment()
	}

	var confirmRows []ui.Node
	for _, l := range props.ConfirmLabels {
		confirmRows = append(confirmRows, P(css.Class("muted"), l))
	}

	inner := IfElse(props.Confirming,
		Div(css.Class("alloc-confirm"),
			H3(css.Class("set-label"), uistate.T("allocate.applyConfirmTitle")),
			P(css.Class("muted"), uistate.T("allocate.applyConfirmDesc")),
			Div(css.Class("alloc-confirm-rows"), confirmRows),
			Div(css.Class(tw.Flex, tw.Gap2),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(props.OnConfirm), uistate.T("allocate.applyConfirm")),
				Button(css.Class("btn"), Type("button"), OnClick(props.OnCancel), uistate.T("allocate.applyCancel")),
			),
		),
		If(!props.DidApply, Button(css.Class("btn btn-primary alloc-apply-btn"), Type("button"),
			Attr("aria-label", uistate.T("allocate.applyTitle")), Attr("data-testid", "allocate-apply-btn"),
			OnClick(props.OnOpenConfirm), uistate.T("allocate.applyButton"))),
	)

	body := allocSection("sec-apply", uistate.T("allocate.applyTitle"), Fragment(),
		Fragment(
			P(css.Class("muted"), uistate.T("allocate.applyDesc")),
			If(props.Err != "", P(css.Class("err"), Attr("role", "alert"), props.Err)),
			If(props.Msg != "", Div(css.Class(tw.Flex, tw.Gap2, tw.ItemsCenter),
				P(css.Class("muted"), props.Msg),
				If(props.DidApply, Button(css.Class("btn btn-sm"), Type("button"),
					Attr("aria-label", uistate.T("allocate.undoTitle")), OnClick(props.OnUndo), uistate.T("allocate.undoButton"))),
			)),
			inner,
		))
	return allocTile("alloc-apply", body)
}

// --- alloc-formula ---------------------------------------------------------------

// allocFormulaTile is the opt-in FormulaBuilder over the alloc_* plan variables.
func allocFormulaTile() ui.Node {
	body := Fragment(
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("allocate.formulaHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("allocate.metricsTitle"), ShowSaved: true}),
	)
	return allocTile("alloc-formula", body)
}
