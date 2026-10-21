// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/debounce"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgets_tiles.go holds the Native widget bodies the /budgets surface host composes
// (see budgets_widget.go). Each is a self-contained engine tile: it reads the live
// store (props.App) and the shared budgets-page atoms, never surface-local closures.
// The rich per-budget row (BudgetRow) is reused verbatim — these tiles only restructure
// the page around it, matching the /accounts + /transactions surface pattern.

type budgetSummaryProps struct{ App *appstate.App }

type budgetToolbarProps struct{ App *appstate.App }

type budgetListProps struct{ App *appstate.App }

// --- budget-summary --------------------------------------------------------------

// budgetSummaryWidget is the health-summary tile: the spent / budgeted / left stat
// grid, the income-vs-methodology assign banner, the sinking-fund set-aside note, and
// the over/near alert banner + count badges. It renders nothing when there are no
// budgets (the list tile owns the first-run CTA), so the surface stays clean.
func budgetSummaryWidget(props budgetSummaryProps) ui.Node {
	// Subscribe to the data revision so totals/badges refresh after any mutation — the
	// tile's props are the same *App pointer across host renders.
	_ = uistate.UseDataRevision().Get()
	app := props.App
	memberLens := uistate.UseMemberLens()
	activeMemberID := ""
	if len(memberLens) == 1 {
		activeMemberID = memberLens[0]
	}
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	// "Last month's spend" overlay: when on, the summary graph shows last period's total
	// spend too (matching the tiles), not just this month.
	showLM := uistate.UseBudgetsLastMonth().Get()
	futurePeriod := isFuturePeriod(vw, budgetNow())
	// Income-basis modal opener. Called unconditionally (before any early return) so the
	// hook order is stable across renders. Opening seeds the modal's draft from the
	// current prefs, so the Save/Cancel modal starts from today's basis.
	basisOpen := uistate.UseBudgetBasisOpen()
	basisDraft := uistate.UseBudgetBasisDraft()
	openBasis := ui.UseEvent(Prevent(func() {
		basisDraft.Set(uistate.NewBudgetBasisDraft(pr))
		basisOpen.Set(true)
	}))
	basisOnClick := OnClick(openBasis)
	// B1 hero: the attention chip narrows the list via the shared attention filter
	// (the same atom the issues rail's counts set). Hook registered before the early
	// return; the counts it reads are assigned later in this render — the closure
	// captures the variables, and it only runs on click, after they're set.
	attnAtom := uistate.UseBudgetAttention()
	var attnOver, attnNear int
	filterAttn := ui.UseEvent(Prevent(func() {
		if attnOver+attnNear > 0 {
			attnAtom.Set("needs-attention")
		}
	}))
	v := computeBudgetView(app, activeMemberID, vw, pr, showLM)
	if len(v.Statuses) == 0 {
		if len(v.Incomplete) > 0 {
			return P(css.Class("form-err"), Attr("role", "status"), uistate.T("budgets.incomplete"))
		}
		return Fragment()
	}
	summaryPlan := budgetSummaryPlan(v, app, vw)
	// A discoverable button that opens the "Income to budget with" modal (the income-
	// source picker + rules) — present in every method, so simple/envelope users can set
	// which income funds the budget just like zero-based users.
	// Same .btn-tool chrome as Cover-all beside it — the hero side cluster speaks
	// one control language (Cam 2026-07-19).
	basisBtn := Button(css.Class("btn btn-tool zbb-basis-open", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-basis-open"), Title(uistate.T("budgets.basisButtonTitle")), basisOnClick,
		uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("budgets.basisButton")))

	summaryPool := budgetSummaryPool(vw, v, summaryPlan.hist, summaryPlan.normalizedBudgetTotal)

	// BUD2: count the same over-or-near set that the list's attention filter uses.
	for _, s := range v.Statuses {
		switch s.State {
		case budgeting.StateOver:
			attnOver++
		case budgeting.StateNear:
			attnNear++
		}
	}
	_, toAssignChip, ageChip := budgetSummaryChips(budgetSummaryChipsProps{
		v: v, hist: summaryPlan.hist, filterAttn: filterAttn, fundHasPool: summaryPool.fundHasPool, fundFree: summaryPool.fundFree,
		zbbHero: summaryPool.zbbHero, spendCap: summaryPool.spendCap, smartSettings: summaryPlan.smartSettings,
	})
	incomeActual, funding, actualIncome := budgetSummaryFunding(budgetSummaryFundingProps{
		app: app, wStart: summaryPlan.wStart, wEnd: summaryPlan.wEnd, v: v, normalizationOK: summaryPlan.normalizationOK,
		normalizedBudgetTotal: summaryPlan.normalizedBudgetTotal, hist: summaryPlan.hist,
	})
	summaryAllocation := budgetSummaryAllocation(budgetSummaryAllocationProps{
		futurePeriod: futurePeriod, planView: summaryPlan.planView, basisBtn: basisBtn, hist: summaryPlan.hist, v: v,
		normalizationOK: summaryPlan.normalizationOK, normalizedBudgetTotal: summaryPlan.normalizedBudgetTotal, wEnd: summaryPlan.wEnd,
		actualIncome: actualIncome, pr: pr,
	})

	rail := budgetSummaryIssues(budgetSummaryIssuesProps{
		app: app, v: v, smartSettings: summaryPlan.smartSettings, overAssigned: summaryAllocation.overAssigned,
		fundFree: summaryPool.fundFree, fundHasPool: summaryPool.fundHasPool, hist: summaryPlan.hist,
	})

	statusLine := budgetSummaryStatus(budgetSummaryStatusProps{
		normalizedBudgetTotal: summaryPlan.normalizedBudgetTotal, vw: vw, v: summaryPlan.planView, futurePeriod: futurePeriod,
		normalizationOK: summaryPlan.normalizationOK, overAssigned: summaryAllocation.overAssigned, funding: funding, hist: summaryPlan.hist,
		excluded: summaryPlan.excluded, issues: summaryPlan.issues, filterAttn: filterAttn,
		attnCount: len(budgetAttentionStatuses(v.Statuses)), attnOver: attnOver, smartSettings: summaryPlan.smartSettings,
	})
	details := budgetSummaryDetails(budgetSummaryDetailsProps{
		v: v, normalizationOK: summaryPlan.normalizationOK, alloc: summaryAllocation.alloc,
		futurePeriod: futurePeriod, funding: funding, hist: summaryPlan.hist, vw: vw, app: app, activeMemberID: activeMemberID, pr: pr,
		rail: rail, incomeActual: incomeActual, lastMonthTag: summaryPool.lastMonthTag,
		closeOffer: summaryAllocation.closeOffer, toAssignChip: toAssignChip,
		hasAlloc: summaryAllocation.hasAlloc, basisBtn: basisBtn, ageChip: ageChip,
	})

	body := Div(Div(css.Class("budget-hero"), Attr("data-testid", "budgets-status-strip"), statusLine, details), summaryPool.rangeHint)
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

type budgetStatusActionProps struct {
	Label   string
	Percent float64
	TestID  string
}

// budgetStatusAction opens the existing preview-first adjustment flow from the
// status strip. The adjustment remains explicit and undoable; the strip never
// writes the budget plan itself.
func budgetStatusAction(props budgetStatusActionProps) ui.Node {
	open := uistate.UseBudgetAdjustOpen()
	onClick := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetAdjustSeed(strconv.FormatFloat(props.Percent, 'f', -1, 64), string(budgeting.AdjustThisPeriod))
		open.Set(true)
	}))
	return Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", props.TestID),
		OnClick(onClick), props.Label)
}

// lastMonthLabelKey picks the toolbar toggle's label: a call-to-action when off,
// a "you're viewing last month" state when on.
func lastMonthLabelKey(on bool) string {
	if on {
		return "budgets.lastMonthOn"
	}
	return "budgets.lastMonth"
}

// budgetDensityTitleKey picks the density toggle's tooltip for its CURRENT state.
//
// C596: the button's visible label is now fixed ("Compact list") and its
// aria-pressed carries the state, because a label that swapped between "Compact
// list" and "Card view" described the destination in one state and the current
// view in the other — and paired with aria-pressed=true it announced "Card view,
// pressed" while the compact list was the thing on screen. A two-state toggle
// with a stable name says both halves without contradiction; the tooltip spells
// out what a click does.
func budgetDensityTitleKey(density string) string {
	if density == uistate.BudgetDensityCompact {
		return "budgets.densityTitleOn"
	}
	return "budgets.densityTitleOff"
}

// budgetDensityLabelKey picks the toggle's VISIBLE label, which names what a
// click produces rather than what is on screen (Cam, 2026-08-31).
//
// C596 had frozen this label because a moving one beside aria-pressed announced
// "Card view, pressed" over a compact list. The half that could not be true in
// both states was aria-pressed, not the label: a button named for its destination
// is never "pressed", it is just a button. So the label moves and aria-pressed is
// gone — the current view is carried by the accessible-name suffix for assistive
// tech and by data-density for anything that needs to read it.
func budgetDensityLabelKey(density string) string {
	if density == uistate.BudgetDensityCompact {
		return "budgets.densityCards"
	}
	return "budgets.densityCompact"
}

// budgetDensityStateKey picks the screen-reader-only suffix that makes the
// toggle's ACCESSIBLE name stateful (C670).
//
// C596 fixed a contradiction by freezing the visible label, and left a keyboard
// or screen-reader user with a control whose name never says which view is on
// screen or what pressing it will produce — the tooltip said both, and a tooltip
// is a mouse affordance. The suffix restores the resulting-view wording where the
// assistive tech reads it, while the visible label and aria-pressed keep saying
// the same thing they say now.
func budgetDensityStateKey(density string) string {
	if density == uistate.BudgetDensityCompact {
		return "budgets.densityStateOn"
	}
	return "budgets.densityStateOff"
}

// hiddenIf returns the " hidden-menu" class suffix when hide is true — the same
// convention the shared kebab menus use to keep closed popovers in the DOM.
func hiddenIf(hide bool) string {
	if hide {
		return " hidden-menu"
	}
	return ""
}

// nearBadgeText renders the near-limit count, past-tense on a closed period
// ("2 finished near the limit") and present-tense while the period is live.
func nearBadgeText(count int, hist bool) string {
	if hist {
		return uistate.T("budgets.histNearBadge", count)
	}
	return uistate.T("budgets.nearBadge", count)
}

// overBannerText renders the over-budget banner copy with correct grammar for a
// single over-budget category.
func overBannerText(count int, total string, hist bool) string {
	if hist {
		if count == 1 {
			return uistate.T("budgets.histOverBannerOne", total)
		}
		return uistate.T("budgets.histOverBanner", count, total)
	}
	if count == 1 {
		return uistate.T("budgets.overBannerOne", total)
	}
	return uistate.T("budgets.overBanner", count, total)
}

// zbbLegendItem is one entry in the allocation legend: a color swatch matching a bar
// segment (tone = is-exp / is-sav / is-gap), a label, and a tabular amount.
func zbbLegendItem(tone, label string, minor int64, base string) ui.Node {
	return zbbLegendItemTone(tone, "", label, minor, base)
}

// zbbLegendItemTone is zbbLegendItem with an extra class on the amount (valCls), so the
// over-assigned slot can render its figure in the money-negative tone.
func zbbLegendItemTone(tone, valCls, label string, minor int64, base string) ui.Node {
	return Div(css.Class("zbb-legend-item"),
		Span(css.Class("zbb-legend-dot "+tone)),
		Span(css.Class("zbb-legend-label"), label),
		Span(css.Class("zbb-legend-val fig "+valCls), fmtMoney(money.New(minor, base))),
	)
}

func minorToMajorStr(minor int64, base string) string {
	if minor <= 0 {
		return ""
	}
	dec := currency.Decimals(base)
	mult := int64(1)
	for i := 0; i < dec; i++ {
		mult *= 10
	}
	whole, fraction := minor/mult, minor%mult
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	frac := strings.TrimRight(fmt.Sprintf("%0*d", dec, fraction), "0")
	return strconv.FormatInt(whole, 10) + "." + frac
}

// majorStrToMinor parses an edit-field major-unit string back to minor units;
// ok=false for blank/invalid input so the caller can distinguish "clear" from "keep".
func majorStrToMinor(s, base string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true // explicit clear
	}
	minor, err := money.ParseMinor(s, currency.Decimals(base))
	if err != nil || minor < 0 {
		return 0, false
	}
	return minor, true
}

type incomeBasisProps struct {
	Base            string
	Sources         []incomeSource // prepared source averages and availability
	SourceAmountErr error
	SourceRankErr   error
	Income          int64 // canonical combined preview, rounded independently of rows
	IncomeErr       error
}

// budgetIncomeBasisControl is the income-source picker inside the "Income to budget with"
// modal: whether the income is ALL of last month's income, only regular paychecks
// (deposits at or above a threshold), income from a chosen set of SOURCES (categories),
// or a fixed monthly figure — plus the roll-leftover rule. It edits the STAGED DRAFT
// (UseBudgetBasisDraft), never prefs directly, so the modal's Save commits and Cancel
// discards. props.Income is the live preview total computed from that draft.
func budgetIncomeBasisControl(props incomeBasisProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	draftAtom := uistate.UseBudgetBasisDraft()
	d := draftAtom.Get()
	mode := d.Mode
	if mode == "" {
		mode = budgeting.IncomeModeAll
	}

	onMode := ui.UseEvent(func(e ui.Event) {
		dd := draftAtom.Get()
		next := e.GetValue()
		// Switching to "by source" with nothing chosen yet seeds every source that
		// actually earned last month, so the basis starts equal to "all income" and the
		// user removes what to hold aside — a friendlier start than $0.
		if next == budgeting.IncomeModeCategories && len(dd.Cats) == 0 && props.SourceAmountErr == nil {
			var seed []string
			for _, s := range props.Sources {
				if s.Known && s.Minor > 0 {
					seed = append(seed, s.CategoryID)
				}
			}
			dd.Cats = seed
		}
		dd.Mode = next
		draftAtom.Set(dd)
	})
	onThreshold := ui.UseEvent(func(e ui.Event) {
		if v, ok := majorStrToMinor(e.GetValue(), props.Base); ok {
			dd := draftAtom.Get()
			dd.PaycheckMin = v
			draftAtom.Set(dd)
		}
	})
	onFixed := ui.UseEvent(func(e ui.Event) {
		if v, ok := majorStrToMinor(e.GetValue(), props.Base); ok {
			dd := draftAtom.Get()
			dd.Fixed = v
			draftAtom.Set(dd)
		}
	})
	onToggleRollover := ui.UseEvent(func() {
		dd := draftAtom.Get()
		dd.Rollover = !dd.Rollover
		draftAtom.Set(dd)
	})
	onToggleAverage := ui.UseEvent(func() {
		dd := draftAtom.Get()
		if dd.AvgMonths >= 2 {
			dd.AvgMonths = 0
		} else {
			dd.AvgMonths = 3
		}
		draftAtom.Set(dd)
	})
	// Include-all / hold-all-aside bulk actions for the source ledger.
	selectAllSources := ui.UseEvent(Prevent(func() {
		if props.SourceAmountErr != nil {
			return // keep manual selections when positive-source facts are unknown
		}
		dd := draftAtom.Get()
		var all []string
		for _, s := range props.Sources {
			if s.Known && s.Minor > 0 {
				all = append(all, s.CategoryID)
			}
		}
		dd.Cats = all
		draftAtom.Set(dd)
	}))
	holdAllSources := ui.UseEvent(Prevent(func() {
		dd := draftAtom.Get()
		dd.Cats = nil
		draftAtom.Set(dd)
	}))
	modeOnChange := OnChange(onMode)
	thresholdOnInput := OnInput(onThreshold)
	fixedOnInput := OnInput(onFixed)
	selectAllOnClick := OnClick(selectAllSources)
	holdAllOnClick := OnClick(holdAllSources)
	averageOnChange := OnChange(onToggleAverage)
	rolloverOnChange := OnChange(onToggleRollover)
	// toggleSource adds or removes one income category from the by-source draft.
	toggleSource := func(catID string) {
		dd := draftAtom.Get()
		out := dd.Cats[:0:0]
		found := false
		for _, id := range dd.Cats {
			if id == catID {
				found = true
				continue
			}
			out = append(out, id)
		}
		if !found {
			out = append(out, catID)
		}
		dd.Cats = out
		draftAtom.Set(dd)
	}

	var extra ui.Node = Fragment()
	switch mode {
	case budgeting.IncomeModePaychecks:
		extra = Label(css.Class("zbb-basis-extra"),
			Span(css.Class("zbb-basis-sub"), uistate.T("budgets.zbbPaycheckMin")),
			Input(css.Class("field"), Type("number"), Step("1"), Attr("min", "0"), Attr("data-testid", "budgets-zbb-paycheck-min"),
				Placeholder(uistate.T("budgets.zbbPaycheckMinPh")), thresholdOnInput, uiw.FieldValue(minorToMajorStr(d.PaycheckMin, props.Base))))
	case budgeting.IncomeModeFixed:
		extra = Label(css.Class("zbb-basis-extra"),
			Span(css.Class("zbb-basis-sub"), uistate.T("budgets.zbbFixedAmount")),
			Input(css.Class("field"), Type("number"), Step("1"), Attr("min", "0"), Attr("data-testid", "budgets-zbb-fixed-amount"),
				Placeholder(uistate.T("budgets.zbbFixedAmountPh")), fixedOnInput, uiw.FieldValue(minorToMajorStr(d.Fixed, props.Base))))
	}

	// The income-source ledger appears only in by-source mode.
	var sources ui.Node = Fragment()
	if mode == budgeting.IncomeModeCategories {
		selected := make(map[string]bool, len(d.Cats))
		for _, id := range d.Cats {
			selected[id] = true
		}
		included := 0
		for _, s := range props.Sources {
			if selected[s.CategoryID] {
				included++
			}
		}
		actions := Div(css.Class("zbb-sources-actions"),
			Button(css.Class("zbb-sources-act"), Type("button"), Disabled(props.SourceAmountErr != nil), Attr("data-testid", "budgets-zbb-select-all"), selectAllOnClick, uistate.T("budgets.incomeSelectAll")),
			Span(css.Class("zbb-sources-actsep"), Attr("aria-hidden", "true"), "·"),
			Button(css.Class("zbb-sources-act"), Type("button"), Attr("data-testid", "budgets-zbb-hold-all"), holdAllOnClick, uistate.T("budgets.incomeHoldAll")),
		)
		sources = budgetIncomeSourcesLedger(props, selected, toggleSource, actions, included)
	}

	return Div(css.Class("zbb-basis-wrap"),
		Div(css.Class("zbb-basis"), Attr("data-testid", "budgets-zbb-basis"),
			Label(css.Class("zbb-basis-main"),
				Span(css.Class("zbb-basis-label"), uistate.T("budgets.zbbBasisLabel")),
				Select(css.Class("field"), Attr("data-testid", "budgets-zbb-income-mode"), Attr("aria-label", uistate.T("budgets.zbbBasisLabel")), modeOnChange,
					Option(Value(budgeting.IncomeModeAll), SelectedIf(mode == budgeting.IncomeModeAll), uistate.T("budgets.zbbBasisAll")),
					Option(Value(budgeting.IncomeModePaychecks), SelectedIf(mode == budgeting.IncomeModePaychecks), uistate.T("budgets.zbbBasisPaychecks")),
					Option(Value(budgeting.IncomeModeCategories), SelectedIf(mode == budgeting.IncomeModeCategories), uistate.T("budgets.zbbBasisCategories")),
					Option(Value(budgeting.IncomeModeFixed), SelectedIf(mode == budgeting.IncomeModeFixed), uistate.T("budgets.zbbBasisFixed")),
				)),
			extra,
		),
		sources,
		// Average the basis over the last 3 months — steadier for irregular income.
		Label(css.Class("zbb-rollover"), Style(map[string]string{"cursor": "pointer"}),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgets-zbb-average"), averageOnChange}, checkedAttr(d.AvgMonths >= 2)...)...),
			Div(css.Class("row-main"),
				Span(uistate.T("budgets.zbbAverageToggle")),
				Span(css.Class("row-meta", tw.TextDim), uistate.T("budgets.zbbAverageHint")))),
		// Roll last month's unspent budget into this month's assignable pool.
		Label(css.Class("zbb-rollover"), Style(map[string]string{"cursor": "pointer"}),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgets-zbb-rollover"), rolloverOnChange}, checkedAttr(d.Rollover)...)...),
			Div(css.Class("row-main"),
				Span(uistate.T("budgets.zbbRolloverToggle")),
				Span(css.Class("row-meta", tw.TextDim), uistate.T("budgets.zbbRolloverHint")))),
	)
}

// budgetIncomeSourcesLedger renders the by-source basis: each income category as an
// include / hold-aside toggle with its last-month amount, under a live "budgeting
// against $X" total that equals the Income figure feeding the hero above. Pure node
// builder — each toggle row is its own component so the On* hook stays at a stable
// position (framework rule: never register hooks inside a variable-length loop).
func budgetIncomeSourcesLedger(props incomeBasisProps, selected map[string]bool, toggle func(string), actions ui.Node, included int) ui.Node {
	var sourceNotice ui.Node = Fragment()
	if props.SourceAmountErr != nil || props.SourceRankErr != nil {
		sourceNotice = P(css.Class("form-err"), Attr("role", "status"), Attr("data-testid", "budgets-zbb-source-unavailable"), uistate.T("budgets.incomplete"))
	}
	if len(props.Sources) == 0 {
		if props.SourceAmountErr != nil {
			return Div(css.Class("zbb-sources"), sourceNotice)
		}
		return Div(css.Class("zbb-sources"),
			P(css.Class("zbb-sources-empty", tw.TextDim), Attr("data-testid", "budgets-zbb-sources-empty"),
				uistate.T("budgets.incomeSourcesEmpty")))
	}
	rows := MapKeyed(props.Sources,
		func(s incomeSource) any { return s.CategoryID },
		func(s incomeSource) ui.Node {
			return ui.CreateElement(budgetIncomeSourceRow, budgetIncomeSourceRowProps{
				Source: s, Base: props.Base, Included: selected[s.CategoryID], OnToggle: toggle,
			})
		})
	incomeText := "—"
	if props.IncomeErr == nil {
		incomeText = fmtMoney(money.New(props.Income, props.Base))
	}
	return Div(css.Class("zbb-sources"), Attr("data-testid", "budgets-zbb-sources"),
		Div(css.Class("zbb-sources-head"),
			Span(css.Class("zbb-sources-title"), uistate.T("budgets.incomeSourcesTitle")),
			actions,
		),
		sourceNotice,
		Div(css.Class("zbb-sources-total"),
			Span(css.Class("zbb-sources-total-cap"), uistate.T("budgets.incomeSourcesTotalCap")),
			Span(css.Class("zbb-sources-total-val fig"), Attr("data-testid", "budgets-zbb-sources-total"),
				incomeText),
			Span(css.Class("zbb-sources-count", tw.TextDim), uistate.T("budgets.incomeSourcesCount", included, len(props.Sources))),
		),
		Div(css.Class("zbb-sources-rows"), rows),
	)
}

type budgetIncomeSourceRowProps struct {
	Source   incomeSource
	Base     string
	Included bool
	OnToggle func(string)
}

// budgetIncomeSourceRow is one income category in the by-source basis: an include
// checkbox, the source name, and its last-month amount. Included rows count toward the
// budget (positive tone); excluded rows are held aside (muted). Owns its own OnChange
// hook, so it's safe to render many of these in a MapKeyed loop.
func budgetIncomeSourceRow(props budgetIncomeSourceRowProps) ui.Node {
	onChange := ui.UseEvent(func() { props.OnToggle(props.Source.CategoryID) })
	rowCls := "zbb-source"
	if props.Included {
		rowCls += " is-in"
	}
	hasIncome := props.Source.Minor > 0
	var aside, amtNode ui.Node = Fragment(), Fragment()
	if !props.Source.Known {
		amtNode = Span(css.Class("zbb-source-amt zbb-source-none"), Attr("data-testid", "budgets-zbb-source-amount-"+props.Source.CategoryID), uistate.T("calculation.unavailable"))
	} else if hasIncome {
		// A real source: show its amount, and a "held aside" tag when excluded.
		if !props.Included {
			aside = Span(css.Class("zbb-source-aside"), uistate.T("budgets.incomeSourceHeldAside"))
		}
		amtNode = Span(css.Class("zbb-source-amt fig"), Attr("data-testid", "budgets-zbb-source-amount-"+props.Source.CategoryID), fmtMoney(money.New(props.Source.Minor, props.Base)))
	} else {
		// No income last month: say so plainly, so it's not confused with a source the
		// user simply hasn't checked (no bare dash, no misleading "held aside" tag).
		amtNode = Span(css.Class("zbb-source-amt zbb-source-none"), Attr("data-testid", "budgets-zbb-source-amount-"+props.Source.CategoryID), uistate.T("budgets.incomeSourceNoHistory"))
	}
	return Label(ClassStr(rowCls), Attr("data-testid", "budgets-zbb-source-"+props.Source.CategoryID),
		Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("aria-label", props.Source.Name), OnChange(onChange)}, checkedAttr(props.Included)...)...),
		Span(css.Class("zbb-source-name"), props.Source.Name),
		aside,
		amtNode,
	)
}

// BudgetBasisBody is the "Income to budget with" modal body (mounted at the shell root
// by BudgetBasisHost). It reads the live store for the income-source menu, then previews
// the income the STAGED DRAFT would resolve to — so the running total reflects unsaved
// edits — and renders the income-basis control. Reachable from EVERY budgeting method,
// not just zero-based. Self-contained (no props), matching the AutoBudgetBody convention.
func BudgetBasisBody(_ struct{}) ui.Node {
	if appstate.Default == nil {
		return Fragment()
	}
	return ui.CreateElement(budgetBasisBodyReady, struct{}{})
}

func budgetBasisBodyReady(_ struct{}) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	pr := uistate.UsePrefs().Get()
	d := uistate.UseBudgetBasisDraft().Get()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}

	// Everything the modal shows reflects the STAGED draft (its mode, chosen sources, and
	// averaging window), so the running total and per-source amounts preview unsaved edits
	// and use the same window. Source rows truncate individually; the canonical
	// combined preview may retain a minor-unit rounding remainder.
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// C531: anchor the preview on the month the PAGE is showing, not on today, so
	// the figure approved here is the figure the page will use. Paging back to a
	// closed month previews that month.
	ms := incomeBasisMonth(uistate.UsePeriod().Get(), pr.WeekStartWeekday())
	mode := d.Mode
	if mode == "" {
		mode = budgeting.IncomeModeAll
	}
	draftIncome, incomeErr := resolveIncomeBasis(app, mode, d.PaycheckMin, d.Fixed, d.Cats, d.AvgMonths, ms, base, rates)
	sources := computeIncomeSources(app, rates, ms, d.AvgMonths)
	var incomeNotice ui.Node = Fragment()
	if incomeErr != nil {
		slog.Error("preview selected budget income", "error", incomeErr)
		incomeNotice = P(css.Class("form-err"), Attr("role", "status"), Attr("data-testid", "budgets-zbb-income-unavailable"), uistate.T("budgets.incomplete"))
	}

	return Div(css.Class("zbb-basis-modal"),
		P(css.Class("zbb-basis-modal-help", tw.TextDim), uistate.T("budgets.basisModalHelp")),
		incomeNotice,
		ui.CreateElement(budgetIncomeBasisControl, incomeBasisProps{Base: base, Sources: sources.Rows, SourceAmountErr: sources.AmountErr, SourceRankErr: sources.RankErr, Income: draftIncome, IncomeErr: incomeErr}),
	)
}

// budgetSavingsWidget is the zero-based view's "Savings & investments" tile: every
// savings/investment account with an inline monthly savings budget (Account.
// MonthlySavings) that counts toward the assigned total, a smart "Spread leftover"
// button that splits this month's unassigned money evenly across those accounts, and —
// for an account that funds a goal — the plan-vs-reality timeline plus a one-click
// "Sync to goal" that writes the monthly amount back to the goal. Zero-based only.
func budgetSavingsWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	memberLens := uistate.UseMemberLens()
	activeMemberID := ""
	if len(memberLens) == 1 {
		activeMemberID = memberLens[0]
	}
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()

	// All hooks run before the mode early-return so hook order stays stable when the
	// budgeting method changes mid-session. The spread handler recomputes the view at
	// click time so it always splits the current leftover across the current accounts.
	nav := router.UseNavigate()
	// Collapsed by default: this is a five-row account form at the BOTTOM of the
	// surface, below Plan the year. Open, it is the tallest thing on the page for a
	// figure most months never change; closed, it states its own total and waits.
	open := ui.UseState(false)
	toggleOpen := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	goToGoals := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/goals")) }))
	goToAccounts := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/accounts")) }))
	onSpread := ui.UseEvent(Prevent(func() {
		vv := computeBudgetView(app, activeMemberID, vw, pr, false)
		leftover, complete := budgetSpreadAmount(vv)
		if !complete {
			return
		}
		rates := currency.Rates{Base: vv.Base, Rates: app.Settings().FXRates}
		spreadLeftoverAcrossSavings(app, vv.SavingsAccts, leftover, vv.Base, rates)
	}))
	spreadOnClick := OnClick(onSpread)
	toggleOnClick := OnClick(toggleOpen)
	accountsOnClick := OnClick(goToAccounts)
	goalsOnClick := OnClick(goToGoals)

	v := computeBudgetView(app, activeMemberID, vw, pr, false)
	if v.Method != budgeting.MethodZeroBased || len(v.Statuses) == 0 {
		return Fragment() // only meaningful in zero-based mode with budgets present
	}

	leftover := int64(0)
	if amount, complete := budgetSpreadAmount(v); complete {
		leftover = amount
	}

	var body ui.Node
	if len(v.SavingsAccts) == 0 {
		body = P(css.Class("empty", tw.TextDim), Attr("data-testid", "budgets-savings-empty"), uistate.T("budgets.savingsEmpty"))
	} else {
		rows := MapKeyed(v.SavingsAccts,
			func(a savingsAcct) any { return a.AccountID },
			func(a savingsAcct) ui.Node {
				return ui.CreateElement(budgetSavingsAcctRow, budgetSavingsAcctRowProps{App: app, Acct: a, Base: v.Base})
			})
		body = Div(css.Class("zbb-savings-rows"), rows)
	}

	head := Div(css.Class("zbb-savings-head"),
		Span(css.Class("zbb-savings-title"), uistate.T("budgets.savingsTitle")),
		Span(css.Class("zbb-savings-total fig"), uistate.T("budgets.savingsPerMonthTotal", fmtMoney(money.New(v.SavingsAssigned, v.Base)))))

	// The smart button appears only when there's unassigned money to place and at least
	// one account to place it in — otherwise there's nothing to spread.
	var spreadBtn ui.Node = Fragment()
	if leftover > 0 && len(v.SavingsAccts) > 0 && budgetSpreadReady(v) {
		spreadBtn = Button(css.Class("btn btn-sm btn-accent zbb-savings-spread"), Type("button"),
			Attr("data-testid", "budgets-savings-spread"), Attr("aria-label", uistate.T("budgets.savingsSpreadAria")),
			Attr("title", uistate.T("budgets.savingsSpreadTitle")), spreadOnClick,
			uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("budgets.savingsSpread", fmtMoney(money.New(leftover, v.Base)))))
	}

	// The fold header mirrors "Plan the year" directly below it, so the two
	// collapsible sections that close this surface read as one pattern. The hint
	// carries the monthly total, which is the whole answer most visits need — you
	// only open this to CHANGE an amount.
	caretCls := "budget-fold-caret"
	foldAria := uistate.T("budgets.savingsShowAria")
	if open.Get() {
		caretCls += " is-open"
		foldAria = uistate.T("budgets.savingsHideAria")
	}
	hint := uistate.T("budgets.savingsHintNone")
	if v.SavingsAssigned > 0 {
		hint = uistate.TN("budgets.savingsHintOne", "budgets.savingsHintMany",
			len(v.SavingsAccts), fmtMoney(money.New(v.SavingsAssigned, v.Base)))
	}
	fold := Div(css.Class("budget-fold-head"),
		Button(css.Class("budget-fold-toggle"), Type("button"), Attr("data-testid", "budgets-savings-toggle"),
			Attr("aria-expanded", ariaBool(open.Get())), Attr("aria-label", foldAria), toggleOnClick,
			Span(ClassStr(caretCls), Attr("aria-hidden", "true"),
				uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("budget-fold-toggle-label"), uistate.T("budgets.savingsSectionTitle")),
			Span(css.Class("budget-fold-toggle-hint"), hint),
		),
	)
	// The fold header IS the section title, exactly as it is for Plan the year — so
	// the body is a plain container rather than an EntityListSection, which would
	// print "Savings & investments" a second time directly under the toggle.
	var foldBody ui.Node = Fragment()
	if open.Get() {
		foldBody = Fragment(
			head,
			P(css.Class("muted", tw.Text13), uistate.T("budgets.savingsDesc")),
			body,
			Div(css.Class("zbb-savings-foot"),
				spreadBtn,
				Div(css.Class("zbb-savings-foot-links"),
					Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-savings-accounts-link"), accountsOnClick,
						uiw.Icon(icon.Accounts, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.savingsManageAccounts"))),
					Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-savings-goals-link"), goalsOnClick,
						uiw.Icon(icon.Goals, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.savingsManageGoals"))))),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-savings", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("zbb-savings budget-fold"), fold, foldBody),
	})
}

func budgetSpreadReady(v budgetView) bool {
	if len(v.Incomplete) > 0 || v.SavingsIncomplete {
		return false
	}
	for _, status := range v.Statuses {
		if status.Error != nil {
			return false
		}
	}
	return true
}

func budgetSpreadAmount(v budgetView) (int64, bool) {
	if !budgetSpreadReady(v) {
		return 0, false
	}
	pool, err := money.Add(v.BannerIncome, v.RolledOver)
	if err != nil {
		return 0, false
	}
	assigned, err := money.Add(v.TotalLimit, v.SavingsAssigned)
	if err != nil {
		return 0, false
	}
	leftover, err := money.Sub(pool, assigned)
	if err != nil {
		return 0, false
	}
	return leftover, true
}

type budgetSavingsAcctRowProps struct {
	App  *appstate.App
	Acct savingsAcct
	Base string
}

// budgetSavingsAcctRow is one savings/investment account in the savings section: its
// name and type, an inline number input that sets the account's monthly savings budget
// (Account.MonthlySavings), and — when it funds a goal — a plan-vs-reality line with a
// "Sync to goal" button that writes the monthly amount into the goal's planned monthly
// contribution. Owns its own edit + sync hooks.
func budgetSavingsAcctRow(props budgetSavingsAcctRowProps) ui.Node {
	app := props.App
	sa := props.Acct
	cur := sa.Currency
	if cur == "" {
		cur = props.Base
	}

	// commit writes the account + BumpDataRevision (re-renders the whole budgets surface
	// and re-runs the heavy computeBudgetView for every tile) + RequestPersist (serialize
	// the dataset). Doing that per keystroke made typing lag badly, so it's DEBOUNCED:
	// each keystroke reschedules the commit ~300ms out, so a burst of typing collapses
	// into one commit + one recompute + one persist, and the total updates live once you
	// pause. onCommit flushes the pending debounce and commits immediately on blur/Enter,
	// so tabbing away is instant. The no-change guard skips the work when nothing changed.
	commit := func(s string) {
		v, ok := majorStrToMinor(s, cur)
		if !ok {
			return
		}
		ac, found := findAccount(app, sa.AccountID)
		if !found || (ac.MonthlySavings.Amount == v && ac.MonthlySavings.Currency == cur) {
			return
		}
		ac.MonthlySavings = money.New(v, cur)
		if err := app.PutAccount(ac); err == nil {
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostNotice(uistate.T("common.savedToast"), false) // QA L2
		}
	}
	dbKey := "acct-savings:" + sa.AccountID
	onEdit := ui.UseEvent(func(e ui.Event) {
		s := e.GetValue()
		debounce.Call(dbKey, 300*time.Millisecond, func() { commit(s) })
	})
	onCommit := ui.UseEvent(func(e ui.Event) {
		debounce.Flush(dbKey)
		commit(e.GetValue())
	})
	onSync := ui.UseEvent(Prevent(func() {
		g, found := findGoal(app, sa.GoalID)
		if !found {
			return
		}
		// Written in the goal's currency (computed upstream) so the goal's pace math
		// stays currency-consistent even when the account currency differs.
		g.MonthlyContribution = money.New(sa.SyncMinor, sa.SyncCurrency)
		if err := app.PutGoal(g); err == nil {
			uistate.BumpDataRevision()
			uistate.RequestPersist()
		}
	}))

	val := minorToMajorStr(sa.Monthly, cur)

	var goalNode ui.Node = Fragment()
	if sa.HasGoal {
		goalNode = budgetSavingsGoalNode(sa, onSync)
	}

	return Div(css.Class("zbb-savings-row"),
		Div(css.Class("zbb-savings-main"),
			Div(css.Class("zbb-savings-id"),
				Span(css.Class("zbb-savings-name"), sa.Name),
				Span(css.Class("zbb-savings-type", tw.TextDim), sa.Type)),
			Div(css.Class("zbb-savings-edit"),
				Span(css.Class("zbb-savings-cur", tw.TextDim), currency.Symbol(cur)),
				Input(css.Class("field zbb-savings-input fig"), Type("number"), Step("1"), Attr("min", "0"),
					Attr("data-testid", "budgets-savings-amt-"+sa.AccountID), Attr("aria-label", uistate.T("budgets.savingsMonthlyAria", sa.Name)), OnInput(onEdit), OnChange(onCommit), uiw.FieldValue(val)),
				Span(css.Class("zbb-savings-per", tw.TextDim), uistate.T("budgets.savingsPerMonth")))),
		goalNode,
	)
}

// budgetSavingsGoalNode renders the plan-vs-reality line for an account that funds a
// goal: how the account's monthly savings rate lands against the goal's planned
// timeline (toned green for on-plan/ahead, amber for behind), with a "Sync to goal"
// action — or a "Synced ✓" marker when the goal already carries this amount. Pure node
// builder; the row owns the sync handler hook and passes it in.
func budgetSavingsGoalNode(sa savingsAcct, onSync ui.Handler) ui.Node {
	syncOnClick := OnClick(onSync)
	// Every linked goal is already met — surface the win rather than a projection.
	if sa.GoalComplete {
		return Div(css.Class("zbb-savings-goal is-ontrack"),
			Span(css.Class("zbb-savings-goal-name"), uistate.T("budgets.savingsFunds", sa.GoalName)),
			Span(css.Class("zbb-savings-goal-time"), uistate.T("budgets.savingsFunded")))
	}
	if sa.GoalError != nil {
		return Div(css.Class("zbb-savings-goal"),
			Span(css.Class("zbb-savings-goal-name"), uistate.T("budgets.savingsFunds", sa.GoalName)),
			Span(css.Class("zbb-savings-goal-time"), Attr("role", "status"), Attr("data-testid", "budgets-savings-goal-unavailable-"+sa.AccountID),
				uistate.T("budgets.savingsGoalUnavailable")))
	}
	// No amount set yet — invite one, no projection to make.
	if sa.Monthly <= 0 {
		return Div(css.Class("zbb-savings-goal"),
			Span(css.Class("zbb-savings-goal-name", tw.TextDim), uistate.T("budgets.savingsFundsSet", sa.GoalName)))
	}
	var tone, phrase string
	switch {
	case sa.RateMonths <= 0:
		// Can't project a finish (amount rounds to nothing, or a needed FX rate is missing).
		phrase = uistate.T("budgets.savingsNoProject")
	case sa.PlannedMonths <= 0:
		// Undated goal — say so, since there's no plan to compare against.
		phrase = uistate.T("budgets.savingsRateOnly", sa.RateMonths)
	case sa.DeltaMonths == 0:
		tone, phrase = "is-ontrack", uistate.T("budgets.savingsOnPlan", sa.PlannedMonths)
	case sa.DeltaMonths > 0:
		tone, phrase = "is-behind", uistate.T("budgets.savingsBehind", sa.PlannedMonths, sa.RateMonths, sa.DeltaMonths)
	default:
		tone, phrase = "is-ahead", uistate.T("budgets.savingsAhead", sa.PlannedMonths, sa.RateMonths, -sa.DeltaMonths)
	}
	// Sync only when there's a money-correct amount to write (a failed FX conversion
	// leaves SyncMinor at 0, so we offer no button rather than persist a wrong figure).
	var action ui.Node = Fragment()
	switch {
	case sa.Synced:
		action = Span(css.Class("zbb-savings-synced", tw.TextDim), uistate.T("budgets.savingsSynced"))
	case sa.SyncMinor > 0:
		action = Button(css.Class("btn btn-xs zbb-savings-sync"), Type("button"),
			Attr("data-testid", "budgets-savings-sync-"+sa.AccountID), Attr("title", uistate.T("budgets.savingsSyncTitle", sa.GoalName)),
			syncOnClick, uistate.T("budgets.savingsSync"))
	}
	cls := "zbb-savings-goal"
	if tone != "" {
		cls += " " + tone
	}
	return Div(css.Class(cls),
		Span(css.Class("zbb-savings-goal-name"), uistate.T("budgets.savingsFunds", sa.GoalName)),
		Span(css.Class("zbb-savings-goal-time"), phrase),
		If(sa.MoreGoals > 0, Span(css.Class("zbb-savings-more", tw.TextDim), Attr("title", uistate.T("budgets.savingsMoreTitle")), uistate.T("budgets.savingsMore", sa.MoreGoals))),
		action)
}

// findAccount returns the current stored account by id (false when missing), so a row's
// edit handler mutates a fresh copy rather than a possibly-stale captured value.
func findAccount(app *appstate.App, id string) (domain.Account, bool) {
	for _, a := range app.Accounts() {
		if a.ID == id {
			return a, true
		}
	}
	return domain.Account{}, false
}

// findGoal returns the current stored goal by id (false when missing).
func findGoal(app *appstate.App, id string) (domain.Goal, bool) {
	for _, g := range app.Goals() {
		if g.ID == id {
			return g, true
		}
	}
	return domain.Goal{}, false
}

// spreadLeftoverAcrossSavings raises each savings/investment account's monthly savings
// budget by an even share of the month's leftover (To-Assign), so the smart button
// drives To-Assign toward $0. The base-currency leftover is split evenly (the rounding
// remainder goes to the earliest accounts) and each share is converted into the
// account's own currency before being added. Persists once when anything changed.
func spreadLeftoverAcrossSavings(app *appstate.App, accts []savingsAcct, leftoverBase int64, base string, rates currency.Rates) {
	n := int64(len(accts))
	if n == 0 || leftoverBase <= 0 {
		return
	}
	share := leftoverBase / n
	remainder := leftoverBase - share*n
	changed := false
	for i, sa := range accts {
		add := share
		if int64(i) < remainder {
			add++ // hand the leftover minor units to the first few accounts
		}
		if add <= 0 {
			continue
		}
		ac, ok := findAccount(app, sa.AccountID)
		if !ok {
			continue
		}
		cur := ac.Currency
		if cur == "" {
			cur = base
		}
		// Convert the base-currency share into the account's own currency. If the rate
		// is missing, skip this account rather than persist the base figure as if it
		// were the account's currency (which would silently corrupt its saved amount).
		addAcct := add
		if cur != base {
			conv, err := currency.ConvertBetween(add, base, cur, rates)
			if err != nil {
				continue
			}
			addAcct = conv
		}
		updated, err := money.Add(ac.MonthlySavings.Amount, addAcct)
		if err != nil {
			continue
		}
		ac.MonthlySavings = money.New(updated, cur)
		if err := app.PutAccount(ac); err == nil {
			changed = true
		}
	}
	if changed {
		uistate.BumpDataRevision()
		uistate.RequestPersist()
	}
}

// budgetFundSetAsideNode shows the household's total monthly sinking-fund commitment,
// when non-zero — placed after the income context so it reads as a committed slice.
// freePool is the method's still-unallocated money (To-Assign for zero-based, income −
// budgeted for simple) and hasPool whether the method has such a pool at all (envelope
// doesn't). When the commitment exceeds the pool the footnote ESCALATES to an amber
// planning alert naming the shortfall — the one state where this line demands a
// decision rather than records a fact (design critique #4: the requirement was stated
// but never checked against what's actually left).
// budgetIssuesRailProps feeds the summary's single collapsed issues rail: the
// computed view, the over-assignment magnitude (0 when the plan adds up), and
// the sinking-fund pool check inputs.
type budgetIssuesRailProps struct {
	View          budgetView
	SmartSettings smart.Settings
	OverAssigned  int64
	FundFree      int64
	FundHasPool   bool
	// Hist marks a wholly-past view window (UX-05): counts read in the past tense
	// ("ended over budget") and open follow-ups count as an item to review.
	Hist bool
	// FollowUps is the number of open to-dos linked to any budget.
	FollowUps int
}

// budgetIssuesRail is the audit-P0 replacement for the summary's stacked warning
// banners: over-assignment, overspent categories, and the sinking-fund shortfall
// collapse into ONE "N issues need attention" row that expands to the detailed
// alerts on click. When over-assigned, the header carries a "Resolve $X" figure —
// the page's actual to-do. With no issues it reduces to the quiet sinking-fund
// footnote (or a near-limit pill), never an alert. Its own component so the
// expand state + navigate hooks sit at stable positions.
func budgetIssuesRail(props budgetIssuesRailProps) ui.Node {
	v := props.View
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	nav := router.UseNavigate()
	goAllocate := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/allocate")) }))
	goTodos := ui.UseEvent(Prevent(func() {
		uistate.SetTodoFilterLink(uistate.TodoLinkBudget)
		uistate.SetTodoFilterLinkID("")
		nav.Navigate(uistate.RoutePath("/todo"))
	}))
	allocateOnClick := OnClick(goAllocate)
	toggleOnClick := OnClick(toggle)
	todosOnClick := OnClick(goTodos)

	// July-19 review #1: over/near BUDGETS are owned by the single top review queue
	// (budget-attention) now, so this rail no longer double-reports them. It carries only
	// the PLAN-level concerns the queue can't show — over-assignment, a sinking-fund
	// shortfall, and (on a closed month) open follow-ups — so the page has one decision
	// path for budget health and a separate, quiet note for planning issues.
	fundShort := v.TotalFundSetAside > 0 && props.FundHasPool && v.TotalFundSetAside > props.FundFree
	count := 0
	if props.OverAssigned > 0 {
		count++
	}
	if fundShort {
		count++
	}
	// Open follow-ups are unfinished business worth surfacing in ANY period. The
	// count used to be gated on a closed month while the detail row below was not,
	// so a live period could expand to a follow-ups row the header never mentioned
	// — which is exactly the "predict before clicking" the ticket asks for (C588).
	if props.FollowUps > 0 {
		count++
	}

	if count == 0 {
		// No plan-level issue: at most the quiet sinking-fund set-aside footnote. Over/near
		// budgets are reported by the top review queue, never a second time here.
		return Div(
			If(v.TotalFundSetAside > 0, P(css.Class("budget-sub"), Attr("data-testid", "budgets-fund-setaside"),
				uistate.T("budgets.fundSetAside", fmtMoney(money.New(v.TotalFundSetAside, v.Base))))),
		)
	}

	// C588: the header NAMES what is in the rail. "2 items to review from this
	// period" told the reader nothing about whether opening it led to
	// transactions, budgets, tasks or goals — three different destinations hide
	// behind it, and the wording read like a transaction review inbox, which is
	// the one thing it never is.
	title := budgetIssuesTitle(props.OverAssigned > 0, fundShort, props.FollowUps > 0, props.Hist)
	// Over-assignment is the page's to-do, so Resolve is a REAL button that goes to
	// Allocate. It used to be a Span inside the header button: styled as the loudest
	// call to action on the page — a filled danger pill — while clicking it only
	// expanded the disclosure it sat in. An affordance that looks like the primary
	// action has to BE one (Cam, 2026-08-31). It is also a sibling of the toggle
	// rather than a child, since a button inside a button is invalid markup.
	var resolve ui.Node = Fragment()
	if props.OverAssigned > 0 {
		resolve = Button(css.Class("budget-rail-resolve"), Type("button"),
			Attr("data-testid", "budgets-rail-resolve"), Title(uistate.T("budgets.resolveTitle")),
			allocateOnClick,
			uistate.T("budgets.resolveAmount", fmtMoney(money.New(props.OverAssigned, v.Base))))
	}
	hint := uistate.T("budgets.issuesShow")
	chev := icon.ChevronDown
	if open.Get() {
		hint = uistate.T("budgets.issuesHide")
		chev = icon.ChevronUp
	}
	// Order matters: title (claiming the slack) → Resolve → caret. The caret is the
	// bar's rightmost edge because that is where a disclosure lives; leaving it
	// inside the flex:1 toggle parked it in the middle of the row, to the LEFT of
	// Resolve, which read as a stray glyph rather than "this opens".
	//
	// It is a second control on the same handler rather than part of the toggle, so
	// it is hidden from assistive tech and taken out of the tab order — the labelled
	// toggle beside it already announces and operates the disclosure.
	railClass := "budget-issues-rail"
	followUpsOnly := props.FollowUps > 0 && props.OverAssigned == 0 && !fundShort
	if followUpsOnly {
		railClass += " is-followups-only"
	}
	header := Div(css.Class(railClass),
		Button(css.Class("budget-issues-toggle"), Type("button"),
			Attr("data-testid", "budgets-issues-rail"), Attr("aria-expanded", ariaBool(open.Get())),
			Title(hint), toggleOnClick,
			If(!followUpsOnly, uiw.Icon(icon.AlertTriangle, css.Class("budget-issues-icon", tw.ShrinkO, tw.W4, tw.H4))),
			Span(css.Class("budget-issues-title"), title),
		),
		resolve,
		Button(css.Class("budget-issues-chevbtn"), Type("button"),
			Attr("aria-hidden", "true"), Attr("tabindex", "-1"), toggleOnClick,
			uiw.Icon(chev, css.Class("budget-issues-chev", tw.ShrinkO, tw.W35, tw.H35)),
		),
	)

	if !open.Get() {
		return Div(css.Class("budget-issues-wrap"), header)
	}

	var rows []any
	if props.OverAssigned > 0 {
		rows = append(rows, Div(css.Class("budget-issue-row"), Attr("data-testid", "budgets-issue-overassigned"),
			Div(css.Class("budget-issue-main"),
				Span(css.Class("budget-issue-title"), uistate.T("budgets.issueOverAssigned", fmtMoney(money.New(props.OverAssigned, v.Base)))),
				Span(css.Class("budget-issue-body"), uistate.T("budgets.issueOverAssignedBody")),
			),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-resolve-alloc"),
				Title(uistate.T("budgets.issueReviewAllocTitle")), allocateOnClick,
				uistate.T("budgets.issueGoAllocate")),
		))
	}
	if props.FollowUps > 0 {
		fuTxt := uistate.T("budgets.followUpsCountOne")
		if props.FollowUps != 1 {
			fuTxt = uistate.T("budgets.followUpsCount", props.FollowUps)
		}
		rows = append(rows, Div(css.Class("budget-issue-row"), Attr("data-testid", "budgets-issue-followups"),
			Div(css.Class("budget-issue-main"),
				Span(css.Class("budget-issue-title"), fuTxt),
				Span(css.Class("budget-issue-body"), uistate.T("budgets.followUpsRowBody")),
			),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-followups-view"),
				Title(uistate.T("budgets.followUpsRowView")), todosOnClick, uistate.T("budgets.issueGoTodo")),
		))
	}
	if fundShort {
		rows = append(rows, ui.CreateElement(budgetFundShortAlert, budgetFundShortProps{
			SetAside: v.TotalFundSetAside, Free: props.FundFree, Base: v.Base,
		}))
	}
	detail := append([]any{css.Class("budget-issues-detail"), Attr("data-testid", "budgets-issues-detail")}, rows...)
	return Div(css.Class("budget-issues-wrap"), header, Div(detail...))
}

// budgetFundShortProps feeds the sinking-fund shortfall alert: the month's total
// set-aside requirement, the still-unallocated pool it must fit into, and the base
// currency both are denominated in.
type budgetFundShortProps struct {
	SetAside int64
	Free     int64
	Base     string
}

// budgetFundShortAlert is the sinking-fund shortfall warning: an amber alert row (not
// danger red — nothing is overspent yet, this is a plan that doesn't add up) naming the
// gap, what the funds need, and what's actually free, with a jump to the goals page
// where sinking funds are managed. Own component so its navigate hook sits at a stable
// position even though the alert renders conditionally.
func budgetFundShortAlert(props budgetFundShortProps) ui.Node {
	nav := router.UseNavigate()
	goToGoals := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/goals")) }))
	// With money still free, the shortfall is the uncovered slice; with NOTHING free
	// the whole set-aside is short — never setAside − negativePool, which would fold
	// the over-allocation (already reported by the over-income chip) in twice and
	// claim a shortfall bigger than the need itself.
	short := props.SetAside - props.Free
	need := fmtMoney(money.New(props.SetAside, props.Base))
	body := uistate.T("budgets.fundShortBody", need, fmtMoney(money.New(props.Free, props.Base)))
	if props.Free <= 0 {
		short = props.SetAside
		body = uistate.T("budgets.fundShortBodyNone", need)
	}
	return Div(css.Class("budget-fundshort"), Attr("role", "status"), Attr("data-testid", "budgets-fund-short"),
		uiw.Icon(icon.AlertTriangle, css.Class("budget-fundshort-icon", tw.ShrinkO, tw.W4, tw.H4)),
		Div(css.Class("budget-fundshort-main"),
			Span(css.Class("budget-fundshort-title"), uistate.T("budgets.fundShortTitle", fmtMoney(money.New(short, props.Base)))),
			Span(css.Class("budget-fundshort-body"), body),
		),
		Button(css.Class("btn btn-sm budget-fundshort-btn"), Type("button"), Attr("data-testid", "budgets-fund-short-review"),
			Title(uistate.T("budgets.fundShortReviewTitle")), OnClick(goToGoals), uistate.T("budgets.issueGoGoals")),
	)
}

// --- budget-toolbar --------------------------------------------------------------

// budgetToolbarWidget wraps the toolbar content in its own engine tile. Only the
// FLEX methodology surface still places it standalone (the flex view has no list
// tile to host it); the normal surface renders budgetToolbarContent inside the
// list card's head row instead (B1: no orphan control band above the budgets).
func budgetToolbarWidget(props budgetToolbarProps) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: ui.CreateElement(budgetToolbarContent, props),
	})
}

// budgetMemberScopeNote names the active household member lens beside the list.
// budgetCount matches the scoped budget rows; totalCount shows how many budgets
// exist across the household. Amounts remain household totals in this view.
func budgetMemberScopeNote(app *appstate.App, memberID string, budgetCount int) ui.Node {
	if memberID == "" {
		return Fragment()
	}
	var name string
	for _, member := range app.Members() {
		if member.ID == memberID {
			name = member.Name
			break
		}
	}
	if name == "" {
		return Fragment()
	}
	return P(css.Class("bud2-member-share"), Attr("data-testid", "budgets-member-scope"),
		uistate.T("bud2.memberScopeBR2TB", name, budgetCount, len(app.Budgets()), name))
}

// budgetToolbarContent is the actions cluster: the "Budget settings" popover (the
// in-context methodology picker, sort, density, last-month toggle, Auto-budget,
// month-close, sweep config, bulk adjust) and the primary "Add budget" button. It
// writes the shared method setting so the summary/list/formula tiles react in step.
func budgetToolbarContent(props budgetToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	memberLens := uistate.UseMemberLens()
	activeMemberID := ""
	if len(memberLens) == 1 {
		activeMemberID = memberLens[0]
	}
	visibleBudgets := 0
	for _, b := range app.Budgets() {
		if ownerVisibleTo(b.OwnerID, activeMemberID) {
			visibleBudgets++
		}
	}
	method := budgeting.ParseMethodology(app.Settings().BudgetMethodology)
	sortAtom := uistate.UseBudgetSort()
	onSort := ui.UseEvent(func(e ui.Event) { sortAtom.Set(e.GetValue()) })
	impact := budgeting.MethodChangeImpact(app.Budgets())
	onMethod := budgetMethodChangeHandler(app, impact)
	return Div(css.Class("filter-toolbar budgets-tb"),
		Div(css.Class("filter-toolbar-actions"),
			ui.CreateElement(budgetToolbarSettings, budgetToolbarSettingsProps{App: app, Method: method, Sort: sortAtom.Get(), Impact: impact, OnMethod: onMethod, OnSort: onSort}),
			ui.CreateElement(budgetToolbarTools, budgetToolbarToolsProps{HasBudgets: visibleBudgets > 0}),
			If(visibleBudgets > 0, ui.CreateElement(budgetToolbarDensity, budgetToolbarDensityProps{SeedCompact: visibleBudgets > 6})),
			If(visibleBudgets > 0, ui.CreateElement(budgetToolbarAdd, struct{}{})),
		),
	)
}

func budgetMethodChangeHandler(app *appstate.App, impact budgeting.MethodImpact) ui.Handler {
	return ui.UseEvent(func(e ui.Event) {
		next := e.GetValue()
		cur := app.Settings()
		if next == cur.BudgetMethodology {
			return
		}
		apply := func() bool {
			s := app.Settings()
			s.BudgetMethodology = next
			if budgetWriteFailed("switch budgeting method", app.PutSettings(s)) {
				uistate.BumpDataRevision()
				return false
			}
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			return true
		}
		commit := func() {
			if !apply() {
				return
			}
			uistate.PostUndoable(uistate.T("budgets.methodGlobalSaved", methodDisplayName(next), uistate.T(budgetCountKey("bc4b4.budgetCount", impact.Following), impact.Following)))
		}
		if impact.Overriding > 0 {
			uistate.ConfirmModalLabeled(
				uistate.T("budgets.methodGlobalConfirm", methodDisplayName(next), uistate.T(budgetCountKey("bc4b4.budgetCount", impact.Following), impact.Following), uistate.T(budgetCountKey("bc4b4.budgetCount", impact.Overriding), impact.Overriding)),
				uistate.T("budgets.methodGlobalConfirmAction"), false,
				func(ok bool) {
					if ok {
						commit()
					} else {
						uistate.BumpDataRevision()
					}
				},
			)
			return
		}
		commit()
	})
}

// --- budget-list -----------------------------------------------------------------

// budgetListWidget is the rows tile: the health-sorted BudgetRow list inside an
// EntityListSection (with the smart section action), or the first-run empty-state CTA.
// It owns the per-row callbacks, the "Cover…" funding sources, and the drill-to-
// transactions navigation.
func budgetListWidget(props budgetListProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	memberLens := uistate.UseMemberLens()
	activeMemberID := ""
	if len(memberLens) == 1 {
		activeMemberID = memberLens[0]
	}
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	showLastMonth := uistate.UseBudgetsLastMonth().Get()
	view := computeBudgetView(app, activeMemberID, vw, pr, showLastMonth)
	sortKey := uistate.UseBudgetSort().Get()
	if sortKey != uistate.BudgetSortHealth {
		sorted := append([]budgeting.Status(nil), view.Statuses...)
		sortBudgetStatuses(sorted, sortKey, view.CatName)
		view.Statuses = sorted
	}
	smartSettings := uistate.LoadSmartSettings()
	search := ui.UseState("")
	attention := uistate.UseBudgetAttention()
	compact := uistate.UseBudgetDensitySeeded(len(view.Statuses) > 6).Get() == uistate.BudgetDensityCompact
	viewTransactions := func(scope budgeting.Scope, from, to string) {
		filter := budgetScopeFilter(scope)
		filter.From, filter.To = from, to
		filter = filter.Normalize()
		txFilter.Set(filter)
		uistate.PersistTxFilter(filter)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	drillFrom := func(status budgeting.Status, scope budgeting.Scope, from, to string) {
		applied := budgetScopeFilter(scope)
		uistate.SetBudgetDrill(uistate.BudgetDrill{
			BudgetID: status.Budget.ID, Name: budgetTitle(status.Budget.Name, view.CatName[status.Budget.CategoryID]),
			Spent: fmtMoney(status.Spent), Descendants: scope.HasDescendants(status.Budget), Tags: len(scope.Tags) > 0,
			Scope: uistate.BudgetDrillScope{Category: applied.Category, Categories: applied.Categories, Tags: applied.Tags},
		})
		viewTransactions(scope, from, to)
	}
	var body ui.Node
	switch {
	case len(view.Statuses) == 0 && len(view.Incomplete) > 0:
		body = budgetIncompleteNotice(view.Incomplete)
	case len(view.Statuses) == 0:
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("budgets.empty"), CTALabel: uistate.T("budgets.addFirst"), AddTarget: "budget", Icon: icon.Budgets})
	default:
		body = ui.CreateElement(budgetListFilled, budgetListFilledProps{
			App: app, View: view, Period: vw, ActiveMemberID: activeMemberID, Compact: compact,
			Search: search.Get(), OnSearch: func(value string) { search.Set(value) }, Attention: attention.Get(),
			OnClearAttention: func() { attention.Set("") },
			OnDrill:          drillFrom, OnViewTodos: func() { nav.Navigate(uistate.RoutePath("/todo")) },
		})
	}
	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("nav.budgets"), HeaderAction: budgetSmartFeaturesLink(smartSettings), Body: body,
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true, Body: section,
	})
}

func budgetIncompleteNotice(messages []string) ui.Node {
	if len(messages) == 0 {
		return Fragment()
	}
	items := make([]ui.Node, 0, len(messages)+1)
	items = append(items, P(css.Class("form-err"), Attr("role", "status"), uistate.T("budgets.incomplete")))
	for _, message := range messages {
		items = append(items, P(css.Class("budget-owner-hint", tw.TextFaint), Attr("data-testid", "budget-incomplete-item"), message))
	}
	return Div(css.Class("budget-incomplete"), items)
}

// budgetAnnualGridWidget (BG9) is the year-at-a-glance plan-vs-actual grid as its OWN
// full-width surface cell, below the list — a view-only collapsible section — rather than
// squeezed into the bottom of the list tile. Self-gates to nothing when there are no
// budgets (the list tile owns the first-run CTA).
func budgetAnnualGridWidget(props budgetListProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	memberLens := uistate.UseMemberLens()
	activeMemberID := ""
	if len(memberLens) == 1 {
		activeMemberID = memberLens[0]
	}
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	showLastMonth := uistate.UseBudgetsLastMonth().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr, showLastMonth)
	if len(v.Statuses) == 0 {
		return Fragment() // no budgets → no grid cell (matches the summary tile's self-gate)
	}

	// Drill from an annual-grid cell to that month's filtered transactions.
	drillMonth := func(categoryIDs []string, from, to string) {
		var f uistate.TxFilter
		switch len(categoryIDs) {
		case 0:
			// no tracked category — open the ledger windowed to the month only
		case 1:
			f.Category = categoryIDs[0]
		default:
			f.Categories = strings.Join(categoryIDs, ",")
		}
		f.From, f.To = from, to
		f = f.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	budgetsForGrid := make([]domain.Budget, 0, len(v.Statuses))
	for _, s := range v.Statuses {
		budgetsForGrid = append(budgetsForGrid, s.Budget)
	}
	annualGrid := ui.CreateElement(BudgetAnnualGrid, budgetAnnualGridProps{
		Budgets: budgetsForGrid,
		Txns:    budgetLedgerForApp(app),
		App:     app, Scope: uistate.ActiveMemberFromScope(),
		Cats:       app.Categories(),
		Recurrings: app.Recurring(), // W6/C394 — future-month bill + goal projection
		Goals:      app.Goals(),
		Rates:      currency.Rates{Base: v.Base, Rates: app.Settings().FXRates},
		WeekStart:  pr.WeekStartWeekday(),
		Now:        budgetNow(),
		OnCell:     drillMonth,
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-annualgrid", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: annualGrid,
	})
}

// sortBudgetStatuses re-orders the budget list in place for the toolbar's Sort picker.
// "health" is the default order (left untouched — computeBudgetView already sorts by it).
func sortBudgetStatuses(sts []budgeting.Status, key string, categories map[string]string) {
	// overAmt is how far a budget is OVER its limit (0 when under) — the severity of an
	// overspend, in money.
	overAmt := func(s budgeting.Status) int64 {
		if s.Remaining.IsNegative() {
			return -s.Remaining.Amount
		}
		return 0
	}
	// distFromLimit is how far a budget's usage is from its limit line, in PERCENTAGE
	// POINTS, either way — |% used − 100|. So a budget at 99% or 101% (1 point off) reads
	// as "closest", while one at 0% used (100 points under) OR 200% (100 points over) reads
	// as farthest — proportional proximity, not raw dollars, so a barely-touched small
	// budget doesn't masquerade as "close to the limit".
	distFromLimit := func(s budgeting.Status) int {
		if d := s.Percent - 100; d < 0 {
			return -d
		} else {
			return d
		}
	}
	limitOf := func(s budgeting.Status) int64 { return s.Limit.Amount }
	switch key {
	case uistate.BudgetSortOverage:
		// Severity: the worst overspends first — by how far over in money, then by how far
		// over proportionally (% used) as a tiebreak. Budgets that aren't over (overAmt 0)
		// fall to the end, keeping their health order.
		sort.SliceStable(sts, func(i, j int) bool {
			if sts[i].Remaining.Currency != sts[j].Remaining.Currency {
				return sts[i].Remaining.Currency < sts[j].Remaining.Currency
			}
			oi, oj := overAmt(sts[i]), overAmt(sts[j])
			if oi != oj {
				return oi > oj
			}
			return sts[i].Percent > sts[j].Percent
		})
	case uistate.BudgetSortNearOverage:
		// Closest to the limit line first, sign-agnostic and PROPORTIONAL: smallest
		// |% used − 100| wins, so a budget at 99%/101% ranks above one at 200% (or at 0%).
		sort.SliceStable(sts, func(i, j int) bool { return distFromLimit(sts[i]) < distFromLimit(sts[j]) })
	case uistate.BudgetSortUnderutilized:
		// Least used first, proportionally: lowest % used (a budget at 5% used is more
		// underutilized than one at 50%, regardless of dollar size); over budgets (highest
		// %) fall to the end.
		sort.SliceStable(sts, func(i, j int) bool { return sts[i].Percent < sts[j].Percent })
	case uistate.BudgetSortAmount:
		sort.SliceStable(sts, func(i, j int) bool {
			if sts[i].Limit.Currency != sts[j].Limit.Currency {
				return sts[i].Limit.Currency < sts[j].Limit.Currency
			}
			return limitOf(sts[i]) > limitOf(sts[j])
		})
	case uistate.BudgetSortName:
		sort.SliceStable(sts, func(i, j int) bool {
			left := strings.ToLower(budgetTitle(sts[i].Budget.Name, categories[sts[i].Budget.CategoryID]))
			right := strings.ToLower(budgetTitle(sts[j].Budget.Name, categories[sts[j].Budget.CategoryID]))
			return left < right
		})
	}
}

// spendReportLink points at the existing per-category spend report rather than
// duplicating it.
//
// It lands on "04 · Categories reviewed" rather than the top of a long document
// (C524). The fragment could not be used at first because the report's numbered
// sections are not in the DOM when the browser processes one — the report waits
// for its own target now, so the link works.
func spendReportLink() ui.Node {
	// A quiet text link, not a tool button. It navigates off the page and changes
	// nothing; wearing the same bordered button shape as the controls that DO edit
	// your budgets made four different affordances read as one kind of thing.
	return A(css.Class("budget-meta-link", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
		Href(uistate.RoutePath("/reports")+"#rpta-04"),
		Attr("data-testid", "budgets-see-spend"), Attr("aria-label", uistate.T("budgets.seeSpend")+" — "+uistate.T("budgets.seeSpendTitle")), Title(uistate.T("budgets.seeSpendTitle")),
		Span(uistate.T("budgets.seeSpend")))
}

// lastMonthOverlay returns a budget's last-period figures ONLY when the
// "Last month's spend" overlay is on.
//
// The figures themselves are also computed for a period too young to judge
// (C344), where they are shown as a quiet reading rather than as the tile's
// headline — so reading the map directly would silently switch every card into
// overlay mode on the 1st of the month.
func lastMonthOverlay(v budgetView, budgetID string) budgetLastMonth {
	if !v.LastMonthMode {
		return budgetLastMonth{}
	}
	return v.LastMonth[budgetID]
}
