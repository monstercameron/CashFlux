// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
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
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
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

	// Open the add-goal modal from the card header (G5: discoverable add without
	// hunting for the FAB quick-add panel).
	addGoal := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("goal") }))

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	accounts := app.Accounts()
	errMsg := ui.UseState("")

	deleteGoal := func(goalID string) {
		// Guard the destructive delete with a confirm (matches Transactions/Budgets). Previously the
		// "×" destroyed a goal + its contribution history instantly with no confirm or undo.
		name := uistate.T("goals.thisGoal")
		for _, g := range app.Goals() {
			if g.ID == goalID && g.Name != "" {
				name = g.Name
				break
			}
		}
		uistate.ConfirmModal(uistate.T("goals.deleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			focusIdx := consumeRowDeleteFocus()
			if err := app.DeleteGoal(goalID); err != nil {
				errMsg.Set(err.Error())
				return
			}
			bump()
			focusRowAfterDelete(".goal-list", "[data-testid^='goal-row-']", focusIdx)
		})
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

	contribute := func(g domain.Goal, amtStr string, postLedger bool) {
		cur := g.CurrentAmount.Currency
		if cur == "" {
			cur = base
		}
		amt, err := money.ParseMinor(strings.TrimSpace(amtStr), currency.Decimals(cur))
		if err != nil || amt <= 0 { // reject $0 and negative contributions (L41)
			return
		}
		beforePct := goalsvc.Percent(g)
		updatedG := g
		updatedG.CurrentAmount = money.New(g.CurrentAmount.Amount+amt, cur)
		afterPct := goalsvc.Percent(updatedG)
		res, err := app.ContributeToGoal(g, money.New(amt, cur), postLedger)
		if err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
		notice := uistate.T("goals.contributedToast", fmtMoney(money.New(amt, cur)))
		if postLedger && res.TransactionID != "" {
			notice += " " + uistate.T("goals.contributedLedger")
		}
		uistate.PostNotice(notice, false) // L41
		// Milestone toast: celebrate 25/50/75/100% crossings (L38).
		if m := goalsvc.MilestoneCrossed(beforePct, afterPct); m > 0 {
			key := fmt.Sprintf("goals.milestone%d", m)
			uistate.PostNotice(uistate.T(key), false)
		}
		// Completion prompt: when the goal just became complete, fire a second
		// notice prompting the user to archive it (L59 completion lifecycle).
		if res.BecameComplete {
			uistate.PostNotice(uistate.T("goals.completionPrompt"), false)
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

	// Active list: most actionable first — nearest target date, then highest
	// percent complete, then name (G5). Surfaces the near-complete / time-pressed
	// goal so Aaliyah's "what should I fund next?" is answered at the top.
	sort.SliceStable(activeGoals, func(i, j int) bool {
		return goalsvc.LessForList(activeGoals[i], activeGoals[j])
	})
	// Achieved list: alphabetical.
	sort.SliceStable(achievedGoals, func(i, j int) bool {
		return achievedGoals[i].Name < achievedGoals[j].Name
	})

	// Combined progress across active goals only (archived goals excluded so they
	// don't dilute the headline figure). Each goal is converted to the base currency
	// via the FX table; a missing rate falls back to raw minor units.
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	savedTotalM, targetTotalM := goalsvc.Totals(activeGoals, rates, base, false)
	overallPct, _ := goalsvc.OverallProgress(activeGoals, false)

	members := app.Members()

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
		achievedSection = uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("aria-label", uistate.T("goals.achieved"))},
			Header: H2(css.Class("card-title"),
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
			Body: If(achievedOpen.Get(),
				Div(Attr("id", "goals-achieved-list"), achievedRows),
			),
		})
	}

	goalSmartSettings := uistate.LoadSmartSettings()
	goalSmartPr := uistate.UsePrefs().Get()
	goalSmartIn := buildSmartInput(app, goalSmartPr.WeekStartWeekday())

	var listBody ui.Node
	if len(activeGoals) == 0 {
		listBody = Fragment(
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("goals.empty"), CTALabel: uistate.T("goals.addFirst"), AddTarget: "goal", Icon: icon.Goals}),
			smartEmptyStateFor(goalSmartSettings, smart.PageGoals, goalSmartIn),
		)
	} else {
		rows := MapKeyed(activeGoals,
			func(g domain.Goal) any { return g.ID },
			func(g domain.Goal) ui.Node {
				return ui.CreateElement(GoalRow, goalRowProps{Goal: g, Accounts: accounts, Members: members, OnDelete: deleteGoal, OnContribute: contribute, OnSave: saveGoal, OnDrillAccount: viewAccountTxns, OnArchive: archiveGoal, OnRedirect: redirectToAllocate})
			},
		)
		listBody = Div(css.Class("goal-list"), rows)
	}

	return Div(
		If(len(allGoals) > 0, Div(css.Class("stat-grid"),
			stat(uistate.T("goals.savedSoFar"), fmtMoney(savedTotalM), "pos"),
			stat(uistate.T("goals.totalTarget"), fmtMoney(targetTotalM), ""),
			// Overall progress is the key goals figure — annotated with a smart explainer
			// tooltip so users understand what the combined percentage represents.
			Div(css.Class("stat"),
				Div(css.Class("stat-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("goals.overallProgress"),
					smartTooltipFor(goalSmartSettings, "goal-progress", uistate.T("goals.overallProgress"), uistate.T("smart.tipGoalProgress")),
				),
				Div(css.Class("stat-value"), fmt.Sprintf("%d%%", overallPct)),
			),
		)),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("nav.goals"),
			HeaderAction: Fragment(
				smartSectionAction(goalSmartSettings),
				Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "goals-add"), Title(uistate.T("goals.add")),
					OnClick(addGoal),
					uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("goals.addGoal"))),
			),
			Body: listBody,
		}),
		achievedSection,
	)
}

type goalRowProps struct {
	Goal           domain.Goal
	Accounts       []domain.Account
	Members        []domain.Member
	OnDelete       func(string)
	OnContribute   func(domain.Goal, string, bool) // goal, amountStr, postLedger
	OnSave         func(id, name, target, date, accountID, owner string)
	OnDrillAccount func(accountID string)        // open Transactions filtered to the linked account
	OnArchive      func(id string, archive bool) // move goal to/from the Achieved section
	OnRedirect     func()                        // a completed goal frees its monthly — jump to Allocate (L20)
}

// goalAccountOptions builds the linked-account SelectOptions for a goal, with a
// leading "no link" choice.
func goalAccountOptions(accounts []domain.Account, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("goals.noLink")}}
	for _, a := range accounts {
		opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
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

// barFillStyle is the inline width for a goal's progress bar. The fill *tone* is
// driven by a CSS state class (see paceBarClass) so a near-complete, behind, or
// on-track goal reads differently at a glance instead of one flat accent (G5/C51).
func barFillStyle(pct int) string {
	return fmt.Sprintf("width:%d%%", pct)
}

// paceBarClass maps a goal's pace to a progress-bar fill modifier class. The
// classes (final/overdue/soon) are defined in the shared stylesheet; an empty
// modifier keeps the default accent for on-track / undated goals.
func paceBarClass(p goalsvc.Pace) string {
	switch p {
	case goalsvc.PaceComplete:
		return "done"
	case goalsvc.PaceFinalStretch:
		return "final"
	case goalsvc.PaceOverdue:
		return "overdue"
	case goalsvc.PaceDueSoon:
		return "soon"
	default:
		return ""
	}
}

// paceBadge renders a compact colored badge for a goal's pace, or an empty
// fragment when there's nothing to flag (undated, comfortably on track without a
// near-term signal). It answers Aaliyah's "am I on pace?" at a glance (G5).
func paceBadge(p goalsvc.Pace) ui.Node {
	var label, mod string
	switch p {
	case goalsvc.PaceFinalStretch:
		label, mod = uistate.T("goals.paceFinal"), "final"
	case goalsvc.PaceOverdue:
		label, mod = uistate.T("goals.paceOverdue"), "overdue"
	case goalsvc.PaceDueSoon:
		label, mod = uistate.T("goals.paceDueSoon"), "soon"
	case goalsvc.PaceOnTrack:
		label, mod = uistate.T("goals.paceOnTrack"), "ontrack"
	default:
		return Fragment()
	}
	return Span(ClassStr("pace-badge pace-"+mod), label)
}
