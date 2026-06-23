//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// AddMenu is the top-bar "+ Add" control: a button that opens a small popover of
// add actions so data entry isn't trapped on each entity's screen (C23). "New
// transaction" opens the inline quick-add panel; the other entities route to
// their screen, where the add form lives. The popover and its click-catching
// backdrop are always rendered and shown/hidden with a CSS class, so the On*
// hooks stay at stable positions (the framework's hooks-in-loops rule).
func AddMenu() uic.Node {
	open := uic.UseState(false)
	quickAdd := uistate.UseQuickAdd()
	nav := router.UseNavigate()
	closeMenu := func() { open.Set(false) }

	hidden := ""
	if !open.Get() {
		hidden = " hidden-menu"
	}
	// item builds one menu row. Called a fixed number of times at stable
	// positions (not a variable-length loop), so the OnClick hooks are stable.
	item := func(labelKey string, ic icon.Name, onSelect func()) uic.Node {
		return Button(css.Class("add-item", tw.Flex, tw.ItemsCenter, tw.Gap25), Type("button"), Attr("role", "menuitem"),
			OnClick(func() {
				closeMenu()
				onSelect()
			}),
			ui.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T(labelKey)),
		)
	}
	// aria-expanded reflects the popover state for assistive tech (this is a menu
	// button, now icon-only — so it needs an explicit accessible name + hover title).
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}
	return Div(css.Class("add-wrap"),
		Button(css.Class("add-btn"),
			Attr("title", uistate.T("topbar.add")),
			Attr("aria-label", uistate.T("topbar.add")),
			Attr("aria-haspopup", "menu"),
			Attr("aria-expanded", expanded),
			OnClick(func() { open.Set(!open.Get()) }),
			ui.Icon(icon.Plus, css.Class(tw.W18px, tw.H18px)),
		),
		Div(ClassStr("add-backdrop"+hidden), OnClick(closeMenu)),
		Div(ClassStr("add-menu"+hidden), Attr("role", "menu"),
			item("addmenu.transaction", icon.Transactions, func() { quickAdd.Set(true) }),
			item("addmenu.account", icon.Accounts, func() { uistate.SetAddTarget("account") }),
			item("addmenu.budget", icon.Budgets, func() { uistate.SetAddTarget("budget") }),
			item("addmenu.goal", icon.Goals, func() { uistate.SetAddTarget("goal") }),
			item("addmenu.task", icon.Todo, func() { uistate.SetAddTarget("task") }),
			item("addmenu.category", icon.Tag, func() { uistate.SetAddTarget("category") }),
			item("addmenu.member", icon.Users, func() { uistate.SetAddTarget("member") }),
			item("addmenu.rule", icon.Filter, func() { uistate.SetAddTarget("rule") }),
			item("addmenu.document", icon.ScanLine, func() { nav.Navigate(uistate.RoutePath("/documents")) }),
		),
	)
}
