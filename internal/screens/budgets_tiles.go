// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
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

type budgetFormulaProps struct{ App *appstate.App }

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
	if uistate.UseBudgetsLastMonth().Get() {
		vw = vw.Shift(-1) // one-click "Last month" — evaluate the previous period
	}
	pr := uistate.UsePrefs().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr)
	if len(v.Statuses) == 0 {
		return Fragment()
	}
	smartSettings := uistate.LoadSmartSettings()

	// "Spent" is only red once there's actually spending — red on $0.00 reads as an
	// error rather than a healthy "nothing spent yet" (design critique).
	spentTone := ""
	if v.TotalSpent > 0 {
		spentTone = "neg"
	}
	// The summary is a single big "loader" — an overall spent-of-budgeted progress bar
	// with the spent / budgeted / left figures rendered INSIDE it, so the numbers and the
	// visual fill read as one unit. The fill grows with spending and turns amber/red as it
	// nears/exceeds the total; "Left" (safe-to-spend) is the hero figure on the right.
	leftM := money.New(v.TotalLimit-v.TotalSpent, v.Base)
	over := v.TotalSpent > v.TotalLimit
	fillPct := 0
	if v.TotalLimit > 0 {
		fillPct = int(v.TotalSpent * 100 / v.TotalLimit)
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
				Div(ClassStr("budget-loader-value "+spentTone), fmtMoney(money.New(v.TotalSpent, v.Base))),
			),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("budgets.budgeted")),
				Div(css.Class("budget-loader-value"), fmtMoney(money.New(v.TotalLimit, v.Base))),
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

	body := Div(
		statGrid,
		// C130: clarify a custom top-bar range only changes the view window — it doesn't
		// redefine each budget's own period.
		If(!vw.IsSinglePeriod(), P(css.Class("muted"), Attr("data-testid", "budgets-custom-range-hint"),
			uistate.T("budgets.customRangeHint"))),
		budgetAssignBanner(v),
		If(v.Method == budgeting.MethodZeroBased, ui.CreateElement(budgetIncomeBasisControl, incomeBasisProps{Base: v.Base})),
		budgetFundSetAsideNode(v),
		// C125: lead with a salient over-spend banner, with the count/near pills below.
		If(v.OverCount > 0, Div(css.Class("card-alert", "budget-over-banner", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Attr("role", "status"), Attr("data-testid", "budgets-over-banner"),
			Span(css.Class("budget-over-icon"), Attr("aria-hidden", "true"), "⚠"),
			Span(css.Class("budget-over-text"), overBannerText(v.OverCount, fmtMoney(money.New(v.TotalOver, v.Base)))),
		)),
		If(v.OverCount > 0 || v.NearCount > 0, P(css.Class("budget-sub", tw.Flex, tw.ItemsCenter, tw.Gap2),
			If(v.OverCount > 0, Span(css.Class("pill is-danger"), uistate.T("budgets.overBadge", v.OverCount))),
			If(v.NearCount > 0, Span(css.Class("pill is-warn"), uistate.T("budgets.nearBadge", v.NearCount))),
		)),
	)
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
		return zeroBasedHero(v)
	case budgeting.MethodEnvelope:
		return P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.envelopeNote"))
	}
	return Fragment()
}

// zeroBasedHero is the zero-based view's centrepiece: a big "To Assign" figure —
// income minus everything assigned to EXPENSES and to SAVINGS/INVESTMENTS — that the
// user drives to $0, over a one-line breakdown of the four figures. Green at $0 (every
// dollar has a job), red when over-assigned, neutral while there is money left to
// assign (the status word says how much). Pure node builder.
func zeroBasedHero(v budgetView) ui.Node {
	assigned := v.TotalLimit + v.SavingsAssigned
	toAssign := budgeting.ToAssign(v.BannerIncome, assigned)
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
	return Div(css.Class("zbb-hero"),
		Span(css.Class("zbb-label"), uistate.T("budgets.zbbToAssign")),
		Div(css.Class("zbb-figrow"),
			Span(css.Class(figCls), Attr("data-testid", "budgets-zbb-toassign"), fmtMoney(money.New(toAssign, v.Base))),
			status),
		Div(css.Class("zbb-breakdown"),
			zbbChip(uistate.T("budgets.zbbIncome"), v.BannerIncome, v.Base),
			zbbChip(uistate.T("budgets.zbbExpenses"), v.TotalLimit, v.Base),
			zbbChip(uistate.T("budgets.zbbSavings"), v.SavingsAssigned, v.Base),
		),
	)
}

// zbbChip is one figure of the zero-based breakdown: a small uppercase label above a
// tabular amount.
func zbbChip(label string, minor int64, base string) ui.Node {
	return Div(css.Class("zbb-chip"),
		Span(css.Class("zbb-chip-label"), label),
		Span(css.Class("zbb-chip-val fig"), fmtMoney(money.New(minor, base))))
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

type incomeBasisProps struct{ Base string }

// budgetIncomeBasisControl is the zero-based view's income-source picker: whether the
// "income to assign" is ALL of last month's income, only regular paychecks (deposits at
// or above a threshold, so side hustles are ignored), or a fixed monthly figure. It
// writes the household prefs so the hero recomputes.
func budgetIncomeBasisControl(props incomeBasisProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	prefsAtom := uistate.UsePrefs()
	pr := prefsAtom.Get()
	mode := pr.BudgetIncomeMode
	if mode == "" {
		mode = budgeting.IncomeModeAll
	}

	onMode := ui.UseEvent(func(e ui.Event) {
		p := prefsAtom.Get()
		p.BudgetIncomeMode = e.GetValue()
		uistate.SetPrefs(p)
	})
	onThreshold := ui.UseEvent(func(e ui.Event) {
		if v, ok := majorStrToMinor(e.GetValue(), props.Base); ok {
			p := prefsAtom.Get()
			p.BudgetPaycheckMinMinor = v
			uistate.SetPrefs(p)
		}
	})
	onFixed := ui.UseEvent(func(e ui.Event) {
		if v, ok := majorStrToMinor(e.GetValue(), props.Base); ok {
			p := prefsAtom.Get()
			p.MonthlyIncomeMinor = v
			uistate.SetPrefs(p)
		}
	})

	var extra ui.Node = Fragment()
	switch mode {
	case budgeting.IncomeModePaychecks:
		extra = Label(css.Class("zbb-basis-extra"),
			Span(css.Class("zbb-basis-sub"), uistate.T("budgets.zbbPaycheckMin")),
			Input(css.Class("field"), Type("number"), Step("1"), Attr("min", "0"), Attr("data-testid", "budgets-zbb-paycheck-min"),
				Placeholder(uistate.T("budgets.zbbPaycheckMinPh")), Value(minorToMajorStr(pr.BudgetPaycheckMinMinor, props.Base)), OnInput(onThreshold)))
	case budgeting.IncomeModeFixed:
		extra = Label(css.Class("zbb-basis-extra"),
			Span(css.Class("zbb-basis-sub"), uistate.T("budgets.zbbFixedAmount")),
			Input(css.Class("field"), Type("number"), Step("1"), Attr("min", "0"), Attr("data-testid", "budgets-zbb-fixed-amount"),
				Placeholder(uistate.T("budgets.zbbFixedAmountPh")), Value(minorToMajorStr(pr.MonthlyIncomeMinor, props.Base)), OnInput(onFixed)))
	}

	return Div(css.Class("zbb-basis"), Attr("data-testid", "budgets-zbb-basis"),
		Label(css.Class("zbb-basis-main"),
			Span(css.Class("zbb-basis-label"), uistate.T("budgets.zbbBasisLabel")),
			Select(css.Class("field"), Attr("data-testid", "budgets-zbb-income-mode"), Attr("aria-label", uistate.T("budgets.zbbBasisLabel")), OnChange(onMode),
				Option(Value(budgeting.IncomeModeAll), SelectedIf(mode == budgeting.IncomeModeAll), uistate.T("budgets.zbbBasisAll")),
				Option(Value(budgeting.IncomeModePaychecks), SelectedIf(mode == budgeting.IncomeModePaychecks), uistate.T("budgets.zbbBasisPaychecks")),
				Option(Value(budgeting.IncomeModeFixed), SelectedIf(mode == budgeting.IncomeModeFixed), uistate.T("budgets.zbbBasisFixed")),
			)),
		extra,
	)
}

// budgetSavingsWidget is the zero-based view's "Savings & investments" tile: the goals
// whose monthly contribution counts toward the assigned total, each with a quick inline
// edit of that monthly amount so the user can drive To Assign to $0 without leaving the
// page. Rendered only in zero-based mode.
func budgetSavingsWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	if uistate.UseBudgetsLastMonth().Get() {
		vw = vw.Shift(-1)
	}
	pr := uistate.UsePrefs().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr)
	if v.Method != budgeting.MethodZeroBased || len(v.Statuses) == 0 {
		return Fragment() // only meaningful in zero-based mode with budgets present
	}

	nav := router.UseNavigate()
	goToGoals := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/goals")) }))

	var body ui.Node
	if len(v.SavingsLines) == 0 {
		body = P(css.Class("empty", tw.TextDim), Attr("data-testid", "budgets-savings-empty"), uistate.T("budgets.savingsEmpty"))
	} else {
		rows := MapKeyed(v.SavingsLines,
			func(a goalsvc.Assignment) any { return a.GoalID },
			func(a goalsvc.Assignment) ui.Node {
				return ui.CreateElement(budgetSavingsRow, budgetSavingsRowProps{App: app, Line: a, Base: v.Base})
			})
		body = Div(css.Class("zbb-savings-rows"), rows)
	}

	head := Div(css.Class("zbb-savings-head"),
		Span(css.Class("zbb-savings-title"), uistate.T("budgets.savingsTitle")),
		Span(css.Class("zbb-savings-total fig"), fmtMoney(money.New(v.SavingsAssigned, v.Base))))

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("budgets.savingsSectionTitle"),
		Body: Fragment(
			head,
			P(css.Class("muted", tw.Text13), uistate.T("budgets.savingsDesc")),
			body,
			Div(css.Class("zbb-savings-foot"),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-savings-goals-link"), OnClick(goToGoals),
					uiw.Icon(icon.Goals, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.savingsManageGoals")))),
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-savings", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
}

type budgetSavingsRowProps struct {
	App  *appstate.App
	Line goalsvc.Assignment
	Base string
}

// budgetSavingsRow is one goal in the savings section: its name, its current monthly
// assignment, and an inline number input that sets the goal's explicit
// MonthlyContribution (overriding the target-date-derived pace). Owns its own edit hook.
func budgetSavingsRow(props budgetSavingsRowProps) ui.Node {
	app := props.App
	var goal domain.Goal
	found := false
	for _, g := range app.Goals() {
		if g.ID == props.Line.GoalID {
			goal, found = g, true
			break
		}
	}
	onEdit := ui.UseEvent(func(e ui.Event) {
		if !found {
			return
		}
		cur := goal.TargetAmount.Currency
		if cur == "" {
			cur = props.Base
		}
		if v, ok := majorStrToMinor(e.GetValue(), cur); ok {
			g := goal
			g.MonthlyContribution = money.New(v, cur)
			if err := app.PutGoal(g); err == nil {
				uistate.BumpDataRevision()
				uistate.RequestPersist()
			}
		}
	})

	explicit := ""
	if found && goal.MonthlyContribution.Amount > 0 {
		explicit = minorToMajorStr(goal.MonthlyContribution.Amount, goal.MonthlyContribution.Currency)
	}
	// The effective monthly (explicit or date-derived) as a placeholder hint.
	hint := minorToMajorStr(props.Line.Minor, props.Base)

	return Div(css.Class("zbb-savings-row"),
		Span(css.Class("zbb-savings-name"), props.Line.Name),
		Div(css.Class("zbb-savings-edit"),
			Span(css.Class("zbb-savings-cur", tw.TextDim), currency.Symbol(props.Base)),
			Input(css.Class("field zbb-savings-input fig"), Type("number"), Step("1"), Attr("min", "0"),
				Attr("data-testid", "budgets-savings-amt-"+props.Line.GoalID), Attr("aria-label", uistate.T("budgets.savingsMonthlyAria")),
				Placeholder(hint), Value(explicit), OnInput(onEdit))),
	)
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

// budgetToolbarWidget is the actions tile: the in-context methodology picker, the
// one-click 50/30/20 starter template, an "Add budget" button, a Formulas reveal
// toggle, and the smart-insights action. It writes the shared method setting + the
// Formulas atom so the summary/list/formula tiles react in step.
func budgetToolbarWidget(props budgetToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	formulasAtom := uistate.UseBudgetsShowFormulas()
	smartSettings := uistate.LoadSmartSettings()
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
	onToggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))
	// C114: one-click 50/30/20 starter template over last full month's income.
	// v1.0: this is a bulk mutation (up to ~10 new budgets), so it previews the
	// count and confirms before creating anything.
	apply503020 := ui.UseEvent(Prevent(func() {
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		now := time.Now()
		curStart := dateutil.MonthStart(now)
		prevStart := dateutil.AddMonths(curStart, -1)
		configuredIncome := uistate.CurrentPrefs().MonthlyIncomeMinor
		income := budgeting.IncomeForBudgets(configuredIncome, txns, prevStart, curStart, base, rates)
		if income <= 0 {
			uistate.PostNotice(uistate.T("budgets.tmplNoIncome"), true)
			return
		}
		res := budgeting.Generate5030(income, app.Categories(), txns, now)
		existing := map[string]bool{}
		for _, b := range app.Budgets() {
			existing[b.CategoryID] = true
		}
		var toAdd []domain.Budget
		for _, prop := range res.Proposals {
			if prop.LimitMinor <= 0 || existing[prop.Category.ID] {
				continue
			}
			toAdd = append(toAdd, domain.Budget{
				ID: id.New(), Name: prop.Category.Name, CategoryID: prop.Category.ID,
				Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
				Period: domain.PeriodMonthly, Limit: money.New(prop.LimitMinor, base),
			})
		}
		if len(toAdd) == 0 {
			uistate.PostNotice(uistate.T("budgets.tmplNothingToAdd"), false)
			return
		}
		uistate.ConfirmModalLabeled(uistate.T("budgets.tmplConfirm", plural(len(toAdd), "budget")), uistate.T("budgets.tmplConfirmBtn"), false, func(ok bool) {
			if !ok {
				return
			}
			n := 0
			for _, nb := range toAdd {
				if err := app.PutBudget(nb); err == nil {
					n++
				}
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("budgets.tmplApplied", plural(n, "budget")), false)
		})
	}))

	formulasLabel := uistate.T("budgets.showFormulas")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("budgets.hideFormulas")
	}

	toolbar := Div(css.Class("budgets-toolbar"),
		// Left: a compact, labelled methodology picker — no longer a full-width bar that
		// read like a search box (C112: standard / zero-based / envelope).
		Div(css.Class("budgets-toolbar-method"),
			Span(css.Class("budgets-toolbar-label"), uistate.T("settings.budgetMethod")),
			Select(css.Class("field", "budgets-method-select"), Attr("data-testid", "budgets-method"),
				Attr("aria-label", uistate.T("settings.budgetMethod")), Title(uistate.T("settings.budgetMethod")), OnChange(onMethod),
				Option(Value(string(budgeting.MethodSimple)), SelectedIf(method == budgeting.MethodSimple), uistate.T("settings.budgetMethodSimple")),
				Option(Value(string(budgeting.MethodZeroBased)), SelectedIf(method == budgeting.MethodZeroBased), uistate.T("settings.budgetMethodZero")),
				Option(Value(string(budgeting.MethodEnvelope)), SelectedIf(method == budgeting.MethodEnvelope), uistate.T("settings.budgetMethodEnvelope")),
			),
		),
		// Right: the actions, uniform-height and right-aligned, with the primary
		// "+ Add budget" last so it clearly outranks the ghost controls.
		Div(css.Class("budgets-toolbar-actions"),
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "budgets-last-month"), Attr("aria-pressed", ariaBool(lastMonthAtom.Get())),
				Title(uistate.T("budgets.lastMonthTitle")), OnClick(toggleLastMonth),
				uiw.Icon(icon.History, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T(lastMonthLabelKey(lastMonthAtom.Get())))),
			smartSectionAction(smartSettings),
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "budgets-autobudget"),
				Title(uistate.T("budgets.autoTitleAction")), OnClick(openAutoBudget),
				uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.autoTitle"))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budgets-template-503020"),
				Title(uistate.T("budgets.tmplTitle")), OnClick(apply503020), uistate.T("budgets.tmpl503020")),
			Button(css.Class("btn"), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "budgets-toggle-formulas"), Title(uistate.T("budgets.formulaTitle")),
				OnClick(onToggleFormulas), Text(formulasLabel)),
			If(hasBudgets, Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "budgets-add"), Title(uistate.T("budgets.add")), OnClick(addBudget),
				uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
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
	if uistate.UseBudgetsLastMonth().Get() {
		vw = vw.Shift(-1) // one-click "Last month" — evaluate the previous period
	}
	pr := uistate.UsePrefs().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr)
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

	var body ui.Node
	if len(v.Statuses) == 0 {
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("budgets.empty"), CTALabel: uistate.T("budgets.addFirst"), AddTarget: "budget", Icon: icon.Budgets})
	} else {
		cbs := buildBudgetRowCallbacks(app, v.Base, v.CatName)
		budgetDefs := app.CustomFieldDefsFor("budget")
		members := app.Members()
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
					Envelope: v.EnvAvail[s.Budget.ID], EnvelopeNeg: v.EnvNeg[s.Budget.ID], PaceOver: v.PaceOver[s.Budget.ID],
					RolloverCarry: v.RollCarry[s.Budget.ID], RolloverNeg: v.RollNeg[s.Budget.ID], EffectiveCap: v.RollEffCap[s.Budget.ID],
					ProratedRest: v.ProratedRest[s.Budget.ID], EffectiveMethod: v.EffMethod[s.Budget.ID],
					Covered:  v.Covered[s.Budget.ID],
					OnDelete: cbs.OnDelete, OnRemoveRecurring: cbs.OnRemoveRecurring, OnDrill: viewTransactions,
				})
			},
		)
		// Lay the budget cards out in a responsive grid so each is a compact 1-column
		// block (several per row) rather than a full-width bar — budgets don't need the
		// whole width, and a grid shows far more at a glance.
		body = Div(css.Class("budget-grid"), rows)
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

// --- budget-formula --------------------------------------------------------------

// budgetFormulaWidget is the opt-in "Budget metrics" tile (revealed by the toolbar's
// Formulas toggle). It embeds the reusable FormulaBuilder, which evaluates against the
// live engine variable surface — so it ties custom fields and formulas together for
// budgets: every number-typed budget custom field surfaces as a cf_budget_<key>
// variable (alongside budget count + the household aggregates), ready to compute over.
func budgetFormulaWidget(props budgetFormulaProps) ui.Node {
	body := Div(
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("budgets.formulaTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
