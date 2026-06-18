//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
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
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:goals", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	accounts := app.Accounts()
	name := ui.UseState("")
	target := ui.UseState("")
	current := ui.UseState("0")
	owner := ui.UseState(domain.GroupOwnerID)
	dateStr := ui.UseState("")
	linkAcct := ui.UseState("")
	customVals := ui.UseState(map[string]string{})
	errMsg := ui.UseState("")

	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onTarget := ui.UseEvent(func(v string) { target.Set(v) })
	onCurrent := ui.UseEvent(func(v string) { current.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateStr.Set(v) })
	onOwner := ui.UseEvent(func(e ui.Event) { owner.Set(e.GetValue()) })
	onLinkAcct := ui.UseEvent(func(e ui.Event) { linkAcct.Set(e.GetValue()) })

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
			errMsg.Set(uistate.T("goals.targetRequired"))
			return
		}
		cur, err := money.ParseMinor(strings.TrimSpace(current.Get()), currency.Decimals(base))
		if err != nil {
			cur = 0
		}
		var targetDate time.Time
		if ds := strings.TrimSpace(dateStr.Get()); ds != "" {
			if targetDate, err = dateutil.ParseDate(ds); err != nil {
				errMsg.Set(uistate.T("goals.invalidDate"))
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
			AccountID: linkAcct.Get(), Custom: customValuesToMap(goalDefs, customVals.Get()),
		}
		if err := app.PutGoal(g); err != nil {
			errMsg.Set(err.Error())
			return
		}
		name.Set("")
		target.Set("")
		current.Set("0")
		dateStr.Set("")
		linkAcct.Set("")
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

	saveGoal := func(id, newName, targetStr, dateStr, accountID, ownerID string) {
		for _, g := range app.Goals() {
			if g.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				g.Name = n
			}
			g.AccountID = accountID
			g.OwnerID = ownerID
			if ownerID == domain.GroupOwnerID {
				g.Scope = domain.ScopeShared
			} else {
				g.Scope = domain.ScopeIndividual
			}
			cur := g.TargetAmount.Currency
			if cur == "" {
				cur = base
			}
			amt, err := money.ParseMinor(strings.TrimSpace(targetStr), currency.Decimals(cur))
			if err != nil || amt <= 0 {
				errMsg.Set(uistate.T("goals.targetRequired"))
				return
			}
			g.TargetAmount = money.New(amt, cur)
			if ds := strings.TrimSpace(dateStr); ds != "" {
				d, derr := dateutil.ParseDate(ds)
				if derr != nil {
					errMsg.Set(uistate.T("goals.invalidDate"))
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

	ownerOptions := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(owner.Get() == domain.GroupOwnerID), uistate.T("owner.group"))}
	for _, m := range app.Members() {
		ownerOptions = append(ownerOptions, Option(Value(m.ID), SelectedIf(owner.Get() == m.ID), m.Name))
	}
	linkOptions := goalAccountOptions(accounts, linkAcct.Get())

	form := Section(Class("card"),
		H2(Class("card-title"), uistate.T("goals.add")),
		Form(Class("form-grid"), OnSubmit(add),
			Input(append([]any{Class("field"), Attr("id", "goal-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("common.name")), Value(name.Get()), OnInput(onName)}, errAttrs("goal-err", errMsg.Get())...)...),
			Input(Class("field"), Type("number"), Attr("aria-required", "true"), Placeholder(uistate.T("goals.targetPlaceholder", base)), Value(target.Get()), Step("0.01"), OnInput(onTarget)),
			Input(Class("field"), Type("number"), Placeholder(uistate.T("goals.savedSoFar")), Value(current.Get()), Step("0.01"), OnInput(onCurrent)),
			Select(Class("field"), OnChange(onOwner), ownerOptions),
			Select(Class("field"), Title(uistate.T("goals.linkedOptional")), OnChange(onLinkAcct), linkOptions),
			Input(Class("field"), Type("date"), Value(dateStr.Get()), OnInput(onDate)),
			MapKeyed(goalDefs, func(d customfields.Def) any { return d.ID }, func(d customfields.Def) ui.Node {
				return ui.CreateElement(CustomFieldInput, customFieldInputProps{Def: d, Value: customVals.Get()[d.Key], OnChange: onCustom})
			}),
			Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		),
		errText("goal-err", errMsg.Get()),
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

	// Combined progress across all goals (amounts are in the base currency).
	var savedTotal, targetTotal int64
	for _, g := range goals {
		savedTotal += g.CurrentAmount.Amount
		targetTotal += g.TargetAmount.Amount
	}
	overallPct := 0
	if targetTotal > 0 {
		overallPct = int(savedTotal * 100 / targetTotal)
		if overallPct > 100 {
			overallPct = 100
		}
	}

	var listBody ui.Node
	if len(goals) == 0 {
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("goals.empty"), CTALabel: uistate.T("goals.addFirst"), FocusID: "goal-add"})
	} else {
		rows := MapKeyed(goals,
			func(g domain.Goal) any { return g.ID },
			func(g domain.Goal) ui.Node {
				return ui.CreateElement(GoalRow, goalRowProps{Goal: g, Accounts: accounts, Members: app.Members(), OnDelete: deleteGoal, OnContribute: contribute, OnSave: saveGoal})
			},
		)
		listBody = Div(rows)
	}

	return Div(
		form,
		If(len(goals) > 0, Div(Class("stat-grid"),
			stat(uistate.T("goals.savedSoFar"), fmtMoney(money.New(savedTotal, base)), "pos"),
			stat(uistate.T("goals.totalTarget"), fmtMoney(money.New(targetTotal, base)), ""),
			stat(uistate.T("goals.overallProgress"), fmt.Sprintf("%d%%", overallPct), ""),
		)),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("nav.goals")),
			listBody,
		),
	)
}

type goalRowProps struct {
	Goal         domain.Goal
	Accounts     []domain.Account
	Members      []domain.Member
	OnDelete     func(string)
	OnContribute func(domain.Goal, string)
	OnSave       func(id, name, target, date, accountID, owner string)
}

// goalAccountOptions builds the linked-account <option>s for a goal, with a
// leading "no link" choice; selected matches the current AccountID.
func goalAccountOptions(accounts []domain.Account, selected string) []ui.Node {
	opts := []ui.Node{Option(Value(""), SelectedIf(selected == ""), "— No linked account —")}
	for _, a := range accounts {
		opts = append(opts, Option(Value(a.ID), SelectedIf(selected == a.ID), a.Name))
	}
	return opts
}

// accountName returns an account's name by id, or "" when not found.
func accountName(accounts []domain.Account, id string) string {
	if a, ok := domain.AccountByID(accounts, id); ok {
		return a.Name
	}
	return ""
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
	pr := uistate.UsePrefs().Get()
	editing := ui.UseState(false)
	contributing := ui.UseState(false)
	contribAmtS := ui.UseState("")
	contribute := ui.UseEvent(Prevent(func() {
		contribAmtS.Set("")
		contributing.Set(true)
	}))
	onContribAmt := ui.UseEvent(func(v string) { contribAmtS.Set(v) })
	doContribute := ui.UseEvent(Prevent(func() {
		if v := strings.TrimSpace(contribAmtS.Get()); v != "" {
			props.OnContribute(g, v)
		}
		contributing.Set(false)
	}))
	cancelContribute := ui.UseEvent(Prevent(func() { contributing.Set(false) }))
	nameS := ui.UseState(g.Name)
	targetS := ui.UseState(targetMajor)
	dateS := ui.UseState(dateISO)
	acctS := ui.UseState(g.AccountID)
	ownerS := ui.UseState(g.OwnerID)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onTarget := ui.UseEvent(func(v string) { targetS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onAcct := ui.UseEvent(func(e ui.Event) { acctS.Set(e.GetValue()) })
	onOwner := ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(g.Name)
		targetS.Set(targetMajor)
		dateS.Set(dateISO)
		acctS.Set(g.AccountID)
		ownerS.Set(g.OwnerID)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(g.ID, nameS.Get(), targetS.Get(), dateS.Get(), acctS.Get(), ownerS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when an inline editor opens (§6.7).
	ui.UseEffect(func() func() {
		switch {
		case contributing.Get():
			focusByID("goal-contrib-" + g.ID)
		case editing.Get():
			focusByID("goal-edit-" + g.ID)
		}
		return nil
	}, fmt.Sprintf("%t-%t", editing.Get(), contributing.Get()))

	if contributing.Get() {
		return Div(Class("budget"),
			Div(Class("budget-head"), Span(Class("row-desc"), g.Name)),
			Form(Class("form-grid"), OnSubmit(doContribute),
				Input(Class("field"), Attr("id", "goal-contrib-"+g.ID), Type("number"), Placeholder(uistate.T("goals.contributeAmount")), Value(contribAmtS.Get()), Step("0.01"), OnInput(onContribAmt)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("goals.contribute")),
				Button(Class("btn"), Type("button"), OnClick(cancelContribute), uistate.T("action.cancel")),
			),
		)
	}
	if editing.Get() {
		return Div(Class("budget"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Attr("id", "goal-edit-"+g.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName)),
				Input(Class("field"), Type("number"), Placeholder(uistate.T("goals.targetLabel")), Value(targetS.Get()), Step("0.01"), OnInput(onTarget)),
				Input(Class("field"), Type("date"), Value(dateS.Get()), OnInput(onDate)),
				Select(Class("field"), Title(uistate.T("goals.owner")), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get())),
				Select(Class("field"), Title(uistate.T("goals.linked")), OnChange(onAcct), goalAccountOptions(props.Accounts, acctS.Get())),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	pct := goalsvc.Percent(g)
	rem, _ := goalsvc.Remaining(g)
	complete, _ := goalsvc.IsComplete(g)

	sub := uistate.T("goals.progressFmt", pct, fmtMoney(rem))
	if complete {
		sub = uistate.T("goals.complete")
	}
	if !g.TargetDate.IsZero() {
		sub += uistate.T("goals.bySuffix", pr.FormatDate(g.TargetDate))
		if !complete {
			if per, ok, _ := goalsvc.MonthlyNeeded(g, time.Now()); ok {
				sub += uistate.T("goals.saveSuffix", fmtMoney(per))
			}
		}
	}
	if n := accountName(props.Accounts, g.AccountID); n != "" {
		sub += uistate.T("goals.linkedSuffix", n)
	}

	return Div(Class("budget"),
		Div(Class("budget-head"),
			Span(Class("row-desc"), g.Name),
			Span(Class("budget-amount"), fmtMoney(g.CurrentAmount)+" / "+fmtMoney(g.TargetAmount)),
			Button(Class("btn"), Type("button"), Title(uistate.T("goals.contributeTitle")), OnClick(contribute), uistate.T("goals.contribute")),
			Button(Class("btn"), Type("button"), Title(uistate.T("goals.editTitle")), OnClick(startEdit), uistate.T("action.edit")),
			Button(Class("btn-del"), Type("button"), Title(uistate.T("goals.deleteTitle")), OnClick(del), "✕"),
		),
		Div(Class("bar"), Div(Class("bar-fill"), Attr("style", fmt.Sprintf("width:%d%%", pct)))),
		Span(Class("budget-sub"), sub),
	)
}
