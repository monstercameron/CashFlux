//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
	period := ui.UseState(string(domain.PeriodMonthly))
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")
	monthOffset := ui.UseState(0)
	weekStart := uistate.UsePrefs().Get().WeekStartWeekday()

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limit.Set(v) })
	onCat := ui.UseEvent(func(e ui.Event) { catID.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	onPeriod := ui.UseEvent(func(e ui.Event) { period.Set(e.GetValue()) })

	budgetDefs := app.CustomFieldDefsFor("budget")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}
	prevMonth := ui.UseEvent(func() { monthOffset.Set(monthOffset.Get() - 1) })
	nextMonth := ui.UseEvent(func() { monthOffset.Set(monthOffset.Get() + 1) })

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
			CategoryID: catID.Get(), Period: domain.Period(period.Get()), Limit: money.New(amt, base),
			Custom: customValuesToMap(budgetDefs, customVals.Get()),
		}
		if err := app.PutBudget(b); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		limit.Set("")
		customVals.Set(map[string]string{})
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

	saveBudget := func(id, newName, limitStr, periodStr, ownerID string) {
		for _, b := range app.Budgets() {
			if b.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				b.Name = n
			}
			amt, err := money.ParseMinor(strings.TrimSpace(limitStr), currency.Decimals(base))
			if err != nil || amt <= 0 {
				errMsg.Set("Enter a positive limit.")
				return
			}
			b.Limit = money.New(amt, base)
			if p := domain.Period(periodStr); p.Valid() {
				b.Period = p
			}
			b.OwnerID = ownerID
			if ownerID == domain.GroupOwnerID {
				b.Scope = domain.ScopeShared
			} else {
				b.Scope = domain.ScopeIndividual
			}
			if err := app.PutBudget(b); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
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
				Select(Class("field"), Title("Period"), OnChange(onPeriod), periodOptions(period.Get())),
				Input(Class("field"), Type("number"), Placeholder("Limit ("+base+")"), Value(limit.Get()), Step("0.01"), OnInput(onLimit)),
				MapKeyed(budgetDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
					return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
				}),
				Button(Class("btn btn-primary"), Type("submit"), "Add"),
			),
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		)
	}

	budgets := app.Budgets()
	txns := app.Transactions()
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	viewMonth := dateutil.AddMonths(time.Now(), monthOffset.Get())
	// Each budget is evaluated over its own period window around the viewed date.
	statuses := make([]budgeting.Status, 0, len(budgets))
	for _, b := range budgets {
		bs, be := budgeting.PeriodRange(b.Period, viewMonth, weekStart)
		st, err := budgeting.Evaluate(b, txns, bs, be, rates, budgeting.DefaultNearThreshold)
		if err != nil {
			continue
		}
		statuses = append(statuses, st)
	}

	overCount, nearCount := 0, 0
	var totalSpent, totalLimit int64
	for _, s := range statuses {
		switch s.State {
		case budgeting.StateOver:
			overCount++
		case budgeting.StateNear:
			nearCount++
		}
		totalSpent += s.Spent.Amount
		totalLimit += s.Spent.Amount + s.Remaining.Amount // limit = spent + remaining
	}

	var listBody ui.Node
	if len(statuses) == 0 {
		listBody = P(Class("empty"), "No budgets yet.")
	} else {
		rows := MapKeyed(statuses,
			func(s budgeting.Status) any { return s.Budget.ID },
			func(s budgeting.Status) ui.Node {
				return ui.CreateElement(BudgetRow, budgetRowProps{Status: s, Category: catName[s.Budget.CategoryID], Members: app.Members(), OnDelete: deleteBudget, OnSave: saveBudget})
			},
		)
		listBody = Div(rows)
	}

	return Div(
		formCard,
		If(len(statuses) > 0, Div(Class("stat-grid"),
			stat("Spent", fmtMoney(money.New(totalSpent, base)), "neg"),
			stat("Budgeted", fmtMoney(money.New(totalLimit, base)), ""),
			stat("Left", fmtMoney(money.New(totalLimit-totalSpent, base)), accentFor(money.New(totalLimit-totalSpent, base))),
		)),
		Section(Class("card"),
			Div(Class("budget-head"),
				H2(Class("card-title"), "Budgets"),
				Span(Class("rpill"),
					Button(Class("rstep"), Type("button"), Title("Previous month"), OnClick(prevMonth), "‹"),
					Span(Class("rlabel fig"), viewMonth.Format("January 2006")),
					Button(Class("rstep"), Type("button"), Title("Next month"), OnClick(nextMonth), "›"),
				),
			),
			If(overCount > 0 || nearCount > 0, P(Class("budget-sub"), fmt.Sprintf("%d over budget · %d near the limit", overCount, nearCount))),
			listBody,
		),
	)
}

type budgetRowProps struct {
	Status   budgeting.Status
	Category string
	Members  []domain.Member
	OnDelete func(string)
	OnSave   func(id, name, limit, period, owner string)
}

// periodOptions builds the budget-period <option>s with selected marked.
func periodOptions(selected string) []ui.Node {
	opts := make([]ui.Node, 0, len(domain.AllPeriods))
	for _, p := range domain.AllPeriods {
		opts = append(opts, Option(Value(string(p)), SelectedIf(selected == string(p)), p.Label()))
	}
	return opts
}

// ownerSelectOptions builds owner <option>s (the shared group plus each member)
// with selected marked — used wherever an entity's owner can be chosen.
func ownerSelectOptions(members []domain.Member, selected string) []ui.Node {
	opts := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(selected == domain.GroupOwnerID), "Group (shared)")}
	for _, m := range members {
		opts = append(opts, Option(Value(m.ID), SelectedIf(selected == m.ID), m.Name))
	}
	return opts
}

// BudgetRow renders one budget's spend vs limit with a progress bar. Clicking
// Edit swaps in an inline form for the name, limit, and period. It owns all its
// hooks (declared unconditionally) so the edit toggle never disturbs hook order.
func BudgetRow(props budgetRowProps) ui.Node {
	s := props.Status
	limitMajor := money.FormatMinor(s.Budget.Limit.Amount, currency.Decimals(s.Budget.Limit.Currency))

	del := ui.UseEvent(Prevent(func() { props.OnDelete(s.Budget.ID) }))
	editing := ui.UseState(false)
	nameS := ui.UseState(s.Budget.Name)
	limitS := ui.UseState(limitMajor)
	periodS := ui.UseState(string(s.Budget.Period))
	ownerS := ui.UseState(s.Budget.OwnerID)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onLimit := ui.UseEvent(func(v string) { limitS.Set(v) })
	onPeriod := ui.UseEvent(func(e ui.Event) { periodS.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(s.Budget.Name)
		limitS.Set(limitMajor)
		periodS.Set(string(s.Budget.Period))
		ownerS.Set(s.Budget.OwnerID)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(s.Budget.ID, nameS.Get(), limitS.Get(), periodS.Get(), ownerS.Get())
		editing.Set(false)
	}))

	if editing.Get() {
		return Div(Class("budget"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(nameS.Get()), OnInput(onName)),
				Input(Class("field"), Type("number"), Placeholder("Limit"), Value(limitS.Get()), Step("0.01"), OnInput(onLimit)),
				Select(Class("field"), Title("Period"), OnChange(onPeriod), periodOptions(periodS.Get())),
				Select(Class("field"), Title("Owner"), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get())),
				Button(Class("btn btn-primary"), Type("submit"), "Save"),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), "Cancel"),
			),
		)
	}

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
			Button(Class("btn"), Type("button"), Title("Edit budget"), OnClick(startEdit), "Edit"),
			Button(Class("btn-del"), Type("button"), Title("Delete budget"), OnClick(del), "✕"),
		),
		Div(Class("bar"), Div(Class(fillClass), Attr("style", fmt.Sprintf("width:%d%%", width)))),
		Span(Class("budget-sub"), fmt.Sprintf("%s · %s · %d%% · %s left", s.Budget.Period.Label(), label, s.Percent, fmtMoney(s.Remaining))),
	)
}
