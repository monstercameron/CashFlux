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
	"github.com/monstercameron/CashFlux/internal/billsched"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcatalog"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// liveEngineVarsCache memoizes liveEngineVars by store revision + current month (the
// vars are computed over the current month, so the month is part of the identity).
// Callers only read the returned map, so sharing one instance is safe.
var liveEngineVarsCache = map[string]map[string]float64{}

// liveEngineVars computes the full engine variable surface (atoms + molecules +
// custom fields) over the current month, for live formula evaluation. This is the
// SAME surface the dashboard engine uses, so a formula built here behaves identically
// when bound to a widget — and every figure traces back to atoms. Memoized: walking
// the whole ledger is expensive and several tiles request it per render.
func liveEngineVars(app *appstate.App) map[string]float64 {
	mk := time.Now()
	key := revKey(app) + "|" + strconv.Itoa(mk.Year()*100+int(mk.Month()))
	return memoByRev(liveEngineVarsCache, key, func() map[string]float64 { return liveEngineVarsRaw(app) })
}

// allFormulaMetrics assembles the full metric catalog — every atom, molecule, custom
// field, and per-entity variable a formula can reference — in group order. Shared by
// the FormulaBuilder reference panel and the assistant's list_formula_metrics tool so
// both see exactly the same names.
func allFormulaMetrics(app *appstate.App) []widgetcatalog.Metric {
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
	metrics = append(metrics, widgetcatalog.HealthMetrics()...)
	metrics = append(metrics, widgetcatalog.CreditMetrics()...)
	metrics = append(metrics, widgetcatalog.BillsSmartMetrics()...)
	metrics = append(metrics, widgetcatalog.AssistantMetrics()...)
	metrics = append(metrics, widgetcatalog.SmartMetrics()...)
	return metrics
}

func liveEngineVarsRaw(app *appstate.App) map[string]float64 {
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
		Categories: app.Categories(), WeekStart: uistate.LoadPrefs().WeekStartWeekday(),
		Rates: rates, Now: now, PeriodStart: start, PeriodEnd: end,
		CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(), Pools: livePoolDefs(),
		Alloc: liveAllocData(), Plans: app.Plans(), Planning: livePlanningData(),
		BillsSmart: liveBillsSmartData(app), Smart: liveSmartCounts(),
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
	metrics := allFormulaMetrics(app)

	expr := ui.UseState(props.Initial)
	fName := ui.UseState("")
	fMsg := ui.UseState("")
	editID := ui.UseState("")
	rev := ui.UseState(0)
	_ = rev.Get()

	// Palette navigation: a search box (filters by label, variable name, or doc)
	// and per-group accordions. A nil openGroups map means the default posture
	// (first group open); once the user toggles, the map is authoritative.
	search := ui.UseState("")
	onSearch := ui.UseEvent(func(v string) { search.Set(v) })
	openGroups := ui.UseState[map[string]bool](nil)
	toggleGroup := func(g string) {
		cur := openGroups.Get()
		next := make(map[string]bool, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		if cur == nil && len(metrics) > 0 {
			// Materialize the default posture so the first toggle behaves as seen.
			next[string(metrics[0].Group)] = true
		}
		next[g] = !next[g]
		openGroups.Set(next)
	}

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

	// Variable palette: click-to-insert chips (label + live value). Groups are
	// DERIVED from the metrics themselves in first-appearance order — the old
	// hand-maintained group list silently dropped any group someone forgot to
	// add (the Assistant group was invisible here for exactly that reason).
	var groups []widgetcatalog.Group
	seenGroup := map[widgetcatalog.Group]bool{}
	for _, m := range metrics {
		if !seenGroup[m.Group] {
			seenGroup[m.Group] = true
			groups = append(groups, m.Group)
		}
	}

	q := strings.ToLower(strings.TrimSpace(search.Get()))
	// matchRank orders search results by how directly they answer the query:
	// label-prefix hits first, then label hits, then name hits, then doc hits —
	// so "health" leads with Health Score, not thirty internal weight atoms.
	matchRank := func(m widgetcatalog.Metric) int {
		label, name, doc := strings.ToLower(m.Label), strings.ToLower(m.Name), strings.ToLower(m.Doc)
		switch {
		case strings.HasPrefix(label, q):
			return 0
		case strings.Contains(label, q):
			return 1
		case strings.Contains(name, q):
			return 2
		case strings.Contains(doc, q):
			return 3
		}
		return -1
	}

	var palette []ui.Node
	if q != "" {
		// Searching: one flat grid of every match across all groups, best first.
		type ranked struct {
			m    widgetcatalog.Metric
			rank int
		}
		var hits []ranked
		for _, m := range metrics {
			if r := matchRank(m); r >= 0 {
				hits = append(hits, ranked{m, r})
			}
		}
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].rank < hits[j].rank })
		var chips []ui.Node
		for _, h := range hits {
			chips = append(chips, ui.CreateElement(formulaMetricRow, formulaMetricRowProps{
				Metric: h.m, Value: vars[h.m.Name], OnInsert: insert,
			}))
		}
		if len(chips) == 0 {
			palette = append(palette, P(css.Class("empty"), uistate.T("customize.searchEmpty", search.Get())))
		} else {
			palette = append(palette, Div(css.Class("fb-pal-group"),
				Span(css.Class("fb-pal-title"), uistate.T("customize.searchResults", len(chips))),
				Div(css.Class("fb-pal-grid"), chips),
			))
		}
	} else {
		// Browsing: one accordion per group (first open by default) so the
		// palette reads as a table of contents, not a wall of chips.
		for gi, g := range groups {
			var chips []ui.Node
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
			open := gi == 0
			if om := openGroups.Get(); om != nil {
				open = om[string(g)]
			}
			// Two example labels tell a closed group's story better than a bare
			// count ("Activity — Income, Spending … 81" instead of "ACTIVITY 81").
			var examples []string
			for _, m := range metrics {
				if m.Group == g && len(examples) < 2 {
					examples = append(examples, m.Label)
				}
			}
			palette = append(palette, ui.CreateElement(formulaPalGroup, formulaPalGroupProps{
				Title: string(g), Count: len(chips), Open: open, OnToggle: toggleGroup, Chips: chips,
				Examples: strings.Join(examples, ", "),
			}))
		}
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
			// The scope sentence that separates the page's two save concepts: saved
			// formulas are personal notes; compound variables (edited in the tile
			// below on the Studio surface) are live definitions the app computes from.
			P(css.Class("t-caption", tw.TextFaint), uistate.T("customize.savedScopeHint")),
		),
		// Palette below, separated by a hairline: a search box beside the hint,
		// then the group accordions (or the flat search results).
		Div(css.Class("fb-palette"),
			Div(css.Class("fb-pal-toolbar"),
				Input(css.Class("field", "fb-pal-search"), Type("search"),
					Attr("data-testid", "fb-search"),
					Attr("aria-label", uistate.T("customize.searchLabel")),
					// The placeholder carries the scale ("search 350+ variables") so the
					// box reads as the palette's doorway, not another form field.
					Placeholder(uistate.T("customize.searchPlaceholderN", len(metrics))),
					Value(search.Get()), OnInput(onSearch)),
				Span(css.Class("fb-palette-lead"), uistate.T("customize.varsInsertHint")),
			),
			Div(css.Class("fb-pal-groups"), palette),
		),
	)

	nodes := []ui.Node{workbench}
	if props.ShowSaved {
		nodes = append(nodes, savedFormulasCard(app.Formulas(), vars, loadFormula, deleteFormula))
	}
	return Fragment(nodes)
}

type formulaPalGroupProps struct {
	Title    string
	Count    int
	Open     bool
	OnToggle func(string)
	Chips    []ui.Node
	Examples string // first labels inside, shown while collapsed
}

// formulaPalGroup is one collapsible palette group: a toggle header carrying
// the group name + its variable count, and the chip grid when open. Its own
// component so the toggle hook sits at a stable position (groups render in a
// variable-length loop).
func formulaPalGroup(p formulaPalGroupProps) ui.Node {
	tog := ui.UseEvent(Prevent(func() { p.OnToggle(p.Title) }))
	caret := "▸"
	if p.Open {
		caret = "▾"
	}
	return Div(css.Class("fb-pal-group"),
		Button(css.Class("fb-pal-head"), Type("button"),
			Attr("aria-expanded", ariaBool(p.Open)),
			Attr("data-testid", "fb-group-"+p.Title),
			OnClick(tog),
			Span(css.Class("fb-pal-caret"), caret),
			Span(css.Class("fb-pal-title"), p.Title),
			If(!p.Open && p.Examples != "", Span(css.Class("fb-pal-examples"), p.Examples+", …")),
			Span(css.Class("fb-pal-count"), fmt.Sprintf("%d", p.Count)),
		),
		If(p.Open, Div(css.Class("fb-pal-grid"), p.Chips)),
	)
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
