// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/period"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type budgetListRowsProps struct {
	App         *appstate.App
	View        budgetView
	Statuses    []budgeting.Status
	Members     []domain.Member
	BudgetDefs  []customfields.Def
	Compact     bool
	OnDrill     func(budgeting.Status, budgeting.Scope, string, string)
	OnViewTodos func()
}

func budgetListRows(props budgetListRowsProps) ui.Node {
	cbs := buildBudgetRowCallbacks(props.App, props.View.Base, props.View.CatName)
	todoCounts := budgetTodoCounts(props.App.Tasks())
	return Fragment(MapKeyed(props.Statuses,
		func(s budgeting.Status) any { return s.Budget.ID },
		func(s budgeting.Status) ui.Node {
			return ui.CreateElement(BudgetRow, budgetRowPropsForList(props, s, cbs, todoCounts[s.Budget.ID]))
		},
	))
}

func budgetTodoCounts(tasks []domain.Task) map[string]int {
	counts := make(map[string]int)
	for _, task := range tasks {
		if task.RelatedType == domain.RelatedBudget && task.RelatedID != "" {
			counts[task.RelatedID]++
		}
	}
	return counts
}

func budgetRowPropsForList(props budgetListRowsProps, status budgeting.Status, cbs budgetRowCallbacks, todoCount int) budgetRowProps {
	b := status.Budget
	tracked := make([]string, 0, len(b.CategoryIDs))
	for _, id := range b.CategoryIDs {
		if name := props.View.CatName[id]; name != "" {
			tracked = append(tracked, name)
		}
	}
	id := b.ID
	last := lastMonthOverlay(props.View, id)
	_, hasCommitted := props.View.Committed[id]
	return budgetRowProps{
		Status: status, Category: props.View.CatName[b.CategoryID], TrackedCats: strings.Join(tracked, ", "), Members: props.Members, BudgetDefs: props.BudgetDefs,
		Envelope: props.View.EnvAvail[id], EnvelopeNeg: props.View.EnvNeg[id], EnvelopeDebtStart: props.View.EnvDebtStart[id], PaceOver: props.View.PaceOver[id],
		PaceMarkerPct: props.View.PaceMark[id].MarkerPct, PaceCaption: props.View.PaceMark[id].Caption, PaceHot: props.View.PaceMark[id].Hot,
		RolloverCarry: props.View.RollCarry[id], RolloverNeg: props.View.RollNeg[id], EffectiveCap: props.View.RollEffCap[id], EffectiveCapMath: props.View.RollEffCapMath[id],
		ProratedRest: props.View.ProratedRest[id], EffectiveMethod: props.View.EffMethod[id], Covered: props.View.Covered[id],
		LastMonthSpent: last.Spent, LastMonthDelta: last.Delta, LastMonthOver: last.Over, LastMonthPct: last.Pct, LastMonthFill: last.Fill,
		PeriodEarly: props.View.EarlyPeriod[id], PriorSpent: props.View.LastMonth[id].Spent, PriorDelta: props.View.LastMonth[id].Delta, PriorOver: props.View.LastMonth[id].Over,
		OnDelete: cbs.OnDelete, OnRemoveRecurring: cbs.OnRemoveRecurring,
		OnDrill:    func(scope budgeting.Scope, from, to string) { props.OnDrill(status, scope, from, to) },
		PeriodFrom: props.View.PeriodFrom[id], PeriodTo: props.View.PeriodTo[id], LinkedTodos: todoCount, OnViewTodos: props.OnViewTodos,
		Anchor: props.View.Anchor, Committed: props.View.Committed[id], HasCommitted: hasCommitted, Compact: props.Compact,
	}
}

type budgetListFilledProps struct {
	App              *appstate.App
	View             budgetView
	Period           period.Window
	ActiveMemberID   string
	Compact          bool
	Search           string
	OnSearch         func(string)
	Attention        string
	OnClearAttention func()
	OnDrill          func(budgeting.Status, budgeting.Scope, string, string)
	OnViewTodos      func()
}

func budgetListFilled(props budgetListFilledProps) ui.Node {
	visible := filterBudgetStatuses(props.View.Statuses, props.View.CatName, props.Search, props.Attention)
	searchBar := ui.CreateElement(budgetListSearchField, budgetListSearchFieldProps{
		Count: len(props.View.Statuses), Value: props.Search, OnSearch: props.OnSearch,
	})
	var attentionChip ui.Node = Fragment()
	if props.Attention != "" {
		attentionChip = ui.CreateElement(budgetAttentionChip, budgetAttentionChipProps{OnClear: props.OnClearAttention})
	}
	var matchNote ui.Node = Fragment()
	if strings.TrimSpace(props.Search) != "" && len(visible) == 0 {
		matchNote = Div(css.Class("budget-search-empty"), Attr("data-testid", "budgets-search-empty"),
			Attr("role", "status"), uistate.T("budgets.searchNoMatch", strings.TrimSpace(props.Search)))
	}
	rows := ui.CreateElement(budgetListRows, budgetListRowsProps{
		App: props.App, View: props.View, Statuses: visible, Members: props.App.Members(),
		BudgetDefs: props.App.CustomFieldDefsFor("budget"), Compact: props.Compact,
		OnDrill: props.OnDrill, OnViewTodos: props.OnViewTodos,
	})
	start, end := props.Period.Range()
	unbudgeted := ui.CreateElement(unbudgetedStrip, unbudgetedStripProps{
		App: props.App, Base: props.View.Base, CatName: props.View.CatName,
		From: start, To: end, Label: props.Period.Label(), Historical: !time.Now().Before(end),
	})
	listClass := "budget-grid"
	if props.Compact {
		listClass = "budget-clist"
	}
	head := Div(css.Class("budlist-head"), searchBar,
		budgetMemberScopeNote(props.App, props.ActiveMemberID, len(props.View.Statuses)),
		ui.CreateElement(budgetToolbarContent, budgetToolbarProps{App: props.App}))
	return Fragment(head, budgetIncompleteNotice(props.View.Incomplete), attentionChip, matchNote,
		Div(ClassStr(listClass), Attr("data-testid", "budgets-list"), rows), unbudgeted)
}

type budgetListSearchFieldProps struct {
	Count    int
	Value    string
	OnSearch func(string)
}

func budgetListSearchField(props budgetListSearchFieldProps) ui.Node {
	onInput := ui.UseEvent(func(value string) { props.OnSearch(value) })
	onInputProp := OnInput(onInput)
	if !budgetListSearchVisible(props.Count) {
		return Fragment()
	}
	return Div(css.Class("budget-search"),
		uiw.Icon(icon.Search, css.Class("budget-search-icon", tw.ShrinkO, tw.W4, tw.H4)),
		Input(css.Class("field budget-search-input"), Type("text"), Attr("data-testid", "budgets-search"),
			Attr("aria-label", uistate.T("budgets.searchLabel")), Placeholder(uistate.T("budgets.searchPlaceholder")),
			onInputProp, uiw.FieldValue(props.Value)))
}

type budgetAttentionChipProps struct{ OnClear func() }

func budgetAttentionChip(props budgetAttentionChipProps) ui.Node {
	clear := ui.UseEvent(Prevent(props.OnClear))
	clearOnClick := OnClick(clear)
	return Div(css.Class("budget-search-empty", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Attr("data-testid", "budgets-attention-chip"), Attr("role", "status"),
		Span(uistate.T("bud2.attentionFilter")),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "budgets-attention-clear"),
			Title(uistate.T("budgets.attentionClear")), clearOnClick, uistate.T("budgets.attentionClear")))
}
