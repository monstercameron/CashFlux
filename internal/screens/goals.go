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
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Goals lists savings goals with progress, plus an add form and per-row delete.
func Goals() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:goals", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	// Drill from a goal's linked account to that account's transactions (mirrors
	// Accounts→Transactions and the budget drill, C30/C50).
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewAccountTxns := func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	// A completed goal frees its monthly contribution — jump to Allocate to put it
	// to work elsewhere (L20 "what next").
	redirectToAllocate := func() { nav.Navigate(uistate.RoutePath("/allocate")) }

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	accounts := app.Accounts()
	errMsg := ui.UseState("")

	deleteGoal := func(goalID string) {
		if err := app.DeleteGoal(goalID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	archiveGoal := func(goalID string, archive bool) {
		if err := app.ArchiveGoal(goalID, archive); err != nil {
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
		if err != nil || amt <= 0 { // reject $0 and negative contributions (L41)
			return
		}
		beforePct := goalsvc.Percent(g)
		g.CurrentAmount = money.New(g.CurrentAmount.Amount+amt, cur)
		afterPct := goalsvc.Percent(g)
		if err := app.PutGoal(g); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
		uistate.PostNotice(uistate.T("goals.contributedToast", fmtMoney(money.New(amt, cur))), false) // L41
		// Milestone toast: celebrate 25/50/75/100% crossings (L38).
		if m := goalsvc.MilestoneCrossed(beforePct, afterPct); m > 0 {
			key := fmt.Sprintf("goals.milestone%d", m)
			uistate.PostNotice(uistate.T(key), false)
		}
	}

	allGoals := app.Goals()

	// Partition into active (non-archived) and achieved (archived).
	var activeGoals, achievedGoals []domain.Goal
	for _, g := range allGoals {
		if g.Archived {
			achievedGoals = append(achievedGoals, g)
		} else {
			activeGoals = append(activeGoals, g)
		}
	}

	// Active list: incomplete goals first, then alphabetical within each group.
	sort.SliceStable(activeGoals, func(i, j int) bool {
		ci, _ := goalsvc.IsComplete(activeGoals[i])
		cj, _ := goalsvc.IsComplete(activeGoals[j])
		if ci != cj {
			return !ci
		}
		return activeGoals[i].Name < activeGoals[j].Name
	})
	// Achieved list: alphabetical.
	sort.SliceStable(achievedGoals, func(i, j int) bool {
		return achievedGoals[i].Name < achievedGoals[j].Name
	})

	// Combined progress across active goals only (archived goals excluded so they
	// don't dilute the headline figure). Each goal is converted to the base currency
	// via the FX table; a missing rate falls back to raw minor units.
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	var savedTotal, targetTotal int64
	for _, g := range activeGoals {
		if c, err := rates.Convert(g.CurrentAmount, base); err == nil {
			savedTotal += c.Amount
		} else {
			savedTotal += g.CurrentAmount.Amount
		}
		if c, err := rates.Convert(g.TargetAmount, base); err == nil {
			targetTotal += c.Amount
		} else {
			targetTotal += g.TargetAmount.Amount
		}
	}
	overallPct, _ := goalsvc.OverallProgress(activeGoals, false)

	members := app.Members()

	var listBody ui.Node
	if len(activeGoals) == 0 {
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("goals.empty"), CTALabel: uistate.T("goals.addFirst"), AddTarget: "goal", Icon: icon.Goals})
	} else {
		rows := MapKeyed(activeGoals,
			func(g domain.Goal) any { return g.ID },
			func(g domain.Goal) ui.Node {
				return ui.CreateElement(GoalRow, goalRowProps{Goal: g, Accounts: accounts, Members: members, OnDelete: deleteGoal, OnContribute: contribute, OnSave: saveGoal, OnDrillAccount: viewAccountTxns, OnArchive: archiveGoal, OnRedirect: redirectToAllocate})
			},
		)
		listBody = Div(rows)
	}

	achievedOpen := ui.UseState(true)
	toggleAchieved := ui.UseEvent(Prevent(func() { achievedOpen.Set(!achievedOpen.Get()) }))

	var achievedSection ui.Node = Fragment()
	if len(achievedGoals) > 0 {
		achievedRows := MapKeyed(achievedGoals,
			func(g domain.Goal) any { return g.ID },
			func(g domain.Goal) ui.Node {
				return ui.CreateElement(GoalRow, goalRowProps{Goal: g, Accounts: accounts, Members: members, OnDelete: deleteGoal, OnContribute: contribute, OnSave: saveGoal, OnDrillAccount: viewAccountTxns, OnArchive: archiveGoal, OnRedirect: redirectToAllocate})
			},
		)
		achievedSection = Section(css.Class("card"),
			Attr("aria-label", uistate.T("goals.achieved")),
			H2(css.Class("card-title"),
				Button(
					css.Class("btn"),
					Type("button"),
					Attr("aria-expanded", fmt.Sprintf("%t", achievedOpen.Get())),
					Attr("aria-controls", "goals-achieved-list"),
					OnClick(toggleAchieved),
					uistate.T("goals.achieved"),
					Span(css.Class("budget-sub"), uistate.T("goals.achievedCount", len(achievedGoals))),
				),
			),
			If(achievedOpen.Get(),
				Div(Attr("id", "goals-achieved-list"), achievedRows),
			),
		)
	}

	return Div(
		If(len(allGoals) > 0, Div(css.Class("stat-grid"),
			stat(uistate.T("goals.savedSoFar"), fmtMoney(money.New(savedTotal, base)), "pos"),
			stat(uistate.T("goals.totalTarget"), fmtMoney(money.New(targetTotal, base)), ""),
			stat(uistate.T("goals.overallProgress"), fmt.Sprintf("%d%%", overallPct), ""),
		)),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("nav.goals")),
			listBody,
		),
		achievedSection,
	)
}

type goalRowProps struct {
	Goal           domain.Goal
	Accounts       []domain.Account
	Members        []domain.Member
	OnDelete       func(string)
	OnContribute   func(domain.Goal, string)
	OnSave         func(id, name, target, date, accountID, owner string)
	OnDrillAccount func(accountID string)        // open Transactions filtered to the linked account
	OnArchive      func(id string, archive bool) // move goal to/from the Achieved section
	OnRedirect     func()                        // a completed goal frees its monthly — jump to Allocate (L20)
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

// barFillStyle is the inline style for a goal's progress bar: the width, plus a
// brighter success tone once the goal is complete so a reached goal reads as done
// at a glance instead of using the same flat accent fill (C51). Inline (not a CSS
// class) to stay out of the shared stylesheet.
func barFillStyle(pct int, complete bool) string {
	s := fmt.Sprintf("width:%d%%", pct)
	if complete {
		s += ";background:var(--up)"
	}
	return s
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
	doArchive := ui.UseEvent(Prevent(func() {
		if props.OnArchive != nil {
			props.OnArchive(g.ID, true)
		}
	}))
	doUnarchive := ui.UseEvent(Prevent(func() {
		if props.OnArchive != nil {
			props.OnArchive(g.ID, false)
		}
	}))
	drillAcct := ui.UseEvent(Prevent(func() {
		if props.OnDrillAccount != nil {
			props.OnDrillAccount(g.AccountID)
		}
	}))
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
		return Div(css.Class("budget"),
			Div(css.Class("budget-head"), Span(css.Class("row-desc"), g.Name)),
			Form(css.Class("form-grid"), OnSubmit(doContribute),
				labeledField(uistate.T("goals.contributeAmount"),
					Input(css.Class("field"), Attr("id", "goal-contrib-"+g.ID), Type("number"), Placeholder(uistate.T("goals.contributeAmount")), Value(contribAmtS.Get()), Step("0.01"), OnInput(onContribAmt))),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("goals.contribute")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelContribute), uistate.T("action.cancel")),
			),
		)
	}
	if editing.Get() {
		return Div(css.Class("budget"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("common.name"),
					Input(css.Class("field"), Attr("id", "goal-edit-"+g.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
				labeledField(uistate.T("goals.targetLabel"),
					Input(css.Class("field"), Type("number"), Placeholder(uistate.T("goals.targetLabel")), Value(targetS.Get()), Step("0.01"), OnInput(onTarget))),
				labeledField(uistate.T("goals.dateLabel"),
					Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("goals.dateLabel")), Value(dateS.Get()), OnInput(onDate))),
				labeledField(uistate.T("goals.owner"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("goals.owner")), Title(uistate.T("goals.owner")), OnChange(onOwner), ownerSelectOptions(props.Members, ownerS.Get()))),
				labeledField(uistate.T("goals.linked"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("goals.linked")), Title(uistate.T("goals.linked")), OnChange(onAcct), goalAccountOptions(props.Accounts, acctS.Get()))),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	pct := goalsvc.Percent(g)
	rem, _ := goalsvc.Remaining(g)
	complete, _ := goalsvc.IsComplete(g)
	overfund, _ := goalsvc.Overfund(g)

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

	redirect := ui.UseEvent(Prevent(func() {
		if props.OnRedirect != nil {
			props.OnRedirect()
		}
	}))

	// "What next" prompt: a completed (not-yet-archived) goal frees up whatever was
	// going into it — offer a calm, dismissible jump to Allocate to redirect it
	// toward another goal (L20). No nagging: it's a single low-key line.
	var whatNext ui.Node = Fragment()
	if complete && !g.Archived {
		whatNext = Div(css.Class("budget-sub"), Attr("data-testid", "goal-whatnext-"+g.ID),
			Span(uistate.T("goals.whatNext")+" "),
			Button(css.Class("budget-drill"), Type("button"), Attr("aria-label", uistate.T("goals.whatNextAction")),
				Attr("data-testid", "goal-redirect-"+g.ID), OnClick(redirect), uistate.T("goals.whatNextAction")),
		)
	}

	// Over-funding note: shown whenever the current amount exceeds the target.
	var overfundNote ui.Node = Fragment()
	if overfund.IsPositive() {
		overfundNote = Span(
			css.Class("budget-sub"),
			Attr("data-testid", "goal-overfund-"+g.ID),
			Style(map[string]string{"color": "var(--up)"}),
			uistate.T("goals.overTarget", fmtMoney(overfund)),
		)
	}

	// The linked account is split out of the run-on sub-line into its own clickable
	// element that drills to that account's transactions (C51).
	linkedName := accountName(props.Accounts, g.AccountID)
	var linkedLine ui.Node = Fragment()
	if linkedName != "" {
		linkedLine = Span(css.Class("budget-sub"),
			Button(css.Class("budget-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drillAcct),
				Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
				uistate.T("goals.linkedSuffix", linkedName)),
		)
	}

	// Archive button shown on complete active goals; Unarchive shown on archived goals.
	var archiveBtn ui.Node = Fragment()
	if g.Archived {
		archiveBtn = Button(
			css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
			Type("button"),
			Attr("aria-label", uistate.T("goals.unarchiveTitle")),
			Title(uistate.T("goals.unarchiveTitle")),
			Attr("data-testid", "goal-unarchive-"+g.ID),
			OnClick(doUnarchive),
			Span(uistate.T("goals.unarchive")),
		)
	} else if complete {
		archiveBtn = Button(
			css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
			Type("button"),
			Attr("aria-label", uistate.T("goals.archiveTitle")),
			Title(uistate.T("goals.archiveTitle")),
			Attr("data-testid", "goal-archive-"+g.ID),
			OnClick(doArchive),
			Span(uistate.T("goals.archive")),
		)
	}

	return Div(css.Class("budget"),
		Attr("data-testid", "goal-row-"+g.ID),
		Div(css.Class("budget-head"),
			Span(css.Class("row-desc"), g.Name),
			Span(css.Class("budget-amount"), fmtMoney(g.CurrentAmount)+" / "+fmtMoney(g.TargetAmount)),
			If(!g.Archived, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("goals.contributeTitle")), OnClick(contribute), uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.contribute")))),
			If(!g.Archived, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("goals.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit")))),
			archiveBtn,
			Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("goals.deleteTitle")), Title(uistate.T("goals.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		Div(css.Class("bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(pct)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("goals.progressLabel")), Div(css.Class("bar-fill"), Attr("style", barFillStyle(pct, complete)))),
		Span(css.Class("budget-sub"), sub),
		overfundNote,
		whatNext,
		linkedLine,
	)
}
