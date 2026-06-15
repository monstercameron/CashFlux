//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Goals lists savings goals with progress, plus an add form and per-row delete.
func Goals() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
	}

	rev := state.UseAtom("rev:goals", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	name := ui.UseState("")
	target := ui.UseState("")
	current := ui.UseState("0")
	owner := ui.UseState(domain.GroupOwnerID)
	dateStr := ui.UseState("")
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTarget := ui.UseEvent(func(v string) { target.Set(v) })
	onCurrent := ui.UseEvent(func(v string) { current.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })

	add := ui.UseEvent(Prevent(func() {
		tgt, err := money.ParseMinor(strings.TrimSpace(target.Get()), currency.Decimals(base))
		if err != nil || tgt <= 0 {
			errMsg.Set("Enter a positive target amount.")
			return
		}
		cur, err := money.ParseMinor(strings.TrimSpace(current.Get()), currency.Decimals(base))
		if err != nil {
			cur = 0
		}
		var targetDate time.Time
		if ds := strings.TrimSpace(dateStr.Get()); ds != "" {
			if targetDate, err = dateutil.ParseDate(ds); err != nil {
				errMsg.Set("Enter a valid target date (YYYY-MM-DD).")
				return
			}
		}
		scope := domain.ScopeIndividual
		if owner.Get() == domain.GroupOwnerID {
			scope = domain.ScopeShared
		}
		g := domain.Goal{
			ID: id.New(), Name: strings.TrimSpace(name.Get()), Scope: scope, OwnerID: owner.Get(),
			TargetAmount: money.New(tgt, base), CurrentAmount: money.New(cur, base), TargetDate: targetDate,
		}
		if err := app.PutGoal(g); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		target.Set("")
		current.Set("0")
		dateStr.Set("")
		errMsg.Set("")
		bump()
	}))

	deleteGoal := func(goalID string) {
		if err := app.DeleteGoal(goalID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	ownerOptions := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(owner.Get() == domain.GroupOwnerID), "Group (shared)")}
	for _, m := range app.Members() {
		ownerOptions = append(ownerOptions, Option(Value(m.ID), SelectedIf(owner.Get() == m.ID), m.Name))
	}

	form := Section(Class("card"),
		H2(Class("card-title"), "Add goal"),
		Form(Class("form-grid"), OnSubmit(add),
			Input(Class("field"), Type("text"), Placeholder("Name"), Value(name.Get()), OnInput(onName)),
			Input(Class("field"), Type("number"), Placeholder("Target ("+base+")"), Value(target.Get()), Step("0.01"), OnInput(onTarget)),
			Input(Class("field"), Type("number"), Placeholder("Saved so far"), Value(current.Get()), Step("0.01"), OnInput(onCurrent)),
			Select(Class("field"), OnChange(onOwner), ownerOptions),
			Input(Class("field"), Type("date"), Value(dateStr.Get()), OnInput(onDate)),
			Button(Class("btn btn-primary"), Type("submit"), "Add"),
		),
		If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
	)

	goals := app.Goals()
	var listBody ui.Node
	if len(goals) == 0 {
		listBody = P(Class("empty"), "No goals yet.")
	} else {
		rows := MapKeyed(goals,
			func(g domain.Goal) any { return g.ID },
			func(g domain.Goal) ui.Node {
				return ui.CreateElement(GoalRow, goalRowProps{Goal: g, OnDelete: deleteGoal})
			},
		)
		listBody = Div(rows)
	}

	return Div(
		form,
		Section(Class("card"),
			H2(Class("card-title"), "Goals"),
			listBody,
		),
	)
}

type goalRowProps struct {
	Goal     domain.Goal
	OnDelete func(string)
}

// GoalRow renders one goal's progress toward its target.
func GoalRow(props goalRowProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { props.OnDelete(props.Goal.ID) }))

	g := props.Goal
	pct := goalsvc.Percent(g)
	rem, _ := goalsvc.Remaining(g)
	complete, _ := goalsvc.IsComplete(g)

	sub := fmt.Sprintf("%d%% · %s to go", pct, fmtMoney(rem))
	if complete {
		sub = "Complete 🎉"
	}
	if !g.TargetDate.IsZero() {
		sub += " · by " + dateutil.FormatDate(g.TargetDate)
	}

	return Div(Class("budget"),
		Div(Class("budget-head"),
			Span(Class("row-desc"), g.Name),
			Span(Class("budget-amount"), fmtMoney(g.CurrentAmount)+" / "+fmtMoney(g.TargetAmount)),
			Button(Class("btn-del"), Type("button"), Title("Delete goal"), OnClick(del), "✕"),
		),
		Div(Class("bar"), Div(Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", pct)))),
		Span(Class("budget-sub"), sub),
	)
}
