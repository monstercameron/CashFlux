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
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Customize hosts the formula calculator: write an expression over your live
// figures (net worth, income, expense, counts) and see the result, powered by
// the sandboxed internal/formula engine. The available variables are listed.
func Customize() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
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

	var resultBody ui.Node
	switch e := strings.TrimSpace(expr.Get()); {
	case e == "":
		resultBody = P(Class("muted"), uistate.T("customize.formulaHint"))
	default:
		if val, err := formula.Eval(e, formula.Env{Vars: vars}); err != nil {
			resultBody = P(Class("err"), err.Error())
		} else {
			resultBody = Div(Class("stat-value"), formatFormulaValue(val))
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
		varRows = append(varRows, Div(Class("row"),
			Span(Class("row-desc"), k),
			Span(Class("amount fig"), strconv.FormatFloat(vars[k], 'f', -1, 64)),
		))
	}

	return Div(
		CustomFieldsManager(),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("customize.calcTitle")),
			P(Class("muted"), uistate.T("customize.calcDesc")),
			Form(Class("form-grid"),
				Input(Class("field field-wide"), Type("text"), Placeholder(uistate.T("customize.exprPlaceholder")), Value(expr.Get()), OnInput(onExpr)),
			),
			Div(Class("flex flex-wrap gap-2 mt-2 items-center"),
				Span(Class("muted"), uistate.T("customize.try")),
				Button(Class("data-btn"), Type("button"), OnClick(func() { expr.Set("round((income - expense) / income * 100)") }), uistate.T("customize.exSavings")),
				Button(Class("data-btn"), Type("button"), OnClick(func() { expr.Set("round(expense / income * 100)") }), uistate.T("customize.exSpending")),
				Button(Class("data-btn"), Type("button"), OnClick(func() { expr.Set("net_worth + liabilities") }), uistate.T("customize.exGross")),
				Button(Class("data-btn"), Type("button"), OnClick(func() { expr.Set("if(expense > income, 1, 0)") }), uistate.T("customize.exOverBudget")),
			),
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("customize.resultTitle")),
			resultBody,
		),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("customize.varsTitle")),
			Div(Class("rows"), varRows),
		),
	)
}

// formatFormulaValue renders a formula result (number, bool, or string).
func formatFormulaValue(v formula.Value) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
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
