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
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

type goalSummaryProps struct{ App *appstate.App }
type goalToolbarProps struct{ App *appstate.App }
type goalListProps struct{ App *appstate.App }
type goalFormulaProps struct{ App *appstate.App }

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

// goalToolbarWidget is the actions row: the smart-insights action, a "Goal metrics"
// FormulaBuilder reveal toggle (parity with the budgets toolbar), and the primary
// "Add goal" button.
func goalToolbarWidget(props goalToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	smartSettings := uistate.LoadSmartSettings()
	formulasAtom := uistate.UseGoalsShowFormulas()
	addGoal := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("goal") }))
	onToggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))
	formulasLabel := uistate.T("goals.metricsShow")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("goals.metricsHide")
	}

	toolbar := Div(css.Class("budgets-toolbar"),
		Div(css.Class("budgets-toolbar-actions"),
			smartSectionAction(smartSettings),
			Button(css.Class("btn"), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "goals-toggle-formulas"), Title(uistate.T("goals.metricsTitle")),
				OnClick(onToggleFormulas), Text(formulasLabel)),
			Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "goals-add"), Title(uistate.T("goals.add")), OnClick(addGoal),
				uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
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

	v := computeGoalView(app, activeMemberID)

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
	rowFor := func(g domain.Goal, fundSetAside int64, catName string) ui.Node {
		return ui.CreateElement(GoalRow, goalRowProps{
			Goal: g, Accounts: v.Accounts, Members: v.Members,
			OnDelete:       deleteGoal,
			OnDrillAccount: viewAccountTxns, OnArchive: archiveGoal, OnRedirect: redirectToAllocate,
			FundSetAside: fundSetAside, LinkedCategoryName: catName,
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
	if len(v.Active) == 0 && len(v.Fund) == 0 {
		pr := uistate.UsePrefs().Get()
		in := buildSmartInput(app, pr.WeekStartWeekday())
		listBody = Fragment(
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("goals.empty"), CTALabel: uistate.T("goals.addFirst"), AddTarget: "goal", Icon: icon.Goals}),
			smartEmptyStateFor(smartSettings, smart.PageGoals, in),
		)
	} else {
		rows := MapKeyed(v.Active, func(g domain.Goal) any { return g.ID }, func(g domain.Goal) ui.Node {
			return rowFor(g, 0, "")
		})
		listBody = Div(css.Class("goal-list"), rows)
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

	body := Div(
		fundsSection,
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title:        uistate.T("nav.goals"),
			HeaderAction: smartSectionAction(smartSettings),
			Body:         listBody,
		}),
		If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
		achievedSection,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goal-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- goal-formula ----------------------------------------------------------------

// goalFormulaWidget is the opt-in "Goal metrics" tile (revealed by the toolbar toggle):
// the reusable FormulaBuilder over the live engine surface, so goal custom fields
// (cf_goal_<key>) and each goal's variables (goal_<slug>_*) can be computed over.
func goalFormulaWidget(props goalFormulaProps) ui.Node {
	body := Div(
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("goals.metricsTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goal-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
