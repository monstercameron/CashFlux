// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

type goalSummaryProps struct{ App *appstate.App }
type goalToolbarProps struct{ App *appstate.App }
type goalListProps struct{ App *appstate.App }

// --- goal-summary ----------------------------------------------------------------

// goalSummaryWidget is the headline tile: a "loader" progress bar (saved-of-target)
// with the Saved / Target / Left figures rendered inside it, mirroring the budgets
// summary. Renders nothing when there are no goals (the list tile owns the empty CTA).
func goalSummaryWidget(props goalSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	activeMemberID := uistate.UseActiveMember().Get()
	v := computeGoalView(app, activeMemberID)
	if len(v.All) == 0 {
		return Fragment()
	}
	smartSettings := uistate.LoadSmartSettings()

	saved := v.SavedTotal
	target := v.TargetTotal
	left := money.New(target.Amount-saved.Amount, v.Base)
	if left.Amount < 0 {
		left = money.New(0, v.Base)
	}
	fillW := v.OverallPct
	if fillW > 100 {
		fillW = 100
	}
	if fillW < 0 {
		fillW = 0
	}
	body := Div(ClassStr("budget-loader"),
		Attr("role", "progressbar"), Attr("aria-valuenow", fmt.Sprintf("%d", fillW)),
		Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("goals.overallProgress")),
		Div(css.Class("budget-loader-fill"), Attr("style", fmt.Sprintf("width:%d%%", fillW))),
		Div(css.Class("budget-loader-figs"),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("goals.savedSoFar")),
				Div(css.Class("budget-loader-value pos"), fmtMoney(saved)),
			),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("goals.totalTarget")),
				Div(css.Class("budget-loader-value"), fmtMoney(target)),
			),
			Div(css.Class("budget-loader-fig", "is-right"),
				Div(css.Class("budget-loader-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("goals.overallProgress"),
					smartTooltipFor(smartSettings, "goal-progress", uistate.T("goals.overallProgress"), uistate.T("smart.tipGoalProgress")),
				),
				Div(css.Class("budget-loader-value is-hero"), fmt.Sprintf("%d%%", v.OverallPct)),
			),
		),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goal-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- goal-toolbar ----------------------------------------------------------------

// goalToolbarWidget is the actions row: a "Sort by" picker (how to order the active
// goals) on the left and the primary "Add goal" button on the right.
func goalToolbarWidget(props goalToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	sortAtom := uistate.UseGoalSort()
	viewAtom := uistate.UseGoalsView()
	addGoal := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("goal") }))
	onSort := ui.UseEvent(func(e ui.Event) { sortAtom.Set(e.GetValue()) })
	showGoals := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.GoalsViewGoals) }))
	showEarmarks := ui.UseEvent(Prevent(func() { viewAtom.Set(uistate.GoalsViewEarmarks) }))
	sortVal := sortAtom.Get()
	view := viewAtom.Get()
	goalsActive, earmarksActive := "goals-tab", "goals-tab"
	if view == uistate.GoalsViewGoals {
		goalsActive += " is-active"
	} else {
		earmarksActive += " is-active"
	}
	// A tab strip (Goals · Earmarks) + a "Sort by" picker (goals view only) on the left, the
	// primary "Add goal" pushed to the right.
	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Div(css.Class("goals-tabs"), Attr("role", "tablist"),
				Button(ClassStr(goalsActive), Type("button"), Attr("role", "tab"), Attr("data-testid", "goals-tab-goals"),
					Attr("aria-selected", ariaBool(view == uistate.GoalsViewGoals)), OnClick(showGoals), uistate.T("goals.viewGoals")),
				Button(ClassStr(earmarksActive), Type("button"), Attr("role", "tab"), Attr("data-testid", "goals-tab-earmarks"),
					Attr("aria-selected", ariaBool(view == uistate.GoalsViewEarmarks)), OnClick(showEarmarks), uistate.T("goals.viewEarmarks")),
			),
			If(view == uistate.GoalsViewGoals, Label(css.Class("fctrl"),
				uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(css.Class("fctrl-label"), uistate.T("goals.sortLabel")),
				Select(css.Class("fctrl-select"), Attr("data-testid", "goals-sort"),
					Attr("aria-label", uistate.T("goals.sortLabel")), Title(uistate.T("goals.sortLabel")), OnChange(onSort),
					Option(Value(uistate.GoalSortActionable), SelectedIf(sortVal == uistate.GoalSortActionable), uistate.T("goals.sortActionable")),
					Option(Value(uistate.GoalSortClosest), SelectedIf(sortVal == uistate.GoalSortClosest), uistate.T("goals.sortClosest")),
					Option(Value(uistate.GoalSortFarthest), SelectedIf(sortVal == uistate.GoalSortFarthest), uistate.T("goals.sortFarthest")),
					Option(Value(uistate.GoalSortComplexity), SelectedIf(sortVal == uistate.GoalSortComplexity), uistate.T("goals.sortComplexity")),
					Option(Value(uistate.GoalSortDeadline), SelectedIf(sortVal == uistate.GoalSortDeadline), uistate.T("goals.sortDeadline")),
					Option(Value(uistate.GoalSortName), SelectedIf(sortVal == uistate.GoalSortName), uistate.T("goals.sortName")),
				),
			)),
		),
		Div(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap2),
			// TX11: round-up config lives in the goals toolbar (flip modal, staged Save/Cancel).
			roundupConfigToolbarButton(),
			Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "goals-add"), Title(uistate.T("goals.add")), OnClick(addGoal),
				uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("goals.addGoal"))),
		),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goal-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- goal-list -------------------------------------------------------------------

// goalListWidget is the rows tile: the sinking-funds card, the active-goals list (or
// the first-run empty-state CTA), and the collapsible achieved card. It owns the goal
// mutation closures (delete/archive/save/contribute) and the drill/redirect nav.
func goalListWidget(props goalListProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	activeMemberID := uistate.UseActiveMember().Get()
	errMsg := ui.UseState("")
	achievedOpen := ui.UseState(true)
	toggleAchieved := ui.UseEvent(Prevent(func() { achievedOpen.Set(!achievedOpen.Get()) }))
	goalSort := uistate.UseGoalSort().Get()

	v := computeGoalView(app, activeMemberID)
	// Apply the toolbar's Sort picker to the active list. Sort a copy — computeGoalView is
	// memoized and its slices are shared with the summary tile, so we must not reorder the
	// cached backing array in place.
	activeSorted := append([]domain.Goal(nil), v.Active...)
	sortGoals(activeSorted, goalSort, v.Tasks, time.Now())

	viewAccountTxns := func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	redirectToAllocate := func() { nav.Navigate(uistate.RoutePath("/allocate")) }

	deleteGoal := func(goalID string) {
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
			uistate.BumpDataRevision()
			// Reversible in one click even after the confirm (global snapshot undo).
			uistate.PostUndoable(uistate.T("goals.deletedToast", name))
			focusRowAfterDelete(".goal-list", "[data-testid^='goal-row-']", focusIdx)
		})
	}
	archiveGoal := func(goalID string, archive bool) {
		if err := app.ArchiveGoal(goalID, archive); err != nil {
			errMsg.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
	}
	undoContribution := func(goalID string) {
		for _, g := range app.Goals() {
			if g.ID != goalID {
				continue
			}
			undone, ok, err := app.UndoLastContribution(g)
			if err != nil {
				errMsg.Set(err.Error())
				return
			}
			if !ok {
				return // nothing to undo
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("goals.undoneToast", fmtMoney(undone)), false)
			return
		}
	}
	resetGoal := func(goalID string) {
		name := uistate.T("goals.thisGoal")
		for _, g := range app.Goals() {
			if g.ID == goalID && g.Name != "" {
				name = g.Name
				break
			}
		}
		uistate.ConfirmModal(uistate.T("goals.resetConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			for _, g := range app.Goals() {
				if g.ID != goalID {
					continue
				}
				if err := app.ResetGoalToZero(g); err != nil {
					errMsg.Set(err.Error())
					return
				}
				uistate.BumpDataRevision()
				// Resetting wipes the contribution history — make it one-click reversible.
				uistate.PostUndoable(uistate.T("goals.resetToast", name))
				return
			}
		})
	}
	overbooked := overbookedGoals(app)
	rowFor := func(g domain.Goal, fundSetAside int64, catName string) ui.Node {
		return ui.CreateElement(GoalRow, goalRowProps{
			Goal: g, Accounts: v.Accounts, Members: v.Members, Tasks: v.Tasks,
			OnDelete:       deleteGoal,
			OnDrillAccount: viewAccountTxns, OnArchive: archiveGoal, OnRedirect: redirectToAllocate,
			OnUndoContribution: undoContribution, OnResetGoal: resetGoal,
			FundSetAside: fundSetAside, LinkedCategoryName: catName,
			EarmarkOverbooked: overbooked[g.ID],
		})
	}

	now := time.Now()
	smartSettings := uistate.LoadSmartSettings()

	// Sinking funds card (above the regular goals when any exist).
	var fundsSection ui.Node = Fragment()
	if len(v.Fund) > 0 {
		fundRows := MapKeyed(v.Fund, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return rowFor(g, goalsvc.FundSetAsideMinor(g, now), categoryNameByID(v.Categories, g.CategoryID))
		})
		fundsSection = uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("aria-label", uistate.T("goals.fundsSection"))},
			Header: H2(css.Class("card-title"),
				uistate.T("goals.fundsSection"),
				Span(css.Class("budget-sub"), fmt.Sprintf(" · %d", len(v.Fund))),
			),
			Body: Div(css.Class("goal-list"), fundRows),
		})
	}

	// Active goals list, or the first-run empty state.
	var listBody ui.Node
	if len(v.Active) == 0 && len(v.Fund) == 0 && len(v.Missed) == 0 {
		pr := uistate.UsePrefs().Get()
		in := buildSmartInput(app, pr.WeekStartWeekday())
		listBody = Fragment(
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("goals.empty"), CTALabel: uistate.T("goals.addFirst"), AddTarget: "goal", Icon: icon.Goals}),
			smartEmptyStateFor(smartSettings, smart.PageGoals, in),
		)
	} else {
		// Each active goal card now renders its savings-pace rail INSIDE its own metadata
		// block (see goals_row.go), so the list is just the cards — no trajectory wrapper.
		rows := MapKeyed(activeSorted, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return rowFor(g, 0, "")
		})
		listBody = Div(css.Class("goal-list"), rows)
	}

	// Missed-deadline card (G4): the goals the dashboard widget counts as "Missed" get
	// their own named section here — longest-missed first, warn-tinted header — so the
	// widget's count always has a place the page can show.
	var missedSection ui.Node = Fragment()
	if len(v.Missed) > 0 {
		missedRows := MapKeyed(v.Missed, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return rowFor(g, 0, "")
		})
		missedSection = uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("aria-label", uistate.T("goals.missedSection")), Attr("data-testid", "goals-missed-section")},
			Header: H2(css.Class("card-title", "goals-missed-title"),
				uistate.T("goals.missedSection"),
				Span(css.Class("budget-sub"), fmt.Sprintf(" · %d", len(v.Missed))),
			),
			Body: Fragment(
				P(css.Class("budget-sub"), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("goals.missedHint")),
				Div(css.Class("goal-list"), missedRows),
			),
		})
	}

	// Collapsible achieved card.
	var achievedSection ui.Node = Fragment()
	if len(v.Achieved) > 0 {
		achievedRows := MapKeyed(v.Achieved, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return rowFor(g, 0, "")
		})
		achievedSection = uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("aria-label", uistate.T("goals.achieved"))},
			Header: H2(css.Class("card-title"),
				Button(css.Class("btn"), Type("button"),
					Attr("aria-expanded", fmt.Sprintf("%t", achievedOpen.Get())),
					Attr("aria-controls", "goals-achieved-list"), OnClick(toggleAchieved),
					uistate.T("goals.achieved"),
					Span(css.Class("budget-sub"), uistate.T("goals.achievedCount", len(v.Achieved))),
				),
			),
			Body: If(achievedOpen.Get(), Div(Attr("id", "goals-achieved-list"), achievedRows)),
		})
	}

	var body ui.Node = Div(
		goalsWaterfallCard(),
		// Missed deadlines lead — they're the section that needs a decision (re-date,
		// re-fund, or archive), so they never hide below the healthy list.
		missedSection,
		fundsSection,
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("nav.goals"),
			Body:  listBody,
		}),
		If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
		achievedSection,
	)
	// The toolbar's tab strip can swap the whole list area over to the earmarks manager.
	if uistate.UseGoalsView().Get() == uistate.GoalsViewEarmarks {
		body = ui.CreateElement(goalEarmarksManager, goalEarmarksProps{App: app})
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "goal-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
