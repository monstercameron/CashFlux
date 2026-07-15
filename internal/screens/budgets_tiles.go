// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
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
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	// "Last month's spend" overlay: when on, the summary graph shows last period's total
	// spend too (matching the tiles), not just this month.
	showLM := uistate.UseBudgetsLastMonth().Get()
	// Income-basis modal opener. Called unconditionally (before any early return) so the
	// hook order is stable across renders. Opening seeds the modal's draft from the
	// current prefs, so the Save/Cancel modal starts from today's basis.
	basisOpen := uistate.UseBudgetBasisOpen()
	basisDraft := uistate.UseBudgetBasisDraft()
	openBasis := ui.UseEvent(Prevent(func() {
		basisDraft.Set(uistate.NewBudgetBasisDraft(pr))
		basisOpen.Set(true)
	}))
	v := computeBudgetView(app, activeMemberID, vw, pr, showLM)
	if len(v.Statuses) == 0 {
		return Fragment()
	}
	smartSettings := uistate.LoadSmartSettings()
	// A discoverable button that opens the "Income to budget with" modal (the income-
	// source picker + rules) — present in every method, so simple/envelope users can set
	// which income funds the budget just like zero-based users.
	basisBtn := Button(css.Class("btn btn-sm zbb-basis-open", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-basis-open"), Title(uistate.T("budgets.basisButtonTitle")), OnClick(openBasis),
		uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("budgets.basisButton")))

	// The spend figure the graph shows: this month by default; last period's total when
	// the "Last month's spend" overlay is on, so the top graph matches the tiles.
	barSpent := v.TotalSpent
	if v.LastMonthMode {
		barSpent = v.LastTotalSpent
	}
	// "Spent" is only red once there's actually spending — red on $0.00 reads as an
	// error rather than a healthy "nothing spent yet" (design critique).
	spentTone := ""
	if barSpent > 0 {
		spentTone = "neg"
	}
	// The summary is a single big "loader" — an overall spent-of-budget progress bar with
	// the spent / budget / left figures rendered INSIDE it, so the numbers and the visual
	// fill read as one unit. The fill grows with spending and turns amber/red as it
	// nears/exceeds the cap; "Left" (safe-to-spend) is the hero figure on the right.
	//
	// The cap is the SELECTED MAX BUDGET. In zero-based mode that's the income basis you
	// chose in the modal (all income / paychecks / chosen sources / a set monthly figure)
	// plus any rolled-over leftover — so the bar reads as "spent of your income", not
	// "spent of what's assigned to categories". Simple/envelope keep the total budgeted.
	spendLimit := v.TotalLimit
	spendLimitLabel := uistate.T("budgets.budgeted")
	if v.Method == budgeting.MethodZeroBased {
		spendLimit = v.BannerIncome + v.RolledOver
		spendLimitLabel = uistate.T("budgets.spendBudgetLabel")
	}
	leftM := money.New(spendLimit-barSpent, v.Base)
	over := barSpent > spendLimit
	fillPct := 0
	if spendLimit > 0 {
		fillPct = int(barSpent * 100 / spendLimit)
	}
	fillW := fillPct
	if fillW > 100 {
		fillW = 100
	}
	if fillW < 0 {
		fillW = 0
	}
	fillCls := "budget-loader-fill"
	switch {
	case over:
		fillCls += " is-over"
	case fillPct >= 85:
		fillCls += " is-near"
	}
	loaderCls := "budget-loader"
	if over {
		loaderCls += " is-over"
	}
	statGrid := Div(ClassStr(loaderCls),
		Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(fillW)),
		Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"),
		Attr("aria-label", uistate.T("budgets.progressLabel")),
		Div(ClassStr(fillCls), Attr("style", fmt.Sprintf("width:%d%%", fillW))),
		Div(css.Class("budget-loader-figs"),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("budgets.spent")),
				Div(ClassStr("budget-loader-value "+spentTone), fmtMoney(money.New(barSpent, v.Base))),
			),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), spendLimitLabel),
				Div(css.Class("budget-loader-value"), fmtMoney(money.New(spendLimit, v.Base))),
			),
			// "Left" (safe-to-spend) is the key figure — annotated with a smart explainer.
			Div(css.Class("budget-loader-fig", "is-right"),
				Div(css.Class("budget-loader-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("budgets.left"),
					smartTooltipFor(smartSettings, "budget-safe", uistate.T("budgets.left"), uistate.T("smart.tipBudgetSafe")),
				),
				Div(ClassStr("budget-loader-value is-hero "+accentFor(leftM)), budgetLeftValue(leftM)),
			),
		),
	)

	// C130: clarify a custom top-bar range only changes the view window — it doesn't
	// redefine each budget's own period.
	rangeHint := If(!vw.IsSinglePeriod(), P(css.Class("muted"), Attr("data-testid", "budgets-custom-range-hint"),
		uistate.T("budgets.customRangeHint")))
	// C125: a salient over-spend banner + the count/near pills.
	overBanner := If(v.OverCount > 0, Div(css.Class("card-alert", "budget-over-banner", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Attr("role", "status"), Attr("data-testid", "budgets-over-banner"),
		Span(css.Class("budget-over-icon"), Attr("aria-hidden", "true"), "⚠"),
		Span(css.Class("budget-over-text"), overBannerText(v.OverCount, fmtMoney(money.New(v.TotalOver, v.Base)))),
	))
	pills := If(v.OverCount > 0 || v.NearCount > 0, P(css.Class("budget-sub", tw.Flex, tw.ItemsCenter, tw.Gap2),
		If(v.OverCount > 0, Span(css.Class("pill is-danger"), uistate.T("budgets.overBadge", v.OverCount))),
		If(v.NearCount > 0, Span(css.Class("pill is-warn"), uistate.T("budgets.nearBadge", v.NearCount))),
	))

	// When the overlay is on, an accent "LAST MONTH" tag over the spend graph makes clear
	// the figures are last period's, matching the tiles.
	var lastMonthTag ui.Node = Fragment()
	if v.LastMonthMode {
		lastMonthTag = Div(css.Class("budget-lastmonth-tag"), Attr("data-testid", "budgets-summary-lastmonth"), uistate.T("budgets.lastMonthCap"))
	}

	var body ui.Node
	if v.Method == budgeting.MethodZeroBased {
		// Zero-based leads with the To-Assign hero (the thesis: give every dollar a job),
		// then the income basis, then the spend-progress bar DEMOTED below — spending is
		// context here, not the headline.
		body = Div(
			zeroBasedHero(v, basisBtn),
			overBanner,
			Div(css.Class("zbb-spend"),
				IfElse(v.LastMonthMode, lastMonthTag, P(css.Class("zbb-spend-cap"), uistate.T("budgets.zbbSpendCap"))),
				statGrid),
			rangeHint,
			budgetFundSetAsideNode(v),
			pills,
		)
	} else {
		body = Div(
			lastMonthTag,
			statGrid,
			rangeHint,
			Div(css.Class("budget-basis-row"), budgetAssignBanner(v), basisBtn),
			budgetFundSetAsideNode(v),
			overBanner,
			pills,
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// lastMonthLabelKey picks the toolbar toggle's label: a call-to-action when off,
// a "you're viewing last month" state when on.
func lastMonthLabelKey(on bool) string {
	if on {
		return "budgets.lastMonthOn"
	}
	return "budgets.lastMonth"
}

// budgetAssignBanner renders the method-specific income context line: simple mode
// shows income · budgeted · the unbudgeted/over gap; zero-based shows the amount left
// to assign; envelope shows a short note. Pure (no hooks) — a plain node builder.
// overBannerText renders the over-budget banner copy with correct grammar for a
// single over-budget category (the plural string read "1 budgets are over").
func overBannerText(count int, total string) string {
	if count == 1 {
		return uistate.T("budgets.overBannerOne", total)
	}
	return uistate.T("budgets.overBanner", count, total)
}

func budgetAssignBanner(v budgetView) ui.Node {
	switch v.Method {
	case budgeting.MethodSimple:
		unbudgeted := v.BannerIncome - v.TotalLimit
		var diff ui.Node
		switch {
		case unbudgeted > 0:
			diff = Span(css.Class(tw.TextUp), uistate.T("budgets.simpleUnbudgeted", fmtMoney(money.New(unbudgeted, v.Base))))
		case unbudgeted == 0:
			diff = Span(uistate.T("budgets.simpleFullyAllocated"))
		default:
			diff = Span(css.Class(tw.TextDown), uistate.T("budgets.simpleOverAllocated", fmtMoney(money.New(-unbudgeted, v.Base))))
		}
		return P(css.Class("budget-sub", tw.FontDisplay),
			uistate.T("budgets.simpleIncome", fmtMoney(money.New(v.BannerIncome, v.Base))), " · ",
			uistate.T("budgets.simpleBudgeted", fmtMoney(money.New(v.TotalLimit, v.Base))), " · ", diff)
	case budgeting.MethodZeroBased:
		return zeroBasedHero(v, Fragment())
	case budgeting.MethodEnvelope:
		return P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.envelopeNote"))
	}
	return Fragment()
}

// zeroBasedHero is the zero-based view's centrepiece. The thesis of zero-based
// budgeting — give every dollar a job — is made visual: a big "To Assign" figure (the
// income pool minus everything assigned to expenses and savings) sits over an
// ALLOCATION BAR that splits the income into Expenses, Savings, and the still-
// unassigned gap. Filling the bar (closing the gap) is the goal; the figure reads green
// at $0 and red when over-assigned. `action` is an optional control rendered in the
// header — the income button — pass Fragment() for none. Pure node builder.
func zeroBasedHero(v budgetView, action ui.Node) ui.Node {
	expenses, savings := v.TotalLimit, v.SavingsAssigned
	assigned := expenses + savings
	pool := v.BannerIncome + v.RolledOver // last month's leftover adds to what you can assign
	toAssign := budgeting.ToAssign(pool, assigned)

	figCls := "zbb-figure fig"
	var status ui.Node
	switch {
	case toAssign == 0:
		figCls += " is-done"
		status = Span(css.Class("zbb-status"), uistate.T("budgets.zbbAllAssigned"))
	case toAssign > 0:
		figCls += " is-left"
		status = Span(css.Class("zbb-status"), uistate.T("budgets.zbbLeft"))
	default:
		figCls += " is-over"
		status = Span(css.Class("zbb-status zbb-status-over"), uistate.T("budgets.zbbOver"))
	}

	// Segment widths as a share of the larger of the income pool or what's assigned, so
	// the bar stays sensible when over-assigned. The remainder folds into savings so the
	// segments always fill exactly 100% (no rounding sliver).
	over := toAssign < 0
	base := pool
	if assigned > base {
		base = assigned
	}
	unassigned := toAssign
	if unassigned < 0 {
		unassigned = 0
	}
	pctOf := func(n int64) int {
		if base <= 0 || n <= 0 {
			return 0
		}
		if p := int(n * 100 / base); p < 100 {
			return p
		}
		return 100
	}
	expPct := pctOf(expenses)
	gapPct := pctOf(unassigned)
	savPct := 100 - expPct - gapPct // savings takes the remainder so the bar always fills
	if savPct < 0 || base <= 0 {
		// Guard: no income and nothing assigned → an empty bar, never a false 100% savings.
		savPct = 0
	}
	// Income reference marker: only meaningful when over-assigned — it sits where actual
	// income runs out, so the fill past it reads as the overage rather than "healthy".
	incomeMarkerPct := -1
	if over && base > 0 {
		incomeMarkerPct = int(pool * 100 / base)
	}
	// The third legend slot flips to a red "Over-assigned $X" when over, instead of a
	// misleading "Unassigned $0.00" sitting beside the red headline. Its swatch is a tick
	// (is-over renders as a vertical mark, matching the bar's income marker) not a round
	// dot — the over figure is a threshold reading, not an additive fourth slice.
	thirdLegend := zbbLegendItemTone("is-gap", "", uistate.T("budgets.zbbUnassigned"), unassigned, v.Base)
	if over {
		thirdLegend = zbbLegendItemTone("is-over", "zbb-legend-val-over", uistate.T("budgets.zbbOverAssignedShort"), -toAssign, v.Base)
	}
	allocAria := uistate.T("budgets.zbbAllocAria")
	if over {
		allocAria = uistate.T("budgets.zbbAllocAriaOver")
	}

	return Div(css.Class("zbb-hero"),
		Div(css.Class("zbb-hero-top"),
			Span(css.Class("zbb-label"), uistate.T("budgets.zbbToAssign")),
			action,
		),
		Div(css.Class("zbb-figrow"),
			// Show the magnitude — the status word ("left to assign" / "over-assigned")
			// carries the sign, so an accounting-parens negative is avoided.
			Span(css.Class(figCls), Attr("data-testid", "budgets-zbb-toassign"), fmtMoney(money.New(toAssign, v.Base).Abs())),
			status),
		Div(css.Class("zbb-alloc-cap"),
			Span(css.Class("zbb-alloc-cap-label"), uistate.T("budgets.zbbIncome")),
			Span(css.Class("zbb-alloc-cap-val fig"), fmtMoney(money.New(pool, v.Base))),
			If(v.RolledOver > 0, Span(css.Class("zbb-alloc-cap-note"), uistate.T("budgets.zbbAllocRolled", fmtMoney(money.New(v.RolledOver, v.Base))))),
		),
		// Bar + marker share a non-clipping wrapper so the income tick can protrude above
		// and below the bar (the bar itself clips its segments).
		Div(css.Class("zbb-alloc-wrap"),
			Div(css.Class("zbb-alloc"), Attr("role", "img"), Attr("aria-label", allocAria),
				If(expPct > 0, Div(css.Class("zbb-alloc-seg is-exp"), Attr("style", fmt.Sprintf("width:%d%%", expPct)))),
				If(savPct > 0, Div(css.Class("zbb-alloc-seg is-sav"), Attr("style", fmt.Sprintf("width:%d%%", savPct)))),
				If(gapPct > 0, Div(css.Class("zbb-alloc-seg is-gap"), Attr("style", fmt.Sprintf("width:%d%%", gapPct)))),
			),
			If(incomeMarkerPct >= 0, Div(css.Class("zbb-alloc-marker"), Attr("style", fmt.Sprintf("left:%d%%", incomeMarkerPct)), Attr("title", uistate.T("budgets.zbbIncomeMarker")))),
		),
		Div(css.Class("zbb-legend"),
			zbbLegendItem("is-exp", uistate.T("budgets.zbbExpenses"), expenses, v.Base),
			zbbLegendItem("is-sav", uistate.T("budgets.zbbSavings"), savings, v.Base),
			thirdLegend,
		),
	)
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

// minorToMajorStr renders minor units as an edit-friendly major-unit string (""
// for zero, so an input shows its placeholder instead of "0").
func minorToMajorStr(minor int64, base string) string {
	if minor <= 0 {
		return ""
	}
	dec := currency.Decimals(base)
	mult := 1.0
	for i := 0; i < dec; i++ {
		mult *= 10
	}
	return strconv.FormatFloat(float64(minor)/mult, 'f', -1, 64)
}

// majorStrToMinor parses an edit-field major-unit string back to minor units;
// ok=false for blank/invalid input so the caller can distinguish "clear" from "keep".
func majorStrToMinor(s, base string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true // explicit clear
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	dec := currency.Decimals(base)
	mult := 1.0
	for i := 0; i < dec; i++ {
		mult *= 10
	}
	return int64(f*mult + 0.5), true
}

type incomeBasisProps struct {
	Base    string
	Sources []incomeSource // income categories + last-month amounts (by-source basis)
	Income  int64          // the resolved income feeding the hero (the ledger's running total)
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
		if next == budgeting.IncomeModeCategories && len(dd.Cats) == 0 {
			var seed []string
			for _, s := range props.Sources {
				if s.Minor > 0 {
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
		dd := draftAtom.Get()
		var all []string
		for _, s := range props.Sources {
			if s.Minor > 0 {
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
				Placeholder(uistate.T("budgets.zbbPaycheckMinPh")), Value(minorToMajorStr(d.PaycheckMin, props.Base)), OnInput(onThreshold)))
	case budgeting.IncomeModeFixed:
		extra = Label(css.Class("zbb-basis-extra"),
			Span(css.Class("zbb-basis-sub"), uistate.T("budgets.zbbFixedAmount")),
			Input(css.Class("field"), Type("number"), Step("1"), Attr("min", "0"), Attr("data-testid", "budgets-zbb-fixed-amount"),
				Placeholder(uistate.T("budgets.zbbFixedAmountPh")), Value(minorToMajorStr(d.Fixed, props.Base)), OnInput(onFixed)))
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
			Button(css.Class("zbb-sources-act"), Type("button"), Attr("data-testid", "budgets-zbb-select-all"), OnClick(selectAllSources), uistate.T("budgets.incomeSelectAll")),
			Span(css.Class("zbb-sources-actsep"), Attr("aria-hidden", "true"), "·"),
			Button(css.Class("zbb-sources-act"), Type("button"), Attr("data-testid", "budgets-zbb-hold-all"), OnClick(holdAllSources), uistate.T("budgets.incomeHoldAll")),
		)
		sources = budgetIncomeSourcesLedger(props, selected, toggleSource, actions, included)
	}

	return Div(css.Class("zbb-basis-wrap"),
		Div(css.Class("zbb-basis"), Attr("data-testid", "budgets-zbb-basis"),
			Label(css.Class("zbb-basis-main"),
				Span(css.Class("zbb-basis-label"), uistate.T("budgets.zbbBasisLabel")),
				Select(css.Class("field"), Attr("data-testid", "budgets-zbb-income-mode"), Attr("aria-label", uistate.T("budgets.zbbBasisLabel")), OnChange(onMode),
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
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgets-zbb-average"), OnChange(onToggleAverage)}, checkedAttr(d.AvgMonths >= 2)...)...),
			Div(css.Class("row-main"),
				Span(uistate.T("budgets.zbbAverageToggle")),
				Span(css.Class("row-meta", tw.TextDim), uistate.T("budgets.zbbAverageHint")))),
		// Roll last month's unspent budget into this month's assignable pool.
		Label(css.Class("zbb-rollover"), Style(map[string]string{"cursor": "pointer"}),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "budgets-zbb-rollover"), OnChange(onToggleRollover)}, checkedAttr(d.Rollover)...)...),
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
	if len(props.Sources) == 0 {
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
	return Div(css.Class("zbb-sources"), Attr("data-testid", "budgets-zbb-sources"),
		Div(css.Class("zbb-sources-head"),
			Span(css.Class("zbb-sources-title"), uistate.T("budgets.incomeSourcesTitle")),
			actions,
		),
		Div(css.Class("zbb-sources-total"),
			Span(css.Class("zbb-sources-total-cap"), uistate.T("budgets.incomeSourcesTotalCap")),
			Span(css.Class("zbb-sources-total-val fig"), Attr("data-testid", "budgets-zbb-sources-total"),
				fmtMoney(money.New(props.Income, props.Base))),
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
	if hasIncome {
		// A real source: show its amount, and a "held aside" tag when excluded.
		if !props.Included {
			aside = Span(css.Class("zbb-source-aside"), uistate.T("budgets.incomeSourceHeldAside"))
		}
		amtNode = Span(css.Class("zbb-source-amt fig"), fmtMoney(money.New(props.Source.Minor, props.Base)))
	} else {
		// No income last month: say so plainly, so it's not confused with a source the
		// user simply hasn't checked (no bare dash, no misleading "held aside" tag).
		amtNode = Span(css.Class("zbb-source-amt zbb-source-none"), uistate.T("budgets.incomeSourceNoHistory"))
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
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	pr := uistate.UsePrefs().Get()
	d := uistate.UseBudgetBasisDraft().Get()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	// Everything the modal shows reflects the STAGED draft (its mode, chosen sources, and
	// averaging window), so the running total and per-source amounts preview unsaved edits
	// and stay consistent: the checked rows sum to the previewed income.
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	now := time.Now()
	ms, _ := budgeting.PeriodRange(domain.PeriodMonthly, now, pr.WeekStartWeekday())
	mode := d.Mode
	if mode == "" {
		mode = budgeting.IncomeModeAll
	}
	draftIncome := budgeting.AveragedIncome(mode, d.PaycheckMin, d.Fixed, d.Cats, app.Transactions(), ms, d.AvgMonths, base, rates)
	sources := computeIncomeSources(app, base, rates, ms, d.AvgMonths)

	return Div(css.Class("zbb-basis-modal"),
		P(css.Class("zbb-basis-modal-help", tw.TextDim), uistate.T("budgets.basisModalHelp")),
		ui.CreateElement(budgetIncomeBasisControl, incomeBasisProps{Base: base, Sources: sources, Income: draftIncome}),
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
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()

	// All hooks run before the mode early-return so hook order stays stable when the
	// budgeting method changes mid-session. The spread handler recomputes the view at
	// click time so it always splits the current leftover across the current accounts.
	nav := router.UseNavigate()
	goToGoals := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/goals")) }))
	goToAccounts := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/accounts")) }))
	onSpread := ui.UseEvent(Prevent(func() {
		vv := computeBudgetView(app, activeMemberID, vw, pr, false)
		pool := vv.BannerIncome + vv.RolledOver
		leftover := budgeting.ToAssign(pool, vv.TotalLimit+vv.SavingsAssigned)
		rates := currency.Rates{Base: vv.Base, Rates: app.Settings().FXRates}
		spreadLeftoverAcrossSavings(app, vv.SavingsAccts, leftover, vv.Base, rates)
	}))

	v := computeBudgetView(app, activeMemberID, vw, pr, false)
	if v.Method != budgeting.MethodZeroBased || len(v.Statuses) == 0 {
		return Fragment() // only meaningful in zero-based mode with budgets present
	}

	pool := v.BannerIncome + v.RolledOver
	leftover := budgeting.ToAssign(pool, v.TotalLimit+v.SavingsAssigned)

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
	if leftover > 0 && len(v.SavingsAccts) > 0 {
		spreadBtn = Button(css.Class("btn btn-sm btn-accent zbb-savings-spread"), Type("button"),
			Attr("data-testid", "budgets-savings-spread"), Attr("aria-label", uistate.T("budgets.savingsSpreadAria")),
			Attr("title", uistate.T("budgets.savingsSpreadTitle")), OnClick(onSpread),
			uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("budgets.savingsSpread", fmtMoney(money.New(leftover, v.Base)))))
	}

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("budgets.savingsSectionTitle"),
		Body: Fragment(
			head,
			P(css.Class("muted", tw.Text13), uistate.T("budgets.savingsDesc")),
			body,
			Div(css.Class("zbb-savings-foot"),
				spreadBtn,
				Div(css.Class("zbb-savings-foot-links"),
					Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-savings-accounts-link"), OnClick(goToAccounts),
						uiw.Icon(icon.Accounts, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.savingsManageAccounts"))),
					Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-savings-goals-link"), OnClick(goToGoals),
						uiw.Icon(icon.Goals, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.savingsManageGoals"))))),
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-savings", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
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
					Attr("data-testid", "budgets-savings-amt-"+sa.AccountID), Attr("aria-label", uistate.T("budgets.savingsMonthlyAria", sa.Name)),
					Value(val), OnInput(onEdit), OnChange(onCommit)),
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
	// Every linked goal is already met — surface the win rather than a projection.
	if sa.GoalComplete {
		return Div(css.Class("zbb-savings-goal is-ontrack"),
			Span(css.Class("zbb-savings-goal-name"), uistate.T("budgets.savingsFunds", sa.GoalName)),
			Span(css.Class("zbb-savings-goal-time"), uistate.T("budgets.savingsFunded")))
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
			OnClick(onSync), uistate.T("budgets.savingsSync"))
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
		ac.MonthlySavings = money.New(ac.MonthlySavings.Amount+addAcct, cur)
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
func budgetFundSetAsideNode(v budgetView) ui.Node {
	if v.TotalFundSetAside <= 0 {
		return Fragment()
	}
	return P(css.Class("budget-sub", tw.FontDisplay), Attr("data-testid", "budgets-fund-setaside"),
		uistate.T("budgets.fundSetAside", fmtMoney(money.New(v.TotalFundSetAside, v.Base))))
}

// --- budget-toolbar --------------------------------------------------------------

// budgetToolbarWidget is the actions tile: the in-context methodology picker, a
// last-month toggle, the Auto-budget review, the one-click 50/30/20 starter template,
// a Formulas reveal toggle, and an "Add budget" button — each an icon-and-label
// button. It writes the shared method setting + the Formulas atom so the
// summary/list/formula tiles react in step.
func budgetToolbarWidget(props budgetToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	sortAtom := uistate.UseBudgetSort()
	onSort := ui.UseEvent(func(e ui.Event) { sortAtom.Set(e.GetValue()) })
	activeMemberID := uistate.UseActiveMember().Get()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	method := budgeting.ParseMethodology(app.Settings().BudgetMethodology)

	// Whether any budget is visible in the current scope — the add button shows only
	// then (the empty-state CTA owns the first add otherwise).
	hasBudgets := false
	for _, b := range app.Budgets() {
		if ownerVisibleTo(b.OwnerID, activeMemberID) {
			hasBudgets = true
			break
		}
	}

	// Open the add-budget modal (G4: discoverable add).
	addBudget := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("budget") }))
	// Open the "Auto budget" review modal (suggests budgets from spending history).
	autoBudgetAtom := uistate.UseBudgetAutoOpen()
	openAutoBudget := ui.UseEvent(Prevent(func() { autoBudgetAtom.Set(true) }))
	// One-click "Last month" — flip the whole budgets view to the previous period.
	lastMonthAtom := uistate.UseBudgetsLastMonth()
	toggleLastMonth := ui.UseEvent(Prevent(func() { lastMonthAtom.Set(!lastMonthAtom.Get()) }))
	// C112: switch the budgeting methodology right from /budgets.
	onMethod := ui.UseEvent(func(e ui.Event) {
		s := app.Settings()
		s.BudgetMethodology = e.GetValue()
		_ = app.PutSettings(s)
		uistate.BumpDataRevision()
	})
	sortVal := sortAtom.Get()
	// Standard two-row toolbar (matches the transactions/accounts toolbar): budgets has
	// no free-text search, so row 1 (the primary line) holds the two view-shaping pickers
	// — budgeting method + sort — each growing to fill the width, and row 2 holds the
	// action buttons with "+ Add budget" anchoring the end.
	toolbar := Div(css.Class("filter-toolbar budgets-tb"),
		Div(css.Class("filter-toolbar-primary"),
			Label(css.Class("fctrl"),
				uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(css.Class("fctrl-label"), uistate.T("settings.budgetMethod")),
				Select(css.Class("fctrl-select"), Attr("data-testid", "budgets-method"),
					Attr("aria-label", uistate.T("settings.budgetMethod")), Title(uistate.T("settings.budgetMethod")), OnChange(onMethod),
					Option(Value(string(budgeting.MethodSimple)), SelectedIf(method == budgeting.MethodSimple), uistate.T("settings.budgetMethodSimple")),
					Option(Value(string(budgeting.MethodZeroBased)), SelectedIf(method == budgeting.MethodZeroBased), uistate.T("settings.budgetMethodZero")),
					Option(Value(string(budgeting.MethodEnvelope)), SelectedIf(method == budgeting.MethodEnvelope), uistate.T("settings.budgetMethodEnvelope")),
					Option(Value(string(budgeting.MethodFlex)), SelectedIf(method == budgeting.MethodFlex), uistate.T("settings.budgetMethodFlex")),
				),
			),
			Label(css.Class("fctrl"),
				uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(css.Class("fctrl-label"), uistate.T("budgets.sortLabel")),
				Select(css.Class("fctrl-select"), Attr("data-testid", "budgets-sort"),
					Attr("aria-label", uistate.T("budgets.sortLabel")), Title(uistate.T("budgets.sortLabel")), OnChange(onSort),
					Option(Value(uistate.BudgetSortHealth), SelectedIf(sortVal == uistate.BudgetSortHealth), uistate.T("budgets.sortHealth")),
					Option(Value(uistate.BudgetSortOverage), SelectedIf(sortVal == uistate.BudgetSortOverage), uistate.T("budgets.sortOverage")),
					Option(Value(uistate.BudgetSortNearOverage), SelectedIf(sortVal == uistate.BudgetSortNearOverage), uistate.T("budgets.sortNear")),
					Option(Value(uistate.BudgetSortUnderutilized), SelectedIf(sortVal == uistate.BudgetSortUnderutilized), uistate.T("budgets.sortUnderused")),
					Option(Value(uistate.BudgetSortAmount), SelectedIf(sortVal == uistate.BudgetSortAmount), uistate.T("budgets.sortAmount")),
					Option(Value(uistate.BudgetSortName), SelectedIf(sortVal == uistate.BudgetSortName), uistate.T("budgets.sortName")),
				),
			),
		),
		// Row 2: the actions on their own tidy line, with the primary "+ Add budget"
		// last so it clearly outranks the ghost controls.
		Div(css.Class("filter-toolbar-actions"),
			Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "budgets-last-month"), Attr("aria-pressed", ariaBool(lastMonthAtom.Get())),
				Title(uistate.T("budgets.lastMonthTitle")), OnClick(toggleLastMonth),
				uiw.Icon(icon.History, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T(lastMonthLabelKey(lastMonthAtom.Get())))),
			Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "budgets-autobudget"),
				Title(uistate.T("budgets.autoTitleAction")), OnClick(openAutoBudget),
				uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.autoTitle"))),
			// XC6: open the leftover-sweep config (own component so its click hook is stable).
			sweepConfigToolbarButton(),
			If(hasBudgets, Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "budgets-add"), Title(uistate.T("budgets.add")), OnClick(addBudget),
				uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("budgets.addBudget")))),
		),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
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
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	// "Last month's spend" overlays each budget with last period's actual spending for
	// planning — it no longer re-windows the view to last month.
	showLastMonth := uistate.UseBudgetsLastMonth().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr, showLastMonth)
	// The default view is health-sorted (over → near → at-risk → on-track, from
	// computeBudgetView); the toolbar's Sort picker can re-order by overage, closeness to
	// the limit, how underused a budget is, size, or name.
	sortKey := uistate.UseBudgetSort().Get()
	if sortKey != uistate.BudgetSortHealth {
		sorted := make([]budgeting.Status, len(v.Statuses))
		copy(sorted, v.Statuses)
		sortBudgetStatuses(sorted, sortKey)
		v.Statuses = sorted
	}
	smartSettings := uistate.LoadSmartSettings()

	// Drill from a budget to its spending: open Transactions filtered to the budget's
	// category (mirrors Accounts→Transactions, C30/C50).
	viewTransactions := func(categoryIDs []string) {
		var f uistate.TxFilter
		switch len(categoryIDs) {
		case 0:
			// no tracked category — just open the unfiltered ledger
		case 1:
			f.Category = categoryIDs[0] // single: use the plain category filter (dropdown reflects it)
		default:
			f.Categories = strings.Join(categoryIDs, ",") // multi: OR across all tracked categories
		}
		f = f.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	// BG9: drill from an annual-grid cell to that month's filtered transactions.
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

	var body ui.Node
	if len(v.Statuses) == 0 {
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("budgets.empty"), CTALabel: uistate.T("budgets.addFirst"), AddTarget: "budget", Icon: icon.Budgets})
	} else {
		cbs := buildBudgetRowCallbacks(app, v.Base, v.CatName)
		budgetDefs := app.CustomFieldDefsFor("budget")
		members := app.Members()
		// Count to-dos linked to each budget (Task.RelatedType=budget) so a card can offer a
		// jump to the To-dos page when it has any attached.
		todoCounts := map[string]int{}
		for _, t := range app.Tasks() {
			if t.RelatedType == domain.RelatedBudget && t.RelatedID != "" {
				todoCounts[t.RelatedID]++
			}
		}
		viewTodos := func() { nav.Navigate(uistate.RoutePath("/todo")) }
		rows := MapKeyed(v.Statuses,
			func(s budgeting.Status) any { return s.Budget.ID },
			func(s budgeting.Status) ui.Node {
				tracked := ""
				if len(s.Budget.CategoryIDs) > 0 {
					var names []string
					for _, id := range s.Budget.CategoryIDs {
						if n := v.CatName[id]; n != "" {
							names = append(names, n)
						}
					}
					tracked = strings.Join(names, ", ")
				}
				return ui.CreateElement(BudgetRow, budgetRowProps{
					Status: s, Category: v.CatName[s.Budget.CategoryID], TrackedCats: tracked, Members: members, BudgetDefs: budgetDefs,
					Envelope: v.EnvAvail[s.Budget.ID], EnvelopeNeg: v.EnvNeg[s.Budget.ID], EnvelopeDebtStart: v.EnvDebtStart[s.Budget.ID], PaceOver: v.PaceOver[s.Budget.ID],
					PaceMarkerPct: v.PaceMark[s.Budget.ID].MarkerPct, PaceCaption: v.PaceMark[s.Budget.ID].Caption, PaceHot: v.PaceMark[s.Budget.ID].Hot,
					RolloverCarry: v.RollCarry[s.Budget.ID], RolloverNeg: v.RollNeg[s.Budget.ID], EffectiveCap: v.RollEffCap[s.Budget.ID],
					ProratedRest: v.ProratedRest[s.Budget.ID], EffectiveMethod: v.EffMethod[s.Budget.ID],
					Covered:        v.Covered[s.Budget.ID],
					LastMonthSpent: v.LastMonth[s.Budget.ID].Spent, LastMonthDelta: v.LastMonth[s.Budget.ID].Delta, LastMonthOver: v.LastMonth[s.Budget.ID].Over,
					LastMonthPct: v.LastMonth[s.Budget.ID].Pct, LastMonthFill: v.LastMonth[s.Budget.ID].Fill,
					OnDelete: cbs.OnDelete, OnRemoveRecurring: cbs.OnRemoveRecurring, OnDrill: viewTransactions,
					LinkedTodos: todoCounts[s.Budget.ID], OnViewTodos: viewTodos,
					Committed: v.Committed[s.Budget.ID], HasCommitted: func() bool { _, ok := v.Committed[s.Budget.ID]; return ok }(),
				})
			},
		)
		// Lay the budget cards out in a responsive grid so each is a compact 1-column
		// block (several per row) rather than a full-width bar — budgets don't need the
		// whole width, and a grid shows far more at a glance.
		// BG9: the view-only annual plan-vs-actual grid, a collapsible section below the
		// cards. It projects the same per-month evaluations the engine already computes.
		budgetsForGrid := make([]domain.Budget, 0, len(v.Statuses))
		for _, s := range v.Statuses {
			budgetsForGrid = append(budgetsForGrid, s.Budget)
		}
		annualGrid := ui.CreateElement(BudgetAnnualGrid, budgetAnnualGridProps{
			Budgets:   budgetsForGrid,
			Txns:      app.Transactions(),
			Cats:      app.Categories(),
			Rates:     currency.Rates{Base: v.Base, Rates: app.Settings().FXRates},
			WeekStart: pr.WeekStartWeekday(),
			Now:       time.Now(),
			OnCell:    drillMonth,
		})
		body = Fragment(Div(css.Class("budget-grid"), rows), annualGrid)
	}

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title:        uistate.T("nav.budgets"),
		HeaderAction: smartSectionAction(smartSettings),
		Body:         body,
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
}

// sortBudgetStatuses re-orders the budget list in place for the toolbar's Sort picker.
// "health" is the default order (left untouched — computeBudgetView already sorts by it).
func sortBudgetStatuses(sts []budgeting.Status, key string) {
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
	limitOf := func(s budgeting.Status) int64 { return s.Spent.Amount + s.Remaining.Amount }
	switch key {
	case uistate.BudgetSortOverage:
		// Severity: the worst overspends first — by how far over in money, then by how far
		// over proportionally (% used) as a tiebreak. Budgets that aren't over (overAmt 0)
		// fall to the end, keeping their health order.
		sort.SliceStable(sts, func(i, j int) bool {
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
		sort.SliceStable(sts, func(i, j int) bool { return limitOf(sts[i]) > limitOf(sts[j]) })
	case uistate.BudgetSortName:
		sort.SliceStable(sts, func(i, j int) bool {
			return strings.ToLower(sts[i].Budget.Name) < strings.ToLower(sts[j].Budget.Name)
		})
	}
}
