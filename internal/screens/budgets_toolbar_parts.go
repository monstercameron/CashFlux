// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type budgetToolbarViewProps struct {
	App      *appstate.App
	Method   budgeting.Methodology
	Sort     string
	Impact   budgeting.MethodImpact
	OnMethod ui.Handler
	OnSort   ui.Handler
}

func budgetToolbarViewSection(props budgetToolbarViewProps) ui.Node {
	return Div(css.Class("bud-set-sec"),
		Span(css.Class("bud-set-head"), uistate.T("bud2.settings")),
		Label(css.Class("bud-set-field"),
			Span(css.Class("bud-set-lbl"), uistate.T("budgets.methodGlobalLabel")),
			Select(css.Class("fctrl-select"), Attr("data-testid", "budgets-method"),
				Attr("aria-label", uistate.T("budgets.methodGlobalLabel")), Title(uistate.T("budgets.methodGlobalTitle")), OnChange(props.OnMethod),
				Option(Value(string(budgeting.MethodSimple)), SelectedIf(props.Method == budgeting.MethodSimple), uistate.T("settings.budgetMethodSimple")),
				Option(Value(string(budgeting.MethodZeroBased)), SelectedIf(props.Method == budgeting.MethodZeroBased), uistate.T("settings.budgetMethodZero")),
				Option(Value(string(budgeting.MethodEnvelope)), SelectedIf(props.Method == budgeting.MethodEnvelope), uistate.T("settings.budgetMethodEnvelope")),
				Option(Value(string(budgeting.MethodFlex)), SelectedIf(props.Method == budgeting.MethodFlex), uistate.T("settings.budgetMethodFlex")),
			),
		),
		P(css.Class("bud-set-hint"), Attr("data-testid", "budgets-method-scope"), methodScopeText(props.Impact)),
		Label(css.Class("bud-set-field"),
			Span(css.Class("bud-set-lbl"), uistate.T("budgets.sortLabel")),
			Select(css.Class("fctrl-select"), Attr("data-testid", "budgets-sort"),
				Attr("aria-label", uistate.T("budgets.sortLabel")), Title(uistate.T("budgets.sortLabel")), OnChange(props.OnSort),
				Option(Value(uistate.BudgetSortHealth), SelectedIf(props.Sort == uistate.BudgetSortHealth), uistate.T("budgets.sortRiskBR2TB")),
				Option(Value(uistate.BudgetSortOverage), SelectedIf(props.Sort == uistate.BudgetSortOverage), uistate.T("budgets.sortOverage")),
				Option(Value(uistate.BudgetSortNearOverage), SelectedIf(props.Sort == uistate.BudgetSortNearOverage), uistate.T("budgets.sortNear")),
				Option(Value(uistate.BudgetSortUnderutilized), SelectedIf(props.Sort == uistate.BudgetSortUnderutilized), uistate.T("budgets.sortUnderused")),
				Option(Value(uistate.BudgetSortAmount), SelectedIf(props.Sort == uistate.BudgetSortAmount), uistate.T("budgets.sortAmount")),
				Option(Value(uistate.BudgetSortName), SelectedIf(props.Sort == uistate.BudgetSortName), uistate.T("budgets.sortName")),
			),
		),
	)
}

type budgetToolbarSettingsProps struct {
	App      *appstate.App
	Method   budgeting.Methodology
	Sort     string
	Impact   budgeting.MethodImpact
	OnMethod ui.Handler
	OnSort   ui.Handler
}

func budgetToolbarSettings(props budgetToolbarSettingsProps) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	close := ui.UseEvent(Prevent(func() { open.Set(false) }))
	uiw.DismissPopover(open.Get(), "budgets-settings-wrap", func() { open.Set(false) })
	uiw.AnchorPopover(open.Get(), "budgets-settings-wrap")
	view := ui.CreateElement(budgetToolbarViewSection, budgetToolbarViewProps{
		App: props.App, Method: props.Method, Sort: props.Sort, Impact: props.Impact,
		OnMethod: props.OnMethod, OnSort: props.OnSort,
	})
	return Div(ClassStr("add-wrap"), Attr("id", "budgets-settings-wrap"),
		Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "budgets-settings"), Attr("aria-haspopup", "menu"),
			Attr("aria-expanded", ariaBool(open.Get())), Attr("aria-label", uistate.T("bud2.settingsAccessibleBR2TB")),
			Title(uistate.T("bud2.settingsAccessibleBR2TB")), OnClick(toggle),
			uiw.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("bud2.settings")),
			uiw.Icon(icon.ChevronDown, css.Class(tw.ShrinkO, tw.W35, tw.H35))),
		Div(ClassStr("add-backdrop"+hiddenIf(!open.Get())), OnClick(close)),
		Div(ClassStr("add-menu bud-set-menu"+hiddenIf(!open.Get())), Attr("role", "menu"), Attr("data-testid", "budgets-settings-menu"),
			Attr("style", "max-height: min(70vh, 42rem); overflow-y: auto; overscroll-behavior: contain"), view),
	)
}

type budgetToolbarToolsProps struct{ HasBudgets bool }

func budgetToolbarTools(props budgetToolbarToolsProps) ui.Node {
	auto := uistate.UseBudgetAutoOpen()
	openAuto := ui.UseEvent(Prevent(func() { auto.Set(true) }))
	monthClose := uistate.UseMonthCloseOpen()
	openMonthClose := ui.UseEvent(Prevent(func() { monthClose.Set(true) }))
	sweep := uistate.UseSweepConfigOpen()
	openSweep := ui.UseEvent(Prevent(func() { sweep.Set(true) }))
	adjust := uistate.UseBudgetAdjustOpen()
	openAdjust := ui.UseEvent(Prevent(func() { adjust.Set(true) }))
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	close := ui.UseEvent(Prevent(func() { open.Set(false) }))
	uiw.DismissPopover(open.Get(), "budgets-tools-wrap", func() { open.Set(false) })
	uiw.AnchorPopover(open.Get(), "budgets-tools-wrap")
	menu := ui.CreateElement(budgetToolbarToolsMenu, budgetToolbarToolsMenuProps{
		HasBudgets: props.HasBudgets, OnClose: close,
		OnAuto: openAuto, OnMonthClose: openMonthClose, OnSweep: openSweep, OnAdjust: openAdjust,
	})
	return Div(ClassStr("add-wrap"), Attr("id", "budgets-tools-wrap"),
		Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "budgets-tools"), Attr("aria-haspopup", "menu"),
			Attr("aria-expanded", ariaBool(open.Get())), OnClick(toggle), Span(uistate.T("bud2.tools")),
			uiw.Icon(icon.ChevronDown, css.Class(tw.ShrinkO, tw.W35, tw.H35))),
		Div(ClassStr("add-backdrop"+hiddenIf(!open.Get())), OnClick(close)),
		Div(ClassStr("add-menu bud2-tools-menu"+hiddenIf(!open.Get())), Attr("role", "menu"), Attr("data-testid", "budgets-tools-menu"),
			Attr("style", "max-height: min(70vh, 42rem); overflow-y: auto; overscroll-behavior: contain"), menu),
	)
}

type budgetToolbarToolsMenuProps struct {
	HasBudgets   bool
	OnClose      ui.Handler
	OnAuto       ui.Handler
	OnMonthClose ui.Handler
	OnSweep      ui.Handler
	OnAdjust     ui.Handler
}

func budgetToolbarToolsMenu(props budgetToolbarToolsMenuProps) ui.Node {
	return Div(css.Class("bud-set-sec"), OnClick(props.OnClose),
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budgets-autobudget"),
			Title(uistate.T("budgets.autoTitleAction")), OnClick(props.OnAuto), uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("bud2.autoBudget"))),
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budgets-month-close"),
			Title(uistate.T("monthclose.offerTitle")), OnClick(props.OnMonthClose), Span(uistate.T("bud2.monthClose"))),
		Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budgets-sweep-config"),
			Title(uistate.T("sweep.openConfig")), OnClick(props.OnSweep), uiw.Icon(icon.TrendingUp, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("bud2.sweep"))),
		If(props.HasBudgets, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budgets-adjust-all"),
			Title(uistate.T("budgets.adjustAllTitle")), OnClick(props.OnAdjust), uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("bud2.adjustAllBR2TB")))),
	)
}

type budgetToolbarDensityProps struct{ SeedCompact bool }

func budgetToolbarDensity(props budgetToolbarDensityProps) ui.Node {
	density := uistate.UseBudgetDensitySeeded(props.SeedCompact)
	value := density.Get()
	toggle := ui.UseEvent(Prevent(func() {
		next := uistate.BudgetDensityCompact
		if density.Get() == uistate.BudgetDensityCompact {
			next = uistate.BudgetDensityComfortable
		}
		density.Set(next)
		uistate.PersistBudgetDensity(next)
	}))
	return Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-density"), Attr("data-density", value), Title(uistate.T(budgetDensityTitleKey(value))), OnClick(toggle),
		uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T(budgetDensityLabelKey(value))),
		Span(css.Class("sr-only"), Attr("data-testid", "budgets-density-state"), " — "+uistate.T(budgetDensityStateKey(value))))
}

func budgetToolbarAdd(_ struct{}) ui.Node {
	add := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("budget") }))
	return Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-add"), Title(uistate.T("budgets.add")), OnClick(add),
		uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.addBudget")))
}
