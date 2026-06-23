//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// FormulaCalculator renders the formula-calculator section of the Customize screen:
// expression editor, example buttons, live result, save form, saved-formulas card,
// and the available-variables reference panel. It is the featured power-user tool
// (G15 §1) and appears above the fold in Customize().
func FormulaCalculator() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	net, assets, liabilities, _ := ledger.NetWorth(accounts, txns, rates)
	start, end := dateutil.MonthRange(time.Now())
	income, expense, _ := ledger.PeriodTotals(txns, start, end, rates)

	div := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		div *= 10
	}
	major := func(m money.Money) float64 { return float64(m.Amount) / div }

	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}

	vars := map[string]float64{
		"net_worth":    major(net),
		"assets":       major(assets),
		"liabilities":  major(liabilities),
		"income":       major(income),
		"expense":      major(expense),
		"accounts":     float64(active),
		"transactions": float64(len(txns)),
		"members":      float64(len(app.Members())),
		"budgets":      float64(len(app.Budgets())),
		"goals":        float64(len(app.Goals())),
		"tasks":        float64(len(app.Tasks())),
	}

	expr := ui.UseState("")
	onExpr := ui.UseEvent(func(v string) { expr.Set(v) })
	fName := ui.UseState("")
	fMsg := ui.UseState("")
	// editID tracks the formula loaded into the editor so Save updates it in place
	// instead of minting a new ID (which silently duplicated it on every load→save).
	// Empty = a brand-new formula; cleared after a save or a "New" reset.
	editID := ui.UseState("")
	rev := ui.UseState(0)
	onFName := ui.UseEvent(func(v string) { fName.Set(v) })
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
	deleteFormula := func(fid string) {
		_ = app.DeleteFormula(fid)
		if editID.Get() == fid {
			editID.Set("")
		}
		rev.Set(rev.Get() + 1)
	}
	loadFormula := func(f domain.Formula) { expr.Set(f.Expr); fName.Set(f.Name); editID.Set(f.ID); fMsg.Set("") }

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

	// Available-variables reference, sorted for stable display.
	names := make([]string, 0, len(vars))
	for k := range vars {
		names = append(names, k)
	}
	sort.Strings(names)
	varRows := make([]ui.Node, 0, len(names))
	for _, k := range names {
		kk := k // capture for closure
		varRows = append(varRows, ui.CreateElement(varInsertRow, varInsertRowProps{
			Name: kk, Value: vars[kk], OnInsert: func(name string) {
				cur := expr.Get()
				if cur != "" {
					expr.Set(cur + " " + name)
				} else {
					expr.Set(name)
				}
			},
		}))
	}

	return Fragment(
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("customize.calcTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("customize.calcDesc")),
				Form(css.Class("form-grid"),
					Label(css.Class("labeled-field"),
						Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
						Span(css.Class("muted"), uistate.T("customize.exprLabel")),
						Input(css.Class("field field-wide"), Type("text"), Placeholder(uistate.T("customize.exprPlaceholder")), Value(expr.Get()), OnInput(onExpr)),
					),
				),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt3, tw.ItemsCenter),
					Span(css.Class("muted"), uistate.T("customize.try")),
					Button(css.Class("data-btn"), Type("button"), OnClick(func() { expr.Set("round((income - expense) / income * 100)") }), uistate.T("customize.exSavings")),
					Button(css.Class("data-btn"), Type("button"), OnClick(func() { expr.Set("round(expense / income * 100)") }), uistate.T("customize.exSpending")),
					Button(css.Class("data-btn"), Type("button"), OnClick(func() { expr.Set("net_worth + liabilities") }), uistate.T("customize.exGross")),
					Button(css.Class("data-btn"), Type("button"), OnClick(func() { expr.Set("if(expense > income, 1, 0)") }), uistate.T("customize.exOverBudget")),
				),
				Form(css.Class("form-grid", tw.Mt2), OnSubmit(saveFormula),
					Label(css.Class("labeled-field"),
						Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}),
						Span(css.Class("muted"), uistate.T("customize.nameLabel")),
						Input(css.Class("field"), Type("text"), Placeholder(uistate.T("customize.savePlaceholder")), Value(fName.Get()), OnInput(onFName)),
					),
					Button(css.Class("btn btn-primary"), Type("submit"),
						Style(map[string]string{"width": "fit-content", "align-self": "flex-end"}),
						uistate.T("customize.save")),
				),
				If(fMsg.Get() != "", P(css.Class("muted"), fMsg.Get())),
			),
		}),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("customize.resultTitle"),
			Body:  resultBody,
		}),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("customize.varsTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("customize.varsInsertHint")),
				Div(css.Class("rows"), varRows),
			),
		}),
		savedFormulasCard(app.Formulas(), vars, loadFormula, deleteFormula),
	)
}

// savedFormulasCard lists the user's saved formulas, each evaluated live against
// the current figures, with load-into-editor and delete actions. Hidden when
// there are none.
func savedFormulasCard(formulas []domain.Formula, vars map[string]float64, onLoad func(domain.Formula), onDelete func(string)) ui.Node {
	if len(formulas) == 0 {
		return Fragment()
	}
	rows := make([]ui.Node, 0, len(formulas))
	for _, f := range formulas {
		rows = append(rows, ui.CreateElement(SavedFormulaRow, savedFormulaRowProps{
			Formula: f, Result: evalFormulaDisplay(f.Expr, vars), OnLoad: onLoad, OnDelete: onDelete,
		}))
	}
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("customize.savedTitle"),
		Rows:  rows,
	})
}

// evalFormulaDisplay evaluates an expression against the live vars and returns the
// formatted result, or the error text on failure.
func evalFormulaDisplay(expr string, vars map[string]float64) string {
	v, err := formula.Eval(expr, formula.Env{Vars: vars})
	if err != nil {
		return uistate.T("customize.evalError")
	}
	return formatFormulaValue(v)
}

type savedFormulaRowProps struct {
	Formula  domain.Formula
	Result   string
	OnLoad   func(domain.Formula)
	OnDelete func(string)
}

// SavedFormulaRow renders one saved formula with its live result, a button to
// load it into the editor, and a delete button. It owns its handlers (per the
// no-hooks-in-loops rule).
func SavedFormulaRow(props savedFormulaRowProps) ui.Node {
	f := props.Formula
	load := ui.UseEvent(Prevent(func() { props.OnLoad(f) }))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(f.ID) }))
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), f.Name),
			Span(css.Class("row-meta"), f.Expr),
		),
		Span(css.Class("amount fig"), props.Result),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("customize.loadTitle")), OnClick(load), uistate.T("customize.load")),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("customize.deleteTitle")), Title(uistate.T("customize.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

// groupThousands renders a float with thousands separators and up to two
// decimals (trailing zeros trimmed), so formula results and variable values read
// like the rest of the app's figures (354,070 not 354070) instead of raw floats
// (C61, matching the C2 money-formatting style).
func groupThousands(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	var b strings.Builder
	n := len(intPart)
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(intPart[i])
	}
	out := b.String() + frac
	if neg {
		out = "-" + out
	}
	return out
}

type varInsertRowProps struct {
	Name     string
	Value    float64
	OnInsert func(string)
}

// varInsertRow renders one available variable with its live value and a click-to-insert
// button so the user can tap the name to append it to the formula expression (C61).
func varInsertRow(props varInsertRowProps) ui.Node {
	ins := ui.UseEvent(Prevent(func() { props.OnInsert(props.Name) }))
	return Div(css.Class("row"),
		Button(css.Class("btn-link row-desc"), Type("button"),
			Title("Insert "+props.Name+" into the formula"),
			Attr("aria-label", "Insert "+props.Name),
			OnClick(ins), props.Name),
		Span(css.Class("amount fig"), groupThousands(props.Value)),
	)
}

// formatFormulaValue renders a formula result (number, bool, or string).
func formatFormulaValue(v formula.Value) string {
	switch x := v.(type) {
	case float64:
		return groupThousands(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return x
	default:
		return fmt.Sprintf("%v", v)
	}
}
