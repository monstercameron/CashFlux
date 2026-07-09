// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// GoalEditFormProps drives the goal editor rendered inside the shell-root flip modal
// (see internal/app GoalEditHost). Mode selects edit vs contribute.
type GoalEditFormProps struct {
	GoalID string
	Mode   string // one of uistate.GoalEditMode*
	OnDone func() // clears the atom (closes the modal)
}

// GoalEditForm renders the goal editor (full edit, or contribute) as the body of the
// shell-root flip modal. It owns all its state and its own Save/Cancel and performs the
// mutation directly against the store, mirroring BudgetEditForm. Living at the shell
// root (outside transformed bento/tile ancestors) keeps the modal centred.
func GoalEditForm(props GoalEditFormProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	pr := uistate.UsePrefs().Get()
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	var g domain.Goal
	found := false
	if app != nil {
		for _, gg := range app.Goals() {
			if gg.ID == props.GoalID {
				g, found = gg, true
				break
			}
		}
	}
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	cur := g.TargetAmount.Currency
	if cur == "" {
		cur = base
	}
	dec := currency.Decimals(cur)
	targetMajor := ""
	dateISO := ""
	monthlyMajor := ""
	if found {
		targetMajor = money.FormatMinor(g.TargetAmount.Amount, dec)
		if !g.TargetDate.IsZero() {
			dateISO = dateutil.FormatDate(g.TargetDate)
		}
		if g.MonthlyContribution.Amount > 0 {
			monthlyMajor = money.FormatMinor(g.MonthlyContribution.Amount, dec)
		}
	}

	// Kind + habit-field initial values (empty when not a habit).
	kindInit := string(g.EffectiveKind())
	cadenceInit := string(g.HabitCadence)
	if cadenceInit == "" {
		cadenceInit = string(domain.CadenceWeekly)
	}
	habitTargetInit := ""
	if g.HabitTarget > 0 {
		habitTargetInit = strconv.Itoa(g.HabitTarget)
	}

	// All hooks unconditionally at stable positions.
	nameS := ui.UseState(g.Name)
	targetS := ui.UseState(targetMajor)
	monthlyS := ui.UseState(monthlyMajor)
	dateS := ui.UseState(dateISO)
	acctS := ui.UseState(g.AccountID)
	ownerS := ui.UseState(g.OwnerID)
	contribS := ui.UseState("")
	postLedgerS := ui.UseState(false)
	errS := ui.UseState("")
	kindS := ui.UseState(kindInit)
	cadenceS := ui.UseState(cadenceInit)
	habitTargetS := ui.UseState(habitTargetInit)

	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onTarget := ui.UseEvent(func(v string) { targetS.Set(v) })
	onMonthly := ui.UseEvent(func(v string) { monthlyS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onHabitTarget := ui.UseEvent(func(v string) { habitTargetS.Set(v) })
	onContrib := ui.UseEvent(func(v string) { contribS.Set(v) })
	onPostLedger := ui.UseEvent(func(e ui.Event) { postLedgerS.Set(e.IsChecked()) })
	cancel := ui.UseEvent(Prevent(func() { done() }))

	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		for _, gg := range app.Goals() {
			if gg.ID != props.GoalID {
				continue
			}
			if n := strings.TrimSpace(nameS.Get()); n != "" {
				gg.Name = n
			}
			gg.OwnerID = ownerS.Get()
			if ownerS.Get() == domain.GroupOwnerID {
				gg.Scope = domain.ScopeShared
			} else {
				gg.Scope = domain.ScopeIndividual
			}
			kind := domain.GoalKind(kindS.Get())
			gg.Kind = kind
			c := gg.TargetAmount.Currency
			if c == "" {
				c = base
			}
			switch kind {
			case domain.GoalKindFinancial:
				amt, err := money.ParseMinor(strings.TrimSpace(targetS.Get()), currency.Decimals(c))
				if err != nil || amt <= 0 {
					errS.Set(uistate.T("goals.targetRequired"))
					return
				}
				gg.TargetAmount = money.New(amt, c)
				gg.AccountID = acctS.Get()
				// Optional explicit monthly assignment for zero-based budgeting.
				if mc := strings.TrimSpace(monthlyS.Get()); mc != "" {
					if m, mErr := money.ParseMinor(mc, currency.Decimals(c)); mErr == nil && m >= 0 {
						gg.MonthlyContribution = money.New(m, c)
					}
				} else {
					gg.MonthlyContribution = money.New(0, c)
				}
			case domain.GoalKindHabit:
				n, err := strconv.Atoi(strings.TrimSpace(habitTargetS.Get()))
				if err != nil || n <= 0 {
					errS.Set(uistate.T("goals.habitTargetRequired"))
					return
				}
				gg.HabitCadence = domain.RecurringCadence(cadenceS.Get())
				gg.HabitTarget = n
				// Non-financial: keep a valid zeroed amount + drop the account link.
				gg.TargetAmount = money.New(0, c)
				gg.AccountID = ""
			default: // checklist / milestone
				gg.TargetAmount = money.New(0, c)
				gg.AccountID = ""
			}
			if ds := strings.TrimSpace(dateS.Get()); ds != "" {
				d, derr := dateutil.ParseDate(ds)
				if derr != nil {
					errS.Set(uistate.T("goals.invalidDate"))
					return
				}
				gg.TargetDate = d
			} else {
				gg.TargetDate = time.Time{}
			}
			if err := app.PutGoal(gg); err != nil {
				errS.Set(err.Error())
				return
			}
			break
		}
		uistate.BumpDataRevision()
		done()
	}))

	submitContribute := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		c := g.CurrentAmount.Currency
		if c == "" {
			c = base
		}
		amt, err := money.ParseMinor(strings.TrimSpace(contribS.Get()), currency.Decimals(c))
		if err != nil || amt <= 0 {
			errS.Set(uistate.T("goals.targetRequired"))
			return
		}
		beforePct := goalsvc.Percent(g)
		after := g
		after.CurrentAmount = money.New(g.CurrentAmount.Amount+amt, c)
		afterPct := goalsvc.Percent(after)
		res, cerr := app.ContributeToGoal(g, money.New(amt, c), postLedgerS.Get())
		if cerr != nil {
			errS.Set(cerr.Error())
			return
		}
		uistate.BumpDataRevision()
		notice := uistate.T("goals.contributedToast", fmtMoney(money.New(amt, c)))
		if postLedgerS.Get() && res.TransactionID != "" {
			notice += " " + uistate.T("goals.contributedLedger")
		}
		uistate.PostNotice(notice, false)
		if m := goalsvc.MilestoneCrossed(beforePct, afterPct); m > 0 {
			uistate.PostNotice(uistate.T(fmt.Sprintf("goals.milestone%d", m)), false)
		}
		if res.BecameComplete {
			uistate.PostNotice(uistate.T("goals.completionPrompt"), false)
		}
		done()
	}))

	if app == nil || !found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	var errLine ui.Node = Fragment()
	if errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), errS.Get())
	}

	// --- Contribute: a single amount toward the goal (optionally posted to the ledger). ---
	if props.Mode == uistate.GoalEditModeContribute {
		linkedName := accountName(app.Accounts(), g.AccountID)
		var ledgerRow ui.Node = Fragment()
		if linkedName != "" {
			cbArgs := []any{Type("checkbox"), Attr("id", "goal-contrib-ledger-"+g.ID), OnChange(onPostLedger)}
			if postLedgerS.Get() {
				cbArgs = append(cbArgs, Attr("checked", ""))
			}
			ledgerRow = Label(css.Class("field", "ba-check"),
				Input(cbArgs...),
				Span(uistate.T("goals.contributePostLedger", linkedName)),
			)
		}
		return Form(css.Class("acct-edit-form"), OnSubmit(submitContribute),
			P(css.Class("t-caption", "muted"), Style(map[string]string{"margin": "0"}),
				uistate.T("goals.contributeHint", g.Name, fmtMoney(g.CurrentAmount), fmtMoney(g.TargetAmount))),
			labeledField(uistate.T("goals.contributeAmount"),
				Input(css.Class("field"), Attr("id", "goal-contrib-"+g.ID), Attr("autofocus", ""), Type("number"),
					Placeholder(uistate.T("goals.contributeAmount")), Value(contribS.Get()), Step("0.01"), OnInput(onContrib))),
			ledgerRow,
			errLine,
			Div(css.Class("acct-edit-actions"),
				Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("goals.contribute")),
			),
		)
	}

	// --- Full edit. ---
	_ = pr
	kind := domain.GoalKind(kindS.Get())
	financial := kind.IsFinancial()
	return Form(css.Class("acct-edit-form"), OnSubmit(saveEdit),
		labeledField(uistate.T("common.name"),
			Input(css.Class("field"), Attr("id", "goal-edit-"+g.ID), Attr("autofocus", ""), Type("text"),
				Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
		labeledField(uistate.T("goals.kindLabel"),
			Div(
				uiw.SelectInput(uiw.SelectInputProps{
					Options: goalKindOptions(), Selected: kindS.Get(), TestID: "goal-edit-kind",
					OnChange: func(v string) { kindS.Set(v) }, AriaLabel: uistate.T("goals.kindLabel"),
				}),
				Span(css.Class("budget-sub"), goalKindHint(kind)),
			)),
		If(financial, labeledField(uistate.T("goals.targetLabel"),
			Input(css.Class("field"), Type("number"), Placeholder(uistate.T("goals.targetLabel")), Value(targetS.Get()), Step("0.01"), OnInput(onTarget)))),
		If(financial, labeledField(uistate.T("goals.monthlyContribLabel"),
			Input(css.Class("field"), Type("number"), Attr("data-testid", "goal-edit-monthly"), Attr("min", "0"),
				Placeholder(uistate.T("goals.monthlyContribPlaceholder")), Value(monthlyS.Get()), Step("0.01"), OnInput(onMonthly)))),
		If(kind == domain.GoalKindHabit, labeledField(uistate.T("goals.habitCadenceLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: habitCadenceOptions(), Selected: cadenceS.Get(), TestID: "goal-edit-cadence",
				OnChange: func(v string) { cadenceS.Set(v) }, AriaLabel: uistate.T("goals.habitCadenceLabel"),
			}))),
		If(kind == domain.GoalKindHabit, labeledField(uistate.T("goals.habitTargetLabel"),
			Input(css.Class("field"), Type("number"), Attr("data-testid", "goal-edit-habit-target"), Placeholder(uistate.T("goals.habitTargetPlaceholder")), Value(habitTargetS.Get()), Step("1"), OnInput(onHabitTarget)))),
		labeledField(uistate.T("goals.dateLabel"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("goals.dateLabel")), Value(dateS.Get()), OnInput(onDate))),
		labeledField(uistate.T("goals.owner"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: ownerSelectOptions(app.Members(), ownerS.Get()), Selected: ownerS.Get(),
				OnChange: func(v string) { ownerS.Set(v) }, AriaLabel: uistate.T("goals.owner"),
			})),
		If(financial, labeledField(uistate.T("goals.linked"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options: goalAccountOptions(app.Accounts(), acctS.Get()), Selected: acctS.Get(),
				OnChange: func(v string) { acctS.Set(v) }, AriaLabel: uistate.T("goals.linked"),
			}))),
		errLine,
		Div(css.Class("acct-edit-actions"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
	)
}
