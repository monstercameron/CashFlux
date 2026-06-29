// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
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
		CustomDefs: app.CustomFieldDefs(), Molecules: app.Molecules(),
	})
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

	// Live result.
	var resultBody ui.Node
	switch e := strings.TrimSpace(expr.Get()); {
	case e == "":
		resultBody = P(css.Class("muted"), uistate.T("customize.formulaHint"))
	default:
		if val, err := formula.Eval(e, formula.Env{Vars: vars}); err != nil {
			resultBody = P(css.Class("err"), Attr("role", "alert"), err.Error())
		} else {
			resultBody = Div(css.Class("stat-value"), formatFormulaValue(val))
		}
	}

	// Grouped metric reference: atoms, molecules (with their atom formula), counts,
	// custom fields. Click a row to insert it into the expression.
	groups := []widgetcatalog.Group{widgetcatalog.GroupCore, widgetcatalog.GroupActivity, widgetcatalog.GroupCounts, widgetcatalog.GroupCustom}
	refSections := make([]ui.Node, 0, len(groups))
	for _, g := range groups {
		rows := make([]ui.Node, 0)
		for _, m := range metrics {
			if m.Group != g {
				continue
			}
			rows = append(rows, ui.CreateElement(formulaMetricRow, formulaMetricRowProps{
				Metric: m, Value: vars[m.Name], OnInsert: insert,
			}))
		}
		if len(rows) == 0 {
			continue
		}
		refSections = append(refSections,
			Div(css.Class("fb-group"),
				Span(css.Class("fb-group-title"), string(g)),
				Div(css.Class("rows"), rows),
			),
		)
	}

	title := props.Title
	if title == "" {
		title = uistate.T("customize.calcTitle")
	}

	nodes := []ui.Node{
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: title,
			Body: Fragment(
				P(css.Class("muted"), uistate.T("customize.calcDesc")),
				Form(css.Class("form-grid"),
					Label(css.Class("labeled-field"), Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
						Span(css.Class("muted"), uistate.T("customize.exprLabel")),
						Input(css.Class("field field-wide"), Type("text"), Placeholder(uistate.T("customize.exprPlaceholder")), Value(expr.Get()), OnInput(onExpr)),
					),
				),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt3, tw.ItemsCenter),
					Span(css.Class("muted"), uistate.T("customize.try")),
					ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exSavings"), Expr: "round((income - expense) / income * 100)", OnPick: emit}),
					ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exSpending"), Expr: "round(expense / income * 100)", OnPick: emit}),
					ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exGross"), Expr: "assets", OnPick: emit}),
					ui.CreateElement(formulaPreset, formulaPresetProps{Label: uistate.T("customize.exOverBudget"), Expr: "if(expense > income, 1, 0)", OnPick: emit}),
				),
				Form(css.Class("form-grid", tw.Mt2), OnSubmit(saveFormula),
					Label(css.Class("labeled-field"), Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
						Span(css.Class("muted"), uistate.T("customize.nameLabel")),
						Input(css.Class("field"), Type("text"), Placeholder(uistate.T("customize.savePlaceholder")), Value(fName.Get()), OnInput(onFName)),
					),
					Button(css.Class("btn btn-primary"), Type("submit"), Style(map[string]string{"width": "fit-content", "align-self": "flex-end"}), uistate.T("customize.save")),
				),
				If(fMsg.Get() != "", P(css.Class("muted"), fMsg.Get())),
			),
		}),
		uiw.EntityListSection(uiw.EntityListSectionProps{Title: uistate.T("customize.resultTitle"), Body: resultBody}),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("customize.varsTitle"),
			Body:  Fragment(P(css.Class("muted"), uistate.T("customize.varsInsertHint")), Div(css.Class("fb-groups"), refSections)),
		}),
	}
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

// formulaMetricRow renders one reference metric: its friendly label (click to insert
// the variable name), the variable name with — for molecules — its atom formula, and
// the live value. Its own component (no hooks in loops).
func formulaMetricRow(p formulaMetricRowProps) ui.Node {
	ins := ui.UseEvent(Prevent(func() { p.OnInsert(p.Metric.Name) }))
	meta := p.Metric.Name
	if p.Metric.Molecule && p.Metric.Formula != "" {
		meta = p.Metric.Name + " = " + prettyFormula(p.Metric.Formula)
	}
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Button(css.Class("btn-link row-desc"), Type("button"),
				Title(uistate.T("customize.insertFormula", p.Metric.Name)),
				Attr("aria-label", uistate.T("customize.insertShort", p.Metric.Name)),
				OnClick(ins), p.Metric.Label),
			Span(css.Class("row-meta fb-meta"), meta),
		),
		Span(css.Class("amount fig"), groupThousands(p.Value)),
	)
}
