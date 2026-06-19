//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/uistate"
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
	item := func(labelKey string, onSelect func()) uic.Node {
		return Button(Class("add-item"), Type("button"), Attr("role", "menuitem"),
			OnClick(func() {
				closeMenu()
				onSelect()
			}),
			uistate.T(labelKey),
		)
	}
	return Div(Class("add-wrap"),
		Button(Class("px-3 py-1.5 rounded-[4px] border border-line text-fg hover:bg-hover"),
			Attr("title", uistate.T("topbar.add")),
			Attr("aria-haspopup", "menu"),
			OnClick(func() { open.Set(!open.Get()) }),
			uistate.T("topbar.addLabel"),
		),
		Div(Class("add-backdrop"+hidden), OnClick(closeMenu)),
		Div(Class("add-menu"+hidden), Attr("role", "menu"),
			item("addmenu.transaction", func() { quickAdd.Set(true) }),
			item("addmenu.account", func() { nav.Navigate(uistate.RoutePath("/accounts")) }),
			item("addmenu.budget", func() { nav.Navigate(uistate.RoutePath("/budgets")) }),
			item("addmenu.goal", func() { nav.Navigate(uistate.RoutePath("/goals")) }),
			item("addmenu.document", func() { nav.Navigate(uistate.RoutePath("/documents")) }),
		),
	)
}
