//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Budgets shows spend against each budget for the current month, with an add
// form and per-row delete.
func Budgets() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	rev := state.UseAtom("rev:budgets", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	categories := app.Categories()
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	var expenseCats []domain.Category
	for _, c := range categories {
		if c.Kind == domain.KindExpense {
			expenseCats = append(expenseCats, c)
		}
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	name := ui.UseState("")
	limit := ui.UseState("")
	defaultCat := ""
	if len(expenseCats) > 0 {
		defaultCat = expenseCats[0].ID
	}
	catID := ui.UseState(defaultCat)
	owner := ui.UseState(domain.GroupOwnerID)
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limit.Set(v) })
	onCat := ui.UseEvent(func(e ui.Event) { catID.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })

	add := ui.UseEvent(Prevent(func() {
		amt, err := money.ParseMinor(strings.TrimSpace(limit.Get()), currency.Decimals(base))
		if err != nil || amt <= 0 {
			errMsg.Set("Enter a positive limit.")
			return
		}
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		b := domain.Budget{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), Scope: scope, OwnerID: owner.Get(),
			CategoryID: catID.Get(), Period: domain.PeriodMonthly, Limit: money.New(amt, base),
		}
		if err := app.PutBudget(b); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		limit.Set("")
		errMsg.Set("")
		bump()
	}))

	deleteBudget := func(budgetID string) {
		if err := app.DeleteBudget(budgetID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	var formCard ui.Node
	if len(expenseCats) == 0 {
		formCard = Section(Class("card"), P(Class("empty"), "Add an expense category first, then create budgets."))
	} else {
		catOptions := make([]ui.Node, 0, len(expenseCats))
		for _, c := range expenseCats {
			catOptions = append(catOptions, Option(Value(c.ID), SelectedIf(catID.Get() == c.ID), c.Name))
		}
		ownerOptions := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(owner.Get() == domain.GroupOwnerID), "Group (shared)")}
		for _, m := range app.Members() {
			ownerOptions = append(ownerOptions, Option(Value(m.ID), SelectedIf(owner.Get() == m.ID), m.Name))
		}
		formCard = Section(Class("card"),
			H2(Class("card-title"), "Add budget"),
			Form(Class("form-grid"), OnSubmit(add),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(name.Get()), OnInput(onName)),
				Select(Class("field"), OnChange(onCat), catOptions),
				Select(Class("field"), OnChange(onOwner), ownerOptions),
				Input(Class("field"), Type("number"), Placeholder("Monthly limit ("+base+")"), Value(limit.Get()), Step("0.01"), OnInput(onLimit)),
				Button(Class("btn btn-primary"), Type("submit"), "Add"),
			),
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		)
	}

	budgets := app.Budgets()
	txns := app.Transactions()
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	start, end := dateutil.MonthRange(time.Now())
	statuses, _ := budgeting.EvaluateAll(budgets, txns, start, end, rates, budgeting.DefaultNearThreshold)

	var listBody ui.Node
	if len(statuses) == 0 {
		listBody = P(Class("empty"), "No budgets yet.")
	} else {
		rows := MapKeyed(statuses,
			func(s budgeting.Status) any { return s.Budget.ID },
			func(s budgeting.Status) ui.Node {
				return ui.CreateElement(BudgetRow, budgetRowProps{Status: s, Category: catName[s.Budget.CategoryID], OnDelete: deleteBudget})
			},
		)
		listBody = Div(rows)
	}

	return Div(
		formCard,
		Section(Class("card"),
			H2(Class("card-title"), "This month"),
			listBody,
		),
	)
}

type budgetRowProps struct {
	Status   budgeting.Status
	Category string
	OnDelete func(string)
}

// BudgetRow renders one budget's spend vs limit with a progress bar.
func BudgetRow(props budgetRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Status.Budget.ID) }))

	s := props.Status
	limit, _ := s.Spent.Add(s.Remaining) // limit in base currency

	width := s.Percent
	if width > 100 {
		width = 100
	}
	fillClass := "bar-fill"
	label := "On track"
	switch s.State {
	case budgeting.StateNear:
		fillClass = "bar-fill near"
		label = "Near limit"
	case budgeting.StateOver:
		fillClass = "bar-fill over"
		label = "Over budget"
	}

	title := s.Budget.Name
	if props.Category != "" {
		title += " · " + props.Category
	}

	return Div(Class("budget"),
		Div(Class("budget-head"),
			Span(Class("row-desc"), title),
			Span(Class("budget-amount"), fmtMoney(s.Spent)+" / "+fmtMoney(limit)),
			Button(Class("btn-del"), Type("button"), Title("Delete budget"), OnClick(del), "✕"),
		),
		Div(Class("bar"), Div(Class(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
		Span(Class("budget-sub"), fmt.Sprintf("%s · %d%% · %s left", label, s.Percent, fmtMoney(s.Remaining))),
	)
}
