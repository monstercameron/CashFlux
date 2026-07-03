// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/billsched"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcatalog"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// liveEngineVars computes the full engine variable surface (atoms + molecules +
// custom fields) over the current month, for live formula evaluation. This is the
// SAME surface the dashboard engine uses, so a formula built here behaves identically
// when bound to a widget — and every figure traces back to atoms.
func liveEngineVars(app *appstate.App) map[string]float64 {
	now := time.Now()
	start, end := dateutil.MonthRange(now)
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	return engineenv.Vars(engineenv.Data{
		Accounts: app.Accounts(), Transactions: app.Transactions(), Members: app.Members(),
		Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(), Recurring: app.Recurring(),
		Rates: rates, Now: now, PeriodStart: start, PeriodEnd: end,
		CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(), Pools: livePoolDefs(),
		Alloc: liveAllocData(), Plans: app.Plans(), Planning: livePlanningData(),
		BillsSmart: liveBillsSmartData(app),
	})
}

// liveBillsSmartData builds the smart-bill-schedule inputs for the engine
// variables' fixed 60-day window; views that page further use
// liveBillsSmartHorizon directly.
func liveBillsSmartData(app *appstate.App) engineenv.BillsSmartData {
	return liveBillsSmartHorizon(app, engineenv.BillsSmartHorizonDays)
}

// liveBillsSmartHorizon builds the smart-bill-schedule inputs: paydays projected
// from the prefs pay-cycle anchor + the configured frequency out to horizonDays,
// the expected net income per payday (recurring money-in scaled to the cycle),
// and the keep floor.
func liveBillsSmartHorizon(app *appstate.App, horizonDays int) engineenv.BillsSmartData {
	cfg := uistate.BillsSmartConfigGet()
	out := engineenv.BillsSmartData{MinKeepMinor: cfg.MinKeepMinor}
	anchor, err := time.Parse("2006-01-02", uistate.LoadPrefs().PayCycleAnchor)
	if err != nil {
		return out // no anchor configured — smart figures fall back to raw
	}
	out.Paydays = billsched.Paydays(anchor, cfg.PayFrequency, time.Now(), horizonDays)
	var monthlyIn int64
	for _, r := range app.Recurring() {
		if me := r.MonthlyEquivalent(); me > 0 {
			monthlyIn += me
		}
	}
	out.IncomePerPayday = monthlyIn * 12 / int64(payPeriodsPerYear(cfg.PayFrequency))
	return out
}

// payPeriodsPerYear maps a pay frequency to its yearly period count.
func payPeriodsPerYear(freq string) int {
	switch freq {
	case "weekly":
		return 52
	case "semimonthly":
		return 24
	case "monthly":
		return 12
	default: // biweekly
		return 26
	}
}

// livePlanningData converts the persisted planning config into engine PlanningData, so the
// runway_* / forecast_horizon variables reflect the current policy wherever a formula runs.
func livePlanningData() engineenv.PlanningData {
	c := uistate.PlanningConfigGet()
	return engineenv.PlanningData{RunwayBufferMinor: c.RunwayBufferMinor, RunwayDays: c.RunwayDays, ForecastMonths: c.ForecastMonths}
}

// liveAllocData converts the persisted allocate plan into engine AllocData, so the alloc_*
// variables reflect the current plan wherever a formula is evaluated.
func liveAllocData() engineenv.AllocData {
	c := uistate.AllocConfigGet()
	return engineenv.AllocData{AmountMinor: c.AmountMinor, ReserveMinor: c.ReserveMinor, MaxPerMinor: c.MaxPerMinor}
}

// livePoolDefs converts the persisted investment-pool config into engine PoolDefs, so each
// pool exposes a pool_<slug>_value variable across the formula/widget surface.
func livePoolDefs() []engineenv.PoolDef {
	pools := uistate.InvestPools()
	out := make([]engineenv.PoolDef, 0, len(pools))
	for _, p := range pools {
		out = append(out, engineenv.PoolDef{Name: p.Name, AccountIDs: p.AccountIDs})
	}
	return out
}

// FormulaBuilderProps configures an embeddable FormulaBuilder.
type FormulaBuilderProps struct {
	Title     string       // section heading; defaults to the calculator title
	Initial   string       // starting expression
	ShowSaved bool         // also render the user's saved-formulas list
	OnChange  func(string) // called whenever the expression changes (for host pages)
}

// FormulaBuilder is the reusable, embeddable formula-creation widget: an expression
// editor with example presets, a live result, a grouped reference of every available
// metric (atoms → molecules → custom fields, molecules showing how they're built from
// atoms) where clicking inserts the metric, a save-as-formula form, and an optional
// saved-formulas list. It evaluates against the real engine variable surface
// (liveEngineVars), so a formula behaves the same anywhere it's used. Drop it onto
// any page with FormulaBuilder(FormulaBuilderProps{...}).
func FormulaBuilder(props FormulaBuilderProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	vars := liveEngineVars(app)
	metrics := widgetcatalog.Metrics(app.CustomFieldDefs(), app.Molecules())
	metrics = append(metrics, widgetcatalog.BudgetMetrics(app.Budgets())...)
	metrics = append(metrics, widgetcatalog.AccountMetrics(app.Accounts())...)
	metrics = append(metrics, widgetcatalog.GoalMetrics(app.Goals())...)
	metrics = append(metrics, widgetcatalog.DebtMetrics(app.Accounts())...)
	metrics = append(metrics, widgetcatalog.PoolMetrics(livePoolDefs())...)
	metrics = append(metrics, widgetcatalog.AllocMetrics()...)
	metrics = append(metrics, widgetcatalog.PlanningMetrics(app.Plans())...)
	metrics = append(metrics, widgetcatalog.RecurringMetrics(app.Recurring())...)
	metrics = append(metrics, widgetcatalog.ReportsMetrics()...)
	metrics = append(metrics, widgetcatalog.NetWorthMetrics()...)
	metrics = append(metrics, widgetcatalog.BillsSmartMetrics()...)

	expr := ui.UseState(props.Initial)
	fName := ui.UseState("")
	fMsg := ui.UseState("")
	editID := ui.UseState("")
	rev := ui.UseState(0)
	_ = rev.Get()

	emit := func(v string) {
		expr.Set(v)
		if props.OnChange != nil {
			props.OnChange(v)
		}
	}
	onExpr := ui.UseEvent(func(v string) { emit(v) })
	onFName := ui.UseEvent(func(v string) { fName.Set(v) })
	insert := func(name string) {
		if cur := expr.Get(); cur != "" {
			emit(cur + " " + name)
		} else {
			emit(name)
		}
	}
	saveFormula := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(fName.Get())
		ex := strings.TrimSpace(expr.Get())
		if name == "" || ex == "" {
			fMsg.Set(uistate.T("customize.saveNeedsBoth"))
			return
		}
		fid := editID.Get()
		if fid == "" {
			fid = id.New()
		}
		if err := app.PutFormula(domain.Formula{ID: fid, Name: name, Expr: ex, Enabled: true}); err != nil {
			fMsg.Set(err.Error())
			return
		}
		fName.Set("")
		editID.Set("")
		fMsg.Set(uistate.T("customize.saved"))
		rev.Set(rev.Get() + 1)
	}))
	loadFormula := func(f domain.Formula) { emit(f.Expr); fName.Set(f.Name); editID.Set(f.ID); fMsg.Set("") }
	deleteFormula := func(fid string) {
		_ = app.DeleteFormula(fid)
		if editID.Get() == fid {
			editID.Set("")
		}
		rev.Set(rev.Get() + 1)
	}

	// Live result — shown inline beside the expression, not in a separate card.
	resultCls := "fb-result"
	var resultNode ui.Node
	switch e := strings.TrimSpace(expr.Get()); {
	case e == "":
		resultCls += " is-empty"
		resultNode = Span(css.Class("fb-result-val"), "—")
	default:
		if val, err := formula.Eval(e, formula.Env{Vars: vars}); err != nil {
			resultCls += " is-err"
			resultNode = Span(css.Class("fb-result-err"), Attr("role", "alert"), err.Error())
		} else {
			resultNode = Span(css.Class("fb-result-val"), formatFormulaValue(val))
		}
	}

	// Variable palette: a dense, click-to-insert grid of chips (label + live value),
	// grouped by category. Replaces the sprawling one-row-per-variable list.
	groups := []widgetcatalog.Group{widgetcatalog.GroupCore, widgetcatalog.GroupActivity, widgetcatalog.GroupCounts, widgetcatalog.GroupCustom, widgetcatalog.GroupBudgets, widgetcatalog.GroupAccounts, widgetcatalog.GroupGoals, widgetcatalog.GroupDebt, widgetcatalog.GroupPools, widgetcatalog.GroupAllocate, widgetcatalog.GroupPlanning, widgetcatalog.GroupRecurring, widgetcatalog.GroupReports, widgetcatalog.GroupNetWorth}
	palette := make([]ui.Node, 0, len(groups))
	for _, g := range groups {
		chips := make([]ui.Node, 0)
		for _, m := range metrics {
			if m.Group != g {
				continue
			}
			chips = append(chips, ui.CreateElement(formulaMetricRow, formulaMetricRowProps{
				Metric: m, Value: vars[m.Name], OnInsert: insert,
			}))
		}
		if len(chips) == 0 {
			continue
		}
		palette = append(palette,
			Div(css.Class("fb-pal-group"),
				Span(css.Class("fb-pal-title"), string(g)),
				Div(css.Class("fb-pal-grid"), chips),
			),
		)
	}

	title := props.Title
	if title == "" {
		title = uistate.T("customize.calcTitle")
	}

	workbench := Div(css.Class("fb"),
		// Workbench: the expression is the focal element, with the live result read out
		// inline to its right — no separate result card.
		Div(css.Class("fb-workbench"),
			Div(css.Class("fb-head"),
				Div(css.Class("fb-title"), title),
				Span(css.Class("fb-sub"), uistate.T("customize.calcDesc")),
			),
			Div(css.Class("fb-exprbar"),
				Input(css.Class("field", "fb-expr"), Type("text"), Attr("aria-label", uistate.T("customize.exprLabel")),
					Placeholder(uistate.T("customize.exprPlaceholder")), Value(expr.Get()), OnInput(onExpr)),
				Div(ClassStr(resultCls),
					Span(css.Class("fb-result-eq"), "="),
					resultNode,
				),
			),
			Div(css.Class("fb-presets"),
				Span(css.Class("fb-presets-lead"), uistate.T("customize.try")),
				ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exSavings"), Expr: "round((income - expense) / income * 100)", OnPick: emit}),
				ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exSpending"), Expr: "round(expense / income * 100)", OnPick: emit}),
				ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exGross"), Expr: "assets", OnPick: emit}),
				ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exOverBudget"), Expr: "if(expense > income, 1, 0)", OnPick: emit}),
			),
			Form(css.Class("fb-save"), OnSubmit(saveFormula),
				Input(css.Class("field", "fb-save-name"), Type("text"), Attr("aria-label", uistate.T("customize.nameLabel")),
					Placeholder(uistate.T("customize.savePlaceholder")), Value(fName.Get()), OnInput(onFName)),
				Button(css.Class("btn btn-primary", "fb-save-btn"), Type("submit"), uistate.T("customize.save")),
				If(fMsg.Get() != "", Span(css.Class("fb-msg"), fMsg.Get())),
			),
		),
		// Palette below, separated by a hairline.
		Div(css.Class("fb-palette"),
			Span(css.Class("fb-palette-lead"), uistate.T("customize.varsInsertHint")),
			Div(css.Class("fb-pal-groups"), palette),
		),
	)

	nodes := []ui.Node{workbench}
	if props.ShowSaved {
		nodes = append(nodes, savedFormulasCard(app.Formulas(), vars, loadFormula, deleteFormula))
	}
	return Fragment(nodes)
}

type formulaPresetProps struct {
	Label, Expr string
	OnPick      func(string)
}

// formulaPreset is one example-formula button (own hook for its click handler).
func formulaPreset(p formulaPresetProps) ui.Node {
	h := ui.UseEvent(func() { p.OnPick(p.Expr) })
	return Button(css.Class("data-btn"), Type("button"), OnClick(h), p.Label)
}

type formulaMetricRowProps struct {
	Metric   widgetcatalog.Metric
	Value    float64
	OnInsert func(string)
}

// formulaMetricRow renders one variable as a compact click-to-insert chip: the friendly
// label over its live value, with the raw variable name (and, for molecules, its atom
// formula) in the tooltip. Its own component (no hooks in loops).
func formulaMetricRow(p formulaMetricRowProps) ui.Node {
	ins := ui.UseEvent(Prevent(func() { p.OnInsert(p.Metric.Name) }))
	tip := p.Metric.Name
	if p.Metric.Molecule && p.Metric.Formula != "" {
		tip = p.Metric.Name + " = " + prettyFormula(p.Metric.Formula)
	}
	return Button(css.Class("fb-chip"), Type("button"), Title(tip),
		Attr("aria-label", uistate.T("customize.insertShort", p.Metric.Name)), OnClick(ins),
		Span(css.Class("fb-chip-label"), p.Metric.Label),
		Span(css.Class("fb-chip-val"), groupThousands(p.Value)),
	)
}
