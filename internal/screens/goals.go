//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTarget := ui.UseEvent(func(v string) { target.Set(v) })
	onCurrent := ui.UseEvent(func(v string) { current.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })

	goalDefs := app.CustomFieldDefsFor("goal")
	onCustom := func(key, value string) {
		m := customVals.Get()
		nm := make(map[string]string, len(m)+1)
		for k, v := range m {
			nm[k] = v
		}
		nm[key] = value
		customVals.Set(nm)
	}

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
			Custom: customValuesToMap(goalDefs, customVals.Get()),
		}
		if err := app.PutGoal(g); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		target.Set("")
		current.Set("0")
		dateStr.Set("")
		customVals.Set(map[string]string{})
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

	saveGoal := func(id, newName, targetStr, dateStr string) {
		for _, g := range app.Goals() {
			if g.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				g.Name = n
			}
			cur := g.TargetAmount.Currency
			if cur == "" {
				cur = base
			}
			amt, err := money.ParseMinor(strings.TrimSpace(targetStr), currency.Decimals(cur))
			if err != nil || amt <= 0 {
				errMsg.Set("Enter a positive target amount.")
				return
			}
			g.TargetAmount = money.New(amt, cur)
			if ds := strings.TrimSpace(dateStr); ds != "" {
				d, derr := dateutil.ParseDate(ds)
				if derr != nil {
					errMsg.Set("Enter a valid target date (YYYY-MM-DD).")
					return
				}
				g.TargetDate = d
			} else {
				g.TargetDate = time.Time{}
			}
			if err := app.PutGoal(g); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
	}

	contribute := func(g domain.Goal, amtStr string) {
		cur := g.CurrentAmount.Currency
		if cur == "" {
			cur = base
		}
		amt, err := money.ParseMinor(strings.TrimSpace(amtStr), currency.Decimals(cur))
		if err != nil || amt == 0 {
			return
		}
		g.CurrentAmount = money.New(g.CurrentAmount.Amount+amt, cur)
		if err := app.PutGoal(g); err != nil {
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
			MapKeyed(goalDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
			}),
			Button(Class("btn btn-primary"), Type("submit"), "Add"),
		),
		If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
	)

	goals := app.Goals()
	// Incomplete goals first, then alphabetical.
	sort.SliceStable(goals, func(i, j int) bool {
		ci, _ := goalsvc.IsComplete(goals[i])
		cj, _ := goalsvc.IsComplete(goals[j])
		if ci != cj {
			return !ci
		}
		return goals[i].Name < goals[j].Name
	})

	var listBody ui.Node
	if len(goals) == 0 {
		listBody = P(Class("empty"), "No goals yet.")
	} else {
		rows := MapKeyed(goals,
			func(g domain.Goal) any { return g.ID },
			func(g domain.Goal) ui.Node {
				return ui.CreateElement(GoalRow, goalRowProps{Goal: g, OnDelete: deleteGoal, OnContribute: contribute, OnSave: saveGoal})
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
	Goal         domain.Goal
	OnDelete     func(string)
	OnContribute func(domain.Goal, string)
	OnSave       func(id, name, target, date string)
}

// GoalRow renders one goal's progress toward its target, with contribute and
// (inline) edit actions. All hooks are declared unconditionally so the edit
// toggle never reorders them.
func GoalRow(props goalRowProps) ui.Node {
	g := props.Goal
	targetMajor := money.FormatMinor(g.TargetAmount.Amount, currency.Decimals(g.TargetAmount.Currency))
	dateISO := ""
	if !g.TargetDate.IsZero() {
		dateISO = dateutil.FormatDate(g.TargetDate)
	}

	del := ui.UseEvent(Prevent(func() { props.OnDelete(g.ID) }))
	contribute := ui.UseEvent(Prevent(func() {
		if v := promptText("Contribute how much to " + g.Name + "?"); v != "" {
			props.OnContribute(g, v)
		}
	}))
	pr := uistate.UsePrefs().Get()
	editing := ui.UseState(false)
	nameS := ui.UseState(g.Name)
	targetS := ui.UseState(targetMajor)
	dateS := ui.UseState(dateISO)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onTarget := ui.UseEvent(func(v string) { targetS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(g.Name)
		targetS.Set(targetMajor)
		dateS.Set(dateISO)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(g.ID, nameS.Get(), targetS.Get(), dateS.Get())
		editing.Set(false)
	}))

	if editing.Get() {
		return Div(Class("budget"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Type("text"), Placeholder("Name"), Value(nameS.Get()), OnInput(onName)),
				Input(Class("field"), Type("number"), Placeholder("Target"), Value(targetS.Get()), Step("0.01"), OnInput(onTarget)),
				Input(Class("field"), Type("date"), Value(dateS.Get()), OnInput(onDate)),
				Button(Class("btn btn-primary"), Type("submit"), "Save"),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), "Cancel"),
			),
		)
	}

	pct := goalsvc.Percent(g)
	rem, _ := goalsvc.Remaining(g)
	complete, _ := goalsvc.IsComplete(g)

	sub := fmt.Sprintf("%d%% · %s to go", pct, fmtMoney(rem))
	if complete {
		sub = "Complete 🎉"
	}
	if !g.TargetDate.IsZero() {
		sub += " · by " + pr.FormatDate(g.TargetDate)
		if !complete {
			if per, ok, _ := goalsvc.MonthlyNeeded(g, time.Now()); ok {
				sub += " · save " + fmtMoney(per) + "/mo"
			}
		}
	}

	return Div(Class("budget"),
		Div(Class("budget-head"),
			Span(Class("row-desc"), g.Name),
			Span(Class("budget-amount"), fmtMoney(g.CurrentAmount)+" / "+fmtMoney(g.TargetAmount)),
			Button(Class("btn"), Type("button"), Title("Add to this goal"), OnClick(contribute), "Contribute"),
			Button(Class("btn"), Type("button"), Title("Edit goal"), OnClick(startEdit), "Edit"),
			Button(Class("btn-del"), Type("button"), Title("Delete goal"), OnClick(del), "✕"),
		),
		Div(Class("bar"), Div(Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", pct)))),
		Span(Class("budget-sub"), sub),
	)
}

// promptText shows a browser prompt and returns the entered text ("" if cancelled).
func promptText(message string) string {
	v := js.Global().Get("window").Call("prompt", message)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}
