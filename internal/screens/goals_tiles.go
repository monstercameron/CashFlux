// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
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
	setAside := v.SetAsideMin
	// "Funded so far" is the money working toward goals — saved contributions PLUS
	// reserved set-asides — with the split shown below so it never contradicts the
	// per-card saved/set-aside legend (#5).
	funded := money.New(saved.Amount+setAside.Amount, v.Base)
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
	// Progress semantics live on the childless FILL bar, not the wrapper — the
	// wrapper contains focusable descendants (the smart tooltip), and a labeled
	// role wrapping interactive children fails axe nested-interactive (#67).
	body := Div(ClassStr("budget-loader"),
		Div(css.Class("budget-loader-fill"),
			Attr("role", "img"),
			Attr("aria-label", fmt.Sprintf("%s: %d%%", uistate.T("goals.overallProgress"), fillW)),
			Attr("style", fmt.Sprintf("width:%d%%", fillW))),
		Div(css.Class("budget-loader-figs"),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("goals.fundedSoFar")),
				Div(css.Class("budget-loader-value pos"), Attr("data-testid", "goal-summary-funded"), fmtMoney(funded)),
				// Split the headline into its parts so it reconciles with each card's
				// saved-vs-set-aside legend (#5).
				Div(css.Class("budget-loader-sub", tw.TextFaint, tw.Text12), Attr("data-testid", "goal-summary-split"),
					uistate.T("goals.fundedSplit", fmtMoney(saved), fmtMoney(setAside))),
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
		// The headline counts active + missed goals only; when sinking funds
		// exist, say so — their card targets otherwise look like a $-mismatch
		// against "Total target" (2026-07-18 assessment: users had to
		// reverse-engineer which cards contribute).
		fundScopeNote(app, v),
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
	compareAtom := uistate.UseGoalCompareOpen()
	openCompare := ui.UseEvent(Prevent(func() { compareAtom.Set(true) }))
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
					Option(Value(uistate.GoalSortPriority), SelectedIf(sortVal == uistate.GoalSortPriority), uistate.T("goals.sortPriority")),
				),
			)),
		),
		Div(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap2),
			// Goal-vs-goal comparison (flip modal): pick two goals, read the figures
			// side by side.
			Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "goals-compare-btn"),
				Title(uistate.T("goalcompare.title")), Attr("aria-haspopup", "dialog"), OnClick(openCompare),
				uistate.T("goalcompare.button")),
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
			Health:            v.Health[g.ID],
			Base:              v.Base,
		})
	}

	now := time.Now()
	smartSettings := uistate.LoadSmartSettings()

	// "Needs a plan" partitioning (2026-07-19 Watch-first ordering): the goals that
	// need a decision now — missed deadlines, plus the Watch / At-risk pace verdicts
	// pulled up out of the active and sinking-fund lists — lead the page so the
	// actionable items are in the first viewport instead of sitting below healthy
	// funds. This ONLY reorders: each card is the same GoalRow with the same pace
	// badge and reason; the verdicts come straight from v.Health (computeGoalHealth),
	// never recomputed. Pulled goals are filtered OUT of the healthy sections below so
	// nothing renders twice.
	type planEntry struct {
		g      domain.Goal
		missed bool
	}
	var planEntries []planEntry
	for _, g := range v.Missed {
		planEntries = append(planEntries, planEntry{g: g, missed: true})
	}
	for _, g := range v.Active {
		if healthNeedsPlan(v.Health[g.ID].Health) {
			planEntries = append(planEntries, planEntry{g: g})
		}
	}
	for _, g := range v.Fund {
		if healthNeedsPlan(v.Health[g.ID].Health) {
			planEntries = append(planEntries, planEntry{g: g})
		}
	}
	// Ordering: severity (missed → at risk → watch) is the presentation of the
	// DEFAULT "Most actionable" sort. When the user picks an explicit sort in
	// the toolbar it applies here too — the Watch-first pass silently ignored
	// e.g. "Sort by name" inside this section, which both broke the sort
	// promise and goals.spec's A→Z regression check.
	if goalSort != uistate.GoalSortActionable {
		planGoals := make([]domain.Goal, len(planEntries))
		missedByID := make(map[string]bool, len(planEntries))
		for i, e := range planEntries {
			planGoals[i] = e.g
			if e.missed {
				missedByID[e.g.ID] = true
			}
		}
		sortGoals(planGoals, goalSort, v.Tasks, now)
		planEntries = planEntries[:0]
		for _, g := range planGoals {
			planEntries = append(planEntries, planEntry{g: g, missed: missedByID[g.ID]})
		}
	} else {
		sort.SliceStable(planEntries, func(i, j int) bool {
			ri := needsPlanRank(planEntries[i].missed, v.Health[planEntries[i].g.ID].Health)
			rj := needsPlanRank(planEntries[j].missed, v.Health[planEntries[j].g.ID].Health)
			if ri != rj {
				return ri < rj
			}
			return goalsvc.LessForList(planEntries[i].g, planEntries[j].g)
		})
	}

	// Healthy remainders for the sections below: the goals NOT pulled up (on-track /
	// no-verdict). Missed goals were never in v.Active, so activeSorted only sheds its
	// Watch / At-risk cards here.
	var healthyActive []domain.Goal
	for _, g := range activeSorted {
		if !healthNeedsPlan(v.Health[g.ID].Health) {
			healthyActive = append(healthyActive, g)
		}
	}
	var healthyFund []domain.Goal
	for _, g := range v.Fund {
		if !healthNeedsPlan(v.Health[g.ID].Health) {
			healthyFund = append(healthyFund, g)
		}
	}

	rowForPlan := func(g domain.Goal) ui.Node {
		if g.IsSinkingFund {
			return rowFor(g, goalsvc.FundSetAsideMinor(g, now), categoryNameByID(v.Categories, g.CategoryID))
		}
		return rowFor(g, 0, "")
	}

	// The "Needs a plan" lead card.
	var needsPlanSection ui.Node = Fragment()
	if len(planEntries) > 0 {
		planRows := MapKeyed(planEntries, func(e planEntry) any { return e.g.ID }, func(e planEntry) ui.Node {
			return rowForPlan(e.g)
		})
		needsPlanSection = uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("aria-label", uistate.T("goals.needsPlanSection")), Attr("data-testid", "goals-needsplan-section")},
			Header: H2(css.Class("card-title", "goals-needsplan-title"),
				uistate.T("goals.needsPlanSection"),
				Span(css.Class("goal-count-inline"), Attr("data-testid", "goals-needsplan-count"), fmt.Sprintf(" · %d", len(planEntries))),
			),
			Body: Fragment(
				P(css.Class("budget-sub"), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("goals.needsPlanHint")),
				Div(css.Class("goal-list"), planRows),
			),
		})
	}

	// Sinking funds card (healthy funds only — the rest lead in "Needs a plan").
	var fundsSection ui.Node = Fragment()
	if len(healthyFund) > 0 {
		fundRows := MapKeyed(healthyFund, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return rowFor(g, goalsvc.FundSetAsideMinor(g, now), categoryNameByID(v.Categories, g.CategoryID))
		})
		fundsSection = uiw.Card(uiw.CardProps{
			Attrs: []any{Attr("aria-label", uistate.T("goals.fundsSection"))},
			// The count span is .goal-count-inline, NOT .budget-sub — budget-sub is
			// display:block, which wrapped the " · 2" onto its own line under the
			// heading (UX-06 formatting bug).
			Header: H2(css.Class("card-title"),
				uistate.T("goals.fundsSection"),
				Span(css.Class("goal-count-inline"), Attr("data-testid", "goals-funds-count"), fmt.Sprintf(" · %d", len(healthyFund))),
			),
			Body: Div(css.Class("goal-list"), fundRows),
		})
	}

	// Active goals list (healthy actives), or the first-run empty state. When every
	// active goal has been pulled up into "Needs a plan", the main list section is
	// omitted rather than showing an empty "Goals" heading.
	var listSection ui.Node = Fragment()
	if len(v.Active) == 0 && len(v.Fund) == 0 && len(v.Missed) == 0 {
		pr := uistate.UsePrefs().Get()
		in := buildSmartInput(app, pr.WeekStartWeekday())
		listSection = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("nav.goals"),
			Body: Fragment(
				ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("goals.empty"), CTALabel: uistate.T("goals.addFirst"), AddTarget: "goal", Icon: icon.Goals}),
				smartEmptyStateFor(smartSettings, smart.PageGoals, in),
			),
		})
	} else if len(healthyActive) > 0 {
		// Each active goal card now renders its savings-pace rail INSIDE its own metadata
		// block (see goals_row.go), so the list is just the cards — no trajectory wrapper.
		rows := MapKeyed(healthyActive, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return rowFor(g, 0, "")
		})
		listSection = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("nav.goals"),
			Body:  Div(css.Class("goal-list"), rows),
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
					Span(css.Class("goal-count-inline"), uistate.T("goals.achievedCount", len(v.Achieved))),
				),
			),
			Body: If(achievedOpen.Get(), Div(Attr("id", "goals-achieved-list"), achievedRows)),
		})
	}

	var body ui.Node = Div(
		goalsWaterfallCard(),
		// #65: shared-claim conflicts, the next-paycheck preview, and the payday
		// funding-order control (each self-gates to nothing when not applicable).
		ui.CreateElement(goalConflictStrip, struct{}{}),
		ui.CreateElement(goalsPaycheckPreviewCard, struct{}{}),
		ui.CreateElement(goalsFundingOrderCard, struct{}{}),
		// "Needs a plan" leads — missed deadlines plus the Watch / At-risk goals pulled
		// up from the active and fund lists, most-severe first — so the goals needing a
		// decision (re-date, re-fund, or archive) never hide below the healthy list.
		needsPlanSection,
		fundsSection,
		listSection,
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

// fundScopeNote is the one-line scope statement under the goals headline: which
// cards the totals count, and the sinking funds' own target when any exist.
func fundScopeNote(app *appstate.App, v goalView) ui.Node {
	if len(v.Fund) == 0 {
		return Fragment()
	}
	rates := currency.Rates{Base: v.Base, Rates: app.Settings().FXRates}
	_, fundTarget := goalsvc.Totals(v.Fund, rates, v.Base, false)
	return P(css.Class("budget-sub"), Attr("data-testid", "goal-headline-scope"),
		Style(map[string]string{"margin": "0.35rem 0 0"}),
		uistate.T("goals.headlineScope", fmtMoney(fundTarget)))
}
